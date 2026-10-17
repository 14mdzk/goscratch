package port

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/14mdzk/goscratch/pkg/logger"
)

// Auditor defines the interface for audit logging
type Auditor interface {
	// Log records an audit entry
	Log(ctx context.Context, entry AuditEntry) error

	// Query retrieves audit entries with filters
	Query(ctx context.Context, filter AuditFilter) ([]AuditEntry, error)

	// Close closes any resources
	Close() error
}

// AuditEntry represents a single audit log entry
type AuditEntry struct {
	ID         string         `json:"id"`
	UserID     string         `json:"user_id"`
	Action     AuditAction    `json:"action"`
	Resource   string         `json:"resource"` // e.g., "user", "order"
	ResourceID string         `json:"resource_id"`
	OldValue   any            `json:"old_value,omitempty"`
	NewValue   any            `json:"new_value,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	IPAddress  string         `json:"ip_address,omitempty"`
	UserAgent  string         `json:"user_agent,omitempty"`
	Timestamp  time.Time      `json:"timestamp"`
}

// AuditAction represents the type of action being audited
type AuditAction string

const (
	AuditActionCreate AuditAction = "CREATE"
	AuditActionRead   AuditAction = "READ"
	AuditActionUpdate AuditAction = "UPDATE"
	AuditActionDelete AuditAction = "DELETE"
	AuditActionLogin  AuditAction = "LOGIN"
	AuditActionLogout AuditAction = "LOGOUT"
)

// AuditFilter defines filters for querying audit logs
type AuditFilter struct {
	UserID     string
	Action     AuditAction
	Resource   string
	ResourceID string
	StartTime  *time.Time
	EndTime    *time.Time
	Limit      int
	// Cursor is an opaque keyset token minted by EncodeAuditCursor: rows
	// strictly older than the cursor position are returned. Empty = first page.
	Cursor string
}

// EncodeAuditCursor mints an opaque keyset cursor for (createdAt, id).
// The pair orders audit rows deterministically under ORDER BY
// created_at DESC, id DESC.
func EncodeAuditCursor(createdAt time.Time, id string) string {
	raw := createdAt.UTC().Format(time.RFC3339Nano) + "|" + id
	return base64.URLEncoding.EncodeToString([]byte(raw))
}

// ErrInvalidAuditCursor marks cursor-decode failures. The adapter surfaces
// them unchanged so callers can map them to 400 with errors.Is.
var ErrInvalidAuditCursor = errors.New("invalid audit cursor")

// DecodeAuditCursor parses a cursor minted by EncodeAuditCursor.
func DecodeAuditCursor(cursor string) (time.Time, string, error) {
	raw, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("%w: %w", ErrInvalidAuditCursor, err)
	}
	ts, id, ok := strings.Cut(string(raw), "|")
	if !ok || id == "" {
		return time.Time{}, "", fmt.Errorf("%w: malformed payload", ErrInvalidAuditCursor)
	}
	if _, err := uuid.Parse(id); err != nil {
		return time.Time{}, "", fmt.Errorf("%w: %w", ErrInvalidAuditCursor, err)
	}
	createdAt, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("%w: %w", ErrInvalidAuditCursor, err)
	}
	return createdAt, id, nil
}

// AuditContext extracts audit-relevant information from context
type AuditContext struct {
	UserID    string
	IPAddress string
	UserAgent string
}

// ExtractAuditContext extracts audit context from request context
func ExtractAuditContext(ctx context.Context) AuditContext {
	ac := AuditContext{}

	if userID, ok := ctx.Value(logger.UserIDKey).(string); ok {
		ac.UserID = userID
	}
	if ip, ok := ctx.Value(logger.IPAddressKey).(string); ok {
		ac.IPAddress = ip
	}
	if ua, ok := ctx.Value(logger.UserAgentKey).(string); ok {
		ac.UserAgent = ua
	}

	return ac
}

// NewAuditEntry creates a new audit entry with context
func NewAuditEntry(ctx context.Context, action AuditAction, resource, resourceID string) AuditEntry {
	ac := ExtractAuditContext(ctx)
	return AuditEntry{
		UserID:     ac.UserID,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		IPAddress:  ac.IPAddress,
		UserAgent:  ac.UserAgent,
		Timestamp:  time.Now(),
	}
}
