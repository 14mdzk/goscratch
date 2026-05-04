package usecase

import (
	"context"

	"github.com/14mdzk/goscratch/internal/module/job/dto"
)

// UseCase defines the interface for job business logic operations.
// Handlers and decorators depend on this interface rather than on the
// concrete *UseCase struct, enabling testability and the audit decorator.
type UseCase interface {
	Dispatch(ctx context.Context, jobType string, payload any, maxRetry int) (*dto.JobResponse, error)
	ListJobTypes(ctx context.Context) *dto.ListJobTypesResponse
}
