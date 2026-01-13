package worker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Job represents a background job to be processed
type Job struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	Attempts  int             `json:"attempts"`
	MaxRetry  int             `json:"max_retry"`
	CreatedAt time.Time       `json:"created_at"`
}

// NewJob creates a new job with the given type and payload
func NewJob(jobType string, payload any) (*Job, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return &Job{
		ID:        uuid.New().String(),
		Type:      jobType,
		Payload:   data,
		Attempts:  0,
		MaxRetry:  3,
		CreatedAt: time.Now(),
	}, nil
}

// NewJobWithRetry creates a new job with custom retry count
func NewJobWithRetry(jobType string, payload any, maxRetry int) (*Job, error) {
	job, err := NewJob(jobType, payload)
	if err != nil {
		return nil, err
	}
	job.MaxRetry = maxRetry
	return job, nil
}

// Encode serializes the job to JSON bytes
func (j *Job) Encode() ([]byte, error) {
	return json.Marshal(j)
}

// DecodeJob deserializes a job from JSON bytes
func DecodeJob(data []byte) (*Job, error) {
	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, err
	}
	return &job, nil
}

// UnmarshalPayload decodes the job payload into the given struct
func (j *Job) UnmarshalPayload(v any) error {
	return json.Unmarshal(j.Payload, v)
}

// CanRetry returns true if the job can be retried
func (j *Job) CanRetry() bool {
	return j.Attempts < j.MaxRetry
}

// IncrementAttempts increments the attempt counter
func (j *Job) IncrementAttempts() {
	j.Attempts++
}

// JobHandler defines the interface for job handlers
type JobHandler interface {
	// Type returns the job type this handler processes
	Type() string

	// Handle processes the job payload
	// Returns an error if processing fails
	Handle(ctx context.Context, job *Job) error
}

// JobResult represents the result of processing a job
type JobResult struct {
	JobID   string
	Success bool
	Error   error
	Retry   bool
}

// Common job types
const (
	JobTypeEmailSend    = "email.send"
	JobTypeAuditCleanup = "audit.cleanup"
	JobTypeNotification = "notification.send"
)
