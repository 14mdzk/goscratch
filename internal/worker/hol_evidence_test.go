//go:build integration

// Head-of-line blocking evidence for wayfinder ticket #148 (child of #141).
// Scratch repro, not a deliverable: measures how serial per-consumer dispatch
// in the RabbitMQ adapter delays fast jobs behind slow / held-retry jobs.
// Numbers go on the ticket; this file lives only on scratch/148-hol-evidence.
package worker_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/14mdzk/goscratch/internal/adapter/queue"
	"github.com/14mdzk/goscratch/internal/platform/testutil"
	"github.com/14mdzk/goscratch/internal/worker"
	"github.com/14mdzk/goscratch/pkg/logger"
)

type holHandler struct {
	jobType  string
	handleFn func(ctx context.Context, job *worker.Job) error
}

func (h *holHandler) Type() string { return h.jobType }
func (h *holHandler) Handle(ctx context.Context, job *worker.Job) error {
	return h.handleFn(ctx, job)
}

func holLogger() *logger.Logger {
	return logger.New(logger.Config{Level: "error", Format: "json", Output: &bytes.Buffer{}})
}

func holPublish(t *testing.T, ctx context.Context, q *queue.RabbitMQ, queueName string, types ...string) time.Time {
	t.Helper()
	for _, typ := range types {
		job, err := worker.NewJob(typ, map[string]string{"q": queueName})
		require.NoError(t, err)
		data, err := job.Encode()
		require.NoError(t, err)
		require.NoError(t, q.Publish(ctx, "", queueName, data))
	}
	return time.Now()
}

func holWorker(t *testing.T, q *queue.RabbitMQ, queueName string, concurrency int, handlers ...*holHandler) *worker.Worker {
	t.Helper()
	w := worker.New(q, holLogger(), worker.Config{QueueName: queueName, Concurrency: concurrency})
	for _, h := range handlers {
		w.RegisterHandler(h)
	}
	require.NoError(t, w.Start())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = w.Shutdown(ctx)
	})
	return w
}

// Exp A: concurrency=1. slow(5s) then fast. Serial dispatch predicts fast
// completes ~5s after publish.
func TestHOL_SerialSingleConsumer(t *testing.T) {
	ctx := context.Background()
	url, cleanup, err := testutil.StartRabbitMQ(ctx)
	require.NoError(t, err)
	defer cleanup()

	q, err := queue.NewRabbitMQWithOptions(url, queue.Options{PrefetchCount: 10})
	require.NoError(t, err)
	defer func() { _ = q.Close() }()

	slowStart := make(chan time.Time, 1)
	fastDone := make(chan time.Time, 1)
	holWorker(t, q, "hol-a", 1,
		&holHandler{jobType: "hol-slow", handleFn: func(ctx context.Context, job *worker.Job) error {
			slowStart <- time.Now()
			time.Sleep(5 * time.Second)
			return nil
		}},
		&holHandler{jobType: "hol-fast", handleFn: func(ctx context.Context, job *worker.Job) error {
			fastDone <- time.Now()
			return nil
		}},
	)

	t0 := holPublish(t, ctx, q, "hol-a", "hol-slow", "hol-fast")

	select {
	case done := <-fastDone:
		t.Logf("EXP-A conc=1 prefetch=10: fast latency from publish = %s", done.Sub(t0).Truncate(time.Millisecond))
		require.GreaterOrEqual(t, done.Sub(t0), 4500*time.Millisecond, "serial dispatch should hold fast behind the 5s slow job")
	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for fast job")
	}
}

