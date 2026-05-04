package usecase

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/textproto"
	"testing"
	"time"

	"github.com/14mdzk/goscratch/internal/module/storage/dto"
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockStorageUseCase is a testify mock satisfying the UseCase interface.
type mockStorageUseCase struct {
	mock.Mock
}

func (m *mockStorageUseCase) Upload(ctx context.Context, file multipart.File, header *multipart.FileHeader, directory string) (*dto.UploadResponse, error) {
	args := m.Called(ctx, file, header, directory)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.UploadResponse), args.Error(1)
}

func (m *mockStorageUseCase) Download(ctx context.Context, path string) (io.ReadCloser, string, error) {
	args := m.Called(ctx, path)
	if args.Get(0) == nil {
		return nil, args.String(1), args.Error(2)
	}
	return args.Get(0).(io.ReadCloser), args.String(1), args.Error(2)
}

func (m *mockStorageUseCase) Delete(ctx context.Context, path string) error {
	return m.Called(ctx, path).Error(0)
}

func (m *mockStorageUseCase) GetURL(ctx context.Context, path string, expires time.Duration) (*dto.FileResponse, error) {
	args := m.Called(ctx, path, expires)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.FileResponse), args.Error(1)
}

func (m *mockStorageUseCase) List(ctx context.Context, prefix string) (*dto.ListFilesResponse, error) {
	args := m.Called(ctx, prefix)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.ListFilesResponse), args.Error(1)
}

// mockStorageAuditor is a simple in-memory auditor for decorator tests.
type mockStorageAuditor struct {
	Entries []port.AuditEntry
}

func (m *mockStorageAuditor) Log(_ context.Context, entry port.AuditEntry) error {
	m.Entries = append(m.Entries, entry)
	return nil
}

func (m *mockStorageAuditor) Query(_ context.Context, _ port.AuditFilter) ([]port.AuditEntry, error) {
	return m.Entries, nil
}

func (m *mockStorageAuditor) Close() error { return nil }

func newTestFileHeader(t *testing.T, name string) (*multipart.FileHeader, multipart.File) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="`+name+`"`)
	h.Set("Content-Type", "application/octet-stream")
	part, _ := writer.CreatePart(h)
	_, _ = part.Write([]byte("payload"))
	_ = writer.Close()

	reader := multipart.NewReader(body, writer.Boundary())
	form, err := reader.ReadForm(int64(body.Len() + 1024))
	if err != nil {
		t.Fatalf("ReadForm: %v", err)
	}
	fh := form.File["file"][0]
	f, err := fh.Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return fh, f
}

func TestStorageAuditDecorator_Upload(t *testing.T) {
	ctx := context.Background()

	t.Run("on success, logs CREATE audit entry with file metadata", func(t *testing.T) {
		inner := new(mockStorageUseCase)
		auditor := &mockStorageAuditor{}
		dec := NewAuditedUseCase(inner, auditor)

		fh, f := newTestFileHeader(t, "doc.pdf")
		defer f.Close()

		resp := &dto.UploadResponse{Path: "uploads/x.pdf", URL: "http://x", Size: 7}
		inner.On("Upload", ctx, f, fh, "uploads").Return(resp, nil)

		got, err := dec.Upload(ctx, f, fh, "uploads")
		assert.NoError(t, err)
		assert.Equal(t, resp, got)
		assert.Len(t, auditor.Entries, 1)
		entry := auditor.Entries[0]
		assert.Equal(t, port.AuditActionCreate, entry.Action)
		assert.Equal(t, "file", entry.Resource)
		assert.Equal(t, "uploads/x.pdf", entry.ResourceID)
		newVal := entry.NewValue.(map[string]any)
		assert.Equal(t, int64(7), newVal["size"])
		assert.Equal(t, "doc.pdf", newVal["original_filename"])
		inner.AssertExpectations(t)
	})

	t.Run("on failure, does NOT log audit entry", func(t *testing.T) {
		inner := new(mockStorageUseCase)
		auditor := &mockStorageAuditor{}
		dec := NewAuditedUseCase(inner, auditor)

		fh, f := newTestFileHeader(t, "doc.pdf")
		defer f.Close()

		inner.On("Upload", ctx, f, fh, "").Return(nil, errors.New("boom"))

		_, err := dec.Upload(ctx, f, fh, "")
		assert.Error(t, err)
		assert.Empty(t, auditor.Entries)
	})
}

func TestStorageAuditDecorator_Delete(t *testing.T) {
	ctx := context.Background()

	t.Run("on success, logs DELETE audit entry", func(t *testing.T) {
		inner := new(mockStorageUseCase)
		auditor := &mockStorageAuditor{}
		dec := NewAuditedUseCase(inner, auditor)

		inner.On("Delete", ctx, "uploads/x.pdf").Return(nil)

		err := dec.Delete(ctx, "uploads/x.pdf")
		assert.NoError(t, err)
		assert.Len(t, auditor.Entries, 1)
		entry := auditor.Entries[0]
		assert.Equal(t, port.AuditActionDelete, entry.Action)
		assert.Equal(t, "file", entry.Resource)
		assert.Equal(t, "uploads/x.pdf", entry.ResourceID)
	})

	t.Run("on failure, does NOT log audit entry", func(t *testing.T) {
		inner := new(mockStorageUseCase)
		auditor := &mockStorageAuditor{}
		dec := NewAuditedUseCase(inner, auditor)

		inner.On("Delete", ctx, "uploads/x.pdf").Return(errors.New("boom"))

		err := dec.Delete(ctx, "uploads/x.pdf")
		assert.Error(t, err)
		assert.Empty(t, auditor.Entries)
	})
}

func TestStorageAuditDecorator_ReadOnly_NoAudit(t *testing.T) {
	ctx := context.Background()

	t.Run("Download delegates without audit", func(t *testing.T) {
		inner := new(mockStorageUseCase)
		auditor := &mockStorageAuditor{}
		dec := NewAuditedUseCase(inner, auditor)

		rc := io.NopCloser(bytes.NewBufferString("data"))
		inner.On("Download", ctx, "x").Return(rc, "application/octet-stream", nil)

		_, _, err := dec.Download(ctx, "x")
		assert.NoError(t, err)
		assert.Empty(t, auditor.Entries)
	})

	t.Run("GetURL delegates without audit", func(t *testing.T) {
		inner := new(mockStorageUseCase)
		auditor := &mockStorageAuditor{}
		dec := NewAuditedUseCase(inner, auditor)

		fr := &dto.FileResponse{Path: "x", URL: "http://x"}
		inner.On("GetURL", ctx, "x", time.Hour).Return(fr, nil)

		_, err := dec.GetURL(ctx, "x", time.Hour)
		assert.NoError(t, err)
		assert.Empty(t, auditor.Entries)
	})

	t.Run("List delegates without audit", func(t *testing.T) {
		inner := new(mockStorageUseCase)
		auditor := &mockStorageAuditor{}
		dec := NewAuditedUseCase(inner, auditor)

		lr := &dto.ListFilesResponse{}
		inner.On("List", ctx, "p").Return(lr, nil)

		_, err := dec.List(ctx, "p")
		assert.NoError(t, err)
		assert.Empty(t, auditor.Entries)
	})
}
