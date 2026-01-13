.PHONY: help dev build test lint clean migrate-up migrate-down migrate-create sqlc wire docker-up docker-down worker worker-build

# Default target
help:
	@echo "Available commands:"
	@echo "  make dev            - Run API in development mode"
	@echo "  make worker         - Run worker in development mode"
	@echo "  make build          - Build API and worker binaries"
	@echo "  make test           - Run tests"
	@echo "  make lint           - Run linter"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make migrate-up     - Run database migrations up"
	@echo "  make migrate-down   - Rollback last migration"
	@echo "  make migrate-create - Create new migration (NAME=migration_name)"
	@echo "  make sqlc           - Generate SQLC code"
	@echo "  make wire           - Generate Wire dependencies"
	@echo "  make docker-up      - Start Docker services"
	@echo "  make docker-down    - Stop Docker services"
	@echo "  make docker-full    - Start all Docker services (including Redis, RabbitMQ)"
	@echo "  make seed           - Seed database with initial data"

# Variables
APP_NAME := goscratch
BINARY := ./bin/$(APP_NAME)
MAIN_PATH := ./cmd/api
DATABASE_URL ?= postgres://postgres:postgres@localhost:5432/goscratch?sslmode=disable

# Development
dev:
	@echo "Starting development server..."
	@go run $(MAIN_PATH)/main.go

# Worker
worker:
	@echo "Starting worker..."
	@go run ./cmd/worker/main.go

worker-build:
	@echo "Building worker..."
	@mkdir -p bin
	@go build -ldflags="-w -s" -o ./bin/worker ./cmd/worker
	@echo "Worker binary built at ./bin/worker"

# Build
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p bin
	@go build -ldflags="-w -s" -o $(BINARY) $(MAIN_PATH)
	@go build -ldflags="-w -s" -o ./bin/worker ./cmd/worker
	@echo "Binaries built at bin/"

# Test
test:
	@echo "Running tests..."
	@go test -v -race -cover ./...

test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Lint
lint:
	@echo "Running linter..."
	@golangci-lint run ./...

# Clean
clean:
	@echo "Cleaning..."
	@rm -rf bin/
	@rm -f coverage.out coverage.html
	@echo "Clean complete"

# Migrations
migrate-up:
	@echo "Running migrations up..."
	@migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	@echo "Rolling back last migration..."
	@migrate -path migrations -database "$(DATABASE_URL)" down 1

migrate-create:
ifndef NAME
	@echo "Error: NAME is required. Usage: make migrate-create NAME=migration_name"
	@exit 1
endif
	@echo "Creating migration: $(NAME)"
	@migrate create -ext sql -dir migrations -seq $(NAME)

migrate-force:
ifndef VERSION
	@echo "Error: VERSION is required. Usage: make migrate-force VERSION=1"
	@exit 1
endif
	@echo "Forcing migration version to $(VERSION)..."
	@migrate -path migrations -database "$(DATABASE_URL)" force $(VERSION)

# SQLC
sqlc:
	@echo "Generating SQLC code..."
	@sqlc generate
	@echo "SQLC generation complete"

# Wire (if using)
wire:
	@echo "Generating Wire dependencies..."
	@cd cmd/api && wire
	@echo "Wire generation complete"

# Docker
docker-up:
	@echo "Starting Docker services..."
	@docker compose up -d postgres
	@echo "Waiting for PostgreSQL to be ready..."
	@sleep 3
	@echo "Docker services started"

docker-down:
	@echo "Stopping Docker services..."
	@docker compose down
	@echo "Docker services stopped"

docker-full:
	@echo "Starting all Docker services..."
	@docker compose --profile full up -d
	@echo "All Docker services started"

docker-observability:
	@echo "Starting observability stack..."
	@docker compose --profile observability up -d
	@echo "Observability stack started"
	@echo "Grafana: http://localhost:3001 (admin/admin)"
	@echo "Prometheus: http://localhost:9090"

docker-tools:
	@echo "Running migrations in Docker..."
	@docker compose --profile tools up migrate

docker-logs:
	@docker compose logs -f

# Database
db-reset:
	@echo "Resetting database..."
	@docker compose down -v postgres
	@docker compose up -d postgres
	@sleep 3
	@make migrate-up
	@echo "Database reset complete"

# Seeding
seed:
	@echo "Seeding database..."
	@go run ./scripts/seed/main.go
	@echo "Seeding complete"

seed-fresh: db-reset seed
	@echo "Fresh seed complete"

# Installation helpers
install-tools:
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	@go install github.com/google/wire/cmd/wire@latest
	@go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	@echo "Tools installed successfully"

# Dependencies
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy
	@echo "Dependencies updated"

# All-in-one setup
setup: install-tools deps docker-up migrate-up
	@echo "Setup complete! Run 'make dev' to start the server."
