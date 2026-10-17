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
//   - A single watcher goroutine listens for connection-level NotifyClose and
//     marks the cached publisher channel dead. The next publish then redials
//     once (bounded by the reconnect knobs below) and retries the publish
//     once; if the retry also fails the original error is returned unchanged.
//     Consumer channels reconnect independently in runConsumer with the same
//     bounds. No new config keys: the knobs stay code constants.

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
	QueueDeclarePassive(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error)
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
	// pubDead marks the cached publisher channel as known-dead (connection
	// NotifyClose fired). The next publish redials instead of failing on the
	// stale channel. Guarded by pubMu.
	pubDead bool
	// connCloseCh carries connection-level close notifications for the
	// current connection. Replaced on every redial; the watcher goroutine
	// re-reads it under pubMu. Guarded by pubMu.
	connCloseCh chan *amqp.Error
	// wakeup nudges the watcher goroutine to re-read connCloseCh after a
	// redial. Buffered so redial never blocks. watchStop terminates the
	// watcher; closed once in Close.
	wakeup    chan struct{}
	watchStop chan struct{}
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
	q := &RabbitMQ{
		url:         url,
		dial:        dial,
		opts:        opts.withDefaults(),
		conn:        conn,
		pubCh:       pubCh,
		connCloseCh: conn.NotifyClose(make(chan *amqp.Error, 1)),
		wakeup:      make(chan struct{}, 1),
		watchStop:   make(chan struct{}),
	}
	go q.watchConnection()
	return q, nil
}

// watchConnection marks the cached publisher channel dead when the broker
// closes the connection. Exactly one watcher runs per adapter; it exits when
// Close closes watchStop. Redial replaces connCloseCh and nudges wakeup so
// the watcher follows the current connection without leaking goroutines.
func (q *RabbitMQ) watchConnection() {
	for {
		q.pubMu.Lock()
		connCloseCh := q.connCloseCh
		q.pubMu.Unlock()
		select {
		case <-q.watchStop:
			return
		case <-q.wakeup:
			continue
		case connErr := <-connCloseCh:
			q.pubMu.Lock()
			q.pubDead = true
			q.pubMu.Unlock()
			slog.Warn("rabbitmq connection closed; publisher marked dead, next publish redials", "reason", connErr)
			select {
			case <-q.watchStop:
				return
			case <-q.wakeup:
			}
		}
	}
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

// isChannelClosedError reports whether err is a dead channel/connection as
// opposed to a genuine broker rejection (undeclared exchange, too large,
// access refused). Only the former heals with a redial.
func isChannelClosedError(err error) bool {
	return errors.Is(err, amqp.ErrClosed)
}

func (q *RabbitMQ) publishOnce(ctx context.Context, exchange, routingKey string, body []byte, contentType string) error {
	return q.withPubChannel(func(ch amqpChannel) error {
		return ch.PublishWithContext(
			ctx,
			exchange,
			routingKey,
			false, // mandatory
			false, // immediate
			amqp.Publishing{
				ContentType:  contentType,
				Body:         body,
				DeliveryMode: amqp.Persistent,
				Timestamp:    time.Now(),
			},
		)
	})
}

func (q *RabbitMQ) Publish(ctx context.Context, exchange, routingKey string, body []byte) error {
	return q.publishWithRedial(ctx, exchange, routingKey, body, "application/octet-stream")
}

// publishWithRedial publishes once; when the attempt dies on a dead
// channel/connection it redials once and retries the publish once. A failed
// retry surfaces the ORIGINAL error unchanged so callers (and the usecase
// 500 mapping) see the failure that actually happened, not the redial noise.
func (q *RabbitMQ) publishWithRedial(ctx context.Context, exchange, routingKey string, body []byte, contentType string) error {
	err := q.publishOnce(ctx, exchange, routingKey, body, contentType)
	if err == nil || !isChannelClosedError(err) {
		return err
	}
	q.pubMu.Lock()
	defer q.pubMu.Unlock()
	if q.closed {
		return err
	}
	// The failed call proves the cached channel dead (even when the watcher
	// has not fired yet, e.g. a server-side channel exception with a live
	// connection). Mark it and heal with exactly one redial + one retry.
	q.pubDead = true
	if rerr := q.redialLocked(); rerr != nil {
		slog.Warn("rabbitmq publisher redial failed; returning original publish error", "redial_error", rerr)
		return err
	}
	if retryErr := q.publishLocked(ctx, exchange, routingKey, body, contentType); retryErr != nil {
		return err
	}
	return nil
}

// publishLocked publishes on the cached channel; caller holds pubMu.
func (q *RabbitMQ) publishLocked(ctx context.Context, exchange, routingKey string, body []byte, contentType string) error {
	if q.closed {
		return errors.New("rabbitmq: adapter closed")
	}
	return q.pubCh.PublishWithContext(
		ctx,
		exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  contentType,
			Body:         body,
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
		},
	)
}

