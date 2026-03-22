package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/14mdzk/goscratch/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// LocalStorage Tests
// =============================================================================

func TestLocalStorage_ImplementsInterface(t *testing.T) {
	var _ port.Storage = (*LocalStorage)(nil)
}

func newTestLocalStorage(t *testing.T) *LocalStorage {
	t.Helper()
	tmpDir := t.TempDir()
	s, err := NewLocalStorage(tmpDir, "http://localhost:8080/files")
	require.NoError(t, err)
	return s
}

func TestLocalStorage_NewLocalStorage_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	nestedDir := filepath.Join(tmpDir, "a", "b", "c")

	s, err := NewLocalStorage(nestedDir, "")
	require.NoError(t, err)
	assert.NotNil(t, s)

	info, err := os.Stat(nestedDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestLocalStorage_Upload_CreatesFileAndDirectories(t *testing.T) {
	s := newTestLocalStorage(t)
	ctx := context.Background()

	data := bytes.NewReader([]byte("file contents"))
	path, err := s.Upload(ctx, "subdir/test.txt", data)
	require.NoError(t, err)
	assert.Equal(t, "subdir/test.txt", path)

	// Verify file exists
	fullPath := filepath.Join(s.basePath, "subdir", "test.txt")
	content, err := os.ReadFile(fullPath)
	require.NoError(t, err)
	assert.Equal(t, "file contents", string(content))
}

func TestLocalStorage_Upload_WithOptions(t *testing.T) {
	s := newTestLocalStorage(t)
	ctx := context.Background()

	data := bytes.NewReader([]byte("content"))
	path, err := s.Upload(ctx, "file.txt", data,
		port.WithContentType("text/plain"),
		port.WithMetadata(map[string]string{"key": "val"}),
		port.WithPublic(true),
	)
	require.NoError(t, err)
	assert.Equal(t, "file.txt", path)
}

func TestLocalStorage_Download_ReadsFile(t *testing.T) {
	s := newTestLocalStorage(t)
	ctx := context.Background()

	// Upload first
	_, err := s.Upload(ctx, "download-me.txt", bytes.NewReader([]byte("read this")))
	require.NoError(t, err)

	// Download
	rc, err := s.Download(ctx, "download-me.txt")
	require.NoError(t, err)
	defer rc.Close()

	content, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, "read this", string(content))
}

func TestLocalStorage_Download_FileNotFound(t *testing.T) {
	s := newTestLocalStorage(t)
	ctx := context.Background()

	_, err := s.Download(ctx, "nonexistent.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file not found")
}

