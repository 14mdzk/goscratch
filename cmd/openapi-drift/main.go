// cmd/openapi-drift checks that every route registered in the Fiber app has
// a matching entry in internal/module/docs/openapi.yaml, and vice-versa.
//
// Exit codes:
//
//	0  — routes and spec are in sync
//	1  — drift detected (missing or extra entries printed to stderr)
//
// The binary constructs the Fiber app by calling RegisterRoutes on each module
// with nil-safe or stub dependencies. It does NOT import internal/platform/app
// and does NOT open any network connections (no DB, no Redis, no RabbitMQ).
package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	authmod "github.com/14mdzk/goscratch/internal/module/auth"
	authusecase "github.com/14mdzk/goscratch/internal/module/auth/usecase"
	"github.com/14mdzk/goscratch/internal/module/docs"
	healthmod "github.com/14mdzk/goscratch/internal/module/health"
	jobmod "github.com/14mdzk/goscratch/internal/module/job"
	rolemod "github.com/14mdzk/goscratch/internal/module/role"
	ssemod "github.com/14mdzk/goscratch/internal/module/sse"
	storagemod "github.com/14mdzk/goscratch/internal/module/storage"
	usermod "github.com/14mdzk/goscratch/internal/module/user"
	"github.com/14mdzk/goscratch/internal/platform/config"
	"github.com/gofiber/fiber/v2"
	"gopkg.in/yaml.v3"
)

// defaultSpecPath is the path to the OpenAPI spec relative to the repository
// root. The binary is always invoked from the repo root (make / CI), so this
// relative default resolves correctly without any runtime source-path tricks.
const defaultSpecPath = "internal/module/docs/openapi.yaml"

// excludedPrefixes lists path prefixes that are intentionally absent from the
// OpenAPI spec (infrastructure routes, doc UI). Routes whose path starts with
// any of these are filtered out before the diff.
var excludedPrefixes = []string{
	"/docs",
	"/metrics",
}

// openapiMethods is the set of HTTP methods treated as OpenAPI operation verbs.
// CONNECT and TRACE are excluded: they never appear as real API endpoints in
// this project and would only originate from Fiber's Use() middleware expansion.
var openapiMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true,
}

// reParam matches any OpenAPI-style path parameter segment, e.g. {id} or {path}.
var reParam = regexp.MustCompile(`\{[^}]+\}`)

// routeKey is the canonical comparison unit: uppercase method + " " + normalized path.
type routeKey = string

func main() {
	specPath := flag.String("spec", defaultSpecPath,
		"path to openapi.yaml (relative to cwd or absolute)")
	flag.Parse()

	fiberRoutes, err := collectFiberRoutes()
	if err != nil {
		fmt.Fprintf(os.Stderr, "openapi-drift: collecting Fiber routes: %v\n", err)
		os.Exit(1)
	}

	specRoutes, err := collectSpecRoutes(*specPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "openapi-drift: parsing spec %s: %v\n", *specPath, err)
		os.Exit(1)
	}

	missing, extra := diff(fiberRoutes, specRoutes)
	if len(missing) == 0 && len(extra) == 0 {
		fmt.Println("openapi-drift: clean — routes match spec")
		return
	}

	for _, r := range missing {
		fmt.Fprintf(os.Stderr, "openapi-drift: MISSING from spec:   %s\n", r)
	}
	for _, r := range extra {
		fmt.Fprintf(os.Stderr, "openapi-drift: EXTRA in spec:       %s\n", r)
	}
	os.Exit(1)
}

// --- Fiber route collection ---

