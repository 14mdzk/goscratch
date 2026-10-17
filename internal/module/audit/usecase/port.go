package usecase

import (
	"context"

	shareddomain "github.com/14mdzk/goscratch/internal/shared/domain"

	"github.com/14mdzk/goscratch/internal/module/audit/dto"
)

// UseCase defines the interface for audit-log query operations.
type UseCase interface {
	List(ctx context.Context, req dto.ListAuditLogsRequest) (shareddomain.CursorPage[dto.AuditLogResponse], error)
}
