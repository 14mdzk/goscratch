package email

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

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

// TestSMTPSender_Send_ContextDeadline asserts that Send returns within the
// context deadline when the SMTP server accepts the TCP connection but never
// responds — proving that a blackhole SMTP server cannot wedge the worker for
// the OS TCP timeout (which is the bug this test locks in place).
func TestSMTPSender_Send_ContextDeadline(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	// Accept connections but never write the SMTP greeting. The accept loop
	// owns the conns it accepts and closes them itself when the listener
	// shuts down, avoiding any cross-goroutine ownership of the conn slice.
	acceptDone := make(chan struct{})
	go func() {
		defer close(acceptDone)
		var conns []net.Conn
		defer func() {
			for _, c := range conns {
				_ = c.Close()
			}
		}()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conns = append(conns, conn)
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		<-acceptDone
	})

	host, portStr, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	portNum, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	sender := NewSMTPSender(SMTPConfig{
		Host: host,
		Port: portNum,
		From: "test@example.com",
	})

	msg := port.EmailMessage{
		To:      []string{"user@example.com"},
		Subject: "Test",
		Body:    "Body",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Hard upper bound: if Send hangs for 5s we fail fast instead of blocking
	// the whole test binary on the OS TCP timeout.
	hardStop := time.AfterFunc(5*time.Second, func() {
		_ = listener.Close()
	})
	defer hardStop.Stop()

	start := time.Now()
	sendErr := sender.Send(ctx, msg)
	elapsed := time.Since(start)

	require.Error(t, sendErr, "blackhole SMTP server must produce an error, not hang forever")
	require.Less(t, elapsed, 3*time.Second, "Send must honour ctx deadline (~500ms), got %s", elapsed)
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
