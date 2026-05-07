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

// consume handles incoming messages for a single worker goroutine.
//
// queue.Consume registers a delivery goroutine and returns immediately, so the
// worker.wg would otherwise mark itself done before any handler ran (block-ship
// #14). We block on w.ctx.Done() so the wg actually covers the consumer's
// active window — Shutdown's wg.Wait will not return until ctx is cancelled and
// the underlying delivery goroutine has its cancel signal in hand.
func (w *Worker) consume(workerID int) {
	defer w.wg.Done()

	w.logger.Debug("Worker goroutine started", "worker_id", workerID)

	err := w.queue.Consume(w.ctx, w.queueName, func(body []byte) error {
		return w.handleMessage(workerID, body)
	})
	if err != nil {
		w.logger.Error("Consumer error", "error", err, "worker_id", workerID)
		return
	}

	<-w.ctx.Done()

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

// retryJob re-queues a failed job for retry after an exponential backoff.
//
// The retry goroutine is registered on w.wg so Shutdown's wg.Wait() does not
// return until pending retries either fire or cancel. The delay uses a Timer
// + select on w.ctx.Done() instead of time.Sleep so a long backoff cannot
// outlive a shutdown signal (block-ship #14: prior code slept past ctx and
// then attempted Publish on a closed channel).
func (w *Worker) retryJob(job *Job) {
	delay := time.Duration(job.Attempts*job.Attempts) * time.Second

	w.logger.Info("Scheduling job retry",
		"job_id", job.ID,
		"job_type", job.Type,
		"attempt", job.Attempts,
		"delay", delay,
	)

	data, err := job.Encode()
	if err != nil {
		w.logger.Error("Failed to encode job for retry", "error", err, "job_id", job.ID)
		return
	}

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()

		timer := time.NewTimer(delay)
		defer timer.Stop()

		select {
		case <-timer.C:
		case <-w.ctx.Done():
			w.logger.Debug("Retry cancelled by shutdown", "job_id", job.ID)
			return
		}

		// Re-check ctx before publishing — Shutdown may have closed the queue
		// channel between timer fire and this point.
		if err := w.ctx.Err(); err != nil {
			w.logger.Debug("Retry skipped: ctx cancelled before publish", "job_id", job.ID)
			return
		}

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
