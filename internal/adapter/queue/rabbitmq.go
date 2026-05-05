package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Channel shape:
//   - One cached publisher channel guarded by a mutex (used by Publish,
//     PublishJSON, DeclareQueue, DeclareExchange, BindQueue). Cached because
//     declares run at startup and publishes are hot-path; opening a channel
//     per call would add a network round-trip per message.
//   - One channel per Consume call, opened inside Consume and dedicated to a
//     single goroutine. AMQP channels are not goroutine-safe, so the consumer
//     must never share with the publisher.
//   - On NotifyClose for either the connection or a consumer channel, the
//     consumer loop attempts a bounded reconnect (capped backoff, max attempts)
//     while honoring the parent context.

// Default values for tunables not present in config.
const (
	defaultPrefetchCount      = 10
	defaultMaxReconnectTries  = 5
	defaultReconnectBaseDelay = time.Second
	defaultReconnectMaxDelay  = 30 * time.Second
)

// amqpConnection is the minimal subset of *amqp.Connection used by this
// adapter. Defined as an interface so tests can inject a fake transport.
type amqpConnection interface {
	Channel() (amqpChannel, error)
	NotifyClose(c chan *amqp.Error) chan *amqp.Error
	Close() error
	IsClosed() bool
}

// amqpChannel is the minimal subset of *amqp.Channel used by this adapter.
type amqpChannel interface {
	PublishWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
	Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error)
	QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error)
	ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error
	QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error
	Qos(prefetchCount, prefetchSize int, global bool) error
	NotifyClose(c chan *amqp.Error) chan *amqp.Error
	Close() error
}

// dialer opens a connection. Swappable in tests.
type dialer func(url string) (amqpConnection, error)

// realConnAdapter wraps *amqp.Connection so it satisfies amqpConnection.
type realConnAdapter struct {
	*amqp.Connection
}

func (r *realConnAdapter) Channel() (amqpChannel, error) {
	ch, err := r.Connection.Channel()
	if err != nil {
		return nil, err
	}
	return ch, nil
}

func defaultDialer(url string) (amqpConnection, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	return &realConnAdapter{Connection: conn}, nil
}

// Options tunes the RabbitMQ adapter. Zero values fall back to defaults.
type Options struct {
	PrefetchCount      int
	MaxReconnectTries  int
	ReconnectBaseDelay time.Duration
	ReconnectMaxDelay  time.Duration
}

func (o Options) withDefaults() Options {
	if o.PrefetchCount <= 0 {
		o.PrefetchCount = defaultPrefetchCount
	}
	if o.MaxReconnectTries <= 0 {
		o.MaxReconnectTries = defaultMaxReconnectTries
	}
	if o.ReconnectBaseDelay <= 0 {
		o.ReconnectBaseDelay = defaultReconnectBaseDelay
	}
	if o.ReconnectMaxDelay <= 0 {
		o.ReconnectMaxDelay = defaultReconnectMaxDelay
	}
	return o
}

// RabbitMQ implements port.Queue using RabbitMQ.
type RabbitMQ struct {
	url    string
	dial   dialer
	opts   Options
	conn   amqpConnection
	pubMu  sync.Mutex
	pubCh  amqpChannel
	closed bool
}

// NewRabbitMQ creates a new RabbitMQ connection with default options.
func NewRabbitMQ(url string) (*RabbitMQ, error) {
	return NewRabbitMQWithOptions(url, Options{})
}

// NewRabbitMQWithOptions creates a new RabbitMQ connection with explicit
// tunables (prefetch, reconnect bounds).
func NewRabbitMQWithOptions(url string, opts Options) (*RabbitMQ, error) {
	return newRabbitMQ(url, opts, defaultDialer)
}

func newRabbitMQ(url string, opts Options, dial dialer) (*RabbitMQ, error) {
	conn, err := dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to rabbitmq: %w", err)
	}
	pubCh, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to open publisher channel: %w", err)
	}
	return &RabbitMQ{
		url:   url,
		dial:  dial,
		opts:  opts.withDefaults(),
		conn:  conn,
		pubCh: pubCh,
	}, nil
}