func (q *RabbitMQ) PublishJSON(ctx context.Context, exchange, routingKey string, message any) error {
	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	return q.publishWithRedial(ctx, exchange, routingKey, body, "application/json")
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
	q.pubMu.Lock()
	defer q.pubMu.Unlock()
	return q.redialLocked()
}

// redialLocked dials a fresh connection, swaps it (and the cached publisher
// channel) in, re-registers connection NotifyClose for the watcher, clears
// the dead flag, and nudges the watcher to follow the new connection.
// Caller holds pubMu.
func (q *RabbitMQ) redialLocked() error {
	conn, err := q.dial(q.url)
	if err != nil {
		return err
	}
	pubCh, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return err
	}
	// Best-effort close of the previous publisher channel.
	if q.pubCh != nil {
		_ = q.pubCh.Close()
	}
	if q.conn != nil {
		_ = q.conn.Close()
	}
	q.conn = conn
	q.pubCh = pubCh
	q.pubDead = false
	q.connCloseCh = conn.NotifyClose(make(chan *amqp.Error, 1))
	select {
	case q.wakeup <- struct{}{}:
	default:
	}
	return nil
}

func nextBackoff(current, ceiling time.Duration) time.Duration {
	next := current * 2
	if next > ceiling {
		return ceiling
	}
	return next
}

// healthzProbeQueue is the conventional queue name passed to
// QueueDeclarePassive during a Ping. It is never created on the broker —
// passive declare returns NOT_FOUND when the queue is absent, which Ping
// treats as broker-reachable success.
const healthzProbeQueue = "healthz.probe"

// Ping verifies the broker is reachable without mutating broker state.
// Opens a transient channel (NOT the cached publisher channel) so that a
// missing-queue NOT_FOUND, which closes the channel, cannot poison hot-path
// publishes. Calls QueueDeclarePassive on a reserved name; treats NOT_FOUND
// (404) as healthy because the round-trip succeeded.
func (q *RabbitMQ) Ping(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	q.pubMu.Lock()
	conn := q.conn
	closed := q.closed
	q.pubMu.Unlock()
	if closed {
		return errors.New("queue: closed")
	}
	if conn == nil {
		return errors.New("queue: not connected")
	}
	if conn.IsClosed() {
		return errors.New("queue: connection closed")
	}

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("queue ping: open channel: %w", err)
	}
	defer func() { _ = ch.Close() }()

	if _, err := ch.QueueDeclarePassive(healthzProbeQueue, true, false, false, false, nil); err != nil {
		var amqpErr *amqp.Error
		if errors.As(err, &amqpErr) && amqpErr.Code == amqp.NotFound {
			return nil
		}
		return fmt.Errorf("queue ping: %w", err)
	}
	return nil
}

func (q *RabbitMQ) DeclareQueue(ctx context.Context, name string, durable bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
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
	close(q.watchStop)
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
