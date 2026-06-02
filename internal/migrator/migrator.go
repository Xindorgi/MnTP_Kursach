package migrator

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunUp applies all pending up-migrations from the given directory.
// It uses a schema_migrations table to track which migrations have already been applied.
func RunUp(ctx context.Context, pool *pgxpool.Pool, migrationsDir string) error {
	// Ensure the tracking table exists
	if err := ensureMigrationsTable(ctx, pool); err != nil {
		return fmt.Errorf("failed to ensure schema_migrations table: %w", err)
	}

	// Read all .up.sql files sorted by name
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory %s: %w", migrationsDir, err)
	}

	var upFiles []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".up.sql") {
			upFiles = append(upFiles, e.Name())
		}
	}
	sort.Strings(upFiles)

	if len(upFiles) == 0 {
		log.Println("[migrator] no up migration files found")
		return nil
	}

	// Get already applied migrations
	applied, err := getAppliedMigrations(ctx, pool)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	appliedSet := make(map[string]bool, len(applied))
	for _, m := range applied {
		appliedSet[m] = true
	}

	for _, fname := range upFiles {
		if appliedSet[fname] {
			log.Printf("[migrator] already applied: %s", fname)
			continue
		}

		content, err := os.ReadFile(filepath.Join(migrationsDir, fname))
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", fname, err)
		}

		sql := strings.TrimSpace(string(content))
		if sql == "" {
			log.Printf("[migrator] skipping empty migration: %s", fname)
			continue
		}

		if err := applyMigration(ctx, pool, fname, sql); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", fname, err)
		}

		log.Printf("[migrator] applied: %s", fname)
	}

	return nil
}

// ensureMigrationsTable creates the schema_migrations tracking table if it doesn't exist.
func ensureMigrationsTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename    TEXT        PRIMARY KEY,
			applied_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	return err
}

// getAppliedMigrations returns the list of already applied migration filenames.
func getAppliedMigrations(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	rows, err := pool.Query(ctx, `SELECT filename FROM schema_migrations ORDER BY filename`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		result = append(result, name)
	}
	return result, rows.Err()
}

// applyMigration runs a single migration SQL in a transaction and records it.
func applyMigration(ctx context.Context, pool *pgxpool.Pool, filename, sql string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) // no-op if committed

	if _, err := tx.Exec(ctx, sql); err != nil {
		return fmt.Errorf("migration SQL failed: %w", err)
	}

	if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (filename) VALUES ($1)`, filename); err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	return tx.Commit(ctx)
}
