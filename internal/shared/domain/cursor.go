package domain

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// Cursor represents a cursor for pagination
type Cursor struct {
	LastID    string `json:"last_id"`
	LastValue any    `json:"last_value,omitempty"`
}

// Encode encodes the cursor to a base64 string
func (c *Cursor) Encode() string {
	if c == nil || c.LastID == "" {
		return ""
	}
	data, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(data)
}

// DecodeCursor decodes a base64 encoded cursor string
func DecodeCursor(encoded string) (*Cursor, error) {
	if encoded == "" {
		return nil, nil
	}

	data, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor format: %w", err)
	}

	var cursor Cursor
	if err := json.Unmarshal(data, &cursor); err != nil {
		return nil, fmt.Errorf("invalid cursor data: %w", err)
	}

	return &cursor, nil
}

// CursorPage represents a paginated response with cursor
type CursorPage[T any] struct {
	Data       []T     `json:"data"`
	NextCursor *string `json:"next_cursor,omitempty"`
	HasMore    bool    `json:"has_more"`
	Total      *int64  `json:"total,omitempty"` // Optional total count
}

// NewCursorPage creates a new cursor page from data
// limit is the requested page size, used to determine if there are more items
func NewCursorPage[T any](data []T, limit int, cursorFn func(T) *Cursor) CursorPage[T] {
	hasMore := len(data) > limit
	if hasMore {
		data = data[:limit] // Remove the extra item used for hasMore check
	}

	var nextCursor *string
	if hasMore && len(data) > 0 {
		cursor := cursorFn(data[len(data)-1])
		if cursor != nil {
			encoded := cursor.Encode()
			nextCursor = &encoded
		}
	}

	return CursorPage[T]{
		Data:       data,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}
}

// WithTotal adds a total count to the cursor page
func (p CursorPage[T]) WithTotal(total int64) CursorPage[T] {
	p.Total = &total
	return p
}

// PaginationParams holds common pagination parameters
type PaginationParams struct {
	Cursor string
	Limit  int
}

// DefaultLimit is the default page size
const DefaultLimit = 20

// MaxLimit is the maximum allowed page size
const MaxLimit = 100

// NormalizeLimit ensures the limit is within valid bounds
func NormalizeLimit(limit int) int {
	if limit <= 0 {
		return DefaultLimit
	}
	if limit > MaxLimit {
		return MaxLimit
	}
	return limit
}
