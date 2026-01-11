package domain

import (
	"time"

	"github.com/14mdzk/goscratch/pkg/types"
	"github.com/google/uuid"
)

// User represents a user entity
type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Name         string    `json:"name"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// UserFilter contains filter options for listing users with optional filtering
type UserFilter struct {
	// Pagination
	Cursor string
	Limit  int

	// Search filters (optional)
	Search   types.Opt[string] // Search by name or email
	Email    types.Opt[string] // Exact email match
	IsActive types.Opt[bool]   // Filter by active status

	// Sorting
	SortBy    string // Field to sort by (default: created_at)
	SortOrder string // asc or desc (default: desc)
}

// DefaultLimit is the default pagination limit
const DefaultLimit = 20

// MaxLimit is the maximum pagination limit
const MaxLimit = 100

// NormalizeFilter applies defaults to the filter
func (f *UserFilter) NormalizeFilter() {
	if f.Limit <= 0 {
		f.Limit = DefaultLimit
	}
	if f.Limit > MaxLimit {
		f.Limit = MaxLimit
	}
	if f.SortBy == "" {
		f.SortBy = "created_at"
	}
	if f.SortOrder == "" {
		f.SortOrder = "desc"
	}
}
