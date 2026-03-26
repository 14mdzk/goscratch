package sanitizer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitize_StripsTags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain text unchanged",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "strips script tags",
			input:    `<script>alert("xss")</script>`,
			expected: "",
		},
		{
			name:     "strips HTML tags but keeps text",
			input:    "<b>bold</b> and <i>italic</i>",
			expected: "bold and italic",
		},
		{
			name:     "strips img tags",
			input:    `<img src="x" onerror="alert(1)">`,
			expected: "",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "mixed content",
			input:    `Hello <a href="http://evil.com">click here</a>!`,
			expected: "Hello click here!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Sanitize(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeStruct_BasicStruct(t *testing.T) {
	type Request struct {
		Name  string
		Email string
		Age   int
	}

	req := &Request{
		Name:  `<script>alert("xss")</script>John`,
		Email: "user@example.com",
		Age:   25,
	}

	SanitizeStruct(req)

	assert.Equal(t, "John", req.Name)
	assert.Equal(t, "user@example.com", req.Email)
	assert.Equal(t, 25, req.Age)
}

func TestSanitizeStruct_NestedStruct(t *testing.T) {
	type Address struct {
		Street string
		City   string
	}
	type Person struct {
		Name    string
		Address Address
	}

	p := &Person{
		Name: "<b>Bold Name</b>",
		Address: Address{
			Street: "<script>bad</script>123 Main St",
			City:   "Springfield",
		},
	}

	SanitizeStruct(p)

	assert.Equal(t, "Bold Name", p.Name)
	assert.Equal(t, "123 Main St", p.Address.Street)
	assert.Equal(t, "Springfield", p.Address.City)
}

func TestSanitizeStruct_WithSlice(t *testing.T) {
	type Tags struct {
		Items []string
	}

	s := &Tags{
		Items: []string{"<b>tag1</b>", "tag2", "<script>bad</script>"},
	}

	SanitizeStruct(s)

	assert.Equal(t, "tag1", s.Items[0])
	assert.Equal(t, "tag2", s.Items[1])
	assert.Equal(t, "", s.Items[2])
}

func TestSanitizeStruct_WithPointer(t *testing.T) {
	type Request struct {
		Name *string
	}

	name := "<b>Test</b>"
	req := &Request{Name: &name}

	SanitizeStruct(req)

	assert.Equal(t, "Test", *req.Name)
}

func TestSanitizeStruct_NilPointer(t *testing.T) {
	type Request struct {
		Name *string
	}

	req := &Request{Name: nil}

	// Should not panic
	SanitizeStruct(req)

	assert.Nil(t, req.Name)
}

func TestSanitizeStruct_NonPointerInput(t *testing.T) {
	// Passing a non-pointer struct should not panic (no-op since fields aren't settable)
	type Request struct {
		Name string
	}
	req := Request{Name: "<b>test</b>"}
	SanitizeStruct(req) // no-op, can't set fields
	// original unchanged since we passed by value
	assert.Equal(t, "<b>test</b>", req.Name)
}
