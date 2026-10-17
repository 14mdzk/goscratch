package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/14mdzk/goscratch/pkg/logger"
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

func TestHandleMessage_UnknownJobType_Parks(t *testing.T) {
	q := &mockQueue{}
	log := newTestLogger()
	w := New(q, log, Config{QueueName: "t"})

	job, _ := NewJob("unknown.type", "data")
	data, _ := job.Encode()

	// Parks, then ACKs (nil error).
	err := w.handleMessage(0, data)
	assert.NoError(t, err)

	q.mu.Lock()
	defer q.mu.Unlock()
	require.Len(t, q.publishCalls, 1, "unhandled job must be parked exactly once")
	assert.Equal(t, "t.dlq", q.publishCalls[0].routingKey)
	assert.Equal(t, data, q.publishCalls[0].body, "parked payload must be the original bytes verbatim")
}

func TestHandleMessage_InvalidJSON_Parks(t *testing.T) {
	q := &mockQueue{}
	log := newTestLogger()
	w := New(q, log, Config{QueueName: "t"})

	raw := []byte("not-json")
	err := w.handleMessage(0, raw)
	assert.NoError(t, err) // park, then ACK

	q.mu.Lock()
	defer q.mu.Unlock()
	require.Len(t, q.publishCalls, 1, "undecodable payload must be parked exactly once")
	assert.Equal(t, "t.dlq", q.publishCalls[0].routingKey)
	assert.Equal(t, raw, q.publishCalls[0].body)
}

func TestHandleMessage_RetryOnFailure_HoldsAndRepublishes(t *testing.T) {
	q := &mockQueue{}
	log := newTestLogger()
	w := New(q, log, Config{QueueName: "t"})

	handler := &testHandler{
		jobType: "fail.job",
		handleFn: func(_ context.Context, _ *Job) error {
			return errors.New("processing failed")
		},
	}
	w.RegisterHandler(handler)

	// Attempts=0 -> increment to 1 -> CanRetry, held backoff [0.5s, 1s).
	job, _ := NewJob("fail.job", "data")
	data, _ := job.Encode()

	err := w.handleMessage(0, data)
	assert.NoError(t, err, "held retry republishes, then ACKs")

	// Synchronous now: exactly one republish, attempts incremented once.
	q.mu.Lock()
	defer q.mu.Unlock()
	require.Len(t, q.publishCalls, 1, "should have re-published the job for retry exactly once")
	call := q.publishCalls[0]
	assert.Equal(t, "t", call.routingKey, "retry republishes to the main queue, not the DLQ")
	rep := &Job{}
	require.NoError(t, json.Unmarshal(call.body, rep))
	assert.Equal(t, 1, rep.Attempts, "republished job must carry attempts+1")
	assert.Equal(t, job.ID, rep.ID)
}

