package setup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunMigration applies every versioned migration in db/migrations to the target
// database in ascending version order. Atlas authors these files (native,
// forward-only: <version>_<name>.sql); the test runner executes them directly
// via pgx so there is no migration-tool dependency at test time. Each test runs
// against a fresh database that is dropped on cleanup, so forward-only
// application is sufficient.
func RunMigration(pgURL string, t *testing.T) error {
	t.Log("Running database migrations...")

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Test packages run from tests/integration/<pkg>; the project root (and thus
	// db/migrations) is three levels up.
	migrationsDir := filepath.Join(wd, "..", "..", "..", "db", "migrations")

	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.sql"))
	if err != nil {
		return fmt.Errorf("failed to list migration files: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("no migration files found in %s", migrationsDir)
	}
	// Zero-padded sequence prefixes (and later timestamp prefixes) sort
	// lexicographically into the correct apply order.
	sort.Strings(files)

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, pgURL)
	if err != nil {
		return fmt.Errorf("failed to connect for migration: %w", err)
	}
	defer pool.Close()

	for _, file := range files {
		sqlBytes, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", filepath.Base(file), err)
		}

		// No-argument Exec uses the simple query protocol, which allows multiple
		// statements per file (CREATE TABLE + indexes + seed INSERTs).
		if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", filepath.Base(file), err)
		}
	}

	t.Logf("Applied %d migrations successfully", len(files))
	return nil
}
