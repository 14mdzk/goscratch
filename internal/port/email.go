package port

import "context"

// EmailSender defines the interface for sending emails
type EmailSender interface {
	// Send sends an email message
	Send(ctx context.Context, msg EmailMessage) error
	// Close cleans up any resources
	Close() error
}

// EmailMessage represents an email to be sent
type EmailMessage struct {
	To      []string `json:"to"`
	CC      []string `json:"cc,omitempty"`
	BCC     []string `json:"bcc,omitempty"`
	Subject string   `json:"subject"`
	Body    string   `json:"body"`
	IsHTML  bool     `json:"is_html,omitempty"`
}
