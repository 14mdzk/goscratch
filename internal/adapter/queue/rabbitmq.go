package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQ implements port.Queue using RabbitMQ
type RabbitMQ struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

// NewRabbitMQ creates a new RabbitMQ connection
func NewRabbitMQ(url string) (*RabbitMQ, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to rabbitmq: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	return &RabbitMQ{
		conn:    conn,
		channel: ch,
	}, nil
}

func (q *RabbitMQ) Publish(ctx context.Context, exchange, routingKey string, body []byte) error {
	return q.channel.PublishWithContext(
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
}

func (q *RabbitMQ) PublishJSON(ctx context.Context, exchange, routingKey string, message any) error {
	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	return q.channel.PublishWithContext(
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
}

func (q *RabbitMQ) Consume(ctx context.Context, queueName string, handler func(body []byte) error) error {
	msgs, err := q.channel.Consume(
		queueName,
		"",    // consumer tag
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-msgs:
				if !ok {
					return
				}
				if err := handler(msg.Body); err != nil {
					// Reject and requeue on error
					msg.Nack(false, true)
				} else {
					msg.Ack(false)
				}
			}
		}
	}()

	return nil
}

func (q *RabbitMQ) DeclareQueue(ctx context.Context, name string, durable bool) error {
	_, err := q.channel.QueueDeclare(
		name,
		durable,
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}
	return nil
}

func (q *RabbitMQ) DeclareExchange(ctx context.Context, name, kind string, durable bool) error {
	err := q.channel.ExchangeDeclare(
		name,
		kind,
		durable,
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}
	return nil
}

func (q *RabbitMQ) BindQueue(ctx context.Context, queueName, exchange, routingKey string) error {
	err := q.channel.QueueBind(
		queueName,
		routingKey,
		exchange,
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to bind queue: %w", err)
	}
	return nil
}

func (q *RabbitMQ) Close() error {
	if err := q.channel.Close(); err != nil {
		return err
	}
	return q.conn.Close()
}
