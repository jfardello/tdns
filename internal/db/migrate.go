package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/jfardello/tdns/internal/sqliteutil"
)

type Target string

const (
	TargetDNSLog Target = "dnslog"
	TargetTagger Target = "tagger"
	TargetConfig Target = "config"
	TargetAuth   Target = "auth"
)

//go:embed migrations/**/*.sql
var migrationsFS embed.FS

func RunMigrations(ctx context.Context, dbPath string, target Target) error {
	if strings.TrimSpace(dbPath) == "" {
		return fmt.Errorf("migration target %s: empty database path", target)
	}

	conn, err := sql.Open(sqliteutil.DriverName(), sqliteutil.ReadWriteDSN(dbPath))
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := sqliteutil.ConfigureConnection(ctx, conn, true); err != nil {
		return err
	}

	if err := ensureMetadataTable(ctx, conn); err != nil {
		return err
	}

	dir := path.Join("migrations", string(target))
	entries, err := fs.ReadDir(migrationsFS, dir)
	if err != nil {
		return err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() || path.Ext(entry.Name()) != ".sql" {
			continue
		}

		version := entry.Name()
		applied, err := migrationApplied(ctx, conn, target, version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		sqlBytes, err := migrationsFS.ReadFile(path.Join(dir, version))
		if err != nil {
			return err
		}

		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, string(sqlBytes)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s/%s: %w", target, version, err)
		}

		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO schema_migrations(target, version, applied_at) VALUES (?, ?, ?)`,
			string(target),
			version,
			time.Now().UTC(),
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s/%s: %w", target, version, err)
		}

		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

func ensureMetadataTable(ctx context.Context, conn *sql.DB) error {
	_, err := conn.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
	target TEXT NOT NULL,
	version TEXT NOT NULL,
	applied_at TIMESTAMP NOT NULL,
	PRIMARY KEY (target, version)
);`)
	return err
}

func migrationApplied(ctx context.Context, conn *sql.DB, target Target, version string) (bool, error) {
	var count int
	if err := conn.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM schema_migrations WHERE target = ? AND version = ?`,
		string(target),
		version,
	).Scan(&count); err != nil {
		return false, err
	}

	return count > 0, nil
}
