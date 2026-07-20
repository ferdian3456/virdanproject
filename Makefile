export $(shell sed 's/=.*//' .env)
include .env

# Database Migrations (Atlas) — edit db/schema.sql, then `make migrate-diff name=xyz`
ATLAS_ENV := local

.PHONY: migrate-diff
migrate-diff:
	@atlas migrate diff $(name) --env $(ATLAS_ENV)

.PHONY: migrate-apply
migrate-apply:
	@atlas migrate apply --env $(ATLAS_ENV) --url "$(POSTGRES_URL)"

.PHONY: migrate-lint
migrate-lint:
	@atlas migrate lint --env $(ATLAS_ENV) --latest 1

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
