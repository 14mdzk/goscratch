package worker

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/14mdzk/goscratch/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testHandler is a mock JobHandler for testing
type testHandler struct {
	jobType  string
	handleFn func(ctx context.Context, job *Job) error
}

func (h *testHandler) Type() string { return h.jobType }
func (h *testHandler) Handle(ctx context.Context, job *Job) error {
	if h.handleFn != nil {
		return h.handleFn(ctx, job)
	}
	return nil
}

func newTestLogger() *logger.Logger {
	return logger.New(logger.Config{
		Level:  "debug",
		Format: "json",
		Output: &bytes.Buffer{},
	})
}

func TestRegisterHandler(t *testing.T) {
	q := &mockQueue{}
	log := newTestLogger()
	w := New(q, log, Config{QueueName: "test", Concurrency: 1})

	handler := &testHandler{jobType: "email.send"}
	w.RegisterHandler(handler)

	stats := w.Stats()
	handlers := stats["handlers"].([]string)
	assert.Contains(t, handlers, "email.send")
}

func TestRegisterHandler_Multiple(t *testing.T) {
	q := &mockQueue{}
	log := newTestLogger()
	w := New(q, log, Config{})

	w.RegisterHandler(&testHandler{jobType: "email.send"})
	w.RegisterHandler(&testHandler{jobType: "audit.cleanup"})

	stats := w.Stats()
	handlers := stats["handlers"].([]string)
	assert.Len(t, handlers, 2)
	assert.Contains(t, handlers, "email.send")
	assert.Contains(t, handlers, "audit.cleanup")
}

func TestStats(t *testing.T) {
	q := &mockQueue{}
	log := newTestLogger()
	w := New(q, log, Config{QueueName: "my-queue", Concurrency: 4})

	w.RegisterHandler(&testHandler{jobType: "email.send"})

	stats := w.Stats()
	assert.Equal(t, "my-queue", stats["queue"])
	assert.Equal(t, 4, stats["concurrency"])
	handlers := stats["handlers"].([]string)
	assert.Contains(t, handlers, "email.send")
}

func TestNew_Defaults(t *testing.T) {
	q := &mockQueue{}
	log := newTestLogger()
	w := New(q, log, Config{})

	stats := w.Stats()
	assert.Equal(t, "jobs", stats["queue"])
	assert.Equal(t, 1, stats["concurrency"])
}

func TestHandleMessage_DispatchesToCorrectHandler(t *testing.T) {
	q := &mockQueue{}
	log := newTestLogger()
	w := New(q, log, Config{})

	var receivedJobID string
	handler := &testHandler{
		jobType: "email.send",
		handleFn: func(_ context.Context, job *Job) error {
			receivedJobID = job.ID
			return nil
		},
	}
	w.RegisterHandler(handler)

	job, err := NewJob("email.send", map[string]string{"to": "test@example.com"})
	require.NoError(t, err)

	data, err := job.Encode()
	require.NoError(t, err)

	err = w.handleMessage(0, data)
	assert.NoError(t, err)
	assert.Equal(t, job.ID, receivedJobID)
}

func TestHandleMessage_UnknownJobType(t *testing.T) {
	q := &mockQueue{}
	log := newTestLogger()
	w := New(q, log, Config{})

	job, _ := NewJob("unknown.type", "data")
	data, _ := job.Encode()

	// Should not error (ack malformed/unknown)
	err := w.handleMessage(0, data)
	assert.NoError(t, err)
}

func TestHandleMessage_InvalidJSON(t *testing.T) {
	q := &mockQueue{}
	log := newTestLogger()
	w := New(q, log, Config{})

	err := w.handleMessage(0, []byte("not-json"))
	assert.NoError(t, err) // ack malformed messages
}

func TestHandleMessage_RetryOnFailure(t *testing.T) {
	q := &mockQueue{}
	log := newTestLogger()
	w := New(q, log, Config{})

	handler := &testHandler{
		jobType: "fail.job",
		handleFn: func(_ context.Context, _ *Job) error {
			return errors.New("processing failed")
		},
	}
	w.RegisterHandler(handler)

	// Job with MaxRetry=3, Attempts=0. After handleMessage increments, Attempts=1 < MaxRetry=3 -> retry
	job, _ := NewJob("fail.job", "data")
	data, _ := job.Encode()

	err := w.handleMessage(0, data)
	assert.NoError(t, err) // still returns nil (ack to avoid immediate redelivery)

	// Give the goroutine in retryJob time to publish
	time.Sleep(2 * time.Second)

	q.mu.Lock()
	callCount := len(q.publishCalls)
	q.mu.Unlock()
	assert.Equal(t, 1, callCount, "should have re-published the job for retry")
}

