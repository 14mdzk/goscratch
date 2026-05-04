package handler

import (
	"github.com/14mdzk/goscratch/internal/module/job/dto"
	"github.com/14mdzk/goscratch/internal/module/job/usecase"
	"github.com/14mdzk/goscratch/internal/platform/validator"
	"github.com/14mdzk/goscratch/pkg/response"
	"github.com/gofiber/fiber/v2"
)

// Handler handles job HTTP requests
type Handler struct {
	useCase usecase.UseCase
}

// NewHandler creates a new job handler
func NewHandler(useCase usecase.UseCase) *Handler {
	return &Handler{useCase: useCase}
}

// Dispatch handles POST /jobs/dispatch
func (h *Handler) Dispatch(c *fiber.Ctx) error {
	var req dto.DispatchJobRequest
	if err := validator.ValidateAndBind(c, &req); err != nil {
		return validator.HandleValidationError(c, err)
	}

	maxRetry := 3
	if req.MaxRetry != nil {
		maxRetry = *req.MaxRetry
	}

	job, err := h.useCase.Dispatch(c.UserContext(), req.Type, req.Payload, maxRetry)
	if err != nil {
		return response.Fail(c, err)
	}
	return response.Created(c, job)
}

// ListTypes handles GET /jobs/types
func (h *Handler) ListTypes(c *fiber.Ctx) error {
	result := h.useCase.ListJobTypes(c.UserContext())
	return response.Success(c, result)
}
