package notification

import (
	"context"

	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/pkg/logger"
)

// NoOpSender implements port.NotificationSender as a no-op.
// Used in development when the notification system is disabled.
type NoOpSender struct {
	logger *logger.Logger
}

// NewNoOpSender creates a new no-op notification sender.
func NewNoOpSender(log *logger.Logger) *NoOpSender {
	return &NoOpSender{logger: log}
}

// Send logs the notification details without delivering.
func (s *NoOpSender) Send(ctx context.Context, msg port.NotificationMessage) error {
	s.logger.Info("NoOp notification send",
		"user_id", msg.UserID,
		"title", msg.Title,
		"channel", msg.Channel,
		"body_length", len(msg.Body),
	)
	return nil
}

// Close cleans up resources (no-op).
func (s *NoOpSender) Close() error {
	return nil
}
