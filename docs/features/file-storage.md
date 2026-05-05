# File Storage

## Overview

File upload, download, deletion, and listing via a pluggable storage adapter (local filesystem or S3). Includes content type validation, file size limits, path traversal protection, and unique filename generation.

## API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/files/upload` | JWT | Upload a file (multipart/form-data) |
| GET | `/api/files` | JWT | List files with optional prefix filter |
| GET | `/api/files/url/*` | JWT | Get a URL for a file (signed URL for S3) |
| GET | `/api/files/download/*` | JWT | Download a file |
| DELETE | `/api/files/*` | JWT | Delete a file |

## Request/Response Examples

### POST /api/files/upload

**Request:** `multipart/form-data`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `file` | file | yes | The file to upload |
| `directory` | string | no | Target directory path |

**Response (201):**
```json
{
  "success": true,
  "data": {
    "path": "uploads/a1b2c3d4-e5f6-7890-abcd-ef1234567890.png",
    "url": "/storage/uploads/a1b2c3d4-e5f6-7890-abcd-ef1234567890.png",
    "size": 204800
  }
}
```

### GET /api/files?prefix=uploads

**Response (200):**
```json
{
  "success": true,
  "data": {
    "files": [
      {
        "path": "uploads/a1b2c3d4.png",
        "size": 204800,
        "content_type": "image/png",
        "mod_time": "2025-01-15T10:30:00Z"
      }
    ]
  }
}
```

### GET /api/files/url/uploads/a1b2c3d4.png?expires=3600

Query parameter `expires` is optional (seconds, default: 3600).

**Response (200):**
```json
{
  "success": true,
  "data": {
    "path": "uploads/a1b2c3d4.png",
    "url": "https://s3.example.com/bucket/uploads/a1b2c3d4.png?X-Amz-Expires=3600&..."
  }
}
```

### GET /api/files/download/uploads/a1b2c3d4.png

**Response:** Binary file stream with `Content-Type` and `Content-Disposition: attachment` headers.

### DELETE /api/files/uploads/a1b2c3d4.png

**Response:** `204 No Content`

## File Constraints

| Constraint | Value |
|-----------|-------|
| Max file size | 10 MB (`DefaultMaxFileSize`) |
| Allowed content types | `image/jpeg`, `image/png`, `image/gif`, `image/webp`, `application/pdf` |

These defaults are set in `domain/file.go` and can be overridden via the
`usecase.Config` struct (`AllowedContentTypes` map).

### Content-Type Detection

The upload pipeline does **not** trust the client-supplied multipart
`Content-Type` header. The first 512 bytes of the upload stream are
passed through `http.DetectContentType`, and the resulting MIME type is
matched against the configured allowlist. Mismatched or
non-allowlisted types are rejected with HTTP `415 Unsupported Media
Type` (`apperr.ErrUnsupportedMediaType`). The buffered prefix is
re-streamed to the storage adapter so no bytes are dropped.

Operators extending the allowlist (for example to accept `text/csv` or
`text/plain` for a data-import endpoint) should override
`Config.AllowedContentTypes` rather than editing
`domain.DefaultAllowedContentTypes`.

## Path Sanitization

File paths are sanitized in two layers:

1. **Usecase layer** (`internal/module/storage/usecase`):
   - Null bytes removed.
   - `filepath.Clean` applied.
   - Leading `/` and `.` stripped.
   - `..` segments rejected.
2. **Adapter layer** (`internal/adapter/storage/local.go`): defense in
   depth — every key is joined onto the configured base path and the
   cleaned absolute result must remain prefixed by the base directory.
   Any key that would resolve outside the base (e.g. via symlinks or a
   bypass of the usecase sanitizer) is rejected with
   `ErrPathEscapesBase` before any filesystem operation runs.

## Configuration

| Key | Env | Default | Description |
|-----|-----|---------|-------------|
| `storage.mode` | `STORAGE_MODE` | `local` | `local` or `s3` |
| `storage.local.base_path` | `STORAGE_LOCAL_PATH` | (none) | Base directory for local storage |
| `storage.s3.endpoint` | `S3_ENDPOINT` | (none) | S3-compatible endpoint |
| `storage.s3.bucket` | `S3_BUCKET` | (none) | S3 bucket name |
| `storage.s3.region` | `S3_REGION` | (none) | AWS region |
| `storage.s3.access_key` | `S3_ACCESS_KEY` | (none) | S3 access key |
| `storage.s3.secret_key` | `S3_SECRET_KEY` | (none) | S3 secret key |

## Architecture

- `internal/module/storage/` - Handler, usecase, DTO, domain
- `internal/adapter/storage/` - Local and S3 adapters implementing `port.Storage`
- Uploaded files get a UUID-based unique name preserving the original extension
- If S3 initialization fails, falls back to local storage automatically

## Audit Logging

State-mutating operations are recorded in `audit_logs` via `port.Auditor`:

| Operation | `action` | `resource` | `resource_id` |
|-----------|----------|------------|---------------|
| Upload | `CREATE` | `file` | uploaded path / object key |
| Delete | `DELETE` | `file` | deleted path / object key |

Read-only operations (`Download`, `GetURL`, `List`) are intentionally not audited to keep the audit log signal high.

## Dependencies

| Port | Adapter | Purpose |
|------|---------|---------|
| `port.Storage` | Local / S3 | All file operations |
| `port.Auditor` | PostgreSQL / NoOp | Upload / Delete audit logging |
