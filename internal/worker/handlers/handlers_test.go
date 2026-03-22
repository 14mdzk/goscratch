package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/internal/worker"
	"github.com/14mdzk/goscratch/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockEmailSender struct {
	sent []port.EmailMessage
}

func (m *mockEmailSender) Send(_ context.Context, msg port.EmailMessage) error {
	m.sent = append(m.sent, msg)
	return nil
}

func (m *mockEmailSender) Close() error { return nil }

func newTestLogger() *logger.Logger {
	return logger.New(logger.Config{
		Level:  "debug",
		Format: "json",
		Output: &bytes.Buffer{},
	})
}

func makeJob(t *testing.T, jobType string, payload any) *worker.Job {
	t.Helper()
	job, err := worker.NewJob(jobType, payload)
	require.NoError(t, err)
	return job
}

// --- EmailHandler Tests ---

func TestEmailHandler_Type(t *testing.T) {
	h := NewEmailHandler(newTestLogger(), &mockEmailSender{})
	assert.Equal(t, worker.JobTypeEmailSend, h.Type())
}

func TestEmailHandler_Handle(t *testing.T) {
	t.Run("valid_payload", func(t *testing.T) {
		h := NewEmailHandler(newTestLogger(), &mockEmailSender{})
		job := makeJob(t, worker.JobTypeEmailSend, EmailPayload{
			To:      "user@example.com",
			Subject: "Welcome",
			Body:    "Hello!",
		})

		err := h.Handle(context.Background(), job)
		assert.NoError(t, err)
	})

	t.Run("missing_recipient", func(t *testing.T) {
		h := NewEmailHandler(newTestLogger(), &mockEmailSender{})
		job := makeJob(t, worker.JobTypeEmailSend, EmailPayload{
			To:      "",
			Subject: "Welcome",
			Body:    "Hello!",
		})

		err := h.Handle(context.Background(), job)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "email recipient is required")
	})

	t.Run("missing_subject", func(t *testing.T) {
		h := NewEmailHandler(newTestLogger(), &mockEmailSender{})
		job := makeJob(t, worker.JobTypeEmailSend, EmailPayload{
			To:      "user@example.com",
			Subject: "",
			Body:    "Hello!",
		})

		err := h.Handle(context.Background(), job)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "email subject is required")
	})

	t.Run("invalid_payload_json", func(t *testing.T) {
		h := NewEmailHandler(newTestLogger(), &mockEmailSender{})
		job := &worker.Job{
			ID:      "test-id",
			Type:    worker.JobTypeEmailSend,
			Payload: json.RawMessage(`not-json`),
		}

		err := h.Handle(context.Background(), job)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal email payload")
	})

	t.Run("with_html_flag", func(t *testing.T) {
		h := NewEmailHandler(newTestLogger(), &mockEmailSender{})
		job := makeJob(t, worker.JobTypeEmailSend, EmailPayload{
			To:      "user@example.com",
			Subject: "HTML Email",
			Body:    "<h1>Hello</h1>",
			HTML:    true,
		})

		err := h.Handle(context.Background(), job)
		assert.NoError(t, err)
	})
}

// --- AuditCleanupHandler Tests ---

func TestAuditCleanupHandler_Type(t *testing.T) {
	// We pass nil for db since we only test Type()
	h := NewAuditCleanupHandler(nil, newTestLogger())
	assert.Equal(t, worker.JobTypeAuditCleanup, h.Type())
}

// Note: AuditCleanupHandler.Handle() requires a real *pgxpool.Pool for the DB call.
// We test the type and payload unmarshaling without a live database.
func TestAuditCleanupHandler_Handle_InvalidPayload(t *testing.T) {
	h := NewAuditCleanupHandler(nil, newTestLogger())
	job := &worker.Job{
		ID:      "test-id",
		Type:    worker.JobTypeAuditCleanup,
		Payload: json.RawMessage(`not-json`),
	}

	err := h.Handle(context.Background(), job)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal audit cleanup payload")
}
