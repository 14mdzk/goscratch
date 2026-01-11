package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCursor_Encode(t *testing.T) {
	t.Run("valid_cursor", func(t *testing.T) {
		cursor := &Cursor{
			LastID: "123e4567-e89b-12d3-a456-426614174000",
		}

		encoded := cursor.Encode()
		assert.NotEmpty(t, encoded)
	})

	t.Run("nil_cursor", func(t *testing.T) {
		var cursor *Cursor
		encoded := cursor.Encode()
		assert.Empty(t, encoded)
	})

	t.Run("empty_lastid", func(t *testing.T) {
		cursor := &Cursor{LastID: ""}
		encoded := cursor.Encode()
		assert.Empty(t, encoded)
	})

	t.Run("with_last_value", func(t *testing.T) {
		cursor := &Cursor{
			LastID:    "abc123",
			LastValue: "2024-01-01",
		}

		encoded := cursor.Encode()
		assert.NotEmpty(t, encoded)

		// Decode and verify
		decoded, err := DecodeCursor(encoded)
		assert.NoError(t, err)
		assert.Equal(t, "abc123", decoded.LastID)
	})
}

func TestDecodeCursor(t *testing.T) {
	t.Run("valid_encoded", func(t *testing.T) {
		original := &Cursor{LastID: "test-id-123"}
		encoded := original.Encode()

		decoded, err := DecodeCursor(encoded)
		assert.NoError(t, err)
		assert.NotNil(t, decoded)
		assert.Equal(t, "test-id-123", decoded.LastID)
	})

	t.Run("empty_string", func(t *testing.T) {
		decoded, err := DecodeCursor("")
		assert.NoError(t, err)
		assert.Nil(t, decoded)
	})

	t.Run("invalid_base64", func(t *testing.T) {
		_, err := DecodeCursor("not-valid-base64!!!")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid cursor format")
	})

	t.Run("invalid_json", func(t *testing.T) {
		// Valid base64 but invalid JSON
		_, err := DecodeCursor("bm90LWpzb24=") // "not-json" in base64
		assert.Error(t, err)
	})
}

func TestNewCursorPage(t *testing.T) {
	type Item struct {
		ID   string
		Name string
	}

	cursorFn := func(item Item) *Cursor {
		return &Cursor{LastID: item.ID}
	}

	t.Run("with_more_items", func(t *testing.T) {
		// Simulating limit of 2, with 3 items (hasMore = true)
		items := []Item{
			{ID: "1", Name: "Item 1"},
			{ID: "2", Name: "Item 2"},
			{ID: "3", Name: "Item 3"},
		}

		page := NewCursorPage(items, 2, cursorFn)

		assert.True(t, page.HasMore)
		assert.Len(t, page.Data, 2)
		assert.NotNil(t, page.NextCursor)
	})

	t.Run("exactly_limit", func(t *testing.T) {
		items := []Item{
			{ID: "1", Name: "Item 1"},
			{ID: "2", Name: "Item 2"},
		}

		page := NewCursorPage(items, 2, cursorFn)

		assert.False(t, page.HasMore)
		assert.Len(t, page.Data, 2)
		assert.Nil(t, page.NextCursor)
	})

	t.Run("less_than_limit", func(t *testing.T) {
		items := []Item{
			{ID: "1", Name: "Item 1"},
		}

		page := NewCursorPage(items, 5, cursorFn)

		assert.False(t, page.HasMore)
		assert.Len(t, page.Data, 1)
		assert.Nil(t, page.NextCursor)
	})

	t.Run("empty", func(t *testing.T) {
		var items []Item

		page := NewCursorPage(items, 10, cursorFn)

		assert.False(t, page.HasMore)
		assert.Empty(t, page.Data)
		assert.Nil(t, page.NextCursor)
	})
}

func TestCursorPage_WithTotal(t *testing.T) {
	type Item struct{ ID string }

	page := CursorPage[Item]{
		Data:    []Item{{ID: "1"}},
		HasMore: false,
	}

	pageWithTotal := page.WithTotal(100)

	assert.NotNil(t, pageWithTotal.Total)
	assert.Equal(t, int64(100), *pageWithTotal.Total)
}

func TestNormalizeLimit(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{0, DefaultLimit},
		{-1, DefaultLimit},
		{10, 10},
		{50, 50},
		{100, 100},
		{150, MaxLimit},
		{1000, MaxLimit},
	}

	for _, tt := range tests {
		result := NormalizeLimit(tt.input)
		assert.Equal(t, tt.expected, result, "NormalizeLimit(%d) should be %d", tt.input, tt.expected)
	}
}
