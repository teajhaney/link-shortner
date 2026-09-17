package database

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Migrate applies every embedded SQL file that is not already recorded in the
// schema_migrations table. Files run in filename order, each in its own
// transaction, so a failure leaves the schema unchanged and the version table
// honest about what actually applied.
func Migrate(ctx context.Context, pg *Postgres, source fs.FS) error {
	if err := ensureMigrationsTable(ctx, pg); err != nil {
		return err
	}

	applied, err := appliedVersions(ctx, pg)
	if err != nil {
		return err
	}

	names, err := migrationNames(source)
	if err != nil {
		return err
	}

	for _, name := range names {
		if _, ok := applied[name]; ok {
			continue
		}
		if err := applyMigration(ctx, pg, source, name); err != nil {
			return err
		}
		log.Printf("migration applied: %s", name)
	}

	return nil
}

// migrationNames returns the .sql files in source, sorted so that 0001 runs
// before 0002 regardless of the underlying filesystem's ordering.
func migrationNames(source fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(source, ".")
	if err != nil {
		return nil, fmt.Errorf("reading migrations: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	return names, nil
}

func applyMigration(ctx context.Context, pg *Postgres, source fs.FS, name string) error {
	sql, err := fs.ReadFile(source, name)
	if err != nil {
		return fmt.Errorf("reading %s: %w", name, err)
	}

	tx, err := pg.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("starting transaction for %s: %w", name, err)
	}
	// No-op once the transaction has committed; otherwise it rolls the schema
	// change back so a half-applied file is never recorded as done.
	defer tx.Rollback(ctx)

	// A parameterless statement goes through the simple query protocol, which
	// is what lets an entire .sql file run as a single call.
	if _, err := tx.Exec(ctx, string(sql)); err != nil {
		return fmt.Errorf("applying %s: %w", name, err)
	}

	if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, name); err != nil {
		return fmt.Errorf("recording %s: %w", name, err)
	}

	return tx.Commit(ctx)
}

func ensureMigrationsTable(ctx context.Context, pg *Postgres) error {
	const sql = `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`

	_, err := pg.pool.Exec(ctx, sql)
	return err
}

func appliedVersions(ctx context.Context, pg *Postgres) (map[string]struct{}, error) {
	rows, err := pg.pool.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	versions := make(map[string]struct{})
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		versions[version] = struct{}{}
	}

	return versions, rows.Err()
}
