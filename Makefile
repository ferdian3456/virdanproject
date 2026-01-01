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
