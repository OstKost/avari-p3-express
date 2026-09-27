.PHONY: help dev dev-api dev-web build build-api build-web test test-api test-web lint lint-api lint-web clean docker-up docker-down swagger context check check-api check-web check-harness

PYTHON ?= python3

# Default target
help:
	@echo "Available commands:"
	@echo "  make demo-seed / demo-reset / demo-clean - Isolated demo fixtures"
	@echo "  make demo-api / demo-test - Run demo API / fixture tests"
	@echo "  make context      - Compact agent context and working-tree status"
	@echo "  make check-smoke / check-full - P3 smoke / all gates including browser"
	@echo "  make check        - All local/CI gates with compact output and full logs"
	@echo "  make check-api    - Go formatting, vet, race tests and CGO-free build"
	@echo "  make check-web    - ESLint, TypeScript and Vite build (not browser tests)"
	@echo "  make check-harness - Validate instructions, skills, agents and runner"
	@echo "  make dev          - Run both backend and frontend in development mode"
	@echo "  make dev-api      - Run backend server (Go)"
	@echo "  make dev-web      - Run frontend dev server (Vite/React)"
	@echo "  make test         - Run all tests (API & Web)"
	@echo "  make test-api     - Run Go unit & integration tests with race detector"
	@echo "  make test-web     - Run Frontend type checks & tests"
	@echo "  make lint         - Run linters for both API and Web"
	@echo "  make lint-api     - Run golangci-lint on Go code"
	@echo "  make lint-web     - Run ESLint on Web code"
	@echo "  make build        - Build both backend binary and frontend bundle"
	@echo "  make build-api    - Build Go binary"
	@echo "  make build-web    - Build React production bundle"
	@echo "  make swagger      - Generate Swagger/OpenAPI documentation"
	@echo "  make docker-up    - Build and run containers via Docker Compose"
	@echo "  make docker-down  - Stop and remove Docker Compose containers"
	@echo "  make clean        - Remove build artifacts and temporary files"

context:
	@$(PYTHON) scripts/harness.py context

check:
	@$(PYTHON) scripts/harness.py check all

check-api:
	@$(PYTHON) scripts/harness.py check api

check-web:
	@$(PYTHON) scripts/harness.py check web

check-harness:
	@$(PYTHON) scripts/harness.py check harness

# Development
dev:
	@echo "Starting backend and frontend..."
	@mkdir -p apps/api/data
	@(trap 'kill 0' SIGINT; make dev-api & make dev-web & wait)

dev-api:
	@echo "Starting Go API server..."
	@cd apps/api && GOSUMDB=off go run ./cmd/server/main.go

dev-web:
	@echo "Starting React Web application..."
	@cd apps/web && pnpm run dev

# Testing
test: test-api test-web

test-api:
	@echo "Running Go tests with race detector..."
	@cd apps/api && GOSUMDB=off go test -v -race -cover ./...

test-web:
	@echo "Running Web TypeScript check..."
	@cd apps/web && pnpm run build

# Linting
lint: lint-api lint-web

lint-api:
	@echo "Running Go linter..."
	@cd apps/api && GOSUMDB=off go vet ./...

lint-web:
	@echo "Running Web linter..."
	@cd apps/web && pnpm run lint

# Building
build: build-api build-web

build-api:
	@echo "Building Go API binary..."
	@mkdir -p apps/api/bin
	@cd apps/api && GOSUMDB=off CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/server ./cmd/server/main.go

build-web:
	@echo "Building React bundle..."
	@cd apps/web && pnpm run build

# Swagger docs generation
swagger:
	@echo "Generating Swagger documentation..."
	@cd apps/api && if command -v swag > /dev/null; then swag init -g cmd/server/main.go -o docs; else echo "swag not found, install via 'go install github.com/swaggo/swag/cmd/swag@latest'"; fi

# Docker
docker-up:
	@docker compose -f deployments/docker-compose.yml up --build -d

docker-down:
	@docker compose -f deployments/docker-compose.yml down

# Clean
clean:
	@rm -rf apps/api/bin apps/api/tmp apps/web/dist
	@echo "Cleaned build artifacts."

.PHONY: demo-seed demo-reset demo-clean demo-api demo-test

demo-seed:
	@$(PYTHON) scripts/demo.py seed

demo-reset:
	@$(PYTHON) scripts/demo.py reset

demo-clean:
	@$(PYTHON) scripts/demo.py clean

demo-api:
	@$(PYTHON) scripts/demo.py api

demo-test:
	@$(PYTHON) -m unittest discover -s scripts -p test_demo.py

.PHONY: check-smoke check-full check-browser

check-smoke:
	@$(PYTHON) scripts/harness.py check smoke

check-full:
	@$(PYTHON) scripts/harness.py check full

check-browser:
	@$(PYTHON) scripts/harness.py check browser
