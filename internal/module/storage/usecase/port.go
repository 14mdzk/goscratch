package usecase

import (
	"context"
	"io"
	"mime/multipart"
	"time"

	"github.com/14mdzk/goscratch/internal/module/storage/dto"
)

// UseCase defines the interface for storage business logic operations.
// Handlers and decorators depend on this interface rather than on the
// concrete *UseCase struct, enabling testability and the audit decorator.
type UseCase interface {
	Upload(ctx context.Context, file multipart.File, header *multipart.FileHeader, directory string) (*dto.UploadResponse, error)
	Download(ctx context.Context, path string) (io.ReadCloser, string, error)
	Delete(ctx context.Context, path string) error
	GetURL(ctx context.Context, path string, expires time.Duration) (*dto.FileResponse, error)
	List(ctx context.Context, prefix string) (*dto.ListFilesResponse, error)
}
