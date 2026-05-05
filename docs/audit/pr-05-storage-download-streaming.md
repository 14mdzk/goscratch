# PR #5 — Storage download streaming + path-prefix guard + content-type sniff

Branch: `fix/storage-download-streaming`
Closes: block-ship #13 + 2 should-fix items from `2026-05-02-preship-audit.md`
(content-type sniff for uploads, defense-in-depth path-prefix guard on the
local storage adapter).
Risk: low. Estimate: ~3h.
Status: implemented in worktree, awaiting main-thread review.

## Goal

The storage feature ships three correctness/security bugs:

1. **Empty downloads.** The handler defers `reader.Close()` before invoking
   `c.SendStream(reader)`. fasthttp's `BodyStreamWriter` streams the body
   asynchronously after the handler returns, so the reader is closed before
   any bytes are written. Result: every download returns an empty or
   truncated body in production.
2. **Trusted client Content-Type.** The upload usecase reads the multipart
   `Content-Type` header verbatim and matches it against the allowlist. An
   attacker can upload an executable with `Content-Type: image/png` and
   bypass the allowlist entirely.
3. **No defense-in-depth on the local adapter path join.** The adapter
   `filepath.Join`s the caller-supplied key onto `basePath` without
   verifying the result is still rooted at `basePath`. The usecase
   `sanitizePath` strips `..`, but a future caller that bypasses the
   usecase (or a symlink inside the storage tree) can still escape.

## Findings closed

- **Block-ship #13** — `internal/module/storage/handler/storage_handler.go:58-71`:
  premature `reader.Close()` before async stream.
- **Should-fix** — `internal/module/storage/usecase/storage_usecase.go:61`:
  client-controlled `Content-Type` accepted as truth.
- **Should-fix** — `internal/adapter/storage/local.go:39`: missing
  `HasPrefix(fullPath, basePath)` guard after `filepath.Join`.

## Tasks

### 1. Streaming lifetime fix

- [x] **1.1** Drop `defer reader.Close()` from `Handler.Download` in
      `internal/module/storage/handler/storage_handler.go`. Add a comment
      pointing at the audit finding so the next reader does not "fix" it
      back. fasthttp's `BodyStreamWriter` calls `Close` on the
      `io.ReadCloser` after streaming completes.
- [x] **1.2** Add `TestDownloadHandler_StreamingLifetime`: wires the real
      `Handler` + `usecase.NewUseCase` + `LocalStorage` against a
      `t.TempDir`, uploads a >100 KB known payload, downloads it via
      `app.Test`, and asserts the byte slice is identical. Locks the
      regression.

### 2. Path-prefix guard

- [x] **2.1** Add `LocalStorage.resolveWithinBase(key)` helper that
      `filepath.Join`s the key, `filepath.Clean`s the result, and rejects
      anything not prefixed by `filepath.Clean(basePath) + os.PathSeparator`
      with the new typed `ErrPathEscapesBase` error.
- [x] **2.2** Route `Upload`, `Download`, `Delete`, `Exists`, `List`
      through the helper instead of bare `filepath.Join`.
- [x] **2.3** Add `TestLocalStorage_PathTraversalRejected` covering
      `../../etc/passwd`, sibling-directory escape, and embedded
      `a/b/../../../escape.txt`.

### 3. Content-type sniff

- [x] **3.1** Add `apperr.ErrUnsupportedMediaType` + `CodeUnsupportedMedia`
      + `UnsupportedMediaTypef` helper in `pkg/apperr/error.go` (HTTP 415).
- [x] **3.2** In `storageUseCase.Upload`, wrap the incoming
      `multipart.File` in a `bufio.NewReaderSize(file, 512)`, call
      `Peek(512)` to obtain the sniff window without consuming bytes,
      pass to `http.DetectContentType`, strip any `; charset=...`
      parameters, and validate against `uc.allowedContentTypes`. Reject
      mismatches with `apperr.UnsupportedMediaTypef`.
- [x] **3.3** Pass the buffered reader (not the raw file) to
      `storage.Upload` so the peeked prefix is re-streamed; pass the
      sniffed type via `port.WithContentType`.
- [x] **3.4** Document operator extension knob
      (`Config.AllowedContentTypes`) in a code comment above the sniff.
- [x] **3.5** Update `TestUseCase_Upload/success` and `with_directory` /
      `file_too_large` to use real PNG magic bytes (so the new sniff
      passes); replace the `disallowed_content_type` payload with an MZ
      executable header and assert `apperr.CodeUnsupportedMedia`.
- [x] **3.6** Add `TestUseCase_Upload/sniff_overrides_client_header`:
      multipart header claims `text/plain`, payload starts with the JFIF
      JPEG magic; the storage adapter must receive
      `port.WithContentType("image/jpeg")`.

### 4. Docs

- [x] **4.1** `docs/features/file-storage.md` — document the
      content-type-sniff allowlist behavior, the operator extension
      knob, and the two-layer path sanitization (usecase + adapter).
- [x] **4.2** `CHANGELOG.md` `[Unreleased]` — `Fixed` entry for the
      streaming bug, `Security` entries for the prefix guard and sniff,
      `Added` entry for `apperr.ErrUnsupportedMediaType`.

### 5. Verification

- [x] **5.1** `make lint` clean.
- [x] **5.2** `make test` clean.

### 6. PR description checklist

- [ ] Quote the broken-vs-fixed download example (empty body → full body).
- [ ] Note the operator-must-change item: any caller relying on the
      multipart `Content-Type` to dictate the stored type must now match
      one of the sniffed allowlist entries. Operators can extend
      `Config.AllowedContentTypes` if their workload needs `text/csv` /
      `text/plain`.
- [ ] Link `docs/audit/2026-05-02-preship-audit.md` block-ship #13 plus
      the two cited should-fix lines.

---

## Out of scope (defer to later PRs)

- S3 adapter audit (separate adapter, separate code path; no equivalent
  finding in the punch-list yet — file as a new row if surfaced).
- Per-directory allowlist or virus scanning — explicit overengineer
  guard.
- Migrating the upload pipeline to streaming-with-size-limit (today's
  size check uses `header.Size` which is client-reported). New row in
  `punch-list.md` if we want to harden this; not in PR #5.
- Audit decorator changes — already shipped in PR #1.

## Acceptance

A reviewer running `make lint test` sees both clean. Pulling the branch
and running:

```
curl -sS -o out.bin http://localhost:8080/api/files/download/<key>
```

returns the full byte payload (was previously empty). Attempting an
upload with `Content-Type: image/png` but an `MZ` payload returns HTTP
415. A request with key `../../etc/passwd` against the local adapter
fails with `ErrPathEscapesBase` before any filesystem call.
