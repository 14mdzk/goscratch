package usecase

import (
	"context"
	"time"

	"github.com/14mdzk/goscratch/internal/module/job/dto"
	"github.com/14mdzk/goscratch/internal/worker"
	"github.com/14mdzk/goscratch/pkg/apperr"
	"github.com/google/uuid"
)

// validJobTypes maps job type identifiers to their descriptions
var validJobTypes = map[string]string{
	worker.JobTypeEmailSend:    "Send an email to a recipient",
	worker.JobTypeAuditCleanup: "Clean up old audit log entries",
	worker.JobTypeNotification: "Send a notification to a user",
}

// UseCase handles job business logic
type UseCase struct {
	publisher *worker.Publisher
}

// NewUseCase creates a new job use case
func NewUseCase(publisher *worker.Publisher) *UseCase {
	return &UseCase{
		publisher: publisher,
	}
}

// Dispatch validates the job type and publishes a job to the queue
func (uc *UseCase) Dispatch(ctx context.Context, jobType string, payload any, maxRetry int) (*dto.JobResponse, error) {
	// Validate job type
	if _, ok := validJobTypes[jobType]; !ok {
		return nil, apperr.BadRequestf("invalid job type: %s", jobType)
	}

	// Publish with retry
	if err := uc.publisher.PublishWithRetry(ctx, jobType, payload, maxRetry); err != nil {
		return nil, apperr.Internalf("failed to dispatch job: %s", err.Error())
	}

	return &dto.JobResponse{
		ID:        uuid.New().String(),
		Type:      jobType,
		Status:    "queued",
		CreatedAt: time.Now().Format(time.RFC3339),
	}, nil
}

// ListJobTypes returns all available job types with descriptions
func (uc *UseCase) ListJobTypes(ctx context.Context) *dto.ListJobTypesResponse {
	types := make([]dto.JobTypeInfo, 0, len(validJobTypes))
	for t, desc := range validJobTypes {
		types = append(types, dto.JobTypeInfo{
			Type:        t,
			Description: desc,
		})
	}
	return &dto.ListJobTypesResponse{Types: types}
}
