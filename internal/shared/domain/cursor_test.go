package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		assert.Len(t, page.Items, 2)
		assert.NotNil(t, page.NextCursor)
	})

	t.Run("exactly_limit", func(t *testing.T) {
		items := []Item{
			{ID: "1", Name: "Item 1"},
			{ID: "2", Name: "Item 2"},
		}

		page := NewCursorPage(items, 2, cursorFn)

		assert.False(t, page.HasMore)
		assert.Len(t, page.Items, 2)
		assert.Nil(t, page.NextCursor)
	})

	t.Run("less_than_limit", func(t *testing.T) {
		items := []Item{
			{ID: "1", Name: "Item 1"},
		}

		page := NewCursorPage(items, 5, cursorFn)

		assert.False(t, page.HasMore)
		assert.Len(t, page.Items, 1)
		assert.Nil(t, page.NextCursor)
	})

	t.Run("empty", func(t *testing.T) {
		var items []Item

		page := NewCursorPage(items, 10, cursorFn)

		assert.False(t, page.HasMore)
		assert.Empty(t, page.Items)
		assert.Nil(t, page.NextCursor)
	})
}

func TestCursorPage_WithTotal(t *testing.T) {
	type Item struct{ ID string }

	page := CursorPage[Item]{
		Items: []Item{{ID: "1"}},
		PaginationMeta: PaginationMeta{
			HasMore: false,
		},
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

func TestNewBidirectionalCursorPage(t *testing.T) {
	type Item struct {
		ID   string
		Name string
	}

	cursorFn := func(item Item) *Cursor {
		return &Cursor{LastID: item.ID}
	}

	t.Run("forward_with_more_items", func(t *testing.T) {
		// limit=2, 3 items = has extra -> hasMore=true
		items := []Item{
			{ID: "1", Name: "Item 1"},
			{ID: "2", Name: "Item 2"},
			{ID: "3", Name: "Item 3"},
		}

		page := NewBidirectionalCursorPage(items, 2, "next", false, cursorFn)

		assert.True(t, page.HasMore, "should have more items forward")
		assert.False(t, page.HasPrev, "first page should not have prev")
		assert.Len(t, page.Items, 2, "extra item should be trimmed from end")
		assert.NotNil(t, page.NextCursor)
		assert.Nil(t, page.PrevCursor)
	})

	t.Run("forward_with_cursor_has_prev", func(t *testing.T) {
		// Forward with a cursor means we're not on the first page -> hasPrev=true
		items := []Item{
			{ID: "3", Name: "Item 3"},
			{ID: "4", Name: "Item 4"},
			{ID: "5", Name: "Item 5"},
		}

		page := NewBidirectionalCursorPage(items, 2, "next", true, cursorFn)

		assert.True(t, page.HasMore, "extra item means more")
		assert.True(t, page.HasPrev, "cursor present means prev page exists")
		assert.Len(t, page.Items, 2)
		assert.NotNil(t, page.NextCursor)
		assert.NotNil(t, page.PrevCursor)
	})

	t.Run("forward_exactly_limit", func(t *testing.T) {
		items := []Item{
			{ID: "1", Name: "Item 1"},
			{ID: "2", Name: "Item 2"},
		}

		page := NewBidirectionalCursorPage(items, 2, "next", false, cursorFn)

		assert.False(t, page.HasMore)
		assert.False(t, page.HasPrev)
		assert.Len(t, page.Items, 2)
		assert.Nil(t, page.NextCursor)
		assert.Nil(t, page.PrevCursor)
	})

	t.Run("forward_empty_results", func(t *testing.T) {
		var items []Item

		page := NewBidirectionalCursorPage(items, 10, "next", false, cursorFn)

		assert.False(t, page.HasMore)
		assert.False(t, page.HasPrev)
		assert.Empty(t, page.Items)
		assert.Nil(t, page.NextCursor)
		assert.Nil(t, page.PrevCursor)
	})

	t.Run("backward_with_more_items", func(t *testing.T) {
		// Going backward: extra item means there are items BEFORE
		// Items are returned in order, extra is at the beginning
		items := []Item{
			{ID: "1", Name: "Item 1"}, // extra
			{ID: "2", Name: "Item 2"},
			{ID: "3", Name: "Item 3"},
		}

		page := NewBidirectionalCursorPage(items, 2, "prev", true, cursorFn)

		assert.True(t, page.HasPrev, "extra item backward means more prev items")
		assert.True(t, page.HasMore, "backward with cursor means we came from a later page")
		assert.Len(t, page.Items, 2, "extra item should be trimmed from beginning")
		assert.Equal(t, "2", page.Items[0].ID, "first item after trim should be ID 2")
		assert.NotNil(t, page.NextCursor)
		assert.NotNil(t, page.PrevCursor)
	})

	t.Run("backward_no_extra", func(t *testing.T) {
		// Going backward without extra item: first page reached
		items := []Item{
			{ID: "1", Name: "Item 1"},
			{ID: "2", Name: "Item 2"},
		}

		page := NewBidirectionalCursorPage(items, 2, "prev", true, cursorFn)

		assert.False(t, page.HasPrev, "no extra means no more prev items")
		assert.True(t, page.HasMore, "backward with cursor means later page exists")
		assert.Len(t, page.Items, 2)
		assert.NotNil(t, page.NextCursor)
		assert.Nil(t, page.PrevCursor)
	})

	t.Run("backward_empty", func(t *testing.T) {
		var items []Item

		page := NewBidirectionalCursorPage(items, 10, "prev", true, cursorFn)

		assert.False(t, page.HasPrev)
		assert.True(t, page.HasMore, "backward with cursor means later page exists")
		assert.Empty(t, page.Items)
		assert.Nil(t, page.NextCursor)
		assert.Nil(t, page.PrevCursor)
	})

	t.Run("cursor_direction_is_set_correctly", func(t *testing.T) {
		items := []Item{
			{ID: "1", Name: "Item 1"},
			{ID: "2", Name: "Item 2"},
			{ID: "3", Name: "Item 3"},
		}

		page := NewBidirectionalCursorPage(items, 2, "next", true, cursorFn)

		// Decode next cursor and verify direction
		require.NotNil(t, page.NextCursor)
		nextDecoded, err := DecodeCursor(*page.NextCursor)
		require.NoError(t, err)
		assert.Equal(t, CursorDirectionNext, nextDecoded.Direction)

		// Decode prev cursor and verify direction
		require.NotNil(t, page.PrevCursor)
		prevDecoded, err := DecodeCursor(*page.PrevCursor)
		require.NoError(t, err)
		assert.Equal(t, CursorDirectionPrev, prevDecoded.Direction)
	})
}
