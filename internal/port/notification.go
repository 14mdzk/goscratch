package port

import "context"

// NotificationSender defines the interface for sending notifications to users.
// Implementations may deliver via webhook, log, or external service.
type NotificationSender interface {
	// Send delivers a notification message to the target user.
	Send(ctx context.Context, msg NotificationMessage) error
	// Close cleans up any resources held by the sender.
	Close() error
}

// NotificationMessage represents a notification to be delivered.
type NotificationMessage struct {
	UserID  string `json:"user_id"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	Channel string `json:"channel,omitempty"` // e.g. "in_app", "webhook" — adapter-specific
}
