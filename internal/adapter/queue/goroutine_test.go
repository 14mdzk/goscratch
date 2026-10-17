package queue

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/14mdzk/goscratch/internal/port"
)

func TestGoroutineQueue_ImplementsInterface(t *testing.T) {
	var _ port.Queue = (*GoroutineQueue)(nil)
}

func TestGoroutineQueue_PublishConsumeRoundTrip(t *testing.T) {
	q := NewGoroutineQueue()
	defer q.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	received := make(chan []byte, 4)
	consumeErr := make(chan error, 1)
	go func() {
		consumeErr <- q.Consume(ctx, "jobs", func(body []byte) error {
			received <- body
			return nil
		})
	}()

	for _, want := range [][]byte{[]byte("one"), []byte("two"), []byte("three")} {
		require.NoError(t, q.Publish(ctx, "", "jobs", want))
	}

	for _, want := range []string{"one", "two", "three"} {
		select {
		case got := <-received:
			assert.Equal(t, want, string(got))
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for message %q", want)
		}
	}
}

func TestGoroutineQueue_PublishJSONRoundTrip(t *testing.T) {
	q := NewGoroutineQueue()
	defer q.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	received := make(chan []byte, 1)
	go q.Consume(ctx, "events", func(body []byte) error {
		received <- body
		return nil
	})

	require.NoError(t, q.PublishJSON(ctx, "", "events", map[string]string{"a": "b"}))
	select {
	case got := <-received:
		assert.JSONEq(t, `{"a":"b"}`, string(got))
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for JSON message")
	}
}

func TestGoroutineQueue_ConsumeSpreadsAcrossCallers(t *testing.T) {
	q := NewGoroutineQueue()
	defer q.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const callers = 3
	const total = 30
	var mu sync.Mutex
	seen := map[string]int{}
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = q.Consume(ctx, "jobs", func(body []byte) error {
				mu.Lock()
				seen[string(body)]++
				mu.Unlock()
				return nil
			})
		}()
	}

	for i := 0; i < total; i++ {
		require.NoError(t, q.Publish(ctx, "", "jobs", []byte{byte(i)}))
	}

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		n := 0
		for _, c := range seen {
			n += c
		}
		return n == total
	}, 5*time.Second, 10*time.Millisecond, "every message must be delivered exactly once")

	mu.Lock()
	defer mu.Unlock()
	for i := 0; i < total; i++ {
		assert.Equal(t, 1, seen[string([]byte{byte(i)})], "message delivered more than once")
	}
}

func TestGoroutineQueue_HandlerErrorDropsAndContinues(t *testing.T) {
	q := NewGoroutineQueue()
	defer q.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	received := make(chan []byte, 2)
	go q.Consume(ctx, "jobs", func(body []byte) error {
		if string(body) == "bad" {
			return assert.AnError
		}
		received <- body
		return nil
	})

	require.NoError(t, q.Publish(ctx, "", "jobs", []byte("bad")))
	require.NoError(t, q.Publish(ctx, "", "jobs", []byte("good")))

	select {
	case got := <-received:
		assert.Equal(t, "good", string(got))
	case <-time.After(2 * time.Second):
		t.Fatal("consumer stalled after handler error")
	}
}

func TestGoroutineQueue_CloseUnblocksConsumer(t *testing.T) {
	q := NewGoroutineQueue()

	ctx := context.Background()
	done := make(chan error, 1)
	go func() {
		done <- q.Consume(ctx, "jobs", func(body []byte) error { return nil })
	}()

	time.Sleep(50 * time.Millisecond)
	require.NoError(t, q.Close())

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Consume did not return after Close")
	}

	assert.Error(t, q.Publish(ctx, "", "jobs", []byte("late")))
	assert.Error(t, q.Ping(ctx))
	assert.NoError(t, q.Close(), "Close must be idempotent")
}

func TestGoroutineQueue_ContextCancelUnblocksConsumer(t *testing.T) {
	q := NewGoroutineQueue()
	defer q.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- q.Consume(ctx, "jobs", func(body []byte) error { return nil })
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("Consume did not return after context cancel")
	}
}

func TestGoroutineQueue_Ping(t *testing.T) {
	q := NewGoroutineQueue()
	assert.NoError(t, q.Ping(context.Background()))
	require.NoError(t, q.Close())
	assert.Error(t, q.Ping(context.Background()))
}
