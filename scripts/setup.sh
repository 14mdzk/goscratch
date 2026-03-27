#!/usr/bin/env bash
set -euo pipefail

# Goscratch Project Setup Script
# Renames the project for your own use.
#
# Usage: ./scripts/setup.sh <new-project-name> [new-module-path]
# Example: ./scripts/setup.sh myapi github.com/myorg/myapi

OLD_NAME="goscratch"
OLD_MODULE="github.com/14mdzk/goscratch"

NEW_NAME="${1:-}"
NEW_MODULE="${2:-}"

if [ -z "$NEW_NAME" ]; then
    echo "Usage: ./scripts/setup.sh <new-project-name> [new-module-path]"
    echo ""
    echo "Examples:"
    echo "  ./scripts/setup.sh myapi github.com/myorg/myapi"
    echo "  ./scripts/setup.sh myapi  # module path defaults to github.com/youruser/myapi"
    exit 1
fi

if [ -z "$NEW_MODULE" ]; then
    NEW_MODULE="github.com/youruser/$NEW_NAME"
    echo "No module path specified, defaulting to: $NEW_MODULE"
    echo "You can change this later in go.mod"
fi

# Check required tools
for cmd in go sed git find; do
    if ! command -v "$cmd" &>/dev/null; then
        echo "Error: $cmd is required but not found."
        exit 1
    fi
done

echo ""
echo "Renaming project:"
echo "  Name:   $OLD_NAME -> $NEW_NAME"
echo "  Module: $OLD_MODULE -> $NEW_MODULE"
echo ""

# Detect sed variant (macOS vs GNU)
if sed --version 2>/dev/null | grep -q GNU; then
    SED_CMD="sed -i"
else
    SED_CMD="sed -i ''"
fi

# Replace Go module path in all Go files, go.mod, and config files
echo "Replacing module path in source files..."
find . \
    -not -path './.git/*' \
    -not -path './vendor/*' \
    -not -path './bin/*' \
    -not -path './tmp/*' \
    -not -path './scripts/setup.sh' \
    -type f \( -name '*.go' -o -name 'go.mod' -o -name 'go.sum' -o -name '*.yaml' -o -name '*.yml' -o -name '*.json' -o -name '*.toml' -o -name 'Makefile' -o -name 'Dockerfile' -o -name '*.md' -o -name '*.sh' -o -name '*.service' -o -name '*.conf' -o -name '*.env*' \) \
    -exec sh -c "
        if grep -q '$OLD_MODULE' \"\$1\" 2>/dev/null; then
            $SED_CMD 's|$OLD_MODULE|$NEW_MODULE|g' \"\$1\"
            echo \"  Updated module path: \$1\"
        fi
    " _ {} \;

# Replace project name in config and infrastructure files
echo ""
echo "Replacing project name in config files..."
find . \
    -not -path './.git/*' \
    -not -path './vendor/*' \
    -not -path './bin/*' \
    -not -path './tmp/*' \
    -not -path './scripts/setup.sh' \
    -type f \( -name '*.json' -o -name '*.yaml' -o -name '*.yml' -o -name '*.toml' -o -name 'Makefile' -o -name 'Dockerfile' -o -name '*.service' -o -name '*.conf' -o -name '*.env*' -o -name 'docker-compose*' \) \
    -exec sh -c "
        if grep -q '$OLD_NAME' \"\$1\" 2>/dev/null; then
            $SED_CMD 's/$OLD_NAME/$NEW_NAME/g' \"\$1\"
            echo \"  Updated project name: \$1\"
        fi
    " _ {} \;

# Copy .env.example to .env if it doesn't exist
if [ ! -f .env ]; then
    cp .env.example .env
    echo ""
    echo "Created .env from .env.example"
fi

# Run go mod tidy
echo ""
echo "Running go mod tidy..."
go mod tidy

# Remove project-specific history
echo ""
echo "Removing project-specific files..."
rm -f CHANGELOG.md
rm -f docs/ROADMAP.md
rm -f docs/VISION.md

# Reinitialize git
echo ""
echo "Reinitializing git repository..."
rm -rf .git
git init
git add -A
git commit -m "Initial commit (from goscratch starterkit)"

echo ""
echo "Done! Your project '$NEW_NAME' is ready."
echo ""
echo "Next steps:"
echo "  1. Review and update .env with your settings"
echo "  2. make docker-up        # Start PostgreSQL"
echo "  3. make migrate-up       # Run migrations"
echo "  4. make seed             # Seed test data"
echo "  5. make dev              # Start the API server"
echo ""
echo "Visit http://localhost:3000/docs for the API reference."
