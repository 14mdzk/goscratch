package worker

import (
	"context"
	"fmt"

	"github.com/14mdzk/goscratch/internal/port"
)

// Publisher provides a convenient way to publish jobs to the queue
type Publisher struct {
	queue     port.Queue
	queueName string
	exchange  string
}

// NewPublisher creates a new job publisher
func NewPublisher(queue port.Queue, queueName, exchange string) *Publisher {
	if queueName == "" {
		queueName = "jobs"
	}
	return &Publisher{
		queue:     queue,
		queueName: queueName,
		exchange:  exchange,
	}
}

// Publish creates and publishes a job to the queue
func (p *Publisher) Publish(ctx context.Context, jobType string, payload any) error {
	job, err := NewJob(jobType, payload)
	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}

	data, err := job.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode job: %w", err)
	}

	if err := p.queue.Publish(ctx, p.exchange, p.queueName, data); err != nil {
		return fmt.Errorf("failed to publish job: %w", err)
	}

	return nil
}

// PublishWithRetry creates and publishes a job with custom retry count
func (p *Publisher) PublishWithRetry(ctx context.Context, jobType string, payload any, maxRetry int) error {
	job, err := NewJobWithRetry(jobType, payload, maxRetry)
	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}

	data, err := job.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode job: %w", err)
	}

	if err := p.queue.Publish(ctx, p.exchange, p.queueName, data); err != nil {
		return fmt.Errorf("failed to publish job: %w", err)
	}

	return nil
}

// PublishRaw publishes a pre-created job to the queue
func (p *Publisher) PublishRaw(ctx context.Context, job *Job) error {
	data, err := job.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode job: %w", err)
	}

	if err := p.queue.Publish(ctx, p.exchange, p.queueName, data); err != nil {
		return fmt.Errorf("failed to publish job: %w", err)
	}

	return nil
}
