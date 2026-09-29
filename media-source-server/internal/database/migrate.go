package database

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func Migrate(ctx context.Context, p *pgxpool.Pool) error {
	dir := os.Getenv("MIGRATIONS_DIR")
	if dir == "" {
		dir = "/app/migrations"
	}
	files, err := filepath.Glob(filepath.Join(dir, "[0-9][0-9][0-9]_*.sql"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no migrations found in %s", dir)
	}
	sort.Strings(files)
	conn, err := p.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	// Serialize migrations across concurrently starting API instances.
	if _, err = conn.Exec(ctx, "SELECT pg_advisory_lock(72104501)"); err != nil {
		return err
	}
	defer conn.Exec(context.Background(), "SELECT pg_advisory_unlock(72104501)")
	if _, err = conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	for _, file := range files {
		name := filepath.Base(file)
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		var applied bool
		if err = conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name=$1)", name).Scan(&applied); err != nil {
			return err
		}
		if applied {
			continue
		}
		body, readErr := os.ReadFile(file)
		if readErr != nil {
			return readErr
		}
		tx, beginErr := conn.Begin(ctx)
		if beginErr != nil {
			return beginErr
		}
		if _, err = tx.Exec(ctx, string(body)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migration %s: %w", name, err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO schema_migrations(name) VALUES($1)", name); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err = tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}