func TestRetryDelay_Bounds(t *testing.T) {
	// attempt -> [cap/2, cap) with cap = min(1s*2^(n-1), 60s).
	cases := []struct {
		attempts int
		min      time.Duration
		max      time.Duration
	}{
		{attempts: 0, min: 500 * time.Millisecond, max: time.Second},
		{attempts: 1, min: 500 * time.Millisecond, max: time.Second},
		{attempts: 2, min: time.Second, max: 2 * time.Second},
		{attempts: 3, min: 2 * time.Second, max: 4 * time.Second},
		{attempts: 7, min: 30 * time.Second, max: 60 * time.Second},
		{attempts: 100, min: 30 * time.Second, max: 60 * time.Second},
	}
	for _, tc := range cases {
		for i := 0; i < 25; i++ {
			d := retryDelay(tc.attempts)
			assert.GreaterOrEqual(t, d, tc.min, "attempts=%d", tc.attempts)
			assert.Less(t, d, tc.max, "attempts=%d (cap respected, jitter below cap)", tc.attempts)
		}
	}
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
	assert.NoError(t, err) // parked, then ACKs

	// No retry: exactly one DLQ park with the original bytes.
	q.mu.Lock()
	defer q.mu.Unlock()
	require.Len(t, q.publishCalls, 1, "exhausted job must be parked exactly once, never retried")
	assert.Equal(t, "jobs.dlq", q.publishCalls[0].routingKey)
	assert.Equal(t, data, q.publishCalls[0].body, "parked payload must be the original bytes verbatim")
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

// dispatchingQueue is a mock port.Queue that, when Consume is called, immediately
// delivers one pre-encoded job to the handler in the same goroutine (simulating
// a broker delivering a message), then blocks until ctx is cancelled. This lets
// us test that Shutdown waits for the handler to finish before returning.
type dispatchingQueue struct {
	mockQueue
	jobData []byte
}

func (d *dispatchingQueue) Consume(ctx context.Context, _ string, handler func([]byte) error) error {
	if d.jobData != nil {
		_ = handler(d.jobData)
	}
	<-ctx.Done()
	return nil
}

// TestShutdown_WaitsForSlowHandler is a regression test for block-ship #14.
// It verifies that Shutdown's wg.Wait() does not return before an in-flight
// handler finishes. The wg must cover the consumer's active window: if
// wg.Done() were called before the handler returned, the assertion would fire
// because elapsed < slowDuration.
func TestShutdown_WaitsForSlowHandler(t *testing.T) {
	const (
		slowDuration            = 500 * time.Millisecond
		dispatchToShutdownDelay = 100 * time.Millisecond
		epsilon                 = 50 * time.Millisecond
		budget                  = 300 * time.Millisecond
	)

	job, err := NewJob("slow.job", "payload")
	require.NoError(t, err)
	jobData, err := job.Encode()
	require.NoError(t, err)

	q := &dispatchingQueue{jobData: jobData}
	log := newTestLogger()
	w := New(q, log, Config{QueueName: "t", Concurrency: 1})

	handlerStarted := make(chan struct{})
	w.RegisterHandler(&testHandler{
		jobType: "slow.job",
		handleFn: func(_ context.Context, _ *Job) error {
			close(handlerStarted)
			time.Sleep(slowDuration)
			return nil
		},
	})

	require.NoError(t, w.Start())

	// Wait until the slow handler is actually executing before triggering shutdown.
	select {
	case <-handlerStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("handler never started")
	}

	time.Sleep(dispatchToShutdownDelay)

	ctx, cancel := context.WithTimeout(context.Background(), slowDuration+budget)
	defer cancel()

	start := time.Now()
	err = w.Shutdown(ctx)
	elapsed := time.Since(start)

	require.NoError(t, err, "Shutdown must not time out — wg must cover the slow handler")

	// Shutdown must have waited: elapsed must be at least the remaining handler
	// sleep minus epsilon. Because we slept dispatchToShutdownDelay after
	// handlerStarted, the handler still has ~slowDuration-dispatchToShutdownDelay
	// left to sleep.
	minExpected := slowDuration - dispatchToShutdownDelay - epsilon
	assert.GreaterOrEqual(t, elapsed, minExpected,
		"Shutdown returned too early — wg.Done must not fire before handler returns")

	assert.Less(t, elapsed, slowDuration+budget,
		"Shutdown took unexpectedly long — something else is blocking")
}

// TestShutdown_SlowHandler_WGDoneBeforeReturn_WouldFail documents the failure
// mode: if the fix is reverted (wg.Done fires before handler returns), the
// WaitsForSlowHandler test above would fail. This test serves as a sentinel to
// make the coverage intent explicit in code review.
//
// It is NOT a test of broken behaviour — it is intentionally skipped; its
// purpose is to document what "broken" looks like so reviewers understand the
// other test's invariant.
func TestShutdown_SlowHandler_WGDoneBeforeReturn_WouldFail(t *testing.T) {
	t.Skip("documentation test — skip always; see TestShutdown_WaitsForSlowHandler for the live assertion")
}

// TestHandleMessage_MidBackoff_CancelsOnCtxDone: a shutdown-cancel during the
// held backoff must NOT republish; handleMessage returns an error so the
// adapter Nacks with requeue (attempts were never republished: no
// double-count on redelivery).
func TestHandleMessage_MidBackoff_CancelsOnCtxDone(t *testing.T) {
	q := &mockQueue{}
	log := newTestLogger()
	w := New(q, log, Config{QueueName: "t", Concurrency: 1})

	w.RegisterHandler(&testHandler{
		jobType: "always.fail",
		handleFn: func(_ context.Context, _ *Job) error {
			return errors.New("forced failure")
		},
	})

	// Attempts=1 -> after increment Attempts=2, held backoff [1s, 2s).
	job, err := NewJob("always.fail", "payload")
	require.NoError(t, err)
	job.Attempts = 1
	jobData, err := job.Encode()
	require.NoError(t, err)

	result := make(chan error, 1)
	go func() {
		result <- w.handleMessage(0, jobData)
	}()

	// Let the handler fail and the hold begin, then cancel mid-backoff.
	time.Sleep(200 * time.Millisecond)
	w.cancel()

	select {
	case err := <-result:
		require.Error(t, err, "cancelled hold must return an error (Nack path)")
	case <-time.After(5 * time.Second):
		t.Fatal("handleMessage did not return after ctx cancel")
	}

	q.mu.Lock()
	defer q.mu.Unlock()
	assert.Equal(t, 0, len(q.publishCalls),
		"no publish must occur after cancel — ctx-cancel guard must skip republish")
}

// TestHandleMessage_ParkFailure_Requeues: when the DLQ park itself fails
// (broker down), an exhausted job must come back as an error so the adapter
// requeues — never ACK a message that was not parked.
func TestHandleMessage_ParkFailure_Requeues(t *testing.T) {
	q := &mockQueue{publishErr: errors.New("broker down")}
	log := newTestLogger()
	w := New(q, log, Config{QueueName: "t"})

	w.RegisterHandler(&testHandler{
		jobType: "fail.job",
		handleFn: func(_ context.Context, _ *Job) error {
			return errors.New("processing failed")
		},
	})

	job, _ := NewJob("fail.job", "data")
	job.Attempts = 99 // exhausted
	data, _ := job.Encode()

	err := w.handleMessage(0, data)
	assert.Error(t, err, "failed park must surface as an error (requeue), never an ACK")
}

// TestHandleMessage_UndecodableParkFailure_Acks: poison-message guard —
// requeueing an undecodable payload loops forever with no attempt counter,
// so a failed park still ACKs (with an error log).
func TestHandleMessage_UndecodableParkFailure_Acks(t *testing.T) {
	q := &mockQueue{publishErr: errors.New("broker down")}
	log := newTestLogger()
	w := New(q, log, Config{QueueName: "t"})

	err := w.handleMessage(0, []byte("not-json"))
	assert.NoError(t, err, "failed park of an undecodable payload must still ACK (poison guard)")
}

// TestWorker_Start_DeclaresDLQ: the DLQ must exist before the first job can
// die — Start declares it durable next to the main queue.
func TestWorker_Start_DeclaresDLQ(t *testing.T) {
	q := &mockQueue{}
	log := newTestLogger()
	w := New(q, log, Config{QueueName: "t", Concurrency: 1})

	require.NoError(t, w.Start())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, w.Shutdown(ctx))

	q.mu.Lock()
	defer q.mu.Unlock()
	assert.Contains(t, q.declareCalls, "t")
	assert.Contains(t, q.declareCalls, "t.dlq")
}