func TestLocalStorage_Delete_RemovesFile(t *testing.T) {
	s := newTestLocalStorage(t)
	ctx := context.Background()

	_, err := s.Upload(ctx, "to-delete.txt", bytes.NewReader([]byte("bye")))
	require.NoError(t, err)

	err = s.Delete(ctx, "to-delete.txt")
	require.NoError(t, err)

	exists, err := s.Exists(ctx, "to-delete.txt")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestLocalStorage_Delete_NonexistentFile_NoError(t *testing.T) {
	s := newTestLocalStorage(t)
	ctx := context.Background()

	err := s.Delete(ctx, "ghost.txt")
	assert.NoError(t, err)
}

func TestLocalStorage_Exists_True(t *testing.T) {
	s := newTestLocalStorage(t)
	ctx := context.Background()

	_, err := s.Upload(ctx, "exists.txt", bytes.NewReader([]byte("here")))
	require.NoError(t, err)

	exists, err := s.Exists(ctx, "exists.txt")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestLocalStorage_Exists_False(t *testing.T) {
	s := newTestLocalStorage(t)
	ctx := context.Background()

	exists, err := s.Exists(ctx, "nope.txt")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestLocalStorage_GetURL_WithBaseURL(t *testing.T) {
	s := newTestLocalStorage(t)
	ctx := context.Background()

	url, err := s.GetURL(ctx, "images/photo.jpg", time.Hour)
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:8080/files/images/photo.jpg", url)
}

func TestLocalStorage_GetURL_WithoutBaseURL(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := NewLocalStorage(tmpDir, "")
	require.NoError(t, err)

	url, err := s.GetURL(context.Background(), "images/photo.jpg", time.Hour)
	require.NoError(t, err)
	assert.Equal(t, "/uploads/images/photo.jpg", url)
}

func TestLocalStorage_GetURL_BaseURLTrailingSlash(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := NewLocalStorage(tmpDir, "http://cdn.example.com/")
	require.NoError(t, err)

	url, err := s.GetURL(context.Background(), "file.txt", time.Hour)
	require.NoError(t, err)
	assert.Equal(t, "http://cdn.example.com/file.txt", url)
}

func TestLocalStorage_List_FindsFiles(t *testing.T) {
	s := newTestLocalStorage(t)
	ctx := context.Background()

	// Upload multiple files
	files := []struct {
		path    string
		content string
	}{
		{"docs/readme.txt", "readme content"},
		{"docs/guide.txt", "guide content"},
		{"docs/sub/deep.txt", "deep content"},
		{"images/photo.jpg", "image data"},
	}

	for _, f := range files {
		_, err := s.Upload(ctx, f.path, bytes.NewReader([]byte(f.content)))
		require.NoError(t, err)
	}

	// List docs directory
	result, err := s.List(ctx, "docs")
	require.NoError(t, err)
	assert.Len(t, result, 3)

	// Verify paths are relative
	paths := make([]string, len(result))
	for i, f := range result {
		paths[i] = f.Path
	}
	assert.Contains(t, paths, filepath.Join("docs", "readme.txt"))
	assert.Contains(t, paths, filepath.Join("docs", "guide.txt"))
	assert.Contains(t, paths, filepath.Join("docs", "sub", "deep.txt"))
}

func TestLocalStorage_List_EmptyPrefix(t *testing.T) {
	s := newTestLocalStorage(t)
	ctx := context.Background()

	_, err := s.Upload(ctx, "file1.txt", bytes.NewReader([]byte("a")))
	require.NoError(t, err)
	_, err = s.Upload(ctx, "file2.txt", bytes.NewReader([]byte("b")))
	require.NoError(t, err)

	result, err := s.List(ctx, "")
	require.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestLocalStorage_List_NonexistentPrefix(t *testing.T) {
	s := newTestLocalStorage(t)
	ctx := context.Background()

	result, err := s.List(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestLocalStorage_List_FileInfo(t *testing.T) {
	s := newTestLocalStorage(t)
	ctx := context.Background()

	content := "hello world"
	_, err := s.Upload(ctx, "info-test.txt", bytes.NewReader([]byte(content)))
	require.NoError(t, err)

	result, err := s.List(ctx, "")
	require.NoError(t, err)
	require.Len(t, result, 1)

	fi := result[0]
	assert.Equal(t, "info-test.txt", fi.Path)
	assert.Equal(t, int64(len(content)), fi.Size)
	assert.False(t, fi.ModifiedAt.IsZero())
}

func TestLocalStorage_Close_ReturnsNil(t *testing.T) {
	s := newTestLocalStorage(t)
	assert.NoError(t, s.Close())
}

func TestLocalStorage_Upload_EmptyFile(t *testing.T) {
	s := newTestLocalStorage(t)
	ctx := context.Background()

	path, err := s.Upload(ctx, "empty.txt", bytes.NewReader([]byte{}))
	require.NoError(t, err)
	assert.Equal(t, "empty.txt", path)

	exists, err := s.Exists(ctx, "empty.txt")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestLocalStorage_Upload_LargeFile(t *testing.T) {
	s := newTestLocalStorage(t)
	ctx := context.Background()

	// 1MB of data
	data := make([]byte, 1024*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	path, err := s.Upload(ctx, "large.bin", bytes.NewReader(data))
	require.NoError(t, err)
	assert.Equal(t, "large.bin", path)

	// Verify roundtrip
	rc, err := s.Download(ctx, "large.bin")
	require.NoError(t, err)
	defer rc.Close()

	downloaded, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, data, downloaded)
}

// =============================================================================
// S3Storage Tests (interface compliance only - no real S3)
// =============================================================================

func TestS3Storage_ImplementsInterface(t *testing.T) {
	var _ port.Storage = (*S3Storage)(nil)
}

func TestS3Storage_Close_ReturnsNil(t *testing.T) {
	s := &S3Storage{
		client:    nil,
		bucket:    "test-bucket",
		presigner: nil,
	}
	err := s.Close()
	assert.NoError(t, err)
}

// =============================================================================
// UploadOption Tests
// =============================================================================

func TestApplyOptions_Defaults(t *testing.T) {
	cfg := port.ApplyOptions(nil)
	assert.Equal(t, "application/octet-stream", cfg.ContentType)
	assert.NotNil(t, cfg.Metadata)
	assert.False(t, cfg.Public)
}

func TestApplyOptions_WithContentType(t *testing.T) {
	cfg := port.ApplyOptions([]port.UploadOption{
		port.WithContentType("image/png"),
	})
	assert.Equal(t, "image/png", cfg.ContentType)
}

func TestApplyOptions_WithMetadata(t *testing.T) {
	meta := map[string]string{"author": "test"}
	cfg := port.ApplyOptions([]port.UploadOption{
		port.WithMetadata(meta),
	})
	assert.Equal(t, meta, cfg.Metadata)
}

func TestApplyOptions_WithPublic(t *testing.T) {
	cfg := port.ApplyOptions([]port.UploadOption{
		port.WithPublic(true),
	})
	assert.True(t, cfg.Public)
}

func TestApplyOptions_MultipleOptions(t *testing.T) {
	cfg := port.ApplyOptions([]port.UploadOption{
		port.WithContentType("text/html"),
		port.WithPublic(true),
		port.WithMetadata(map[string]string{"a": "b"}),
	})
	assert.Equal(t, "text/html", cfg.ContentType)
	assert.True(t, cfg.Public)
	assert.Equal(t, "b", cfg.Metadata["a"])
}
