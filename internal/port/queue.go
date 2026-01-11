package port

import "context"

// Queue defines the interface for message queue operations
type Queue interface {
	// Publish sends a message to a queue/exchange
	Publish(ctx context.Context, exchange, routingKey string, body []byte) error

	// PublishJSON sends a JSON-serializable message
	PublishJSON(ctx context.Context, exchange, routingKey string, message any) error

	// Consume starts consuming messages from a queue
	// The handler function is called for each message
	// Return nil to acknowledge, error to reject/requeue
	Consume(ctx context.Context, queue string, handler func(body []byte) error) error

	// DeclareQueue ensures a queue exists
	DeclareQueue(ctx context.Context, name string, durable bool) error

	// DeclareExchange ensures an exchange exists
	DeclareExchange(ctx context.Context, name, kind string, durable bool) error

	// BindQueue binds a queue to an exchange with a routing key
	BindQueue(ctx context.Context, queue, exchange, routingKey string) error

	// Close closes the queue connection
	Close() error
}

// Message represents a queue message
type Message struct {
	ID          string
	Body        []byte
	ContentType string
	Headers     map[string]any
	Redelivered bool
}
