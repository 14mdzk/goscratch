package usecase

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/textproto"
	"testing"
	"time"

	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/pkg/apperr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockStorage is a mock implementation of port.Storage
type MockStorage struct {
	mock.Mock
}

func (m *MockStorage) Upload(ctx context.Context, path string, data io.Reader, opts ...port.UploadOption) (string, error) {
	args := m.Called(ctx, path, data, opts)
	return args.String(0), args.Error(1)
}

func (m *MockStorage) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	args := m.Called(ctx, path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *MockStorage) Delete(ctx context.Context, path string) error {
	args := m.Called(ctx, path)
	return args.Error(0)
}

func (m *MockStorage) Exists(ctx context.Context, path string) (bool, error) {
	args := m.Called(ctx, path)
	return args.Bool(0), args.Error(1)
}

func (m *MockStorage) GetURL(ctx context.Context, path string, expires time.Duration) (string, error) {
	args := m.Called(ctx, path, expires)
	return args.String(0), args.Error(1)
}

func (m *MockStorage) List(ctx context.Context, prefix string) ([]port.FileInfo, error) {
	args := m.Called(ctx, prefix)
	return args.Get(0).([]port.FileInfo), args.Error(1)
}

func (m *MockStorage) Close() error {
	return nil
}

// pngMagic is the 8-byte PNG file signature; http.DetectContentType
// reports "image/png" for any payload starting with these bytes.
var pngMagic = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

// jpegMagic is the standard JFIF JPEG header.
var jpegMagic = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}

// createMultipartFileHeader creates a multipart.FileHeader for testing
func createMultipartFileHeader(filename string, contentType string, content []byte) (*multipart.FileHeader, *bytes.Buffer) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
	h.Set("Content-Type", contentType)

	part, _ := writer.CreatePart(h)
	part.Write(content)
	writer.Close()

	reader := multipart.NewReader(body, writer.Boundary())
	form, _ := reader.ReadForm(int64(len(content) + 1024))
	fh := form.File["file"][0]

	return fh, body
}

