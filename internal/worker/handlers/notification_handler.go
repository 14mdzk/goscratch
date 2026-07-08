package handlers

import (
	"context"
	"fmt"

	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/internal/worker"
	"github.com/14mdzk/goscratch/pkg/logger"
)

// NotificationPayload represents the data for sending a notification.
type NotificationPayload struct {
	UserID  string `json:"user_id"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	Channel string `json:"channel,omitempty"`
}

// NotificationHandler handles notification.send jobs.
type NotificationHandler struct {
	logger             *logger.Logger
	notificationSender port.NotificationSender
}

// NewNotificationHandler creates a new notification handler.
func NewNotificationHandler(log *logger.Logger, sender port.NotificationSender) *NotificationHandler {
	return &NotificationHandler{
		logger:             log,
		notificationSender: sender,
	}
}

// Type returns the job type this handler processes.
func (h *NotificationHandler) Type() string {
	return worker.JobTypeNotification
}

// Handle processes a notification sending job.
func (h *NotificationHandler) Handle(ctx context.Context, job *worker.Job) error {
	var payload NotificationPayload
	if err := job.UnmarshalPayload(&payload); err != nil {
		return fmt.Errorf("failed to unmarshal notification payload: %w", err)
	}

	if payload.UserID == "" {
		return fmt.Errorf("notification user_id is required")
	}
	if payload.Title == "" {
		return fmt.Errorf("notification title is required")
	}

	h.logger.Info("Sending notification",
		"user_id", payload.UserID,
		"title", payload.Title,
		"channel", payload.Channel,
		"job_id", job.ID,
	)

	msg := port.NotificationMessage{
		UserID:  payload.UserID,
		Title:   payload.Title,
		Body:    payload.Body,
		Channel: payload.Channel,
	}

	if err := h.notificationSender.Send(ctx, msg); err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}

	h.logger.Info("Notification sent successfully",
		"user_id", payload.UserID,
		"title", payload.Title,
		"job_id", job.ID,
	)

	return nil
}
