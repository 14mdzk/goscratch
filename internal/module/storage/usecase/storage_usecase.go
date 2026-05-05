package usecase

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/14mdzk/goscratch/internal/module/storage/domain"
	"github.com/14mdzk/goscratch/internal/module/storage/dto"
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/pkg/apperr"
	"github.com/google/uuid"
)

// sniffSize is the number of leading bytes inspected by http.DetectContentType.
const sniffSize = 512

// storageUseCase handles storage business logic.
// Returned via the UseCase interface; the concrete type is unexported so
// callers depend on the interface (enables the audit decorator).
type storageUseCase struct {
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
func NewUseCase(storage port.Storage, cfg *Config) UseCase {
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

	return &storageUseCase{
		storage:             storage,
		maxFileSize:         maxSize,
		allowedContentTypes: allowed,
	}
}

// Upload validates and uploads a file
func (uc *storageUseCase) Upload(ctx context.Context, file multipart.File, header *multipart.FileHeader, directory string) (*dto.UploadResponse, error) {
	// Validate file size
	if header.Size > uc.maxFileSize {
		return nil, apperr.BadRequestf("file size %d exceeds maximum allowed size %d", header.Size, uc.maxFileSize)
	}

	// Sniff the actual content type from the first 512 bytes rather than
	// trusting the client-supplied multipart Content-Type header (which is
	// attacker-controlled). The buffered reader is then re-used as the
	// upload source so the peeked bytes are not consumed.
	//
	// Operators: extend `domain.DefaultAllowedContentTypes` (or pass a
	// custom map via Config.AllowedContentTypes) to widen the allowlist —
	// the default set covers images and PDFs.
	bufReader := bufio.NewReaderSize(file, sniffSize)
	head, err := bufReader.Peek(sniffSize)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrBufferFull) {
		return nil, apperr.Internalf("failed to inspect uploaded file: %v", err)
	}
	contentType := http.DetectContentType(head)
	// http.DetectContentType returns "<base>; charset=..." for some types;
	// strip the parameters before allowlist lookup.
	baseContentType := contentType
	if idx := strings.Index(baseContentType, ";"); idx >= 0 {
		baseContentType = strings.TrimSpace(baseContentType[:idx])
	}
	if !uc.allowedContentTypes[baseContentType] {
		return nil, apperr.UnsupportedMediaTypef("content type %q is not allowed", baseContentType)
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

	// Upload via storage adapter — reuse the buffered reader so the peeked
	// bytes are still part of the stream.
	path, err := uc.storage.Upload(ctx, storagePath, bufReader, port.WithContentType(baseContentType))
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
func (uc *storageUseCase) Download(ctx context.Context, path string) (io.ReadCloser, string, error) {
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
func (uc *storageUseCase) Delete(ctx context.Context, path string) error {
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
func (uc *storageUseCase) GetURL(ctx context.Context, path string, expires time.Duration) (*dto.FileResponse, error) {
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
func (uc *storageUseCase) List(ctx context.Context, prefix string) (*dto.ListFilesResponse, error) {
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
