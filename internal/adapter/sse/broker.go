package sse

import (
	"sync"

	"github.com/14mdzk/goscratch/internal/port"
)

// Broker implements port.SSEBroker for Server-Sent Events
type Broker struct {
	mu         sync.RWMutex
	clients    map[string]clientInfo
	bufferSize int
}

type clientInfo struct {
	channel chan port.Event
	topics  map[string]struct{}
}

// NewBroker creates a new SSE broker
func NewBroker(bufferSize int) *Broker {
	if bufferSize <= 0 {
		bufferSize = 100
	}
	return &Broker{
		clients:    make(map[string]clientInfo),
		bufferSize: bufferSize,
	}
}

func (b *Broker) Subscribe(clientID string, topics ...string) <-chan port.Event {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan port.Event, b.bufferSize)
	topicSet := make(map[string]struct{})
	for _, t := range topics {
		topicSet[t] = struct{}{}
	}

	b.clients[clientID] = clientInfo{
		channel: ch,
		topics:  topicSet,
	}

	return ch
}

func (b *Broker) Unsubscribe(clientID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if info, ok := b.clients[clientID]; ok {
		close(info.channel)
		delete(b.clients, clientID)
	}
}

func (b *Broker) Broadcast(event port.Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, info := range b.clients {
		select {
		case info.channel <- event:
		default:
			// Channel full, skip this client
		}
	}
}

func (b *Broker) BroadcastToTopic(topic string, event port.Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, info := range b.clients {
		// Check if client is subscribed to this topic
		if _, subscribed := info.topics[topic]; !subscribed {
			// Also broadcast to clients with no specific topics (they get everything)
			if len(info.topics) > 0 {
				continue
			}
		}

		select {
		case info.channel <- event:
		default:
			// Channel full, skip
		}
	}
}

func (b *Broker) SendTo(clientID string, event port.Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if info, ok := b.clients[clientID]; ok {
		select {
		case info.channel <- event:
		default:
			// Channel full
		}
	}
}

func (b *Broker) ClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}

func (b *Broker) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	for clientID, info := range b.clients {
		close(info.channel)
		delete(b.clients, clientID)
	}

	return nil
}
