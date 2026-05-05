package queue

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Fakes for amqpConnection / amqpChannel
// =============================================================================

type fakeConn struct {
	mu        sync.Mutex
	channels  []*fakeChannel
	closed    bool
	notifyCh  chan *amqp.Error
	failOpen  bool
	openCount int32
}

func newFakeConn() *fakeConn {
	return &fakeConn{}
}

func (c *fakeConn) Channel() (amqpChannel, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failOpen {
		return nil, assertErr("channel open failed")
	}
	atomic.AddInt32(&c.openCount, 1)
	ch := newFakeChannel()
	c.channels = append(c.channels, ch)
	return ch, nil
}

func (c *fakeConn) NotifyClose(ch chan *amqp.Error) chan *amqp.Error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.notifyCh = ch
	return ch
}

func (c *fakeConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func (c *fakeConn) IsClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func (c *fakeConn) ChannelCount() int {
	return int(atomic.LoadInt32(&c.openCount))
}

type fakeChannel struct {
	mu               sync.Mutex
	deliveries       chan amqp.Delivery
	closeNotify      chan *amqp.Error
	qosPrefetch      int
	qosCalled        bool
	consumeCalled    bool
	qosBeforeConsume bool
	closed           bool
	publishCount     int
}

func newFakeChannel() *fakeChannel {
	return &fakeChannel{
		deliveries: make(chan amqp.Delivery, 1),
	}
}

func (c *fakeChannel) PublishWithContext(_ context.Context, _, _ string, _, _ bool, _ amqp.Publishing) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.publishCount++
	return nil
}

func (c *fakeChannel) Consume(_, _ string, _, _, _, _ bool, _ amqp.Table) (<-chan amqp.Delivery, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.consumeCalled = true
	if c.qosCalled {
		c.qosBeforeConsume = true
	}
	return c.deliveries, nil
}

func (c *fakeChannel) QueueDeclare(_ string, _, _, _, _ bool, _ amqp.Table) (amqp.Queue, error) {
	return amqp.Queue{}, nil
}

func (c *fakeChannel) ExchangeDeclare(_, _ string, _, _, _, _ bool, _ amqp.Table) error {
	return nil
}

func (c *fakeChannel) QueueBind(_, _, _ string, _ bool, _ amqp.Table) error {
	return nil
}

func (c *fakeChannel) Qos(prefetchCount, _ int, _ bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.qosCalled = true
	c.qosPrefetch = prefetchCount
	return nil
}

func (c *fakeChannel) NotifyClose(ch chan *amqp.Error) chan *amqp.Error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeNotify = ch
	return ch
}

func (c *fakeChannel) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	close(c.deliveries)
	return nil
}

// triggerClose simulates the broker tearing down the channel.
func (c *fakeChannel) triggerClose(err *amqp.Error) {
	c.mu.Lock()
	notify := c.closeNotify
	c.mu.Unlock()
	if notify != nil {
		select {
		case notify <- err:
		default:
		}
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }

// =============================================================================
// Tests
// =============================================================================

func TestRabbitMQ_PublishAndConsume_OpenDistinctChannels(t *testing.T) {
	conn := newFakeConn()
	dial := func(url string) (amqpConnection, error) { return conn, nil }

	q, err := newRabbitMQ("amqp://test", Options{PrefetchCount: 7}, dial)
	require.NoError(t, err)
	defer q.Close()

	// One channel opened in constructor for publisher.
	assert.Equal(t, 1, conn.ChannelCount())

	// Publish reuses publisher channel — no new channel.
	require.NoError(t, q.Publish(context.Background(), "ex", "rk", []byte("x")))
	assert.Equal(t, 1, conn.ChannelCount())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, q.Consume(ctx, "queue", func(b []byte) error { return nil }))

	// Consume opens a second, distinct channel.
	assert.Equal(t, 2, conn.ChannelCount())
	assert.NotSame(t, conn.channels[0], conn.channels[1], "publisher and consumer must use separate channels")
}

