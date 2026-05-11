package worker

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockQueue implements port.Queue for testing
type mockQueue struct {
	mu           sync.Mutex
	publishCalls []publishCall
	publishErr   error
}

type publishCall struct {
	exchange   string
	routingKey string
	body       []byte
}

func (m *mockQueue) Publish(_ context.Context, exchange, routingKey string, body []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.publishCalls = append(m.publishCalls, publishCall{
		exchange:   exchange,
		routingKey: routingKey,
		body:       body,
	})
	return m.publishErr
}

func (m *mockQueue) PublishJSON(_ context.Context, _, _ string, _ any) error { return nil }
func (m *mockQueue) Consume(_ context.Context, _ string, _ func(body []byte) error) error {
	return nil
}
func (m *mockQueue) Ping(_ context.Context) error                                 { return nil }
func (m *mockQueue) DeclareQueue(_ context.Context, _ string, _ bool) error       { return nil }
func (m *mockQueue) DeclareExchange(_ context.Context, _, _ string, _ bool) error { return nil }
func (m *mockQueue) BindQueue(_ context.Context, _, _, _ string) error            { return nil }
func (m *mockQueue) Close() error                                                 { return nil }

func (m *mockQueue) lastCall() publishCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.publishCalls[len(m.publishCalls)-1]
}

func TestNewPublisher(t *testing.T) {
	t.Run("with_custom_queue_name", func(t *testing.T) {
		q := &mockQueue{}
		pub := NewPublisher(q, "custom-queue", "custom-exchange")
		assert.Equal(t, "custom-queue", pub.queueName)
		assert.Equal(t, "custom-exchange", pub.exchange)
	})

	t.Run("default_queue_name", func(t *testing.T) {
		q := &mockQueue{}
		pub := NewPublisher(q, "", "ex")
		assert.Equal(t, "jobs", pub.queueName)
	})
}

func TestPublisher_Publish(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		q := &mockQueue{}
		pub := NewPublisher(q, "test-queue", "test-exchange")

		err := pub.Publish(context.Background(), "email.send", map[string]string{"to": "a@b.com"})
		require.NoError(t, err)

		call := q.lastCall()
		assert.Equal(t, "test-exchange", call.exchange)
		assert.Equal(t, "test-queue", call.routingKey)
		assert.NotEmpty(t, call.body)

		// Verify body is a valid job
		job, err := DecodeJob(call.body)
		require.NoError(t, err)
		assert.Equal(t, "email.send", job.Type)
		assert.Equal(t, 3, job.MaxRetry)
	})

	t.Run("queue_error", func(t *testing.T) {
		q := &mockQueue{publishErr: errors.New("connection refused")}
		pub := NewPublisher(q, "q", "ex")

		err := pub.Publish(context.Background(), "test", map[string]string{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to publish job")
	})

	t.Run("unmarshalable_payload", func(t *testing.T) {
		q := &mockQueue{}
		pub := NewPublisher(q, "q", "ex")

		err := pub.Publish(context.Background(), "test", make(chan int))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create job")
	})
}

func TestPublisher_PublishWithRetry(t *testing.T) {
	t.Run("custom_retry", func(t *testing.T) {
		q := &mockQueue{}
		pub := NewPublisher(q, "q", "ex")

		err := pub.PublishWithRetry(context.Background(), "email.send", map[string]string{"to": "x"}, 7)
		require.NoError(t, err)

		job, err := DecodeJob(q.lastCall().body)
		require.NoError(t, err)
		assert.Equal(t, 7, job.MaxRetry)
	})

	t.Run("queue_error", func(t *testing.T) {
		q := &mockQueue{publishErr: errors.New("fail")}
		pub := NewPublisher(q, "q", "ex")

		err := pub.PublishWithRetry(context.Background(), "test", "payload", 5)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to publish job")
	})
}

func TestPublisher_PublishRaw(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		q := &mockQueue{}
		pub := NewPublisher(q, "q", "ex")

		job, err := NewJob("raw.test", map[string]string{"key": "val"})
		require.NoError(t, err)

		err = pub.PublishRaw(context.Background(), job)
		require.NoError(t, err)

		decoded, err := DecodeJob(q.lastCall().body)
		require.NoError(t, err)
		assert.Equal(t, job.ID, decoded.ID)
		assert.Equal(t, "raw.test", decoded.Type)
	})

	t.Run("queue_error", func(t *testing.T) {
		q := &mockQueue{publishErr: errors.New("fail")}
		pub := NewPublisher(q, "q", "ex")

		job, _ := NewJob("test", "data")
		err := pub.PublishRaw(context.Background(), job)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to publish job")
	})
}
