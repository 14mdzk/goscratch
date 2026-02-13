package domain

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// CursorDirection represents the direction of pagination
type CursorDirection string

const (
	// CursorDirectionNext indicates forward pagination (default)
	CursorDirectionNext CursorDirection = "next"
	// CursorDirectionPrev indicates backward pagination
	CursorDirectionPrev CursorDirection = "prev"
)

// Cursor represents a cursor for pagination
type Cursor struct {
	LastID    string          `json:"last_id"`
	LastValue any             `json:"last_value,omitempty"`
	Direction CursorDirection `json:"direction,omitempty"`
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

// PaginationMeta contains pagination metadata separate from the data
type PaginationMeta struct {
	NextCursor *string `json:"next_cursor,omitempty"`
	PrevCursor *string `json:"prev_cursor,omitempty"`
	HasMore    bool    `json:"has_more"`
	HasPrev    bool    `json:"has_prev"`
	Total      *int64  `json:"total,omitempty"`
}

// CursorPage represents a paginated response with cursor
type CursorPage[T any] struct {
	Items []T
	PaginationMeta
}

// GetItems returns the data items (for use in response)
func (p CursorPage[T]) GetItems() []T {
	return p.Items
}

// GetMeta returns the pagination metadata (for use in response)
func (p CursorPage[T]) GetMeta() PaginationMeta {
	return p.PaginationMeta
}

// NewCursorPage creates a new cursor page from data (forward-only, for backward compatibility)
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
			cursor.Direction = CursorDirectionNext
			encoded := cursor.Encode()
			nextCursor = &encoded
		}
	}

	return CursorPage[T]{
		Items: data,
		PaginationMeta: PaginationMeta{
			NextCursor: nextCursor,
			HasMore:    hasMore,
		},
	}
}

// NewBidirectionalCursorPage creates a cursor page with both next and prev cursors.
// direction indicates which way we're paginating ("prev" for backward, anything else for forward).
// When going forward: extra item → hasMore=true, hasPrev is based on cursor presence.
// When going backward: extra item → hasPrev=true, hasNext=true always (we came from a later page).
func NewBidirectionalCursorPage[T any](data []T, limit int, direction string, hasCursor bool, cursorFn func(T) *Cursor) CursorPage[T] {
	hasExtra := len(data) > limit

	isBackward := direction == "prev"

	var hasMore, hasPrev bool

	if isBackward {
		// Backward: extra item means there are items BEFORE this page
		hasPrev = hasExtra
		// We always have a next page when going backward (we navigated from it)
		hasNext := hasCursor
		hasMore = hasNext

		if hasExtra {
			// Trim the extra item from the BEGINNING (first item is the oldest/extra after reverse)
			data = data[1:]
		}
	} else {
		// Forward: extra item means there are items AFTER this page
		hasMore = hasExtra
		// We have a prev page if we used a cursor (not the first page)
		hasPrev = hasCursor

		if hasExtra {
			// Trim the extra item from the END
			data = data[:limit]
		}
	}

	var nextCursor, prevCursor *string

	if len(data) > 0 {
		// Next cursor - points to the last item
		if hasMore {
			cursor := cursorFn(data[len(data)-1])
			if cursor != nil {
				cursor.Direction = CursorDirectionNext
				encoded := cursor.Encode()
				nextCursor = &encoded
			}
		}

		// Prev cursor - points to the first item
		if hasPrev {
			cursor := cursorFn(data[0])
			if cursor != nil {
				cursor.Direction = CursorDirectionPrev
				encoded := cursor.Encode()
				prevCursor = &encoded
			}
		}
	}

	return CursorPage[T]{
		Items: data,
		PaginationMeta: PaginationMeta{
			NextCursor: nextCursor,
			PrevCursor: prevCursor,
			HasMore:    hasMore,
			HasPrev:    hasPrev,
		},
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
