package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	shareddomain "github.com/14mdzk/goscratch/internal/shared/domain"

	"github.com/14mdzk/goscratch/internal/module/audit/dto"
	"github.com/14mdzk/goscratch/internal/module/audit/usecase"
	"github.com/14mdzk/goscratch/pkg/apperr"
)

// stubUseCase is a scriptable usecase.UseCase.
type stubUseCase struct {
	page shareddomain.CursorPage[dto.AuditLogResponse]
	err  error
}

func (s *stubUseCase) List(_ context.Context, _ dto.ListAuditLogsRequest) (shareddomain.CursorPage[dto.AuditLogResponse], error) {
	return s.page, s.err
}

var _ usecase.UseCase = (*stubUseCase)(nil)

func testPage() shareddomain.CursorPage[dto.AuditLogResponse] {
	return shareddomain.CursorPage[dto.AuditLogResponse]{
		Items: []dto.AuditLogResponse{
			{ID: "id-1", Action: "LOGIN", Resource: "user", Timestamp: "2026-01-01T00:00:00Z"},
		},
		PaginationMeta: shareddomain.PaginationMeta{HasMore: false},
	}
}

func TestList_SuccessEnvelope(t *testing.T) {
	app := fiber.New()
	h := NewHandler(&stubUseCase{page: testPage()})
	app.Get("/logs", h.List)

	req := httptest.NewRequest("GET", "/logs?action=LOGIN&limit=10", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var body struct {
		Success    bool                   `json:"success"`
		Data       []dto.AuditLogResponse `json:"data"`
		Pagination struct {
			HasMore bool `json:"has_more"`
		} `json:"pagination"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.True(t, body.Success)
	require.Len(t, body.Data, 1)
	assert.Equal(t, "id-1", body.Data[0].ID)
	assert.False(t, body.Pagination.HasMore)
}

func TestList_UsecaseErrorSurfaces(t *testing.T) {
	app := fiber.New()
	h := NewHandler(&stubUseCase{err: apperr.BadRequestf("invalid cursor")})
	app.Get("/logs", h.List)

	req := httptest.NewRequest("GET", "/logs?cursor=bogus", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestList_InternalErrorSurfaces(t *testing.T) {
	app := fiber.New()
	h := NewHandler(&stubUseCase{err: errors.New("boom")})
	app.Get("/logs", h.List)

	req := httptest.NewRequest("GET", "/logs", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}
