package port

import (
	"context"
	"io"
	"time"
)

// Storage defines the interface for file storage operations
type Storage interface {
	// Upload stores a file and returns the path/key
	Upload(ctx context.Context, path string, data io.Reader, opts ...UploadOption) (string, error)

	// Download retrieves a file
	Download(ctx context.Context, path string) (io.ReadCloser, error)

	// Delete removes a file
	Delete(ctx context.Context, path string) error

	// Exists checks if a file exists
	Exists(ctx context.Context, path string) (bool, error)

	// GetURL returns a URL for accessing the file
	// For local storage, this might be a relative path
	// For S3, this could be a signed URL
	GetURL(ctx context.Context, path string, expires time.Duration) (string, error)

	// List lists files with the given prefix
	List(ctx context.Context, prefix string) ([]FileInfo, error)

	// Close closes any resources
	Close() error
}

// FileInfo represents metadata about a stored file
type FileInfo struct {
	Path        string
	Size        int64
	ContentType string
	ModifiedAt  time.Time
}

// UploadOption configures upload behavior
type UploadOption func(*UploadConfig)

// UploadConfig holds upload configuration
type UploadConfig struct {
	ContentType string
	Metadata    map[string]string
	Public      bool
}

// WithContentType sets the content type for upload
func WithContentType(ct string) UploadOption {
	return func(c *UploadConfig) {
		c.ContentType = ct
	}
}

// WithMetadata sets custom metadata for upload
func WithMetadata(meta map[string]string) UploadOption {
	return func(c *UploadConfig) {
		c.Metadata = meta
	}
}

// WithPublic sets the file as publicly accessible
func WithPublic(public bool) UploadOption {
	return func(c *UploadConfig) {
		c.Public = public
	}
}

// ApplyOptions applies upload options to config
func ApplyOptions(opts []UploadOption) UploadConfig {
	cfg := UploadConfig{
		ContentType: "application/octet-stream",
		Metadata:    make(map[string]string),
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}
