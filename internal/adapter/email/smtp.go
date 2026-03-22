package email

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/14mdzk/goscratch/internal/port"
)

// SMTPConfig holds SMTP connection configuration
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// SMTPSender implements port.EmailSender using SMTP
type SMTPSender struct {
	config SMTPConfig
}

// NewSMTPSender creates a new SMTP email sender
func NewSMTPSender(cfg SMTPConfig) *SMTPSender {
	return &SMTPSender{
		config: cfg,
	}
}

// Send sends an email via SMTP
func (s *SMTPSender) Send(ctx context.Context, msg port.EmailMessage) error {
	if len(msg.To) == 0 {
		return fmt.Errorf("email recipient is required")
	}

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	// Build the email message
	var body strings.Builder
	body.WriteString(fmt.Sprintf("From: %s\r\n", s.config.From))
	body.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(msg.To, ", ")))

	if len(msg.CC) > 0 {
		body.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(msg.CC, ", ")))
	}

	body.WriteString(fmt.Sprintf("Subject: %s\r\n", msg.Subject))

	if msg.IsHTML {
		body.WriteString("MIME-Version: 1.0\r\n")
		body.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	} else {
		body.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	}

	body.WriteString("\r\n")
	body.WriteString(msg.Body)

	// Collect all recipients
	recipients := make([]string, 0, len(msg.To)+len(msg.CC)+len(msg.BCC))
	recipients = append(recipients, msg.To...)
	recipients = append(recipients, msg.CC...)
	recipients = append(recipients, msg.BCC...)

	// Setup authentication
	var auth smtp.Auth
	if s.config.Username != "" {
		auth = smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)
	}

	// Send the email
	if err := smtp.SendMail(addr, auth, s.config.From, recipients, []byte(body.String())); err != nil {
		return fmt.Errorf("failed to send email via SMTP: %w", err)
	}

	return nil
}

// Close cleans up SMTP sender resources
func (s *SMTPSender) Close() error {
	return nil
}
