package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/jfardello/tdns/internal/sqliteutil"
)

func TestResolvePathDirectoryUsesSharedFilename(t *testing.T) {
	got, err := ResolvePath(t.TempDir())
	if err != nil {
		t.Fatalf("ResolvePath error: %v", err)
	}

	if filepath.Base(got) != "tdns.sqlite" {
		t.Fatalf("ResolvePath base got %q, want %q", filepath.Base(got), "tdns.sqlite")
	}
}

func TestBootstrapCreatesUnifiedSchema(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "app.sqlite")

	resolvedPath, err := Bootstrap(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("Bootstrap error: %v", err)
	}
	if resolvedPath != dbPath {
		t.Fatalf("Bootstrap path got %q, want %q", resolvedPath, dbPath)
	}

	conn, err := sql.Open(sqliteutil.DriverName(), sqliteutil.DSN(dbPath))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()
	if err := sqliteutil.ConfigureConnection(context.Background(), conn, true); err != nil {
		t.Fatalf("configure db: %v", err)
	}

	for _, table := range []string{"schema_migrations", "tdnslog", "hosts", "labels", "members", "member_labels", "config_overrides"} {
		var name string
		if err := conn.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name); err != nil {
			t.Fatalf("expected table %s: %v", table, err)
		}
	}
}
