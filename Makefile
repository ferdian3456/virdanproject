export $(shell sed 's/=.*//' .env)
include .env

# Database Migrations (Atlas) — edit db/schema.sql, then `make migrate-diff name=xyz`
ATLAS_ENV := local

.PHONY: migrate-diff
migrate-diff:
	@atlas migrate diff $(name) --env $(ATLAS_ENV)

.PHONY: migrate-apply
migrate-apply:
	@atlas migrate apply --env $(ATLAS_ENV)

.PHONY: migrate-lint
migrate-lint:
	@atlas migrate lint --env $(ATLAS_ENV) --latest 1

# Testing
.PHONY: test
test: test-integration

.PHONY: test-integration
test-integration:
	@echo "Running all integration tests..."
	@go test -v ./tests/integration/...

.PHONY: test-auth
test-auth:
	@echo "Running auth integration tests..."
	@go test -v ./tests/integration/auth/...

.PHONY: test-user
test-user:
	@echo "Running user integration tests..."
	@go test -v ./tests/integration/user/...

.PHONY: test-server
test-server:
	@echo "Running server integration tests..."
	@go test -v ./tests/integration/server/...

.PHONY: test-post
test-post:
	@echo "Running post integration tests..."
	@go test -v ./tests/integration/post/...

.PHONY: test-profile
test-profile:
	@echo "Running profile integration tests..."
	@go test -v ./tests/integration/profile/...

.PHONY: test-system
test-system:
	@echo "Running system integration tests..."
	@go test -v ./tests/integration/system/...

.PHONY: test-one
test-one:
	@read -p "Enter test name: " testname; \
	go test -v ./tests/integration/... -run $$testname

.PHONY: test-coverage
test-coverage:
	@echo "Running integration tests with coverage..."
	@go test -coverprofile=coverage.out ./tests/integration/...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

.PHONY: test-list
test-list:
	@echo "Available integration tests:"
	@echo ""
	@echo "Auth tests:"
	@-cd tests/integration/auth && go test -list . 2>/dev/null | grep -E "^Test" || echo "  (no tests found)"
	@echo ""
	@echo "User tests:"
	@-cd tests/integration/user && go test -list . 2>/dev/null | grep -E "^Test" || echo "  (no tests found)"
	@echo ""
	@echo "Server tests:"
	@-cd tests/integration/server && go test -list . 2>/dev/null | grep -E "^Test" || echo "  (no tests found)"
	@echo ""
	@echo "Post tests:"
	@-cd tests/integration/post && go test -list . 2>/dev/null | grep -E "^Test" || echo "  (no tests found)"
	@echo ""
	@echo "Profile tests:"
	@-cd tests/integration/profile && go test -list . 2>/dev/null | grep -E "^Test" || echo "  (no tests found)"
	@echo ""
	@echo "System tests:"
	@-cd tests/integration/system && go test -list . 2>/dev/null | grep -E "^Test" || echo "  (no tests found)"

# CI Pipeline - Run locally before pushing
.PHONY: ci
ci: ci-build ci-lint ci-vuln ci-test
	@echo ""
	@echo "========================================"
	@echo "CI Pipeline Completed Successfully!"
	@echo "========================================"
	@echo "Safe to push your code!"

.PHONY: ci-build
ci-build:
	@echo "========================================"
	@echo "Building binary..."
	@echo "========================================"
	@go build -o app ./cmd/main.go
	@rm -f app
	@echo "Build: OK"

.PHONY: ci-lint
ci-lint:
	@echo ""
	@echo "========================================"
	@echo "Running golangci-lint..."
	@echo "========================================"
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	@golangci-lint run ./...
	@echo "Lint: OK"

.PHONY: ci-vuln
ci-vuln:
	@echo ""
	@echo "========================================"
	@echo "Running vulnerability check..."
	@echo "========================================"
	@which govulncheck > /dev/null || (echo "Installing govulncheck..." && go install golang.org/x/vuln/cmd/govulncheck@latest)
	@govulncheck ./...
	@echo "Vuln check: OK"

.PHONY: ci-test
ci-test:
	@echo ""
	@echo "========================================"
	@echo "Running integration tests..."
	@echo "========================================"
	@go test -v -timeout 15m ./tests/integration/...
	@echo "Integration tests: OK"

# Tools
.PHONY: generate-swagger-id
generate-swagger-id:
	@swag init -g cmd/main.go -o docs --outputTypes yaml -md docs/flows/id

.PHONY: generate-swagger-en
generate-swagger-en:
	@swag init -g cmd/main.go -o docs --outputTypes yaml -md docs/flows/en

.PHONY: help
help:
	@echo "Virdan Project - Available Commands"
	@echo ""
	@echo "CI Pipeline (run locally before pushing):"
	@echo "  make ci               - Run full CI pipeline (build, lint, vuln, test)"
	@echo "  make ci-build         - Build binary only"
	@echo "  make ci-lint          - Run golangci-lint only"
	@echo "  make ci-vuln          - Run vulnerability check only"
	@echo "  make ci-test          - Run integration tests only"
	@echo ""
	@echo "Testing:"
	@echo "  make test              - Run all integration tests"
	@echo "  make test-integration  - Run all integration tests"
	@echo "  make test-auth         - Run auth integration tests"
	@echo "  make test-user         - Run user integration tests"
	@echo "  make test-server       - Run server integration tests"
	@echo "  make test-post         - Run post integration tests"
	@echo "  make test-profile      - Run profile integration tests"
	@echo "  make test-system       - Run system integration tests (health, etc.)"
	@echo "  make test-one          - Run specific test by name"
	@echo "  make test-list         - List all available tests"
	@echo "  make test-coverage     - Generate coverage for integration tests"
	@echo ""
	@echo "Database Migrations (Atlas — edit db/schema.sql first):"
	@echo "  make migrate-diff name=xyz  - Author migration from schema changes"
	@echo "  make migrate-apply          - Apply pending migrations"
	@echo "  make migrate-lint           - Lint latest migration for unsafe changes"
	@echo ""
	@echo "Tools:"
	@echo "  make generate-swagger-id - Generate Swagger docs (Indonesian, docs/flows/id -> docs/swagger.yaml)"
	@echo "  make generate-swagger-en - Generate Swagger docs (English, docs/flows/en -> docs/swagger.yaml)"
