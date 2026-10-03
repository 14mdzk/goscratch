package notification

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoOpSender_Send(t *testing.T) {
	log := logger.New(logger.Config{Level: "debug", Format: "json"})
	sender := NewNoOpSender(log)
	ctx := context.Background()

	err := sender.Send(ctx, port.NotificationMessage{
		UserID:  "user-1",
		Title:   "Test",
		Body:    "Hello",
		Channel: "in_app",
	})
	assert.NoError(t, err)
}

func TestNoOpSender_Close(t *testing.T) {
	sender := NewNoOpSender(logger.New(logger.Config{Level: "debug", Format: "json"}))
	assert.NoError(t, sender.Close())
}

func TestWebhookSender_Send_Success(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		err := json.NewDecoder(r.Body).Decode(&received)
		require.NoError(t, err)

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sender := NewWebhookSender(WebhookConfig{URL: srv.URL})
	ctx := context.Background()

	err := sender.Send(ctx, port.NotificationMessage{
		UserID:  "user-1",
		Title:   "Test Notification",
		Body:    "This is a test",
		Channel: "webhook",
	})
	assert.NoError(t, err)

	assert.Equal(t, "user-1", received["user_id"])
	assert.Equal(t, "Test Notification", received["title"])
	assert.NotEmpty(t, received["sent_at"])
}

func TestWebhookSender_Send_NoURL(t *testing.T) {
	sender := NewWebhookSender(WebhookConfig{URL: ""})
	err := sender.Send(context.Background(), port.NotificationMessage{
		UserID: "user-1",
		Title:  "Test",
		Body:   "Hello",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "webhook URL is not configured")
}

func TestWebhookSender_Send_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	sender := NewWebhookSender(WebhookConfig{URL: srv.URL})
	err := sender.Send(context.Background(), port.NotificationMessage{
		UserID: "user-1",
		Title:  "Test",
		Body:   "Hello",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 500")
}

func TestWebhookSender_Close(t *testing.T) {
	sender := NewWebhookSender(WebhookConfig{URL: "http://localhost:9999"})
	assert.NoError(t, sender.Close())
}
