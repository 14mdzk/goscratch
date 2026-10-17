package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
)

// defaultGoroutineQueueBuffer bounds each named in-memory queue. Publish
// blocks on ctx (backpressure) rather than growing without bound.
const defaultGoroutineQueueBuffer = 1024

// GoroutineQueue implements port.Queue in process memory: zero
// infrastructure, best-effort durability. Jobs live only in this process —
// they die with it, and anything still buffered at Close is dropped.
// Deployments that must survive restarts use the rabbitmq backend.
//
// Topology is direct: exchange and routing keys are ignored, Publish to a
// routing key lands on the same-named queue Consume reads. DeclareExchange
// and BindQueue are accepted no-ops so worker code needs no backend
// conditionals.
type GoroutineQueue struct {
	mu     sync.Mutex
	queues map[string]chan []byte
	done   chan struct{}
	closed bool
}

// NewGoroutineQueue creates an empty in-memory queue backend.
func NewGoroutineQueue() *GoroutineQueue {
	return &GoroutineQueue{
		queues: make(map[string]chan []byte),
		done:   make(chan struct{}),
	}
}

// channelFor returns the named queue channel, creating it on first use.
// It returns an error once Close has run.
func (q *GoroutineQueue) channelFor(name string) (chan []byte, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return nil, fmt.Errorf("goroutine queue is closed")
	}
	ch, ok := q.queues[name]
	if !ok {
		ch = make(chan []byte, defaultGoroutineQueueBuffer)
		q.queues[name] = ch
	}
	return ch, nil
}

// Publish delivers body to the named queue, applying ctx backpressure when
// the buffer is full.
func (q *GoroutineQueue) Publish(ctx context.Context, _, routingKey string, body []byte) error {
	ch, err := q.channelFor(routingKey)
	if err != nil {
		return err
	}
	select {
	case ch <- body:
		return nil
	case <-q.done:
		return fmt.Errorf("goroutine queue is closed")
	case <-ctx.Done():
		return ctx.Err()
	}
}

// PublishJSON sends a JSON-serializable message.
func (q *GoroutineQueue) PublishJSON(ctx context.Context, exchange, routingKey string, message any) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	return q.Publish(ctx, exchange, routingKey, data)
}

// Consume delivers each message on the named queue to handler, one at a
// time per caller (worker.Start runs one Consume per concurrency slot, so
// deliveries spread across slots). It returns when ctx is cancelled or the
// queue is closed; buffered-but-undelivered messages are dropped on Close.
// A handler error is logged and the message dropped: worker-level retry
// republishes through Publish, so the adapter never requeues behind the
// worker's back.
func (q *GoroutineQueue) Consume(ctx context.Context, queue string, handler func(body []byte) error) error {
	ch, err := q.channelFor(queue)
	if err != nil {
		return err
	}
	for {
		select {
		case body := <-ch:
			if err := handler(body); err != nil {
				slog.Error("goroutine queue handler failed; dropping message", "queue", queue, "error", err)
			}
		case <-q.done:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Ping reports whether the backend accepts work.
func (q *GoroutineQueue) Ping(_ context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return fmt.Errorf("goroutine queue is closed")
	}
	return nil
}

// DeclareQueue ensures the named in-memory queue exists.
func (q *GoroutineQueue) DeclareQueue(_ context.Context, name string, _ bool) error {
	_, err := q.channelFor(name)
	return err
}

// DeclareExchange is an accepted no-op: the in-memory topology is direct.
func (q *GoroutineQueue) DeclareExchange(_ context.Context, _, _ string, _ bool) error {
	return nil
}

// BindQueue is an accepted no-op: the in-memory topology is direct.
func (q *GoroutineQueue) BindQueue(_ context.Context, _, _, _ string) error {
	return nil
}

// Close drops buffered messages and unblocks consumers. Idempotent.
func (q *GoroutineQueue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.closed {
		q.closed = true
		close(q.done)
	}
	return nil
}
