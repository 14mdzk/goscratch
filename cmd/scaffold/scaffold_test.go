package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- name validation ---

func TestValidateName(t *testing.T) {
	valid := []string{
		"foo", "bar", "my_module", "module2", "a", "abc123",
	}
	for _, name := range valid {
		t.Run("valid_"+name, func(t *testing.T) {
			assert.NoError(t, validateName(name), "expected %q to be valid", name)
		})
	}

	invalid := []struct {
		name   string
		input  string
		errSub string
	}{
		{"empty", "", "must not be empty"},
		{"starts_with_digit", "1foo", "must match"},
		{"uppercase", "Foo", "must match"},
		{"hyphen", "my-module", "must match"},
		{"reserved_func", "func", "reserved word"},
		{"reserved_for", "for", "reserved word"},
		{"reserved_type", "type", "reserved word"},
		{"reserved_var", "var", "reserved word"},
		{"reserved_interface", "interface", "reserved word"},
	}
	for _, tc := range invalid {
		t.Run("invalid_"+tc.name, func(t *testing.T) {
			err := validateName(tc.input)
			require.Error(t, err, "expected %q to be invalid", tc.input)
			assert.Contains(t, err.Error(), tc.errSub)
		})
	}
}

// --- titleCase ---

func TestTitleCase(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"foo", "Foo"},
		{"my_module", "MyModule"},
		{"abc", "Abc"},
		{"order_item", "OrderItem"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			assert.Equal(t, tc.want, titleCase(tc.in))
		})
	}
}

// --- readModulePath ---

func TestReadModulePath(t *testing.T) {
	dir := t.TempDir()
	gomod := filepath.Join(dir, "go.mod")
	require.NoError(t, os.WriteFile(gomod, []byte("module github.com/example/repo\n\ngo 1.21\n"), 0644))

	path, err := readModulePath(gomod)
	require.NoError(t, err)
	assert.Equal(t, "github.com/example/repo", path)
}

func TestReadModulePath_Missing(t *testing.T) {
	dir := t.TempDir()
	gomod := filepath.Join(dir, "go.mod")
	require.NoError(t, os.WriteFile(gomod, []byte("go 1.21\n"), 0644))

	_, err := readModulePath(gomod)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "module directive not found")
}

// --- idempotency refusal ---

func TestRunModule_IdempotencyRefusal(t *testing.T) {
	// Build a fake repo tree with go.mod and a pre-existing module directory.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module github.com/example/repo\n\ngo 1.21\n"), 0644))

	existing := filepath.Join(dir, "internal", "module", "mypkg")
	require.NoError(t, os.MkdirAll(existing, 0755))

	// Change cwd so findRepoRoot can locate go.mod.
	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(orig) })

	err = runModule("mypkg")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

// --- template rendering produces compilable Go source ---

func TestRenderTemplate_ProducesExpectedContent(t *testing.T) {
	data := templateData{
		Name:       "widget",
		Title:      "Widget",
		ModulePath: "github.com/example/repo",
	}

	tests := []struct {
		tmpl    string
		contain []string
	}{
		{
			"domain.go.tmpl",
			[]string{"package domain", "type Widget struct"},
		},
		{
			"port.go.tmpl",
			[]string{"package usecase", "type UseCase interface", "GetByID"},
		},
		{
			"usecase.go.tmpl",
			[]string{"package usecase", "type widgetUseCase struct", "func NewUseCase()"},
		},
		{
			"handler.go.tmpl",
			[]string{"package handler", "type Handler struct", "func NewHandler("},
		},
		{
			"handler_test.go.tmpl",
			[]string{"package handler", "func TestSmoke_NewHandler"},
		},
		{
			"usecase_test.go.tmpl",
			[]string{"package usecase", "func TestSmoke_NewUseCase"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.tmpl, func(t *testing.T) {
			out, err := renderTemplate(tc.tmpl, data)
			require.NoError(t, err)
			src := string(out)
			for _, needle := range tc.contain {
				assert.True(t, strings.Contains(src, needle),
					"template %s: expected to contain %q\ngot:\n%s", tc.tmpl, needle, src)
			}
		})
	}
}

// --- full scaffold into temp dir ---

func TestRunModule_ScaffoldsFully(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module github.com/example/repo\n\ngo 1.21\n"), 0644))

	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(orig) })

	require.NoError(t, runModule("widget"))

	wantFiles := []string{
		filepath.Join(dir, "internal", "module", "widget", "domain", "widget_domain.go"),
		filepath.Join(dir, "internal", "module", "widget", "usecase", "port.go"),
		filepath.Join(dir, "internal", "module", "widget", "usecase", "widget_usecase.go"),
		filepath.Join(dir, "internal", "module", "widget", "handler", "widget_handler.go"),
		filepath.Join(dir, "internal", "module", "widget", "handler", "widget_handler_test.go"),
		filepath.Join(dir, "internal", "module", "widget", "usecase", "widget_usecase_test.go"),
	}

	for _, f := range wantFiles {
		_, err := os.Stat(f)
		assert.NoError(t, err, "expected file to exist: %s", f)
	}
}