// withPubChannel runs fn with the cached publisher channel under a mutex.
// AMQP channels are not goroutine-safe; the mutex serializes all publish/
// declare traffic on the shared channel.
func (q *RabbitMQ) withPubChannel(fn func(ch amqpChannel) error) error {
	q.pubMu.Lock()
	defer q.pubMu.Unlock()
	if q.closed {
		return errors.New("rabbitmq: adapter closed")
	}
	return fn(q.pubCh)
}

func (q *RabbitMQ) Publish(ctx context.Context, exchange, routingKey string, body []byte) error {
	return q.withPubChannel(func(ch amqpChannel) error {
		return ch.PublishWithContext(
			ctx,
			exchange,
			routingKey,
			false, // mandatory
			false, // immediate
			amqp.Publishing{
				ContentType:  "application/octet-stream",
				Body:         body,
				DeliveryMode: amqp.Persistent,
				Timestamp:    time.Now(),
			},
		)
	})
}

func (q *RabbitMQ) PublishJSON(ctx context.Context, exchange, routingKey string, message any) error {
	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	return q.withPubChannel(func(ch amqpChannel) error {
		return ch.PublishWithContext(
			ctx,
			exchange,
			routingKey,
			false,
			false,
			amqp.Publishing{
				ContentType:  "application/json",
				Body:         body,
				DeliveryMode: amqp.Persistent,
				Timestamp:    time.Now(),
			},
		)
	})
}

// Consume opens a dedicated channel for the consumer and enters a loop that
// re-establishes the channel on NotifyClose, bounded by Options. The loop
// exits cleanly on ctx cancellation. The first channel-open + Qos + Consume
// is performed synchronously so the caller learns immediately about a bad
// queue name or AMQP-level rejection.
func (q *RabbitMQ) Consume(ctx context.Context, queueName string, handler func(body []byte) error) error {
	ch, deliveries, closeCh, err := q.openConsumer(queueName)
	if err != nil {
		return err
	}

	go q.runConsumer(ctx, queueName, handler, ch, deliveries, closeCh)
	return nil
}

func (q *RabbitMQ) openConsumer(queueName string) (ch amqpChannel, deliveries <-chan amqp.Delivery, closeCh chan *amqp.Error, err error) {
	if q.conn == nil || q.conn.IsClosed() {
		return nil, nil, nil, errors.New("rabbitmq: connection closed")
	}
	ch, err = q.conn.Channel()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to open consumer channel: %w", err)
	}
	if qosErr := ch.Qos(q.opts.PrefetchCount, 0, false); qosErr != nil {
		_ = ch.Close()
		return nil, nil, nil, fmt.Errorf("failed to set qos: %w", qosErr)
	}
	deliveries, err = ch.Consume(
		queueName,
		"",    // consumer tag
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		_ = ch.Close()
		return nil, nil, nil, fmt.Errorf("failed to register consumer: %w", err)
	}
	closeCh = ch.NotifyClose(make(chan *amqp.Error, 1))
	return ch, deliveries, closeCh, nil
}

func (q *RabbitMQ) runConsumer(
	ctx context.Context,
	queueName string,
	handler func(body []byte) error,
	ch amqpChannel,
	deliveries <-chan amqp.Delivery,
	closeCh chan *amqp.Error,
) {
	for {
		stopped := q.drainDeliveries(ctx, handler, deliveries, closeCh)
		// Always close current channel before deciding next step.
		_ = ch.Close()
		if stopped {
			return
		}
		// Channel closed unexpectedly; try to reconnect bounded by options.
		next, nextDeliv, nextClose, err := q.reconnectConsumer(ctx, queueName)
		if err != nil {
			slog.Error("rabbitmq consumer reconnect failed; halting", "queue", queueName, "error", err)
			return
		}
		ch, deliveries, closeCh = next, nextDeliv, nextClose
		slog.Info("rabbitmq consumer reconnected", "queue", queueName)
	}
}

