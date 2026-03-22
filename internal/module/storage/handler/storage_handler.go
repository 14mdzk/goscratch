package handler

import (
	"path/filepath"
	"strconv"
	"time"

	"github.com/14mdzk/goscratch/internal/module/storage/usecase"
	"github.com/14mdzk/goscratch/pkg/apperr"
	"github.com/14mdzk/goscratch/pkg/response"
	"github.com/gofiber/fiber/v2"
)

// Handler handles storage HTTP requests
type Handler struct {
	useCase *usecase.UseCase
}

// NewHandler creates a new storage handler
func NewHandler(useCase *usecase.UseCase) *Handler {
	return &Handler{useCase: useCase}
}

// Upload handles file upload via multipart/form-data
func (h *Handler) Upload(c *fiber.Ctx) error {
	// Parse multipart form
	file, err := c.FormFile("file")
	if err != nil {
		return response.Fail(c, apperr.BadRequestf("file is required: %v", err))
	}

	// Open the uploaded file
	src, err := file.Open()
	if err != nil {
		return response.Fail(c, apperr.Internalf("failed to open uploaded file"))
	}
	defer src.Close()

	// Optional directory field
	directory := c.FormValue("directory", "")

	result, err := h.useCase.Upload(c.UserContext(), src, file, directory)
	if err != nil {
		return response.Fail(c, err)
	}

	return response.Created(c, result)
}

// Download handles file download.
// Route: GET /files/download/*
func (h *Handler) Download(c *fiber.Ctx) error {
	path := c.Params("*")
	if path == "" {
		return response.Fail(c, apperr.BadRequestf("file path is required"))
	}

	reader, contentType, err := h.useCase.Download(c.UserContext(), path)
	if err != nil {
		return response.Fail(c, err)
	}
	defer reader.Close()

	// Use only the filename portion for Content-Disposition
	filename := filepath.Base(path)

	c.Set("Content-Type", contentType)
	c.Set("Content-Disposition", "attachment; filename=\""+sanitizeHeaderValue(filename)+"\"")

	return c.SendStream(reader)
}

// Delete handles file deletion.
// Route: DELETE /files/*
func (h *Handler) Delete(c *fiber.Ctx) error {
	path := c.Params("*")
	if path == "" {
		return response.Fail(c, apperr.BadRequestf("file path is required"))
	}

	if err := h.useCase.Delete(c.UserContext(), path); err != nil {
		return response.Fail(c, err)
	}

	return response.NoContent(c)
}

// GetURL returns a URL for accessing a file.
// Route: GET /files/url/*
func (h *Handler) GetURL(c *fiber.Ctx) error {
	path := c.Params("*")
	if path == "" {
		return response.Fail(c, apperr.BadRequestf("file path is required"))
	}

	// Parse optional expiry duration from query param (in seconds)
	var expires time.Duration
	if expiryStr := c.Query("expires"); expiryStr != "" {
		seconds, err := strconv.Atoi(expiryStr)
		if err != nil || seconds <= 0 {
			return response.Fail(c, apperr.BadRequestf("invalid expires value: must be a positive integer (seconds)"))
		}
		expires = time.Duration(seconds) * time.Second
	}

	result, err := h.useCase.GetURL(c.UserContext(), path, expires)
	if err != nil {
		return response.Fail(c, err)
	}

	return response.Success(c, result)
}

// List lists files with an optional prefix.
// Route: GET /files
func (h *Handler) List(c *fiber.Ctx) error {
	prefix := c.Query("prefix", "")

	result, err := h.useCase.List(c.UserContext(), prefix)
	if err != nil {
		return response.Fail(c, err)
	}

	return response.Success(c, result)
}

// sanitizeHeaderValue removes characters that could cause header injection
func sanitizeHeaderValue(s string) string {
	var result []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\r' && c != '\n' && c != '"' {
			result = append(result, c)
		}
	}
	return string(result)
}