func TestUseCase_Upload(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockStorage := new(MockStorage)
		uc := NewUseCase(mockStorage, nil)

		content := append(append([]byte{}, pngMagic...), []byte(" the rest of the file")...)
		fh, _ := createMultipartFileHeader("test.png", "image/png", content)

		// The storage path will contain a UUID, so match any string
		mockStorage.On("Upload", ctx, mock.AnythingOfType("string"), mock.Anything, mock.Anything).
			Return("test-uuid.png", nil)
		mockStorage.On("GetURL", ctx, "test-uuid.png", 24*time.Hour).
			Return("http://localhost/uploads/test-uuid.png", nil)

		file, _ := fh.Open()
		defer file.Close()

		result, err := uc.Upload(ctx, file, fh, "")

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "test-uuid.png", result.Path)
		assert.Equal(t, "http://localhost/uploads/test-uuid.png", result.URL)
		mockStorage.AssertExpectations(t)
	})

	t.Run("file_too_large", func(t *testing.T) {
		mockStorage := new(MockStorage)
		uc := NewUseCase(mockStorage, &Config{MaxFileSize: 10}) // 10 bytes max

		content := append(append([]byte{}, pngMagic...), []byte("this content is definitely more than 10 bytes")...)
		fh, _ := createMultipartFileHeader("test.png", "image/png", content)

		file, _ := fh.Open()
		defer file.Close()

		_, err := uc.Upload(ctx, file, fh, "")

		assert.Error(t, err)
		appErr, ok := apperr.AsAppError(err)
		assert.True(t, ok)
		assert.Equal(t, apperr.CodeBadRequest, appErr.Code)
	})

	t.Run("disallowed_content_type", func(t *testing.T) {
		mockStorage := new(MockStorage)
		uc := NewUseCase(mockStorage, nil)

		// MZ executable header — http.DetectContentType returns
		// "application/octet-stream" which is not on the allowlist.
		content := []byte{'M', 'Z', 0x90, 0x00, 0x03, 0x00, 0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0xFF, 0xFF}
		fh, _ := createMultipartFileHeader("test.exe", "application/x-executable", content)

		file, _ := fh.Open()
		defer file.Close()

		_, err := uc.Upload(ctx, file, fh, "")

		assert.Error(t, err)
		appErr, ok := apperr.AsAppError(err)
		assert.True(t, ok)
		assert.Equal(t, apperr.CodeUnsupportedMedia, appErr.Code)
	})

	t.Run("sniff_overrides_client_header", func(t *testing.T) {
		// Client claims text/plain in the multipart Content-Type header,
		// but the actual bytes are JPEG. The usecase must sniff the real
		// type, accept it (since image/jpeg is allowed), and pass the
		// sniffed type to the storage adapter.
		mockStorage := new(MockStorage)
		uc := NewUseCase(mockStorage, nil)

		content := append(append([]byte{}, jpegMagic...), bytes.Repeat([]byte{0x00}, 64)...)
		fh, _ := createMultipartFileHeader("evil.txt", "text/plain", content)

		mockStorage.On(
			"Upload",
			ctx,
			mock.AnythingOfType("string"),
			mock.Anything,
			mock.MatchedBy(func(opts []port.UploadOption) bool {
				cfg := port.ApplyOptions(opts)
				return cfg.ContentType == "image/jpeg"
			}),
		).Return("sniffed.txt", nil)
		mockStorage.On("GetURL", ctx, "sniffed.txt", 24*time.Hour).Return("http://x/sniffed.txt", nil)

		file, _ := fh.Open()
		defer file.Close()

		result, err := uc.Upload(ctx, file, fh, "")
		assert.NoError(t, err)
		assert.NotNil(t, result)
		mockStorage.AssertExpectations(t)
	})

	t.Run("with_directory", func(t *testing.T) {
		mockStorage := new(MockStorage)
		uc := NewUseCase(mockStorage, nil)

		content := append(append([]byte{}, pngMagic...), []byte("test file content")...)
		fh, _ := createMultipartFileHeader("test.png", "image/png", content)

		mockStorage.On("Upload", ctx, mock.MatchedBy(func(path string) bool {
			return len(path) > len("avatars/") && path[:8] == "avatars/"
		}), mock.Anything, mock.Anything).
			Return("avatars/test-uuid.png", nil)
		mockStorage.On("GetURL", ctx, "avatars/test-uuid.png", 24*time.Hour).
			Return("http://localhost/uploads/avatars/test-uuid.png", nil)

		file, _ := fh.Open()
		defer file.Close()

		result, err := uc.Upload(ctx, file, fh, "avatars")

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "avatars/test-uuid.png", result.Path)
		mockStorage.AssertExpectations(t)
	})
}

func TestUseCase_Download(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockStorage := new(MockStorage)
		uc := NewUseCase(mockStorage, nil)

		content := io.NopCloser(bytes.NewReader([]byte("file data")))
		mockStorage.On("Exists", ctx, "test.png").Return(true, nil)
		mockStorage.On("Download", ctx, "test.png").Return(content, nil)

		reader, contentType, err := uc.Download(ctx, "test.png")

		assert.NoError(t, err)
		assert.NotNil(t, reader)
		assert.Equal(t, "image/png", contentType)
		reader.Close()
		mockStorage.AssertExpectations(t)
	})

	t.Run("not_found", func(t *testing.T) {
		mockStorage := new(MockStorage)
		uc := NewUseCase(mockStorage, nil)

		mockStorage.On("Exists", ctx, "nonexistent.png").Return(false, nil)

		_, _, err := uc.Download(ctx, "nonexistent.png")

		assert.Error(t, err)
		appErr, ok := apperr.AsAppError(err)
		assert.True(t, ok)
		assert.Equal(t, apperr.CodeNotFound, appErr.Code)
		mockStorage.AssertExpectations(t)
	})

	t.Run("path_traversal_blocked", func(t *testing.T) {
		mockStorage := new(MockStorage)
		uc := NewUseCase(mockStorage, nil)

		mockStorage.On("Exists", ctx, "etc/passwd").Return(false, nil)

		_, _, err := uc.Download(ctx, "../../etc/passwd")

		assert.Error(t, err)
		mockStorage.AssertExpectations(t)
	})

	t.Run("empty_path", func(t *testing.T) {
		mockStorage := new(MockStorage)
		uc := NewUseCase(mockStorage, nil)

		_, _, err := uc.Download(ctx, "")

		assert.Error(t, err)
		appErr, ok := apperr.AsAppError(err)
		assert.True(t, ok)
		assert.Equal(t, apperr.CodeBadRequest, appErr.Code)
	})
}

