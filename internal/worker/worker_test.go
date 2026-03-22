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
		<-w.ctx.Done() // waits for cancel
		time.Sleep(5 * time.Second) // simulate slow cleanup
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := w.Shutdown(ctx)
	assert.Error(t, err, "should timeout")

	// Clean up: let the goroutine finish
	wgDone.Wait()
}
