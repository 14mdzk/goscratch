package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/14mdzk/goscratch/internal/module/audit/dto"
	"github.com/14mdzk/goscratch/internal/module/audit/usecase"
	"github.com/14mdzk/goscratch/internal/platform/validator"
	"github.com/14mdzk/goscratch/pkg/response"
)

// Handler handles audit-log query requests
type Handler struct {
	useCase usecase.UseCase
}

// NewHandler creates a new audit handler
func NewHandler(uc usecase.UseCase) *Handler {
	return &Handler{useCase: uc}
}

// List retrieves a paginated list of audit log entries
func (h *Handler) List(c *fiber.Ctx) error {
	var req dto.ListAuditLogsRequest
	if err := validator.ValidateQuery(c, &req); err != nil {
		return validator.HandleValidationError(c, err)
	}

	result, err := h.useCase.List(c.UserContext(), req)
	if err != nil {
		return response.Fail(c, err)
	}
	return response.Paginated(c, result.GetItems(), result.GetMeta())
}
