package usecase

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/14mdzk/goscratch/internal/module/storage/domain"
	"github.com/14mdzk/goscratch/internal/module/storage/dto"
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/pkg/apperr"
	"github.com/google/uuid"
)

// UseCase handles storage business logic
type UseCase struct {
	storage             port.Storage
	maxFileSize         int64
	allowedContentTypes map[string]bool
}

// Config holds configuration for the storage usecase
type Config struct {
	MaxFileSize         int64
	AllowedContentTypes map[string]bool
}

// NewUseCase creates a new storage use case
func NewUseCase(storage port.Storage, cfg *Config) *UseCase {
	maxSize := domain.DefaultMaxFileSize
	allowed := domain.DefaultAllowedContentTypes

	if cfg != nil {
		if cfg.MaxFileSize > 0 {
			maxSize = cfg.MaxFileSize
		}
		if len(cfg.AllowedContentTypes) > 0 {
			allowed = cfg.AllowedContentTypes
		}
	}

	return &UseCase{
		storage:             storage,
		maxFileSize:         maxSize,
		allowedContentTypes: allowed,
	}
}

// Upload validates and uploads a file
func (uc *UseCase) Upload(ctx context.Context, file multipart.File, header *multipart.FileHeader, directory string) (*dto.UploadResponse, error) {
	// Validate file size
	if header.Size > uc.maxFileSize {
		return nil, apperr.BadRequestf("file size %d exceeds maximum allowed size %d", header.Size, uc.maxFileSize)
	}

	// Validate content type
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if !uc.allowedContentTypes[contentType] {
		return nil, apperr.BadRequestf("content type %q is not allowed", contentType)
	}

	// Generate unique filename preserving original extension
	ext := filepath.Ext(header.Filename)
	uniqueName := uuid.New().String() + ext

	// Sanitize directory to prevent path traversal
	directory = sanitizePath(directory)

	// Build storage path
	var storagePath string
	if directory != "" {
		storagePath = directory + "/" + uniqueName
	} else {
		storagePath = uniqueName
	}

	// Upload via storage adapter
	path, err := uc.storage.Upload(ctx, storagePath, file, port.WithContentType(contentType))
	if err != nil {
		return nil, apperr.Internalf("failed to upload file: %v", err)
	}

	// Get URL for the uploaded file
	url, err := uc.storage.GetURL(ctx, path, 24*time.Hour)
	if err != nil {
		// Non-fatal: file is uploaded but URL generation failed
		url = ""
	}

	return &dto.UploadResponse{
		Path: path,
		URL:  url,
		Size: header.Size,
	}, nil
}

// Download retrieves a file and its content type
func (uc *UseCase) Download(ctx context.Context, path string) (io.ReadCloser, string, error) {
	path = sanitizePath(path)
	if path == "" {
		return nil, "", apperr.BadRequestf("invalid file path")
	}

	// Check if file exists
	exists, err := uc.storage.Exists(ctx, path)
	if err != nil {
		return nil, "", apperr.Internalf("failed to check file existence: %v", err)
	}
	if !exists {
		return nil, "", apperr.NotFoundf("file not found: %s", path)
	}

	reader, err := uc.storage.Download(ctx, path)
	if err != nil {
		return nil, "", apperr.Internalf("failed to download file: %v", err)
	}

	// Determine content type from extension
	contentType := "application/octet-stream"
	ext := filepath.Ext(path)
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".png":
		contentType = "image/png"
	case ".gif":
		contentType = "image/gif"
	case ".webp":
		contentType = "image/webp"
	case ".pdf":
		contentType = "application/pdf"
	}

	return reader, contentType, nil
}

// Delete removes a file
func (uc *UseCase) Delete(ctx context.Context, path string) error {
	path = sanitizePath(path)
	if path == "" {
		return apperr.BadRequestf("invalid file path")
	}

	// Check if file exists
	exists, err := uc.storage.Exists(ctx, path)
	if err != nil {
		return apperr.Internalf("failed to check file existence: %v", err)
	}
	if !exists {
		return apperr.NotFoundf("file not found: %s", path)
	}

	if err := uc.storage.Delete(ctx, path); err != nil {
		return apperr.Internalf("failed to delete file: %v", err)
	}

	return nil
}

// GetURL returns a URL for accessing a file
func (uc *UseCase) GetURL(ctx context.Context, path string, expires time.Duration) (*dto.FileResponse, error) {
	path = sanitizePath(path)
	if path == "" {
		return nil, apperr.BadRequestf("invalid file path")
	}

	// Check if file exists
	exists, err := uc.storage.Exists(ctx, path)
	if err != nil {
		return nil, apperr.Internalf("failed to check file existence: %v", err)
	}
	if !exists {
		return nil, apperr.NotFoundf("file not found: %s", path)
	}

	if expires <= 0 {
		expires = 1 * time.Hour
	}

	url, err := uc.storage.GetURL(ctx, path, expires)
	if err != nil {
		return nil, apperr.Internalf("failed to generate URL: %v", err)
	}

	return &dto.FileResponse{
		Path: path,
		URL:  url,
	}, nil
}

// List lists files with the given prefix
func (uc *UseCase) List(ctx context.Context, prefix string) (*dto.ListFilesResponse, error) {
	prefix = sanitizePath(prefix)

	files, err := uc.storage.List(ctx, prefix)
	if err != nil {
		return nil, apperr.Internalf("failed to list files: %v", err)
	}

	responses := make([]dto.FileResponse, 0, len(files))
	for _, f := range files {
		responses = append(responses, dto.FileResponse{
			Path:        f.Path,
			Size:        f.Size,
			ContentType: f.ContentType,
			ModTime:     f.ModifiedAt.Format(time.RFC3339),
		})
	}

	return &dto.ListFilesResponse{
		Files: responses,
	}, nil
}

// sanitizePath cleans a file path to prevent path traversal attacks
func sanitizePath(path string) string {
	// Remove any null bytes
	path = strings.ReplaceAll(path, "\x00", "")

	// Clean the path
	path = filepath.Clean(path)

	// Remove leading slashes and dots
	path = strings.TrimLeft(path, "/.")

	// Reject paths containing ".."
	parts := strings.Split(path, "/")
	var clean []string
	for _, part := range parts {
		if part == ".." || part == "." || part == "" {
			continue
		}
		clean = append(clean, part)
	}

	return strings.Join(clean, "/")
}

// SanitizePath is exported for testing
func SanitizePath(path string) string {
	return sanitizePath(path)
}

// FormatError is a helper to wrap errors with context
func FormatError(op string, err error) error {
	return fmt.Errorf("%s: %w", op, err)
}