func TestHandleMessage_ExhaustedRetries(t *testing.T) {
	q := &mockQueue{}
	log := newTestLogger()
	w := New(q, log, Config{})

	handler := &testHandler{
		jobType: "fail.job",
		handleFn: func(_ context.Context, _ *Job) error {
			return errors.New("processing failed")
		},
	}
	w.RegisterHandler(handler)

	// Job already at max retries: Attempts=2, MaxRetry=3 -> after increment Attempts=3, CanRetry=false
	job, _ := NewJob("fail.job", "data")
	job.Attempts = 2
	data, _ := job.Encode()

	err := w.handleMessage(0, data)
	assert.NoError(t, err)

	// No retry should happen
	time.Sleep(100 * time.Millisecond)
	q.mu.Lock()
	callCount := len(q.publishCalls)
	q.mu.Unlock()
	assert.Equal(t, 0, callCount, "should not retry exhausted job")
}

func TestHandleMessage_IncrementsAttempts(t *testing.T) {
	q := &mockQueue{}
	log := newTestLogger()
	w := New(q, log, Config{})

	var capturedAttempts int
	handler := &testHandler{
		jobType: "test.attempts",
		handleFn: func(_ context.Context, job *Job) error {
			capturedAttempts = job.Attempts
			return nil
		},
	}
	w.RegisterHandler(handler)

	job, _ := NewJob("test.attempts", "data")
	job.Attempts = 0
	data, _ := job.Encode()

	_ = w.handleMessage(0, data)
	assert.Equal(t, 1, capturedAttempts, "attempts should be incremented before handling")
}

func TestShutdown(t *testing.T) {
	q := &mockQueue{}
	log := newTestLogger()
	w := New(q, log, Config{})

	// Shutdown without starting should succeed immediately
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := w.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestShutdown_WithTimeout(t *testing.T) {
	q := &mockQueue{}
	log := newTestLogger()
	w := New(q, log, Config{})

	// Simulate a goroutine that won't finish
	w.wg.Add(1)
	var wgDone sync.WaitGroup
	wgDone.Add(1)
	go func() {
		defer wgDone.Done()
		defer w.wg.Done()
		<-w.ctx.Done()              // waits for cancel
		time.Sleep(5 * time.Second) // simulate slow cleanup
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := w.Shutdown(ctx)
	assert.Error(t, err, "should timeout")

	// Clean up: let the goroutine finish
	wgDone.Wait()
}

// TestRetry_CancelsOnShutdown verifies that a pending retry goroutine exits
// promptly when ctx is cancelled instead of sleeping through the full backoff
// delay (block-ship #14: prior code used time.Sleep without ctx awareness).
func TestRetry_CancelsOnShutdown(t *testing.T) {
	q := &mockQueue{}
	log := newTestLogger()
	w := New(q, log, Config{QueueName: "t", Concurrency: 1})

	// Schedule a retry with a long delay (Attempts=3 -> 9s).
	job, _ := NewJob("retry.test", "data")
	job.Attempts = 3
	w.retryJob(job)

	// Immediately shutdown. Total Shutdown duration must be much shorter than
	// the 9s scheduled delay because the retry goroutine selects on ctx.Done.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	err := w.Shutdown(ctx)
	elapsed := time.Since(start)

	require.NoError(t, err, "Shutdown should not time out — retry goroutine must exit on ctx.Done")
	assert.Less(t, elapsed, 500*time.Millisecond, "Shutdown must not wait for full retry backoff")

	q.mu.Lock()
	callCount := len(q.publishCalls)
	q.mu.Unlock()
	assert.Equal(t, 0, callCount, "no publish must happen after Shutdown — ctx-cancel guard must skip Publish")
}

// TestRetry_TrackedByWaitGroup verifies that the retry goroutine is registered
// on w.wg so Shutdown's wg.Wait actually waits for it. We use a 0-attempt job
// (delay=0) so the retry fires immediately, then assert publish completed
// before Shutdown returned.
func TestRetry_TrackedByWaitGroup(t *testing.T) {
	q := &mockQueue{}
	log := newTestLogger()
	w := New(q, log, Config{QueueName: "t", Concurrency: 1})

	// Attempts=1 -> 1*1*time.Second = 1s delay. Wait for it via Shutdown.
	job, _ := NewJob("retry.test", "data")
	job.Attempts = 1
	w.retryJob(job)

	// Give the timer time to fire. Shutdown waits for wg.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Sleep just past the retry timer so Publish has fired before we cancel.
	time.Sleep(1100 * time.Millisecond)

	require.NoError(t, w.Shutdown(ctx))

	q.mu.Lock()
	callCount := len(q.publishCalls)
	q.mu.Unlock()
	assert.Equal(t, 1, callCount, "retry must have published once before Shutdown returned")
}
