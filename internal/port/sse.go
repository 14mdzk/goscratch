package port

// SSEBroker defines the interface for Server-Sent Events
type SSEBroker interface {
	// Subscribe creates a new subscription for a client
	// Returns a channel that will receive events
	Subscribe(clientID string, topics ...string) <-chan Event

	// Unsubscribe removes a client's subscription
	Unsubscribe(clientID string)

	// Broadcast sends an event to all connected clients
	Broadcast(event Event)

	// BroadcastToTopic sends an event to clients subscribed to a topic
	BroadcastToTopic(topic string, event Event)

	// SendTo sends an event to a specific client
	SendTo(clientID string, event Event)

	// ClientCount returns the number of connected clients
	ClientCount() int

	// Close shuts down the broker
	Close() error
}

// Event represents an SSE event
type Event struct {
	ID    string `json:"id,omitempty"`
	Event string `json:"event,omitempty"` // Event type
	Data  []byte `json:"data"`
	Retry int    `json:"retry,omitempty"` // Reconnection time in milliseconds
}

// NewEvent creates a new SSE event
func NewEvent(eventType string, data []byte) Event {
	return Event{
		Event: eventType,
		Data:  data,
	}
}

// NewEventWithID creates a new SSE event with an ID
func NewEventWithID(id, eventType string, data []byte) Event {
	return Event{
		ID:    id,
		Event: eventType,
		Data:  data,
	}
}
