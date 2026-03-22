package sse

import (
	"sync"
	"testing"
	"time"

	"github.com/14mdzk/goscratch/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// NoOpBroker Tests
// =============================================================================

func TestNoOpBroker_ImplementsInterface(t *testing.T) {
	var _ port.SSEBroker = (*NoOpBroker)(nil)
}

func TestNoOpBroker_Subscribe_ReturnsClosedChannel(t *testing.T) {
	b := NewNoOpBroker()
	ch := b.Subscribe("client1", "topic1")

	// A closed channel should return zero value immediately
	event, ok := <-ch
	assert.False(t, ok, "channel should be closed")
	assert.Equal(t, port.Event{}, event)
}

func TestNoOpBroker_Unsubscribe_NoOp(t *testing.T) {
	b := NewNoOpBroker()
	// Should not panic
	b.Unsubscribe("nonexistent")
}

func TestNoOpBroker_Broadcast_NoOp(t *testing.T) {
	b := NewNoOpBroker()
	// Should not panic
	b.Broadcast(port.Event{Data: []byte("test")})
}

func TestNoOpBroker_BroadcastToTopic_NoOp(t *testing.T) {
	b := NewNoOpBroker()
	// Should not panic
	b.BroadcastToTopic("topic", port.Event{Data: []byte("test")})
}

func TestNoOpBroker_SendTo_NoOp(t *testing.T) {
	b := NewNoOpBroker()
	// Should not panic
	b.SendTo("client1", port.Event{Data: []byte("test")})
}

func TestNoOpBroker_ClientCount_ReturnsZero(t *testing.T) {
	b := NewNoOpBroker()
	assert.Equal(t, 0, b.ClientCount())
}

func TestNoOpBroker_Close_ReturnsNil(t *testing.T) {
	b := NewNoOpBroker()
	assert.NoError(t, b.Close())
}

// =============================================================================
// Broker Tests
// =============================================================================

func TestBroker_ImplementsInterface(t *testing.T) {
	var _ port.SSEBroker = (*Broker)(nil)
}

func TestBroker_NewBroker_DefaultBufferSize(t *testing.T) {
	b := NewBroker(0)
	assert.Equal(t, 100, b.bufferSize)
}

func TestBroker_NewBroker_NegativeBufferSize(t *testing.T) {
	b := NewBroker(-5)
	assert.Equal(t, 100, b.bufferSize)
}

func TestBroker_NewBroker_CustomBufferSize(t *testing.T) {
	b := NewBroker(50)
	assert.Equal(t, 50, b.bufferSize)
}

func TestBroker_Subscribe_ReceiveEvents(t *testing.T) {
	b := NewBroker(10)
	defer b.Close()

	ch := b.Subscribe("client1")

	event := port.Event{Event: "test", Data: []byte("hello")}
	b.SendTo("client1", event)

	select {
	case received := <-ch:
		assert.Equal(t, event, received)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestBroker_Unsubscribe_RemovesClient(t *testing.T) {
	b := NewBroker(10)
	defer b.Close()

	b.Subscribe("client1")
	assert.Equal(t, 1, b.ClientCount())

	b.Unsubscribe("client1")
	assert.Equal(t, 0, b.ClientCount())
}

func TestBroker_Unsubscribe_ClosesChannel(t *testing.T) {
	b := NewBroker(10)
	defer b.Close()

	ch := b.Subscribe("client1")
	b.Unsubscribe("client1")

	// Channel should be closed
	_, ok := <-ch
	assert.False(t, ok)
}

func TestBroker_Unsubscribe_NonexistentClient(t *testing.T) {
	b := NewBroker(10)
	defer b.Close()

	// Should not panic
	b.Unsubscribe("nonexistent")
}

func TestBroker_Broadcast_SendsToAllClients(t *testing.T) {
	b := NewBroker(10)
	defer b.Close()

	ch1 := b.Subscribe("client1")
	ch2 := b.Subscribe("client2")
	ch3 := b.Subscribe("client3")

	event := port.Event{Event: "broadcast", Data: []byte("hello all")}
	b.Broadcast(event)

	for i, ch := range []<-chan port.Event{ch1, ch2, ch3} {
		select {
		case received := <-ch:
			assert.Equal(t, event, received, "client %d should receive the event", i+1)
		case <-time.After(time.Second):
			t.Fatalf("client %d timed out waiting for broadcast", i+1)
		}
	}
}

func TestBroker_BroadcastToTopic_OnlySubscribedClients(t *testing.T) {
	b := NewBroker(10)
	defer b.Close()

	chNews := b.Subscribe("news-client", "news")
	chSports := b.Subscribe("sports-client", "sports")
	chAll := b.Subscribe("all-client") // no specific topics, gets everything

	event := port.Event{Event: "update", Data: []byte("news update")}
	b.BroadcastToTopic("news", event)

	// news-client should receive
	select {
	case received := <-chNews:
		assert.Equal(t, event, received)
	case <-time.After(time.Second):
		t.Fatal("news client should receive topic event")
	}

	// all-client (no topics) should receive
	select {
	case received := <-chAll:
		assert.Equal(t, event, received)
	case <-time.After(time.Second):
		t.Fatal("all-topic client should receive topic event")
	}

	// sports-client should NOT receive
	select {
	case <-chSports:
		t.Fatal("sports client should not receive news event")
	case <-time.After(50 * time.Millisecond):
		// Expected: no event
	}
}

func TestBroker_SendTo_SpecificClient(t *testing.T) {
	b := NewBroker(10)
	defer b.Close()

	ch1 := b.Subscribe("client1")
	ch2 := b.Subscribe("client2")

	event := port.Event{Event: "dm", Data: []byte("for client1 only")}
	b.SendTo("client1", event)

	select {
	case received := <-ch1:
		assert.Equal(t, event, received)
	case <-time.After(time.Second):
		t.Fatal("client1 should receive the event")
	}

	select {
	case <-ch2:
		t.Fatal("client2 should not receive the event")
	case <-time.After(50 * time.Millisecond):
		// Expected
	}
}

func TestBroker_SendTo_NonexistentClient(t *testing.T) {
	b := NewBroker(10)
	defer b.Close()

	// Should not panic
	b.SendTo("ghost", port.Event{Data: []byte("hello?")})
}

func TestBroker_ClientCount(t *testing.T) {
	b := NewBroker(10)
	defer b.Close()

	assert.Equal(t, 0, b.ClientCount())

	b.Subscribe("c1")
	assert.Equal(t, 1, b.ClientCount())

	b.Subscribe("c2")
	assert.Equal(t, 2, b.ClientCount())

	b.Subscribe("c3")
	assert.Equal(t, 3, b.ClientCount())

	b.Unsubscribe("c2")
	assert.Equal(t, 2, b.ClientCount())

	b.Unsubscribe("c1")
	b.Unsubscribe("c3")
	assert.Equal(t, 0, b.ClientCount())
}

func TestBroker_Close_ShutsDownGracefully(t *testing.T) {
	b := NewBroker(10)

	ch1 := b.Subscribe("c1")
	ch2 := b.Subscribe("c2")

	err := b.Close()
	require.NoError(t, err)

	// All client channels should be closed
	_, ok1 := <-ch1
	assert.False(t, ok1, "ch1 should be closed after Close()")

	_, ok2 := <-ch2
	assert.False(t, ok2, "ch2 should be closed after Close()")

	// Client count should be zero
	assert.Equal(t, 0, b.ClientCount())
}

func TestBroker_Broadcast_FullChannel_DoesNotBlock(t *testing.T) {
	b := NewBroker(1) // Very small buffer
	defer b.Close()

	b.Subscribe("slow-client")

	// Fill the buffer
	b.Broadcast(port.Event{Data: []byte("msg1")})

	// This should not block even though buffer is full
	done := make(chan struct{})
	go func() {
		b.Broadcast(port.Event{Data: []byte("msg2")})
		close(done)
	}()

	select {
	case <-done:
		// Good - broadcast did not block
	case <-time.After(time.Second):
		t.Fatal("Broadcast blocked on full channel")
	}
}

func TestBroker_ConcurrencySafety(t *testing.T) {
	b := NewBroker(100)
	defer b.Close()

	var wg sync.WaitGroup
	numGoroutines := 20

	// Concurrent subscribes
	wg.Add(numGoroutines)
	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()
			clientID := "client-" + string(rune('A'+id))
			b.Subscribe(clientID)
		}(i)
	}
	wg.Wait()

	assert.Equal(t, numGoroutines, b.ClientCount())

	// Concurrent broadcasts
	wg.Add(numGoroutines)
	for i := range numGoroutines {
		go func() {
			defer wg.Done()
			b.Broadcast(port.Event{Data: []byte("concurrent")})
		}()
		_ = i
	}
	wg.Wait()

	// Concurrent unsubscribes
	wg.Add(numGoroutines)
	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()
			clientID := "client-" + string(rune('A'+id))
			b.Unsubscribe(clientID)
		}(i)
	}
	wg.Wait()

	assert.Equal(t, 0, b.ClientCount())
}

