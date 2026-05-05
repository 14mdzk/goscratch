package storage

import (
	"context"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/14mdzk/goscratch/internal/port"
)

// LocalStorage implements port.Storage using the local filesystem
type LocalStorage struct {
	basePath string
	baseURL  string // Optional base URL for serving files
}

// ErrPathEscapesBase is returned when a resolved key would escape the
// configured base directory. This is defense-in-depth: the usecase layer
// already strips `..` segments via sanitizePath, but a bug or future caller
// that bypasses the usecase must still not be able to read files outside
// the storage root.
var ErrPathEscapesBase = fmt.Errorf("storage: path escapes base directory")

// resolveWithinBase joins key onto basePath and verifies the cleaned result
// is still rooted at basePath. Returns the absolute joined path or an error.
func (s *LocalStorage) resolveWithinBase(key string) (string, error) {
	fullPath := filepath.Join(s.basePath, key)
	cleanedFull := filepath.Clean(fullPath)
	cleanedBase := filepath.Clean(s.basePath)
	// Allow the base directory itself (used by List with empty prefix).
	if cleanedFull == cleanedBase {
		return cleanedFull, nil
	}
	if !strings.HasPrefix(cleanedFull, cleanedBase+string(os.PathSeparator)) {
		return "", ErrPathEscapesBase
	}
	return cleanedFull, nil
}

// NewLocalStorage creates a new local storage instance
func NewLocalStorage(basePath, baseURL string) (*LocalStorage, error) {
	// Ensure base path exists
	if err := os.MkdirAll(basePath, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	return &LocalStorage{
		basePath: basePath,
		baseURL:  strings.TrimSuffix(baseURL, "/"),
	}, nil
}

func (s *LocalStorage) Upload(ctx context.Context, path string, data io.Reader, opts ...port.UploadOption) (string, error) {
	cfg := port.ApplyOptions(opts)

	// Build full path with path-prefix guard.
	fullPath, err := s.resolveWithinBase(path)
	if err != nil {
		return "", err
	}

	// Ensure directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Create file
	file, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy data
	if _, err := io.Copy(file, data); err != nil {
		os.Remove(fullPath) // Clean up on error
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	// NOTE: metadata and content type are not stored for local filesystem.
	// For simplicity, we rely on file extension for content type.
	// In production, consider using xattr or a metadata database.
	_ = cfg

	return path, nil
}

func (s *LocalStorage) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	fullPath, err := s.resolveWithinBase(path)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, nil
}

func (s *LocalStorage) Delete(ctx context.Context, path string) error {
	fullPath, err := s.resolveWithinBase(path)
	if err != nil {
		return err
	}

	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			return nil // Already deleted
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

func (s *LocalStorage) Exists(ctx context.Context, path string) (bool, error) {
	fullPath, err := s.resolveWithinBase(path)
	if err != nil {
		return false, err
	}

	_, err = os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check file: %w", err)
	}

	return true, nil
}

func (s *LocalStorage) GetURL(ctx context.Context, path string, expires time.Duration) (string, error) {
	// For local storage, return a relative URL
	// In production, you might want to generate signed URLs
	if s.baseURL != "" {
		return fmt.Sprintf("%s/%s", s.baseURL, path), nil
	}
	return fmt.Sprintf("/uploads/%s", path), nil
}

func (s *LocalStorage) List(ctx context.Context, prefix string) ([]port.FileInfo, error) {
	searchPath, err := s.resolveWithinBase(prefix)
	if err != nil {
		return nil, err
	}
	var files []port.FileInfo

	err = filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(s.basePath, path)
		if err != nil {
			return err
		}

		contentType := mime.TypeByExtension(filepath.Ext(path))
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		files = append(files, port.FileInfo{
			Path:        relPath,
			Size:        info.Size(),
			ContentType: contentType,
			ModifiedAt:  info.ModTime(),
		})

		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	return files, nil
}

func (s *LocalStorage) Close() error {
	return nil // No resources to close
}
