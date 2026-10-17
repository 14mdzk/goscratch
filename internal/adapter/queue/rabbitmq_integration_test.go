//go:build integration

package queue_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/14mdzk/goscratch/internal/adapter/queue"
	"github.com/14mdzk/goscratch/internal/platform/testutil"
)

func TestRabbitMQ_LiveBroker_PublishConsume(t *testing.T) {
	ctx := context.Background()

	url, cleanup, err := testutil.StartRabbitMQ(ctx)
	require.NoError(t, err)
	defer cleanup()
	assert.NotEmpty(t, url)

	q, err := queue.NewRabbitMQ(url)
	require.NoError(t, err)
	defer func() { _ = q.Close() }()

	require.NoError(t, q.Ping(ctx))

	const queueName = "integration.test"
	require.NoError(t, q.DeclareQueue(ctx, queueName, false))

	received := make(chan []byte, 1)
	consumeCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, q.Consume(consumeCtx, queueName, func(body []byte) error {
		received <- body
		return nil
	}))

	body := []byte(`{"hello":"broker"}`)
	require.NoError(t, q.Publish(context.Background(), "", queueName, body))

	select {
	case got := <-received:
		assert.Equal(t, body, got)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for message from live broker")
	}
}
