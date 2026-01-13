package worker

import (
	"context"
	"sync"
	"time"

	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/pkg/logger"
)

// Worker consumes jobs from a queue and dispatches them to handlers
type Worker struct {
	queue       port.Queue
	handlers    map[string]JobHandler
	logger      *logger.Logger
	concurrency int
	queueName   string
	exchange    string

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu sync.RWMutex
}

// Config holds worker configuration
type Config struct {
	QueueName   string
	Exchange    string
	Concurrency int
}

// New creates a new Worker instance
func New(queue port.Queue, log *logger.Logger, cfg Config) *Worker {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 1
	}
	if cfg.QueueName == "" {
		cfg.QueueName = "jobs"
	}
	if cfg.Exchange == "" {
		cfg.Exchange = ""
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Worker{
		queue:       queue,
		handlers:    make(map[string]JobHandler),
		logger:      log,
		concurrency: cfg.Concurrency,
		queueName:   cfg.QueueName,
		exchange:    cfg.Exchange,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// RegisterHandler registers a job handler for a specific job type
func (w *Worker) RegisterHandler(handler JobHandler) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.handlers[handler.Type()] = handler
	w.logger.Info("Registered job handler", "type", handler.Type())
}

// Start begins consuming jobs from the queue
func (w *Worker) Start() error {
	w.logger.Info("Starting worker",
		"queue", w.queueName,
		"concurrency", w.concurrency,
	)

	// Ensure queue exists
	if err := w.queue.DeclareQueue(w.ctx, w.queueName, true); err != nil {
		w.logger.Warn("Failed to declare queue (may already exist)", "error", err)
	}

	// Start worker goroutines
	for i := 0; i < w.concurrency; i++ {
		w.wg.Add(1)
		go w.consume(i)
	}

	w.logger.Info("Worker started successfully")
	return nil
}

// consume handles incoming messages for a single worker goroutine
func (w *Worker) consume(workerID int) {
	defer w.wg.Done()

	w.logger.Debug("Worker goroutine started", "worker_id", workerID)

	// Consume uses a callback handler
	err := w.queue.Consume(w.ctx, w.queueName, func(body []byte) error {
		return w.handleMessage(workerID, body)
	})

	if err != nil {
		w.logger.Error("Consumer error", "error", err, "worker_id", workerID)
	}

	w.logger.Debug("Worker goroutine stopped", "worker_id", workerID)
}

// handleMessage processes a single message
func (w *Worker) handleMessage(workerID int, msg []byte) error {
	// Decode job
	job, err := DecodeJob(msg)
	if err != nil {
		w.logger.Error("Failed to decode job", "error", err, "worker_id", workerID)
		return nil // Acknowledge malformed messages to avoid retry loop
	}

	// Increment attempts
	job.IncrementAttempts()

	w.logger.Info("Processing job",
		"job_id", job.ID,
		"job_type", job.Type,
		"attempt", job.Attempts,
		"worker_id", workerID,
	)

	// Find handler
	w.mu.RLock()
	handler, exists := w.handlers[job.Type]
	w.mu.RUnlock()

	if !exists {
		w.logger.Error("No handler registered for job type",
			"job_type", job.Type,
			"job_id", job.ID,
		)
		return nil // Acknowledge unhandled job types
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(w.ctx, 5*time.Minute)
	defer cancel()

	// Execute handler
	start := time.Now()
	err = handler.Handle(ctx, job)
	duration := time.Since(start)

	if err != nil {
		w.logger.Error("Job failed",
			"job_id", job.ID,
			"job_type", job.Type,
			"error", err,
			"duration_ms", duration.Milliseconds(),
			"attempt", job.Attempts,
		)

		// Retry if possible
		if job.CanRetry() {
			w.retryJob(job)
		} else {
			w.logger.Error("Job exhausted retries",
				"job_id", job.ID,
				"job_type", job.Type,
				"attempts", job.Attempts,
			)
		}
		return nil // Acknowledge to avoid immediate redelivery
	}

	w.logger.Info("Job completed successfully",
		"job_id", job.ID,
		"job_type", job.Type,
		"duration_ms", duration.Milliseconds(),
	)

	return nil
}

// retryJob re-queues a failed job for retry
func (w *Worker) retryJob(job *Job) {
	// Exponential backoff delay
	delay := time.Duration(job.Attempts*job.Attempts) * time.Second

	w.logger.Info("Scheduling job retry",
		"job_id", job.ID,
		"job_type", job.Type,
		"attempt", job.Attempts,
		"delay", delay,
	)

	// Re-encode and publish
	data, err := job.Encode()
	if err != nil {
		w.logger.Error("Failed to encode job for retry", "error", err, "job_id", job.ID)
		return
	}

	// Simple retry - publish back to queue after delay
	go func() {
		time.Sleep(delay)
		if err := w.queue.Publish(w.ctx, w.exchange, w.queueName, data); err != nil {
			w.logger.Error("Failed to retry job", "error", err, "job_id", job.ID)
		}
	}()
}

// Shutdown gracefully stops the worker
func (w *Worker) Shutdown(ctx context.Context) error {
	w.logger.Info("Shutting down worker...")

	// Signal workers to stop
	w.cancel()

	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.logger.Info("Worker shutdown complete")
		return nil
	case <-ctx.Done():
		w.logger.Warn("Worker shutdown timed out")
		return ctx.Err()
	}
}

// Stats returns current worker statistics
func (w *Worker) Stats() map[string]any {
	w.mu.RLock()
	defer w.mu.RUnlock()

	handlers := make([]string, 0, len(w.handlers))
	for t := range w.handlers {
		handlers = append(handlers, t)
	}

	return map[string]any{
		"queue":       w.queueName,
		"concurrency": w.concurrency,
		"handlers":    handlers,
	}
}
