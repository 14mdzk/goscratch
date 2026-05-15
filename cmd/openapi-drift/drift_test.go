package main

import (
	"strings"
	"testing"
)

// --- normalizePath tests ---

func TestNormalizePath_NamedParam(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "fiber_named_param",
			input: "/users/:id",
			want:  "/users/{*}",
		},
		{
			name:  "fiber_two_named_params",
			input: "/roles/:role/permissions",
			want:  "/roles/{*}/permissions",
		},
		{
			name:  "openapi_named_param",
			input: "/users/{id}",
			want:  "/users/{*}",
		},
		{
			name:  "openapi_named_param_midpath",
			input: "/roles/{role}/users",
			want:  "/roles/{*}/users",
		},
		{
			name:  "no_params",
			input: "/auth/login",
			want:  "/auth/login",
		},
		{
			name:  "root_path",
			input: "/",
			want:  "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizePath(tt.input)
			if got != tt.want {
				t.Errorf("normalizePath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizePath_Wildcard(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "fiber_trailing_wildcard",
			input: "/files/download/*",
			want:  "/files/download/{*}",
		},
		{
			name:  "fiber_wildcard_at_root_of_group",
			input: "/files/*",
			want:  "/files/{*}",
		},
		{
			name:  "openapi_path_param_acts_as_wildcard",
			input: "/files/download/{path}",
			want:  "/files/download/{*}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizePath(tt.input)
			if got != tt.want {
				t.Errorf("normalizePath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizePath_TrailingSlash(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "strip_trailing_slash",
			input: "/files/",
			want:  "/files",
		},
		{
			name:  "keep_root_slash",
			input: "/",
			want:  "/",
		},
		{
			name:  "no_trailing_slash",
			input: "/files/upload",
			want:  "/files/upload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizePath(tt.input)
			if got != tt.want {
				t.Errorf("normalizePath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizePath_Mixed(t *testing.T) {
	// Ensure Fiber :param and OpenAPI {param} normalize to the same key.
	fiber := normalizePath("/users/:id/permissions/check")
	openapi := normalizePath("/users/{id}/permissions/check")
	if fiber != openapi {
		t.Errorf("fiber(%q) != openapi(%q): %q vs %q",
			"/users/:id/permissions/check", "/users/{id}/permissions/check",
			fiber, openapi)
	}
}

// --- isExcluded tests ---

func TestIsExcluded(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/docs", true},
		{"/docs/openapi.yaml", true},
		{"/docs/", true},
		{"/metrics", true},
		{"/metrics/", true},
		{"/health", false},
		{"/auth/login", false},
		{"/users", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := isExcluded(tt.path)
			if got != tt.want {
				t.Errorf("isExcluded(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// --- diff tests ---

func TestDiff_Clean(t *testing.T) {
	fiber := map[string]struct{}{"GET /users": {}, "POST /auth/login": {}}
	spec := map[string]struct{}{"GET /users": {}, "POST /auth/login": {}}
	missing, extra := diff(fiber, spec)
	if len(missing) != 0 {
		t.Errorf("expected no missing routes, got %v", missing)
	}
	if len(extra) != 0 {
		t.Errorf("expected no extra routes, got %v", extra)
	}
}

func TestDiff_MissingFromSpec(t *testing.T) {
	fiber := map[string]struct{}{"GET /users": {}, "POST /new-route": {}}
	spec := map[string]struct{}{"GET /users": {}}
	missing, extra := diff(fiber, spec)
	if len(missing) != 1 || missing[0] != "POST /new-route" {
		t.Errorf("expected missing=[POST /new-route], got %v", missing)
	}
	if len(extra) != 0 {
		t.Errorf("expected no extra routes, got %v", extra)
	}
}

func TestDiff_ExtraInSpec(t *testing.T) {
	fiber := map[string]struct{}{"GET /users": {}}
	spec := map[string]struct{}{"GET /users": {}, "DELETE /old-route": {}}
	missing, extra := diff(fiber, spec)
	if len(missing) != 0 {
		t.Errorf("expected no missing routes, got %v", missing)
	}
	if len(extra) != 1 || extra[0] != "DELETE /old-route" {
		t.Errorf("expected extra=[DELETE /old-route], got %v", extra)
	}
}

func TestDiff_BothDiverge(t *testing.T) {
	fiber := map[string]struct{}{"GET /users": {}, "POST /fiber-only": {}}
	spec := map[string]struct{}{"GET /users": {}, "PUT /spec-only": {}}
	missing, extra := diff(fiber, spec)
	if len(missing) != 1 || missing[0] != "POST /fiber-only" {
		t.Errorf("unexpected missing: %v", missing)
	}
	if len(extra) != 1 || extra[0] != "PUT /spec-only" {
		t.Errorf("unexpected extra: %v", extra)
	}
}

func TestDiff_SortedOutput(t *testing.T) {
	fiber := map[string]struct{}{
		"GET /z-route":  {},
		"GET /a-route":  {},
		"POST /m-route": {},
	}
	spec := map[string]struct{}{}
	missing, _ := diff(fiber, spec)
	if len(missing) != 3 {
		t.Fatalf("expected 3 missing routes, got %d", len(missing))
	}
	// Verify sorted
	for i := 1; i < len(missing); i++ {
		if missing[i] < missing[i-1] {
			t.Errorf("output not sorted: %v", missing)
		}
	}
}

// --- Integration: collectFiberRoutes should include known routes ---

func TestCollectFiberRoutes_ContainsExpectedRoutes(t *testing.T) {
	routes, err := collectFiberRoutes()
	if err != nil {
		t.Fatalf("collectFiberRoutes() error: %v", err)
	}

	expected := []string{
		"GET /health",
		"GET /healthz/live",
		"GET /healthz/ready",
		"POST /auth/login",
		"POST /auth/refresh",
		"POST /auth/logout",
		"GET /users",
		"POST /users",
		"GET /users/{*}",
		"PUT /users/{*}",
		"DELETE /users/{*}",
		"GET /files",
		"POST /files/upload",
		"GET /files/url/{*}",
		"GET /files/download/{*}",
		"DELETE /files/{*}",
		"GET /roles",
		"POST /jobs/dispatch",
		"GET /jobs/types",
		"GET /sse/subscribe",
	}

	for _, want := range expected {
		if _, ok := routes[want]; !ok {
			t.Errorf("expected route %q not found in collectFiberRoutes() output", want)
		}
	}
}

func TestCollectFiberRoutes_ExcludesDocs(t *testing.T) {
	routes, err := collectFiberRoutes()
	if err != nil {
		t.Fatalf("collectFiberRoutes() error: %v", err)
	}
	for key := range routes {
		// key format: "METHOD /path" — split on first space to isolate the path.
		parts := strings.SplitN(key, " ", 2)
		if len(parts) == 2 && (parts[1] == "/docs" || strings.HasPrefix(parts[1], "/docs/")) {
			t.Errorf("docs route leaked into route set: %q", key)
		}
	}
}

// --- Flag / spec-path tests ---

func TestDefaultSpecPath(t *testing.T) {
	if defaultSpecPath != "internal/module/docs/openapi.yaml" {
		t.Errorf("defaultSpecPath = %q, want %q",
			defaultSpecPath, "internal/module/docs/openapi.yaml")
	}
}

func TestCollectSpecRoutes_NonexistentPath(t *testing.T) {
	_, err := collectSpecRoutes("/nonexistent/path/openapi.yaml")
	if err == nil {
		t.Error("expected error for nonexistent spec path, got nil")
	}
}