// Exp B1: concurrency=2, prefetch=10. slow(5s) + fast + fast published
// back-to-back. Round-robin puts slow+2nd-fast on one consumer; the buffered
// 2nd fast waits ~5s even after the other consumer frees up.
func TestHOL_PrefetchAmplifies(t *testing.T) {
	ctx := context.Background()
	url, cleanup, err := testutil.StartRabbitMQ(ctx)
	require.NoError(t, err)
	defer cleanup()

	q, err := queue.NewRabbitMQWithOptions(url, queue.Options{PrefetchCount: 10})
	require.NoError(t, err)
	defer func() { _ = q.Close() }()

	secondFast := make(chan time.Time, 2)
	holWorker(t, q, "hol-b1", 2,
		&holHandler{jobType: "hol-slow", handleFn: func(ctx context.Context, job *worker.Job) error {
			time.Sleep(5 * time.Second)
			return nil
		}},
		&holHandler{jobType: "hol-fast", handleFn: func(ctx context.Context, job *worker.Job) error {
			secondFast <- time.Now()
			return nil
		}},
	)

	t0 := holPublish(t, ctx, q, "hol-b1", "hol-slow", "hol-fast", "hol-fast")

	var last time.Time
	for i := 0; i < 2; i++ {
		select {
		case done := <-secondFast:
			last = done
		case <-time.After(30 * time.Second):
			t.Fatal("timed out waiting for fast jobs")
		}
	}
	t.Logf("EXP-B1 conc=2 prefetch=10: last fast latency from publish = %s", last.Sub(t0).Truncate(time.Millisecond))
	require.GreaterOrEqual(t, last.Sub(t0), 4500*time.Millisecond, "prefetched fast job should wait behind the slow job on the same consumer")
}

// Exp B2: same as B1 with prefetch=1. The broker cannot strand the 3rd
// message on the busy consumer, so it reaches the freed consumer quickly.
func TestHOL_PrefetchOneMitigates(t *testing.T) {
	ctx := context.Background()
	url, cleanup, err := testutil.StartRabbitMQ(ctx)
	require.NoError(t, err)
	defer cleanup()

	q, err := queue.NewRabbitMQWithOptions(url, queue.Options{PrefetchCount: 1})
	require.NoError(t, err)
	defer func() { _ = q.Close() }()

	secondFast := make(chan time.Time, 2)
	holWorker(t, q, "hol-b2", 2,
		&holHandler{jobType: "hol-slow", handleFn: func(ctx context.Context, job *worker.Job) error {
			time.Sleep(5 * time.Second)
			return nil
		}},
		&holHandler{jobType: "hol-fast", handleFn: func(ctx context.Context, job *worker.Job) error {
			secondFast <- time.Now()
			return nil
		}},
	)

	t0 := holPublish(t, ctx, q, "hol-b2", "hol-slow", "hol-fast", "hol-fast")

	var last time.Time
	for i := 0; i < 2; i++ {
		select {
		case done := <-secondFast:
			last = done
		case <-time.After(30 * time.Second):
			t.Fatal("timed out waiting for fast jobs")
		}
	}
	t.Logf("EXP-B2 conc=2 prefetch=1: last fast latency from publish = %s", last.Sub(t0).Truncate(time.Millisecond))
	require.Less(t, last.Sub(t0), 4*time.Second, "prefetch=1 should let the 3rd job reach the freed consumer")
}

// Exp C: concurrency=1. Always-failing job (retries) then fast. The held
// retry occupies the consumer across the first backoff ([0.5s, 1s)), so
// fast waits ~that long. Higher attempts hold up to the 60s cap (code
// reading: retryMaxDelay in worker.go).
func TestHOL_HeldRetryOccupiesConsumer(t *testing.T) {
	ctx := context.Background()
	url, cleanup, err := testutil.StartRabbitMQ(ctx)
	require.NoError(t, err)
	defer cleanup()

	q, err := queue.NewRabbitMQWithOptions(url, queue.Options{PrefetchCount: 10})
	require.NoError(t, err)
	defer func() { _ = q.Close() }()

	fastDone := make(chan time.Time, 1)
	holWorker(t, q, "hol-c", 1,
		&holHandler{jobType: "hol-fail", handleFn: func(ctx context.Context, job *worker.Job) error {
			return errors.New("hol: intentional failure")
		}},
		&holHandler{jobType: "hol-fast", handleFn: func(ctx context.Context, job *worker.Job) error {
			fastDone <- time.Now()
			return nil
		}},
	)

	t0 := holPublish(t, ctx, q, "hol-c", "hol-fail", "hol-fast")

	select {
	case done := <-fastDone:
		t.Logf("EXP-C conc=1 held-retry: fast latency from publish = %s", done.Sub(t0).Truncate(time.Millisecond))
		require.GreaterOrEqual(t, done.Sub(t0), 400*time.Millisecond, "consumer should be occupied across the first retry backoff")
	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for fast job")
	}
}