func TestBroker_Subscribe_ReplacesExistingClient(t *testing.T) {
	b := NewBroker(10)
	defer b.Close()

	ch1 := b.Subscribe("client1")
	ch2 := b.Subscribe("client1") // Same ID

	// Should still be 1 client
	assert.Equal(t, 1, b.ClientCount())

	// The new channel should work
	b.SendTo("client1", port.Event{Data: []byte("test")})

	select {
	case received := <-ch2:
		assert.Equal(t, []byte("test"), received.Data)
	case <-time.After(time.Second):
		t.Fatal("new channel should receive event")
	}

	// Old channel may or may not be closed depending on implementation
	// Just verify no panic
	_ = ch1
}

func TestBroker_BroadcastToTopic_MultipleTopics(t *testing.T) {
	b := NewBroker(10)
	defer b.Close()

	ch := b.Subscribe("multi-topic", "news", "sports")

	// Should receive news events
	b.BroadcastToTopic("news", port.Event{Data: []byte("news")})
	select {
	case received := <-ch:
		assert.Equal(t, []byte("news"), received.Data)
	case <-time.After(time.Second):
		t.Fatal("should receive news event")
	}

	// Should also receive sports events
	b.BroadcastToTopic("sports", port.Event{Data: []byte("sports")})
	select {
	case received := <-ch:
		assert.Equal(t, []byte("sports"), received.Data)
	case <-time.After(time.Second):
		t.Fatal("should receive sports event")
	}
}

func TestBroker_ConcurrentBroadcastAndSubscribe(t *testing.T) {
	b := NewBroker(100)
	defer b.Close()

	var wg sync.WaitGroup

	// Concurrent subscribe + broadcast + unsubscribe
	for i := range 10 {
		wg.Add(3)

		go func(id int) {
			defer wg.Done()
			cid := "c" + string(rune('0'+id))
			b.Subscribe(cid)
		}(i)

		go func() {
			defer wg.Done()
			b.Broadcast(port.Event{Data: []byte("concurrent")})
		}()

		go func(id int) {
			defer wg.Done()
			cid := "old" + string(rune('0'+id))
			b.Unsubscribe(cid)
		}(i)
	}

	wg.Wait()
	// Should not panic - just verifying safety
}
