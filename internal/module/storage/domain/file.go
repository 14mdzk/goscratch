package domain

import "time"

// File represents a stored file
type File struct {
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	ContentType string    `json:"content_type"`
	URL         string    `json:"url"`
	ModTime     time.Time `json:"mod_time"`
}

// UploadResult represents the result of a file upload
type UploadResult struct {
	Path string `json:"path"`
	URL  string `json:"url"`
	Size int64  `json:"size"`
}

// DefaultMaxFileSize is the default maximum file size (10MB)
const DefaultMaxFileSize int64 = 10 * 1024 * 1024

// DefaultAllowedContentTypes contains the default allowed content types
var DefaultAllowedContentTypes = map[string]bool{
	"image/jpeg":      true,
	"image/png":       true,
	"image/gif":       true,
	"image/webp":      true,
	"application/pdf": true,
}
