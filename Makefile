export $(shell sed 's/=.*//' .env)
include .env

#Contoh create_users_table
.PHONY: migrate-create
migrate-create:
	@ migrate create -ext sql -dir db/migrations -seq $(name)

.PHONY: migrate-up
migrate-up:
	@ migrate -database ${POSTGRES_URL} -path db/migrations up

.PHONY: migrate-down
migrate-down:
	@ migrate -database ${POSTGRES_URL} -path db/migrations down

.PHONY: migrate-fix
migrate-fix:
	@echo "🔍 Current migration status:"
	@psql ${POSTGRES_URL} -c "SELECT version, dirty FROM schema_migrations;" 2>/dev/null || echo "No schema_migrations table found"
	@echo ""
	@echo "Fixing dirty migration state..."
	@read -p "Enter the version to force (or press Enter to use current dirty version): " version; \
	if [ -z "$$version" ]; then \
		migrate -database ${POSTGRES_URL} -path db/migrations force $$(psql ${POSTGRES_URL} -t -c "SELECT version FROM schema_migrations;" | tr -d ' '); \
	else \
		migrate -database ${POSTGRES_URL} -path db/migrations force $$version; \
	fi
	@echo "Migration state fixed!"

.PHONY: migrate-reset
migrate-reset:
	@echo "This will drop ALL tables and re-run migrations!"
	@read -p "Are you sure? [y/N]: " confirm; \
	if [ "$$confirm" = "y" ]; then \
		migrate -database ${POSTGRES_URL} -path db/migrations drop -f; \
		migrate -database ${POSTGRES_URL} -path db/migrations up; \
		echo "Database reset complete!"; \
	else \
		echo "Aborted."; \
	fi

.PHONY: tools
tools:
	@go run tools.go

# Testing
.PHONY: test
test: test-unit test-integration

.PHONY: test-unit
test-unit:
	@echo "Running unit tests..."
	@go test -short -v ./...

.PHONY: test-integration
test-integration:
	@echo "Running integration tests..."
	@go test -v ./tests/integration/...

.PHONY: test-integration-one
test-integration-one:
	@echo "Running specific integration test..."
	@read -p "Enter test name (e.g., TestSignupStart): " testname; \
	go test -v ./tests/integration/... -run $$testname

.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -short -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Docker management
.PHONY: docker-build docker-build-fast docker-up docker-down docker-rebuild
.PHONY: logs logs-api logs-collector logs-clickstack clean health

# Build variables
BUILD_VERSION ?= dev
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")

# Docker build with proper arguments
docker-build:
	@echo "Building production Docker image..."
	@echo "Version: $(BUILD_VERSION)"
	@echo "Commit: $(GIT_COMMIT)"
	@echo "Branch: $(GIT_BRANCH)"
	@echo "Time: $(BUILD_TIME)"
	DOCKER_BUILDKIT=1 \
	BUILD_VERSION=$(BUILD_VERSION) \
	BUILD_TIME="$(BUILD_TIME)" \
	GIT_COMMIT=$(GIT_COMMIT) \
	docker buildx build --build-arg BUILD_VERSION=$(BUILD_VERSION) \
		--build-arg BUILD_TIME="$(BUILD_TIME)" \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		-f Dockerfile \
		-t virdan-api:latest \
		--load \
		--no-cache \
		.

# Fast build using BuildKit cache
docker-build-fast:
	@echo "Building Docker image (fast mode with cache)..."
	DOCKER_BUILDKIT=1 \
	BUILD_VERSION=$(BUILD_VERSION) \
	BUILD_TIME="$(BUILD_TIME)" \
	GIT_COMMIT=$(GIT_COMMIT) \
	docker buildx build --build-arg BUILD_VERSION=$(BUILD_VERSION) \
		--build-arg BUILD_TIME="$(BUILD_TIME)" \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		-f Dockerfile \
		-t virdan-api:latest \
		--load \
		.

# Start all services
docker-up:
	@echo "Starting all services..."
	docker compose up -d
	@echo "Waiting for services to be ready..."
	@sleep 5
	@echo "Services status:"
	docker compose ps

# Stop all services
docker-down:
	@echo "Stopping all services..."
	docker compose down

# Restart services
docker-restart:
	@echo "Restarting all services..."
	docker compose restart

# Show logs
logs:
	docker compose logs -f --tail=100

# Show logs for specific service
logs-api:
	docker compose logs -f --tail=100 virdan-api

logs-collector:
	docker compose logs -f --tail=100 otel-collector

logs-clickstack:
	docker compose logs -f --tail=100 clickstack

# Clean everything
clean:
	@echo "Cleaning up..."
	docker compose down -v --remove-orphans
	docker system prune -f
	docker volume prune -f

# Full rebuild and restart
rebuild: docker-down docker-build docker-up

# Health check
health:
	@echo "Checking service health..."
	@echo -n "virdan-api: "; docker compose exec -T virdan-api /app/main --healthcheck 2>/dev/null && echo "OK" || echo "FAIL"
	@echo -n "postgres: "; docker compose exec -T postgres pg_isready -U ferdian -d virdanproject >/dev/null 2>&1 && echo "OK" || echo "FAIL"
	@echo -n "redis: "; docker compose exec -T redis redis-cli ping >/dev/null 2>&1 && echo "OK" || echo "FAIL"
	@echo -n "clickstack: "; docker compose exec -T clickstack wget -q -O- http://localhost:8080 >/dev/null 2>&1 && echo "OK" || echo "FAIL"

# Show container stats
stats:
	docker stats --no-stream

# Enter container shell
shell-api:
	docker compose exec virdan-api sh

shell-db:
	docker compose exec postgres psql -U ferdian -d virdanproject

shell-redis:
	docker compose exec redis redis-cli
