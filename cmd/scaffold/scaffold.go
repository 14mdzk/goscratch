package main

import (
	"bufio"
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

//go:embed templates/module/*.go.tmpl
var moduleTemplates embed.FS

// goReservedWords is the set of Go reserved keywords that must not be used
// as module names since they would produce uncompilable package-level identifiers.
var goReservedWords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true,
	"default": true, "defer": true, "else": true, "fallthrough": true, "for": true,
	"func": true, "go": true, "goto": true, "if": true, "import": true,
	"interface": true, "map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true, "var": true,
}

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// templateData holds variables available inside every template.
type templateData struct {
	Name       string // lowercase, e.g. "foo"
	Title      string // TitleCase, e.g. "Foo"
	ModulePath string // go.mod module path, e.g. "github.com/14mdzk/goscratch"
}

// runModule is the entry point for the "module" subcommand.
func runModule(name string) error {
	if err := validateName(name); err != nil {
		return err
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		return fmt.Errorf("cannot find repo root (go.mod): %w", err)
	}

	modulePath, err := readModulePath(filepath.Join(repoRoot, "go.mod"))
	if err != nil {
		return fmt.Errorf("cannot read module path from go.mod: %w", err)
	}

	destRoot := filepath.Join(repoRoot, "internal", "module", name)
	if _, err := os.Stat(destRoot); err == nil {
		return fmt.Errorf("directory %s already exists; delete it first if you want to regenerate", destRoot)
	}

	data := templateData{
		Name:       name,
		Title:      titleCase(name),
		ModulePath: modulePath,
	}

	// Map: template file name → output path (relative to destRoot), subdirectory, output filename.
	type fileSpec struct {
		tmplName string
		subDir   string
		outName  string
	}

	specs := []fileSpec{
		{"domain.go.tmpl", "domain", name + "_domain.go"},
		{"port.go.tmpl", "usecase", "port.go"},
		{"usecase.go.tmpl", "usecase", name + "_usecase.go"},
		{"handler.go.tmpl", "handler", name + "_handler.go"},
		{"handler_test.go.tmpl", "handler", name + "_handler_test.go"},
		{"usecase_test.go.tmpl", "usecase", name + "_usecase_test.go"},
	}

	for _, spec := range specs {
		outDir := filepath.Join(destRoot, spec.subDir)
		outPath := filepath.Join(outDir, spec.outName)

		rendered, err := renderTemplate(spec.tmplName, data)
		if err != nil {
			return fmt.Errorf("render %s: %w", spec.tmplName, err)
		}

		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", outDir, err)
		}

		if err := os.WriteFile(outPath, rendered, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", outPath, err)
		}

		fmt.Printf("created %s\n", outPath)
	}

	fmt.Printf("\nModule %q scaffolded at %s\n", name, destRoot)
	fmt.Println("Next steps:")
	fmt.Printf("  1. Review TODO(scaffold) comments in each generated file.\n")
	fmt.Printf("  2. Add domain fields to internal/module/%s/domain/%s_domain.go\n", name, name)
	fmt.Printf("  3. Implement usecase methods in internal/module/%s/usecase/%s_usecase.go\n", name, name)
	fmt.Printf("  4. Wire handler + usecase in internal/platform/app/app.go\n")
	fmt.Printf("  5. Run: go test ./internal/module/%s/...\n", name)

	return nil
}

// validateName checks that name matches the required pattern, is not a Go
// reserved word, and is not the scaffold generator itself.
func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("module name must not be empty")
	}
	if !namePattern.MatchString(name) {
		return fmt.Errorf("module name %q is invalid: must match ^[a-z][a-z0-9_]*$", name)
	}
	if goReservedWords[name] {
		return fmt.Errorf("module name %q is a Go reserved word", name)
	}
	return nil
}

// titleCase converts a lowercase identifier to TitleCase (e.g. "foo" → "Foo").
// Underscores are removed and the following letter is uppercased.
func titleCase(s string) string {
	if s == "" {
		return s
	}
	parts := strings.Split(s, "_")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	return b.String()
}

// findRepoRoot walks up from the current directory until it finds go.mod.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found in any ancestor directory")
		}
		dir = parent
	}
}

// readModulePath reads the module path from a go.mod file.
func readModulePath(goModPath string) (string, error) {
	f, err := os.Open(goModPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	return "", fmt.Errorf("module directive not found in %s", goModPath)
}

// renderTemplate executes the named template with data and returns the result.
func renderTemplate(tmplName string, data templateData) ([]byte, error) {
	tmplPath := "templates/module/" + tmplName
	content, err := moduleTemplates.ReadFile(tmplPath)
	if err != nil {
		return nil, fmt.Errorf("read embedded template %s: %w", tmplPath, err)
	}

	t, err := template.New(tmplName).Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", tmplName, err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute template %s: %w", tmplName, err)
	}

	return buf.Bytes(), nil
}
