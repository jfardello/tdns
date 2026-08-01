package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/jfardello/tdns/internal/sqliteutil"
)

func newTempDBPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "test.sqlite")
}

func openDB(t *testing.T, dbPath string) *sql.DB {
	t.Helper()
	conn, err := sql.Open(sqliteutil.DriverName(), sqliteutil.DSN(dbPath))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := sqliteutil.ConfigureConnection(context.Background(), conn, true); err != nil {
		t.Fatalf("configure db: %v", err)
	}
	return conn
}

func TestRunMigrationsDNSLog(t *testing.T) {
	dbPath := newTempDBPath(t)

	if err := RunMigrations(context.Background(), dbPath, TargetDNSLog); err != nil {
		t.Fatalf("RunMigrations dnslog: %v", err)
	}

	db := openDB(t, dbPath)
	defer db.Close()

	for _, table := range []string{"schema_migrations", "tdnslog", "hosts"} {
		var name string
		if err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name); err != nil {
			t.Fatalf("expected table %s: %v", table, err)
		}
	}
}

func TestRunMigrationsTagger(t *testing.T) {
	dbPath := newTempDBPath(t)

	if err := RunMigrations(context.Background(), dbPath, TargetTagger); err != nil {
		t.Fatalf("RunMigrations tagger: %v", err)
	}

	db := openDB(t, dbPath)
	defer db.Close()

	for _, table := range []string{"schema_migrations", "labels", "members", "member_labels"} {
		var name string
		if err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name); err != nil {
			t.Fatalf("expected table %s: %v", table, err)
		}
	}
}

func TestRunMigrationsConfig(t *testing.T) {
	dbPath := newTempDBPath(t)

	if err := RunMigrations(context.Background(), dbPath, TargetConfig); err != nil {
		t.Fatalf("RunMigrations config: %v", err)
	}

	db := openDB(t, dbPath)
	defer db.Close()

	for _, table := range []string{"schema_migrations", "config_overrides"} {
		var name string
		if err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name); err != nil {
			t.Fatalf("expected table %s: %v", table, err)
		}
	}
}

func TestRunMigrationsAuth(t *testing.T) {
	dbPath := newTempDBPath(t)

	if err := RunMigrations(context.Background(), dbPath, TargetAuth); err != nil {
		t.Fatalf("RunMigrations auth: %v", err)
	}

	conn := openDB(t, dbPath)
	defer conn.Close()

	for _, table := range []string{
		"schema_migrations",
		"browser_sessions",
		"consumed_browser_codes",
		"browser_session_csrf_tokens",
		"local_administrator",
	} {
		var name string
		if err := conn.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name); err != nil {
			t.Fatalf("expected table %s: %v", table, err)
		}
	}

	if err := RunMigrations(context.Background(), dbPath, TargetAuth); err != nil {
		t.Fatalf("RunMigrations auth second pass: %v", err)
	}
	var count int
	if err := conn.QueryRow(
		"SELECT COUNT(*) FROM schema_migrations WHERE target = ?",
		string(TargetAuth),
	).Scan(&count); err != nil {
		t.Fatalf("count auth migrations: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 auth migrations, got %d", count)
	}

	if _, err := conn.Exec(`
INSERT INTO browser_sessions
	(session_hash, subject, scope, csrf_hash, created_at, last_used_at, expires_at)
VALUES (zeroblob(32), 'legacy', 'tdns.kubewire.net:rw', zeroblob(32), 1, 1, 2)`); err != nil {
		t.Fatalf("insert pre-attribution session: %v", err)
	}
	var method string
	if err := conn.QueryRow(`SELECT authentication_method FROM browser_sessions`).Scan(&method); err != nil {
		t.Fatalf("read default authentication method: %v", err)
	}
	if method != "browser_code" {
		t.Fatalf("default authentication method = %q", method)
	}
}

func TestRunMigrationsIsIdempotent(t *testing.T) {
	dbPath := newTempDBPath(t)

	for i := 0; i < 2; i++ {
		if err := RunMigrations(context.Background(), dbPath, TargetDNSLog); err != nil {
			t.Fatalf("RunMigrations pass %d: %v", i+1, err)
		}
	}

	db := openDB(t, dbPath)
	defer db.Close()

	var count int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM schema_migrations WHERE target = ?",
		string(TargetDNSLog),
	).Scan(&count); err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 dnslog migrations, got %d", count)
	}
}

func TestRunMigrationsTracksVersionNames(t *testing.T) {
	dbPath := newTempDBPath(t)

	if err := RunMigrations(context.Background(), dbPath, TargetTagger); err != nil {
		t.Fatalf("RunMigrations tagger: %v", err)
	}

	db := openDB(t, dbPath)
	defer db.Close()

	var version string
	if err := db.QueryRow(
		"SELECT version FROM schema_migrations WHERE target = ? ORDER BY version LIMIT 1",
		string(TargetTagger),
	).Scan(&version); err != nil {
		t.Fatalf("read migration version: %v", err)
	}

	if version != "0001_labels.sql" {
		t.Fatalf("unexpected first tagger migration %q", version)
	}
}