func TestUseCase_Delete(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockStorage := new(MockStorage)
		uc := NewUseCase(mockStorage, nil)

		mockStorage.On("Exists", ctx, "test.png").Return(true, nil)
		mockStorage.On("Delete", ctx, "test.png").Return(nil)

		err := uc.Delete(ctx, "test.png")

		assert.NoError(t, err)
		mockStorage.AssertExpectations(t)
	})

	t.Run("not_found", func(t *testing.T) {
		mockStorage := new(MockStorage)
		uc := NewUseCase(mockStorage, nil)

		mockStorage.On("Exists", ctx, "nonexistent.png").Return(false, nil)

		err := uc.Delete(ctx, "nonexistent.png")

		assert.Error(t, err)
		appErr, ok := apperr.AsAppError(err)
		assert.True(t, ok)
		assert.Equal(t, apperr.CodeNotFound, appErr.Code)
		mockStorage.AssertExpectations(t)
	})
}

func TestUseCase_GetURL(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockStorage := new(MockStorage)
		uc := NewUseCase(mockStorage, nil)

		mockStorage.On("Exists", ctx, "test.png").Return(true, nil)
		mockStorage.On("GetURL", ctx, "test.png", 30*time.Minute).
			Return("http://localhost/uploads/test.png?signed=abc", nil)

		result, err := uc.GetURL(ctx, "test.png", 30*time.Minute)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "test.png", result.Path)
		assert.Contains(t, result.URL, "http://localhost/uploads/test.png")
		mockStorage.AssertExpectations(t)
	})

	t.Run("default_expiry", func(t *testing.T) {
		mockStorage := new(MockStorage)
		uc := NewUseCase(mockStorage, nil)

		mockStorage.On("Exists", ctx, "test.png").Return(true, nil)
		mockStorage.On("GetURL", ctx, "test.png", 1*time.Hour).
			Return("http://localhost/uploads/test.png", nil)

		result, err := uc.GetURL(ctx, "test.png", 0)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		mockStorage.AssertExpectations(t)
	})
}

func TestUseCase_List(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockStorage := new(MockStorage)
		uc := NewUseCase(mockStorage, nil)

		now := time.Now()
		files := []port.FileInfo{
			{Path: "images/test1.png", Size: 1024, ContentType: "image/png", ModifiedAt: now},
			{Path: "images/test2.jpg", Size: 2048, ContentType: "image/jpeg", ModifiedAt: now},
		}
		mockStorage.On("List", ctx, "images").Return(files, nil)

		result, err := uc.List(ctx, "images")

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Files, 2)
		assert.Equal(t, "images/test1.png", result.Files[0].Path)
		assert.Equal(t, int64(1024), result.Files[0].Size)
		mockStorage.AssertExpectations(t)
	})

	t.Run("empty", func(t *testing.T) {
		mockStorage := new(MockStorage)
		uc := NewUseCase(mockStorage, nil)

		mockStorage.On("List", ctx, "").Return([]port.FileInfo{}, nil)

		result, err := uc.List(ctx, "")

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Empty(t, result.Files)
		mockStorage.AssertExpectations(t)
	})
}

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"normal_path", "images/photo.png", "images/photo.png"},
		{"path_traversal", "../../etc/passwd", "etc/passwd"},
		{"leading_slash", "/etc/passwd", "etc/passwd"},
		{"double_dots", "../../../secret.txt", "secret.txt"},
		{"mixed_traversal", "images/../../secret.txt", "secret.txt"},
		{"empty_path", "", ""},
		{"dot_only", ".", ""},
		{"null_bytes", "test\x00.png", "test.png"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizePath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
