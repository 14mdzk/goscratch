package queue

import (
	"context"
	"testing"
	"time"

	"github.com/14mdzk/goscratch/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// NoOpQueue Tests
// =============================================================================

func TestNoOpQueue_ImplementsInterface(t *testing.T) {
	var _ port.Queue = (*NoOpQueue)(nil)
}

func TestNoOpQueue_Publish_ReturnsNil(t *testing.T) {
	q := NewNoOpQueue()
	err := q.Publish(context.Background(), "exchange", "key", []byte("body"))
	assert.NoError(t, err)
}

func TestNoOpQueue_PublishJSON_ReturnsNil(t *testing.T) {
	q := NewNoOpQueue()
	err := q.PublishJSON(context.Background(), "exchange", "key", map[string]string{"a": "b"})
	assert.NoError(t, err)
}

func TestNoOpQueue_Consume_BlocksUntilContextDone(t *testing.T) {
	q := NewNoOpQueue()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := q.Consume(ctx, "test-queue", func(body []byte) error {
		t.Fatal("handler should never be called in NoOpQueue")
		return nil
	})
	elapsed := time.Since(start)

	// Should have blocked until context timed out
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.GreaterOrEqual(t, elapsed, 40*time.Millisecond)
}

func TestNoOpQueue_Consume_CancelledContext(t *testing.T) {
	q := NewNoOpQueue()

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- q.Consume(ctx, "test-queue", func(body []byte) error { return nil })
	}()

	// Cancel after a short delay
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("Consume did not return after context cancellation")
	}
}

func TestNoOpQueue_DeclareQueue_ReturnsNil(t *testing.T) {
	q := NewNoOpQueue()
	err := q.DeclareQueue(context.Background(), "queue", true)
	assert.NoError(t, err)
}

func TestNoOpQueue_DeclareExchange_ReturnsNil(t *testing.T) {
	q := NewNoOpQueue()
	err := q.DeclareExchange(context.Background(), "exchange", "topic", true)
	assert.NoError(t, err)
}

func TestNoOpQueue_BindQueue_ReturnsNil(t *testing.T) {
	q := NewNoOpQueue()
	err := q.BindQueue(context.Background(), "queue", "exchange", "key")
	assert.NoError(t, err)
}

func TestNoOpQueue_Close_ReturnsNil(t *testing.T) {
	q := NewNoOpQueue()
	err := q.Close()
	assert.NoError(t, err)
}

// =============================================================================
// RabbitMQ Tests (constructor validation only - no real connection)
// =============================================================================

func TestNewRabbitMQ_InvalidURL_ReturnsError(t *testing.T) {
	// Attempting to connect to an invalid URL should fail
	_, err := NewRabbitMQ("amqp://nonexistent:5672/")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to rabbitmq")
}

// =============================================================================
// NoOpQueue Table-Driven Tests
// =============================================================================

func TestNoOpQueue_AllMethodsNoOp(t *testing.T) {
	q := NewNoOpQueue()
	ctx := context.Background()

	tests := []struct {
		name string
		fn   func() error
	}{
		{"Publish", func() error { return q.Publish(ctx, "", "", nil) }},
		{"PublishJSON", func() error { return q.PublishJSON(ctx, "", "", nil) }},
		{"DeclareQueue", func() error { return q.DeclareQueue(ctx, "q", false) }},
		{"DeclareExchange", func() error { return q.DeclareExchange(ctx, "ex", "direct", false) }},
		{"BindQueue", func() error { return q.BindQueue(ctx, "q", "ex", "rk") }},
		{"Close", func() error { return q.Close() }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			assert.NoError(t, err)
		})
	}
}

func TestNoOpQueue_Publish_WithNilBody(t *testing.T) {
	q := NewNoOpQueue()
	err := q.Publish(context.Background(), "", "", nil)
	assert.NoError(t, err)
}

func TestNoOpQueue_PublishJSON_WithNilMessage(t *testing.T) {
	q := NewNoOpQueue()
	err := q.PublishJSON(context.Background(), "", "", nil)
	assert.NoError(t, err)
}

func TestNoOpQueue_Close_Idempotent(t *testing.T) {
	q := NewNoOpQueue()
	assert.NoError(t, q.Close())
	assert.NoError(t, q.Close())
}
