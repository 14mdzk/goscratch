package email

import (
	"context"
	"testing"

	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoOpSender_Send(t *testing.T) {
	log := logger.New(logger.Config{
		Level:  "debug",
		Format: "json",
	})

	sender := NewNoOpSender(log)

	t.Run("sends without error", func(t *testing.T) {
		msg := port.EmailMessage{
			To:      []string{"user@example.com"},
			Subject: "Test Subject",
			Body:    "Test Body",
			IsHTML:  false,
		}

		err := sender.Send(context.Background(), msg)
		assert.NoError(t, err)
	})

	t.Run("handles HTML email", func(t *testing.T) {
		msg := port.EmailMessage{
			To:      []string{"user@example.com"},
			CC:      []string{"cc@example.com"},
			BCC:     []string{"bcc@example.com"},
			Subject: "HTML Test",
			Body:    "<h1>Hello</h1>",
			IsHTML:  true,
		}

		err := sender.Send(context.Background(), msg)
		assert.NoError(t, err)
	})

	t.Run("handles empty recipients", func(t *testing.T) {
		msg := port.EmailMessage{
			To:      []string{},
			Subject: "Test",
			Body:    "Body",
		}

		err := sender.Send(context.Background(), msg)
		assert.NoError(t, err)
	})
}

func TestNoOpSender_Close(t *testing.T) {
	log := logger.New(logger.Config{
		Level:  "debug",
		Format: "json",
	})

	sender := NewNoOpSender(log)
	err := sender.Close()
	assert.NoError(t, err)
}

func TestSMTPSender_Send_ValidationError(t *testing.T) {
	sender := NewSMTPSender(SMTPConfig{
		Host:     "localhost",
		Port:     587,
		Username: "test",
		Password: "test",
		From:     "test@example.com",
	})

	t.Run("returns error for empty recipients", func(t *testing.T) {
		msg := port.EmailMessage{
			To:      []string{},
			Subject: "Test",
			Body:    "Body",
		}

		err := sender.Send(context.Background(), msg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "email recipient is required")
	})
}

func TestSMTPSender_Close(t *testing.T) {
	sender := NewSMTPSender(SMTPConfig{
		Host: "localhost",
		Port: 587,
	})

	err := sender.Close()
	assert.NoError(t, err)
}

func TestSMTPSender_Send_ConnectionError(t *testing.T) {
	// Use an invalid SMTP server address to test error handling
	sender := NewSMTPSender(SMTPConfig{
		Host:     "invalid-host-that-does-not-exist",
		Port:     12345,
		Username: "test",
		Password: "test",
		From:     "test@example.com",
	})

	msg := port.EmailMessage{
		To:      []string{"user@example.com"},
		Subject: "Test",
		Body:    "Body",
	}

	err := sender.Send(context.Background(), msg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send email via SMTP")
}

// TestEmailSenderInterface verifies both adapters satisfy the port interface
func TestEmailSenderInterface(t *testing.T) {
	log := logger.New(logger.Config{
		Level:  "debug",
		Format: "json",
	})

	var _ port.EmailSender = NewNoOpSender(log)
	var _ port.EmailSender = NewSMTPSender(SMTPConfig{})
}
