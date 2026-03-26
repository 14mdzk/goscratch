# Email

## Overview

Email sending capability via a pluggable adapter pattern. The SMTP adapter sends real emails; the NoOp adapter logs email details without sending, making it safe for development and testing.

## API Endpoints

There are no dedicated HTTP endpoints for email. Email is sent programmatically from within the application (e.g., from background job handlers) using the `port.EmailSender` interface.

## Email Message Structure

```go
type EmailMessage struct {
    To      []string  // Required: recipient addresses
    CC      []string  // Optional: CC addresses
    BCC     []string  // Optional: BCC addresses
    Subject string    // Email subject line
    Body    string    // Email body (plain text or HTML)
    IsHTML  bool      // Whether Body is HTML
}
```

**Example usage in a job handler:**
```json
{
  "to": ["user@example.com"],
  "cc": ["manager@example.com"],
  "subject": "Welcome to Goscratch",
  "body": "<h1>Welcome!</h1><p>Your account is ready.</p>",
  "is_html": true
}
```

## Adapters

### SMTP Adapter (`email.SMTPSender`)

- Connects to an SMTP server using Go's `net/smtp` package
- Supports plain auth when username is configured
- Builds RFC-compliant email messages with proper headers
- Sets `MIME-Version` and `Content-Type` headers for HTML emails

### NoOp Adapter (`email.NoOpSender`)

- Logs all email details (recipients, subject, body length) at INFO level
- Returns `nil` error (always succeeds)
- Used automatically when `email.enabled` is `false`

## Configuration

| Key | Env | Default | Description |
|-----|-----|---------|-------------|
| `email.enabled` | `EMAIL_ENABLED` | `false` | Enable SMTP email sending |
| `email.host` | `EMAIL_HOST` | (none) | SMTP server host |
| `email.port` | `EMAIL_PORT` | (none) | SMTP server port |
| `email.username` | `EMAIL_USERNAME` | (none) | SMTP auth username |
| `email.password` | `EMAIL_PASSWORD` | (none) | SMTP auth password |
| `email.from` | `EMAIL_FROM` | (none) | Sender address |

## Architecture

- `internal/port/email.go` - `EmailSender` interface definition
- `internal/adapter/email/smtp.go` - SMTP implementation
- `internal/adapter/email/noop.go` - NoOp implementation
- Wired in `app.go`: if `email.enabled` is true, creates `SMTPSender`; otherwise `NoOpSender`

## Dependencies

No external port dependencies. The email adapter is itself a port implementation consumed by other modules (e.g., background job worker).
