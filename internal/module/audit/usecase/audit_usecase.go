package usecase

import (
	"context"
	"errors"
	"time"

	shareddomain "github.com/14mdzk/goscratch/internal/shared/domain"

	"github.com/14mdzk/goscratch/internal/module/audit/dto"
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/pkg/apperr"
)

// defaultListLimit is the page size when the caller passes no (valid) limit.
const defaultListLimit = 50

// maxListLimit caps the page size to protect the database from unbounded reads.
const maxListLimit = 100

// auditUseCase handles audit-log query logic.
type auditUseCase struct {
	auditor port.Auditor
}

// NewUseCase creates a new audit use case
func NewUseCase(auditor port.Auditor) UseCase {
	return &auditUseCase{auditor: auditor}
}

// List returns a page of audit log entries matching the request filters.
// It fetches one row past the page to determine has_more and mints the next
// keyset cursor from the last returned entry.
func (uc *auditUseCase) List(ctx context.Context, req dto.ListAuditLogsRequest) (shareddomain.CursorPage[dto.AuditLogResponse], error) {
	limit := req.Limit
	if limit <= 0 {
		limit = defaultListLimit
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}

	filter := port.AuditFilter{
		UserID:     req.UserID,
		Action:     port.AuditAction(req.Action),
		Resource:   req.Resource,
		ResourceID: req.ResourceID,
		Limit:      limit + 1,
		Cursor:     req.Cursor,
	}
	if req.StartTime != "" {
		start, err := time.Parse(time.RFC3339, req.StartTime)
		if err != nil {
			return shareddomain.CursorPage[dto.AuditLogResponse]{}, apperr.BadRequestf("invalid start_time: must be RFC3339")
		}
		filter.StartTime = &start
	}
	if req.EndTime != "" {
		end, err := time.Parse(time.RFC3339, req.EndTime)
		if err != nil {
			return shareddomain.CursorPage[dto.AuditLogResponse]{}, apperr.BadRequestf("invalid end_time: must be RFC3339")
		}
		filter.EndTime = &end
	}

	entries, err := uc.auditor.Query(ctx, filter)
	if err != nil {
		// An invalid cursor fails the query (fail-closed in the adapter);
		// surface it as a bad request, everything else as internal.
		if errors.Is(err, port.ErrInvalidAuditCursor) {
			return shareddomain.CursorPage[dto.AuditLogResponse]{}, apperr.BadRequestf("invalid cursor")
		}
		return shareddomain.CursorPage[dto.AuditLogResponse]{}, apperr.Internalf("failed to query audit logs")
	}

	hasMore := len(entries) > limit
	if hasMore {
		entries = entries[:limit]
	}

	items := make([]dto.AuditLogResponse, 0, len(entries))
	for _, e := range entries {
		items = append(items, toResponse(e))
	}

	var nextCursor *string
	if hasMore && len(entries) > 0 {
		last := entries[len(entries)-1]
		encoded := port.EncodeAuditCursor(last.Timestamp, last.ID)
		nextCursor = &encoded
	}

	return shareddomain.CursorPage[dto.AuditLogResponse]{
		Items: items,
		PaginationMeta: shareddomain.PaginationMeta{
			NextCursor: nextCursor,
			HasMore:    hasMore,
		},
	}, nil
}

func toResponse(e port.AuditEntry) dto.AuditLogResponse {
	return dto.AuditLogResponse{
		ID:         e.ID,
		UserID:     e.UserID,
		Action:     string(e.Action),
		Resource:   e.Resource,
		ResourceID: e.ResourceID,
		Metadata:   e.Metadata,
		IPAddress:  e.IPAddress,
		UserAgent:  e.UserAgent,
		Timestamp:  e.Timestamp.Format(time.RFC3339),
	}
}
