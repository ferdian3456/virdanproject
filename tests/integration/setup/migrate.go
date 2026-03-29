package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func RunMigration(pgURL string, t *testing.T) error {
	t.Log("Running database migrations...")

	// Get current working directory (changes to test package dir during test execution)
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Go up 3 levels to reach project root
	// tests/integration/auth -> tests/integration -> tests -> virdanproject (3 levels)
	projectRoot := filepath.Join(wd, "..", "..", "..")
	migrationPath := filepath.Join(projectRoot, "db", "migrations")

	// Convert to absolute path
	absPath, err := filepath.Abs(migrationPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Convert to file:// URL format
	migrationURL := "file://" + absPath

	t.Logf("Migration path: %s", migrationURL)

	m, err := migrate.New(
		migrationURL,
		pgURL,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	t.Log("Database migrations completed successfully")
	return nil
}
