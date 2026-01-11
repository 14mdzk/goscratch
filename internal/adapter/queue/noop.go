package queue

import "context"

// NoOpQueue implements port.Queue as a no-op
// Used when RabbitMQ is disabled
type NoOpQueue struct{}

// NewNoOpQueue creates a new no-op queue
func NewNoOpQueue() *NoOpQueue {
	return &NoOpQueue{}
}

func (q *NoOpQueue) Publish(ctx context.Context, exchange, routingKey string, body []byte) error {
	return nil
}

func (q *NoOpQueue) PublishJSON(ctx context.Context, exchange, routingKey string, message any) error {
	return nil
}

func (q *NoOpQueue) Consume(ctx context.Context, queue string, handler func(body []byte) error) error {
	// No-op: just block until context is done
	<-ctx.Done()
	return ctx.Err()
}

func (q *NoOpQueue) DeclareQueue(ctx context.Context, name string, durable bool) error {
	return nil
}

func (q *NoOpQueue) DeclareExchange(ctx context.Context, name, kind string, durable bool) error {
	return nil
}

func (q *NoOpQueue) BindQueue(ctx context.Context, queue, exchange, routingKey string) error {
	return nil
}

func (q *NoOpQueue) Close() error {
	return nil
}