// drainDeliveries returns true when the loop should terminate (ctx done),
// false when the channel closed and the caller should attempt reconnect.
func (q *RabbitMQ) drainDeliveries(
	ctx context.Context,
	handler func(body []byte) error,
	deliveries <-chan amqp.Delivery,
	closeCh chan *amqp.Error,
) bool {
	for {
		select {
		case <-ctx.Done():
			return true
		case err, ok := <-closeCh:
			if ok && err != nil {
				slog.Warn("rabbitmq channel closed", "code", err.Code, "reason", err.Reason)
			} else {
				slog.Warn("rabbitmq channel closed")
			}
			return false
		case msg, ok := <-deliveries:
			if !ok {
				// deliveries closed without a NotifyClose payload; treat as
				// disconnect and let the caller decide whether to reconnect.
				return false
			}
			if err := handler(msg.Body); err != nil {
				_ = msg.Nack(false, true)
			} else {
				_ = msg.Ack(false)
			}
		}
	}
}

// reconnectConsumer attempts to re-open the consumer channel (and connection
// if necessary) with bounded backoff. Returns an error when retries are
// exhausted or the parent context is done.
func (q *RabbitMQ) reconnectConsumer(ctx context.Context, queueName string) (ch amqpChannel, deliveries <-chan amqp.Delivery, closeCh chan *amqp.Error, err error) {
	delay := q.opts.ReconnectBaseDelay
	for attempt := 1; attempt <= q.opts.MaxReconnectTries; attempt++ {
		select {
		case <-ctx.Done():
			return nil, nil, nil, ctx.Err()
		case <-time.After(delay):
		}

		// If the underlying connection died, redial first.
		if q.conn == nil || q.conn.IsClosed() {
			if err := q.redial(); err != nil {
				slog.Warn("rabbitmq redial failed", "attempt", attempt, "error", err)
				delay = nextBackoff(delay, q.opts.ReconnectMaxDelay)
				continue
			}
		}

		ch, deliveries, closeCh, err := q.openConsumer(queueName)
		if err == nil {
			return ch, deliveries, closeCh, nil
		}
		slog.Warn("rabbitmq reopen consumer failed", "attempt", attempt, "error", err)
		delay = nextBackoff(delay, q.opts.ReconnectMaxDelay)
	}
	return nil, nil, nil, fmt.Errorf("rabbitmq: reconnect exhausted after %d attempts", q.opts.MaxReconnectTries)
}

func (q *RabbitMQ) redial() error {
	conn, err := q.dial(q.url)
	if err != nil {
		return err
	}
	pubCh, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return err
	}
	q.pubMu.Lock()
	// Best-effort close of the previous publisher channel.
	if q.pubCh != nil {
		_ = q.pubCh.Close()
	}
	if q.conn != nil {
		_ = q.conn.Close()
	}
	q.conn = conn
	q.pubCh = pubCh
	q.pubMu.Unlock()
	return nil
}

func nextBackoff(current, ceiling time.Duration) time.Duration {
	next := current * 2
	if next > ceiling {
		return ceiling
	}
	return next
}

func (q *RabbitMQ) DeclareQueue(_ context.Context, name string, durable bool) error {
	return q.withPubChannel(func(ch amqpChannel) error {
		_, err := ch.QueueDeclare(name, durable, false, false, false, nil)
		if err != nil {
			return fmt.Errorf("failed to declare queue: %w", err)
		}
		return nil
	})
}

func (q *RabbitMQ) DeclareExchange(_ context.Context, name, kind string, durable bool) error {
	return q.withPubChannel(func(ch amqpChannel) error {
		if err := ch.ExchangeDeclare(name, kind, durable, false, false, false, nil); err != nil {
			return fmt.Errorf("failed to declare exchange: %w", err)
		}
		return nil
	})
}

func (q *RabbitMQ) BindQueue(_ context.Context, queueName, exchange, routingKey string) error {
	return q.withPubChannel(func(ch amqpChannel) error {
		if err := ch.QueueBind(queueName, routingKey, exchange, false, nil); err != nil {
			return fmt.Errorf("failed to bind queue: %w", err)
		}
		return nil
	})
}

func (q *RabbitMQ) Close() error {
	q.pubMu.Lock()
	defer q.pubMu.Unlock()
	if q.closed {
		return nil
	}
	q.closed = true
	var firstErr error
	if q.pubCh != nil {
		if err := q.pubCh.Close(); err != nil {
			firstErr = err
		}
	}
	if q.conn != nil {
		if err := q.conn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
