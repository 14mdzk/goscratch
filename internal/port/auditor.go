package port

import (
	"context"
	"time"
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
	Cursor     string
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

	if userID, ok := ctx.Value("user_id").(string); ok {
		ac.UserID = userID
	}
	if ip, ok := ctx.Value("ip_address").(string); ok {
		ac.IPAddress = ip
	}
	if ua, ok := ctx.Value("user_agent").(string); ok {
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
