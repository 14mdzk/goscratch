package audit

import (
	"context"
	"testing"
	"time"

	"github.com/14mdzk/goscratch/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// NoOpAuditor Tests
// =============================================================================

func TestNoOpAuditor_ImplementsInterface(t *testing.T) {
	var _ port.Auditor = (*NoOpAuditor)(nil)
}

func TestNoOpAuditor_Log_ReturnsNil(t *testing.T) {
	a := NewNoOpAuditor()
	entry := port.AuditEntry{
		UserID:     "user-1",
		Action:     port.AuditActionCreate,
		Resource:   "order",
		ResourceID: "order-1",
		Timestamp:  time.Now(),
	}
	err := a.Log(context.Background(), entry)
	assert.NoError(t, err)
}

func TestNoOpAuditor_Query_ReturnsEmptySlice(t *testing.T) {
	a := NewNoOpAuditor()
	entries, err := a.Query(context.Background(), port.AuditFilter{
		UserID: "user-1",
	})
	require.NoError(t, err)
	assert.Empty(t, entries)
	assert.NotNil(t, entries) // Should return empty slice, not nil
}

func TestNoOpAuditor_Close_ReturnsNil(t *testing.T) {
	a := NewNoOpAuditor()
	err := a.Close()
	assert.NoError(t, err)
}

func TestNoOpAuditor_Log_WithAllFields(t *testing.T) {
	a := NewNoOpAuditor()
	entry := port.AuditEntry{
		UserID:     "user-1",
		Action:     port.AuditActionUpdate,
		Resource:   "user",
		ResourceID: "user-2",
		OldValue:   map[string]string{"name": "old"},
		NewValue:   map[string]string{"name": "new"},
		Metadata:   map[string]any{"source": "api"},
		IPAddress:  "192.168.1.1",
		UserAgent:  "test-agent",
		Timestamp:  time.Now(),
	}
	err := a.Log(context.Background(), entry)
	assert.NoError(t, err)
}

func TestNoOpAuditor_Query_WithAllFilters(t *testing.T) {
	a := NewNoOpAuditor()
	now := time.Now()
	entries, err := a.Query(context.Background(), port.AuditFilter{
		UserID:     "user-1",
		Action:     port.AuditActionDelete,
		Resource:   "order",
		ResourceID: "order-1",
		StartTime:  &now,
		EndTime:    &now,
		Limit:      10,
		Cursor:     "abc",
	})
	require.NoError(t, err)
	assert.Empty(t, entries)
}

// =============================================================================
// PostgresAuditor Tests (interface compliance)
// =============================================================================

func TestPostgresAuditor_ImplementsInterface(t *testing.T) {
	var _ port.Auditor = (*PostgresAuditor)(nil)
}

// =============================================================================
// nullString helper Tests
// =============================================================================

func TestNullString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected any
	}{
		{
			name:     "empty string returns nil",
			input:    "",
			expected: nil,
		},
		{
			name:     "non-empty string returns the string",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "whitespace only returns the string",
			input:    "   ",
			expected: "   ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := nullString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// NoOpAuditor Table-Driven Tests
// =============================================================================

func TestNoOpAuditor_Log_AllActions(t *testing.T) {
	a := NewNoOpAuditor()
	ctx := context.Background()

	actions := []port.AuditAction{
		port.AuditActionCreate,
		port.AuditActionRead,
		port.AuditActionUpdate,
		port.AuditActionDelete,
		port.AuditActionLogin,
		port.AuditActionLogout,
	}

	for _, action := range actions {
		t.Run(string(action), func(t *testing.T) {
			entry := port.AuditEntry{
				Action:    action,
				Resource:  "test",
				Timestamp: time.Now(),
			}
			err := a.Log(ctx, entry)
			assert.NoError(t, err)
		})
	}
}

func TestNoOpAuditor_Close_Idempotent(t *testing.T) {
	a := NewNoOpAuditor()
	assert.NoError(t, a.Close())
	assert.NoError(t, a.Close())
}

// =============================================================================
// PostgresAuditor Close Test
// =============================================================================

func TestPostgresAuditor_Close_ReturnsNil(t *testing.T) {
	// PostgresAuditor.Close returns nil because pool is managed externally
	a := &PostgresAuditor{pool: nil}
	err := a.Close()
	assert.NoError(t, err)
}
