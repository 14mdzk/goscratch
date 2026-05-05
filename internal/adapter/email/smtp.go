package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

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

// defaultSMTPTimeout is the fallback deadline applied when the caller did not
// install one on the context. The standard library's net/smtp.SendMail has no
// timeout and will block on the OS TCP timeout (often >2 minutes) if the SMTP
// server accepts the TCP connection but never responds on the protocol.
const defaultSMTPTimeout = 30 * time.Second

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

// Send sends an email via SMTP. Unlike net/smtp.SendMail, this implementation
// honours the deadline / cancellation on ctx for every network operation, so a
// blackhole SMTP server cannot wedge the caller for the OS TCP timeout.
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

	if err := s.sendMailContext(ctx, addr, auth, s.config.From, recipients, []byte(body.String())); err != nil {
		return fmt.Errorf("failed to send email via SMTP: %w", err)
	}

	return nil
}

// sendMailContext is a context-aware re-implementation of net/smtp.SendMail.
// It dials with net.Dialer.DialContext and applies any context deadline to the
// underlying connection, so cancellation and timeouts are honoured throughout
// the SMTP exchange.
func (s *SMTPSender) sendMailContext(ctx context.Context, addr string, auth smtp.Auth, from string, to []string, body []byte) error {
	// Ensure we always have a bounded deadline so a blackhole server cannot
	// hang the worker for the OS TCP timeout.
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultSMTPTimeout)
		defer cancel()
	}

	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	// Apply the ctx deadline to every read/write on the conn. Without this the
	// SMTP exchange itself (HELO/STARTTLS/AUTH/...) is unbounded.
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	// Cancel-on-context: if ctx is cancelled mid-exchange, force the conn shut
	// so the in-flight read/write returns immediately.
	exchangeDone := make(chan struct{})
	defer close(exchangeDone)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-exchangeDone:
		}
	}()

	client, err := smtp.NewClient(conn, s.config.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp client: %w", err)
	}
	defer func() {
		// Quit best-effort; if it fails, fall back to Close.
		if quitErr := client.Quit(); quitErr != nil {
			_ = client.Close()
		}
	}()

	if err := client.Hello(localName()); err != nil {
		return fmt.Errorf("hello: %w", err)
	}

	// Opportunistic STARTTLS when the server advertises it.
	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{ServerName: s.config.Host, MinVersion: tls.VersionTLS12}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	}

	if auth != nil {
		if ok, _ := client.Extension("AUTH"); ok {
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("auth: %w", err)
			}
		}
	}

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}
	for _, rcpt := range to {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("rcpt to %q: %w", rcpt, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	if _, err := w.Write(body); err != nil {
		_ = w.Close()
		return fmt.Errorf("write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close data: %w", err)
	}

	// Surface ctx error if the watcher goroutine torched the conn.
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return nil
}

// Close cleans up SMTP sender resources
func (s *SMTPSender) Close() error {
	return nil
}

// localName returns the EHLO / HELO local name. net/smtp defaults to
// "localhost" when the caller does not call client.Hello, but we set it
// explicitly so the value is stable across platforms.
func localName() string {
	return "localhost"
}