func TestRabbitMQ_Consume_QosBeforeConsume(t *testing.T) {
	conn := newFakeConn()
	dial := func(url string) (amqpConnection, error) { return conn, nil }

	q, err := newRabbitMQ("amqp://test", Options{PrefetchCount: 42}, dial)
	require.NoError(t, err)
	defer q.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, q.Consume(ctx, "queue", func(b []byte) error { return nil }))

	consumerCh := conn.channels[1]
	consumerCh.mu.Lock()
	defer consumerCh.mu.Unlock()
	assert.True(t, consumerCh.qosCalled, "Qos must be called on consumer channel")
	assert.True(t, consumerCh.consumeCalled, "Consume must be called")
	assert.True(t, consumerCh.qosBeforeConsume, "Qos must be called before Consume")
	assert.Equal(t, 42, consumerCh.qosPrefetch)
}

func TestRabbitMQ_PrefetchDefault(t *testing.T) {
	conn := newFakeConn()
	dial := func(url string) (amqpConnection, error) { return conn, nil }

	// Empty options -> default 10.
	q, err := newRabbitMQ("amqp://test", Options{}, dial)
	require.NoError(t, err)
	defer q.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, q.Consume(ctx, "queue", func(b []byte) error { return nil }))

	consumerCh := conn.channels[1]
	consumerCh.mu.Lock()
	defer consumerCh.mu.Unlock()
	assert.Equal(t, defaultPrefetchCount, consumerCh.qosPrefetch)
}

func TestRabbitMQ_NotifyClose_TriggersReconnect(t *testing.T) {
	conn := newFakeConn()
	dial := func(url string) (amqpConnection, error) { return conn, nil }

	q, err := newRabbitMQ("amqp://test", Options{
		PrefetchCount:      5,
		MaxReconnectTries:  3,
		ReconnectBaseDelay: time.Millisecond,
		ReconnectMaxDelay:  5 * time.Millisecond,
	}, dial)
	require.NoError(t, err)
	defer q.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, q.Consume(ctx, "queue", func(b []byte) error { return nil }))

	// 1 publisher + 1 initial consumer.
	require.Equal(t, 2, conn.ChannelCount())

	// Trigger broker close on the consumer's channel.
	consumerCh := conn.channels[1]
	consumerCh.triggerClose(&amqp.Error{Code: 320, Reason: "CONNECTION_FORCED"})

	// Wait for the reconnect to open a new consumer channel.
	require.Eventually(t, func() bool {
		return conn.ChannelCount() >= 3
	}, time.Second, 5*time.Millisecond, "expected reconnect to open a new consumer channel")
}

func TestRabbitMQ_Reconnect_HaltsOnContextCancel(t *testing.T) {
	conn := newFakeConn()
	dial := func(url string) (amqpConnection, error) { return conn, nil }

	q, err := newRabbitMQ("amqp://test", Options{
		PrefetchCount:      5,
		MaxReconnectTries:  100, // would loop a long time
		ReconnectBaseDelay: 50 * time.Millisecond,
		ReconnectMaxDelay:  100 * time.Millisecond,
	}, dial)
	require.NoError(t, err)
	defer q.Close()

	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, q.Consume(ctx, "queue", func(b []byte) error { return nil }))

	// Force open to fail so reconnect must keep retrying.
	conn.mu.Lock()
	conn.failOpen = true
	conn.mu.Unlock()

	// Trigger close so the consumer enters the reconnect loop.
	consumerCh := conn.channels[1]
	consumerCh.triggerClose(&amqp.Error{Code: 320, Reason: "CONNECTION_FORCED"})

	// Cancel quickly; reconnect loop must observe ctx.Done() and exit.
	time.Sleep(20 * time.Millisecond)
	cancel()

	// Give the goroutine a moment; channel count should not blow past a small number.
	time.Sleep(150 * time.Millisecond)
	// Should not have opened anywhere near 100 channels — ctx cancelled before
	// retries exhaust. We allow a small slack for timing.
	assert.Less(t, conn.ChannelCount(), 10, "ctx cancellation must halt reconnect attempts")
}

func TestRabbitMQ_Close_Idempotent(t *testing.T) {
	conn := newFakeConn()
	dial := func(url string) (amqpConnection, error) { return conn, nil }

	q, err := newRabbitMQ("amqp://test", Options{}, dial)
	require.NoError(t, err)
	require.NoError(t, q.Close())
	require.NoError(t, q.Close())

	// Publish after close must error rather than panic on a nil channel.
	err = q.Publish(context.Background(), "ex", "rk", []byte("x"))
	assert.Error(t, err)
}
