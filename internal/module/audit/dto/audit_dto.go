package dto

// ListAuditLogsRequest represents the query parameters for listing audit logs
type ListAuditLogsRequest struct {
	// Pagination
	Cursor string `query:"cursor"`
	Limit  int    `query:"limit" validate:"omitempty,min=1,max=100"`

	// Filters (optional)
	Action     string `query:"action"`
	UserID     string `query:"user_id"`
	Resource   string `query:"resource"`
	ResourceID string `query:"resource_id"`
	StartTime  string `query:"start_time"`
	EndTime    string `query:"end_time"`
}

// AuditLogResponse represents a single audit log entry
type AuditLogResponse struct {
	ID         string         `json:"id"`
	UserID     string         `json:"user_id,omitempty"`
	Action     string         `json:"action"`
	Resource   string         `json:"resource"`
	ResourceID string         `json:"resource_id,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	IPAddress  string         `json:"ip_address,omitempty"`
	UserAgent  string         `json:"user_agent,omitempty"`
	Timestamp  string         `json:"timestamp"`
}
