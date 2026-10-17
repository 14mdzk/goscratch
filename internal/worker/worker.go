package worker

import (
	"context"
	"math/rand"
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

// Retry backoff bounds. delay = min(1s * 2^(attempts-1), 60s) scaled by a
// jitter factor in [0.5, 1.0): retries spread out instead of firing in
// lockstep after a shared outage. The 60s cap also bounds how long one
// held delivery can occupy a consumer channel. Code constants only.
const (
	retryBaseDelay = time.Second
	retryMaxDelay  = 60 * time.Second
)

// retryDelay returns the backoff for a job that has `attempts` attempts
// recorded (post-increment). attempt 1 -> [0.5s, 1s), 2 -> [1s, 2s),
// converging to [30s, 60s) from attempt 7 on.
func retryDelay(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	shift := attempts - 1
	if shift > 6 {
		shift = 6
	}
	capped := retryBaseDelay << shift
	if capped > retryMaxDelay || capped <= 0 {
		capped = retryMaxDelay
	}
	half := float64(capped) / 2
	return time.Duration(half + rand.Float64()*half)
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

	// Ensure the dead-letter queue exists: every terminal path parks there,
	// so the DLQ must exist before the first job can die.
	if err := w.queue.DeclareQueue(w.ctx, w.dlqName(), true); err != nil {
		w.logger.Warn("Failed to declare DLQ (may already exist)", "error", err)
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

// dlqName derives the dead-letter queue: <queue>.dlq, declared durable at
// startup next to the main queue. Parked payloads are the original bytes
// verbatim so replay is a republish away; operators inspect via the broker
// UI (no redrive tooling in v1.3).
func (w *Worker) dlqName() string {
	return w.queueName + ".dlq"
}

// park publishes the original payload bytes verbatim to the DLQ. Reason
// metadata goes to the structured log at park time, never into the parked
// bytes, so the job stays replayable unmodified.
func (w *Worker) park(original []byte, reason string, fields ...any) error {
	args := append([]any{"reason", reason, "dlq", w.dlqName()}, fields...)
	w.logger.Error("Parking job in DLQ", args...)
	return w.queue.Publish(w.ctx, w.exchange, w.dlqName(), original)
}

// handleMessage processes a single message.
//
// Settlement contract (see port.Queue.Consume): nil ACKs the delivery,
// a non-nil error rejects it for redelivery. Retries hold the delivery
// across the backoff — the broker owns the job while we wait — and only
// settle once the job is republished (ACK), parked (ACK), or handed back
// to the broker on shutdown (error → Nack with requeue). Attempts are
// incremented exactly once per processing run, so a redelivery after a
// shutdown-cancel re-increments to the same number: no double-count.
func (w *Worker) handleMessage(workerID int, msg []byte) error {
	// Decode job
	job, err := DecodeJob(msg)
	if err != nil {
		w.logger.Error("Failed to decode job; attempting DLQ park",
			"error", err, "worker_id", workerID)
		if perr := w.park(msg, "undecodable", "error", err, "worker_id", workerID); perr != nil {
			// Poison guard: requeueing an undecodable payload loops forever
			// with no attempt counter to stop it. ACK and move on.
			w.logger.Error("DLQ park failed for undecodable payload; ACKing to avoid poison loop",
				"park_error", perr, "worker_id", workerID)
		}
		return nil
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
		w.logger.Error("No handler registered for job type; parking",
			"job_type", job.Type,
			"job_id", job.ID,
		)
		if perr := w.park(msg, "unhandled-job-type",
			"job_type", job.Type, "job_id", job.ID); perr != nil {
			return perr // requeue: the job was never parked
		}
		return nil
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

		if !job.CanRetry() {
			w.logger.Error("Job exhausted retries; parking",
				"job_id", job.ID,
				"job_type", job.Type,
				"attempts", job.Attempts,
			)
			if perr := w.park(msg, "retries-exhausted",
				"job_id", job.ID, "job_type", job.Type,
				"attempts", job.Attempts, "last_error", err,
			); perr != nil {
				return perr // requeue: the job was never parked
			}
			return nil
		}

		return w.holdForRetry(job)
	}

	w.logger.Info("Job completed successfully",
		"job_id", job.ID,
		"job_type", job.Type,
		"duration_ms", duration.Milliseconds(),
	)

	return nil
}

// holdForRetry waits out the backoff in the handler's own goroutine, then
// republishes the job with the incremented attempt and returns nil so the
// adapter ACKs the held delivery. The wait honors the worker context (not
// the per-handler timeout): on shutdown-cancel it returns an error WITHOUT
// republishing, so the adapter Nacks with requeue and the broker redelivers
// — attempts were never republished, so the next run re-increments to the
// same number. No retry goroutines, no WaitGroup tracking: shutdown never
// waits out a backoff.
func (w *Worker) holdForRetry(job *Job) error {
	delay := retryDelay(job.Attempts)

	w.logger.Info("Holding delivery for retry backoff",
		"job_id", job.ID,
		"job_type", job.Type,
		"attempt", job.Attempts,
		"delay", delay,
	)

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
	case <-w.ctx.Done():
		w.logger.Debug("Retry cancelled by shutdown; delivery returns to broker",
			"job_id", job.ID)
		return w.ctx.Err()
	}

	// Re-check: shutdown may have landed between timer fire and publish.
	if err := w.ctx.Err(); err != nil {
		w.logger.Debug("Retry skipped: ctx cancelled before republish",
			"job_id", job.ID)
		return err
	}

	data, err := job.Encode()
	if err != nil {
		w.logger.Error("Failed to encode job for retry; ACKing to avoid poison loop",
			"error", err, "job_id", job.ID)
		return nil
	}

	if err := w.queue.Publish(w.ctx, w.exchange, w.queueName, data); err != nil {
		w.logger.Error("Failed to republish job for retry; delivery returns to broker",
			"error", err, "job_id", job.ID)
		return err
	}
	return nil
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
