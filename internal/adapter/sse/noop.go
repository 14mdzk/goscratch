package sse

import "github.com/14mdzk/goscratch/internal/port"

// NoOpBroker implements port.SSEBroker as a no-op
// Used when SSE is disabled
type NoOpBroker struct{}

// NewNoOpBroker creates a new no-op SSE broker
func NewNoOpBroker() *NoOpBroker {
	return &NoOpBroker{}
}

func (b *NoOpBroker) Subscribe(clientID string, topics ...string) <-chan port.Event {
	// Return a closed channel
	ch := make(chan port.Event)
	close(ch)
	return ch
}

func (b *NoOpBroker) Unsubscribe(clientID string) {}

func (b *NoOpBroker) Broadcast(event port.Event) {}

func (b *NoOpBroker) BroadcastToTopic(topic string, event port.Event) {}

func (b *NoOpBroker) SendTo(clientID string, event port.Event) {}

func (b *NoOpBroker) ClientCount() int {
	return 0
}

func (b *NoOpBroker) Close() error {
	return nil
}
