package handlers

import (
	"context"
	"fmt"

	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/internal/worker"
	"github.com/14mdzk/goscratch/pkg/logger"
)

// EmailPayload represents the data for sending an email
type EmailPayload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
	HTML    bool   `json:"html,omitempty"`
}

// EmailHandler handles email sending jobs
type EmailHandler struct {
	logger      *logger.Logger
	emailSender port.EmailSender
}

// NewEmailHandler creates a new email handler
func NewEmailHandler(log *logger.Logger, emailSender port.EmailSender) *EmailHandler {
	return &EmailHandler{
		logger:      log,
		emailSender: emailSender,
	}
}

// Type returns the job type this handler processes
func (h *EmailHandler) Type() string {
	return worker.JobTypeEmailSend
}

// Handle processes an email sending job
func (h *EmailHandler) Handle(ctx context.Context, job *worker.Job) error {
	var payload EmailPayload
	if err := job.UnmarshalPayload(&payload); err != nil {
		return fmt.Errorf("failed to unmarshal email payload: %w", err)
	}

	// Validate payload
	if payload.To == "" {
		return fmt.Errorf("email recipient is required")
	}
	if payload.Subject == "" {
		return fmt.Errorf("email subject is required")
	}

	h.logger.Info("Sending email",
		"to", payload.To,
		"subject", payload.Subject,
		"job_id", job.ID,
	)

	msg := port.EmailMessage{
		To:      []string{payload.To},
		Subject: payload.Subject,
		Body:    payload.Body,
		IsHTML:  payload.HTML,
	}

	if err := h.emailSender.Send(ctx, msg); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	h.logger.Info("Email sent successfully",
		"to", payload.To,
		"subject", payload.Subject,
		"job_id", job.ID,
	)

	return nil
}
