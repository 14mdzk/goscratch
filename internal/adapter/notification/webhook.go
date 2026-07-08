package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/14mdzk/goscratch/internal/port"
)

// defaultWebhookTimeout is the fallback deadline applied when the caller did not
// install one on the context.
const defaultWebhookTimeout = 10 * time.Second

// WebhookSender implements port.NotificationSender by POSTing JSON payloads
// to a configurable webhook URL. Operators wire this to Slack, Discord,
// a custom notification service, or any HTTP endpoint that accepts JSON.
type WebhookSender struct {
	url        string
	httpClient *http.Client
}

// WebhookConfig holds webhook delivery configuration.
type WebhookConfig struct {
	URL string
}

// NewWebhookSender creates a new webhook-based notification sender.
func NewWebhookSender(cfg WebhookConfig) *WebhookSender {
	return &WebhookSender{
		url: cfg.URL,
		httpClient: &http.Client{
			Timeout: defaultWebhookTimeout,
		},
	}
}

// Send delivers a notification by POSTing it to the configured webhook URL.
// The payload is a JSON object with the notification message fields plus a
// "sent_at" timestamp.
func (s *WebhookSender) Send(ctx context.Context, msg port.NotificationMessage) error {
	if s.url == "" {
		return fmt.Errorf("notification webhook URL is not configured")
	}

	payload := struct {
		port.NotificationMessage
		SentAt string `json:"sent_at"`
	}{
		NotificationMessage: msg,
		SentAt:              time.Now().UTC().Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("notification: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("notification: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("notification: send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("notification: webhook returned HTTP %d", resp.StatusCode)
	}

	return nil
}

// Close cleans up resources held by the sender.
func (s *WebhookSender) Close() error {
	s.httpClient.CloseIdleConnections()
	return nil
}
