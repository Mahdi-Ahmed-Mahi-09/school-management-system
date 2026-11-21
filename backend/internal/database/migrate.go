package database

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// migrationLockID is an arbitrary constant used with pg_advisory_lock so that
// two processes never apply migrations concurrently.
const migrationLockID = 7_310_442_901

// Migration is a single versioned schema change.
type Migration struct {
	Version string
	Up      string
	Down    string
}

// MigrationStatus reports whether a migration has been applied.
type MigrationStatus struct {
	Version string
	Applied bool
}

func loadMigrations() ([]Migration, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, err
	}
	byVersion := map[string]*Migration{}
	for _, e := range entries {
		name := e.Name()
		var version, direction string
		switch {
		case strings.HasSuffix(name, ".up.sql"):
			version, direction = strings.TrimSuffix(name, ".up.sql"), "up"
		case strings.HasSuffix(name, ".down.sql"):
			version, direction = strings.TrimSuffix(name, ".down.sql"), "down"
		default:
			continue
		}
		body, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			return nil, err
		}
		m, ok := byVersion[version]
		if !ok {
			m = &Migration{Version: version}
			byVersion[version] = m
		}
		if direction == "up" {
			m.Up = string(body)
		} else {
			m.Down = string(body)
		}
	}

	out := make([]Migration, 0, len(byVersion))
	for _, m := range byVersion {
		if m.Up == "" {
			return nil, fmt.Errorf("migration %s has no .up.sql file", m.Version)
		}
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

// withLock runs fn on a dedicated connection holding the migration lock.
func withLock(ctx context.Context, pool *pgxpool.Pool, fn func(conn *pgxpool.Conn) error) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLockID); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", migrationLockID) //nolint:errcheck

	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	return fn(conn)
}

func appliedVersions(ctx context.Context, conn *pgxpool.Conn) (map[string]bool, error) {
	rows, err := conn.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	versions, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	applied := make(map[string]bool, len(versions))
	for _, v := range versions {
		applied[v] = true
	}
	return applied, nil
}

// MigrateUp applies every pending migration, each in its own transaction.
// It never drops or recreates existing data. It returns the versions applied.
func MigrateUp(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	migrations, err := loadMigrations()
	if err != nil {
		return nil, err
	}
	var done []string
	err = withLock(ctx, pool, func(conn *pgxpool.Conn) error {
		applied, err := appliedVersions(ctx, conn)
		if err != nil {
			return err
		}
		for _, m := range migrations {
			if applied[m.Version] {
				continue
			}
			if err := pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
				if _, err := tx.Exec(ctx, m.Up); err != nil {
					return err
				}
				_, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", m.Version)
				return err
			}); err != nil {
				return fmt.Errorf("apply migration %s: %w", m.Version, err)
			}
			done = append(done, m.Version)
		}
		return nil
	})
	return done, err
}

// MigrateDown rolls back the most recently applied migration.
func MigrateDown(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	migrations, err := loadMigrations()
	if err != nil {
		return "", err
	}
	var rolledBack string
	err = withLock(ctx, pool, func(conn *pgxpool.Conn) error {
		applied, err := appliedVersions(ctx, conn)
		if err != nil {
			return err
		}
		for i := len(migrations) - 1; i >= 0; i-- {
			m := migrations[i]
			if !applied[m.Version] {
				continue
			}
			if m.Down == "" {
				return fmt.Errorf("migration %s has no .down.sql file", m.Version)
			}
			if err := pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
				if _, err := tx.Exec(ctx, m.Down); err != nil {
					return err
				}
				_, err := tx.Exec(ctx, "DELETE FROM schema_migrations WHERE version = $1", m.Version)
				return err
			}); err != nil {
				return fmt.Errorf("roll back migration %s: %w", m.Version, err)
			}
			rolledBack = m.Version
			return nil
		}
		return errors.New("no applied migrations to roll back")
	})
	return rolledBack, err
}

// Status lists every known migration and whether it has been applied.
func Status(ctx context.Context, pool *pgxpool.Pool) ([]MigrationStatus, error) {
	migrations, err := loadMigrations()
	if err != nil {
		return nil, err
	}
	var out []MigrationStatus
	err = withLock(ctx, pool, func(conn *pgxpool.Conn) error {
		applied, err := appliedVersions(ctx, conn)
		if err != nil {
			return err
		}
		for _, m := range migrations {
			out = append(out, MigrationStatus{Version: m.Version, Applied: applied[m.Version]})
		}
		return nil
	})
	return out, err
}
