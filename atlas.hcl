# Atlas configuration — schema-as-code authoring for Virdan migrations.
#
# Workflow: edit db/schema.sql (desired final state), then run
#   atlas migrate diff <name> --env local
# Atlas writes a versioned, forward-only migration into db/migrations (native
# format: <version>_<name>.sql). The integration-test runner executes these SQL
# files directly via pgx (see tests/integration/setup/migrate.go), so there is
# no migration-tool dependency at test time.
#
# POSTGRES_URL is sourced from .env via the Makefile (or the shell environment
# for direct atlas invocations).

env "local" {
  # Desired schema state — the single source of truth.
  src = "file://db/schema.sql"

  # Target database that `atlas migrate apply` runs against.
  url = getenv("POSTGRES_URL")

  # Ephemeral dev database Atlas spins up (and tears down) to plan and lint
  # diffs. Postgres 15 to match the CI/runtime engine. Requires Docker.
  dev = "docker://postgres/15-alpine/dev"

  migration {
    dir = "file://db/migrations"
  }
}
