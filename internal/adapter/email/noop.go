package email

import (
	"context"

	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/pkg/logger"
)

// NoOpSender implements port.EmailSender as a no-op
// Used in development when email sending is disabled
type NoOpSender struct {
	logger *logger.Logger
}

// NewNoOpSender creates a new no-op email sender
func NewNoOpSender(log *logger.Logger) *NoOpSender {
	return &NoOpSender{
		logger: log,
	}
}

// Send logs the email details without actually sending
func (s *NoOpSender) Send(ctx context.Context, msg port.EmailMessage) error {
	s.logger.Info("NoOp email send",
		"to", msg.To,
		"cc", msg.CC,
		"bcc", msg.BCC,
		"subject", msg.Subject,
		"is_html", msg.IsHTML,
		"body_length", len(msg.Body),
	)
	return nil
}

// Close cleans up resources (no-op)
func (s *NoOpSender) Close() error {
	return nil
}