// collectFiberRoutes builds a minimal Fiber app, registers all module routes,
// and returns the normalized set of (method, path) route keys.
//
// Filtering rules applied to app.Stack():
//  1. HEAD entries are skipped (Fiber auto-adds HEAD for every GET).
//  2. Only openapiMethods are kept.
//  3. Paths that appear with a CONNECT entry are middleware Use() routes
//     (Fiber registers Use() handlers for all methods including CONNECT); these
//     are skipped to avoid phantom entries that have no real handler.
//  4. Paths in excludedPrefixes (/docs, /metrics) are skipped.
func collectFiberRoutes() (map[routeKey]struct{}, error) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	// Register all modules with nil-safe or zero-value dependencies.
	// Dependencies are only exercised when HTTP handlers are invoked;
	// RegisterRoutes only binds handler closures to path patterns — no I/O.

	healthmod.NewModule(0).RegisterRoutes(app)

	// Auth: typed-nil UserRepo satisfies the interface without a real repo.
	// nil cache causes RateLimit to fall back to the in-memory backend (safe).
	// Zero JWTConfig is fine — Auth middleware only runs on actual requests.
	var userRepo authusecase.UserRepo // typed-nil interface value
	authmod.NewModule(userRepo, nil, nil, config.JWTConfig{}).RegisterRoutes(app)

	// User: nil pool, transactor, auditor, authorizer, cache; empty secret.
	usermod.NewModule(nil, nil, nil, nil, nil, "", nil).RegisterRoutes(app)

	// Role: nil authorizer; empty secret.
	rolemod.NewModule(nil, "").RegisterRoutes(app)

	// Storage: nil storage, auditor; empty secret.
	storagemod.NewModule(nil, nil, "").RegisterRoutes(app)

	// SSE: nil broker, authorizer; empty secret.
	ssemod.NewModule(nil, nil, "").RegisterRoutes(app)

	// Job: nil publisher, auditor, authorizer; empty secret.
	jobmod.NewModule(nil, nil, nil, "").RegisterRoutes(app)

	// Docs: /docs/* routes — excluded by the path filter below.
	docs.NewModule().RegisterRoutes(app)

	// Pass 1: collect paths that have a CONNECT entry; these are Use() middleware
	// registrations and must be excluded.
	middlewarePaths := make(map[string]struct{})
	for _, stack := range app.Stack() {
		for _, r := range stack {
			if r.Method == "CONNECT" {
				middlewarePaths[r.Path] = struct{}{}
			}
		}
	}

	// Pass 2: collect real endpoint routes.
	routes := make(map[routeKey]struct{})
	for _, stack := range app.Stack() {
		for _, r := range stack {
			if r.Method == "HEAD" {
				continue
			}
			if !openapiMethods[strings.ToUpper(r.Method)] {
				continue
			}
			if _, isMiddleware := middlewarePaths[r.Path]; isMiddleware {
				continue
			}
			if isExcluded(r.Path) {
				continue
			}
			key := strings.ToUpper(r.Method) + " " + normalizePath(r.Path)
			routes[key] = struct{}{}
		}
	}
	return routes, nil
}

// isExcluded reports whether a Fiber path starts with any excluded prefix.
func isExcluded(path string) bool {
	for _, pfx := range excludedPrefixes {
		if path == pfx || strings.HasPrefix(path, pfx+"/") {
			return true
		}
	}
	return false
}

// normalizePath converts a route path to a canonical form for comparison.
// Rules:
//   - Trailing slash is stripped (except bare "/").
//   - Fiber named params (:param) → {*}
//   - Fiber wildcard segments (*) → {*}
//   - OpenAPI named params ({name}) → {*}
//
// Using a single placeholder {*} for all parameters means the diff is
// structural: method + path shape, not parameter name spelling.
func normalizePath(path string) string {
	// Strip trailing slash (except root).
	if len(path) > 1 && strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	}
	// Normalize Fiber segments.
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		if seg == "*" || strings.HasPrefix(seg, ":") {
			segments[i] = "{*}"
		}
	}
	path = strings.Join(segments, "/")
	// Normalize OpenAPI {param} to {*}.
	return reParam.ReplaceAllString(path, "{*}")
}

// --- OpenAPI spec parsing ---

// openAPISpec is a minimal representation of an OpenAPI 3.x document.
type openAPISpec struct {
	Paths map[string]map[string]interface{} `yaml:"paths"`
}

// collectSpecRoutes parses the OpenAPI YAML and returns the set of route keys.
func collectSpecRoutes(specPath string) (map[routeKey]struct{}, error) {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return nil, err
	}

	var spec openAPISpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, err
	}

	routes := make(map[routeKey]struct{})
	for path, ops := range spec.Paths {
		normPath := normalizePath(path)
		for method := range ops {
			m := strings.ToUpper(method)
			if !openapiMethods[m] {
				continue // skip x-... or summary/description keys at path level
			}
			key := m + " " + normPath
			routes[key] = struct{}{}
		}
	}
	return routes, nil
}

// --- Set diff ---

// diff returns routes present in fiber but missing from spec (missing), and
// routes present in spec but absent from fiber (extra). Both slices are sorted.
func diff(fiberRoutes, specRoutes map[routeKey]struct{}) (missing, extra []routeKey) {
	for k := range fiberRoutes {
		if _, ok := specRoutes[k]; !ok {
			missing = append(missing, k)
		}
	}
	for k := range specRoutes {
		if _, ok := fiberRoutes[k]; !ok {
			extra = append(extra, k)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	return
}
