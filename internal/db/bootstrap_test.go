package db

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/jfardello/tdns/internal/sqliteutil"
)

func TestResolvePathCreatesPrivateDirectory(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "runtime", "data")
	resolved, err := ResolvePath(filepath.Join(parent, "tdns.sqlite"))
	if err != nil {
		t.Fatalf("ResolvePath: %v", err)
	}
	if resolved != filepath.Join(parent, "tdns.sqlite") {
		t.Fatalf("resolved path = %q", resolved)
	}
	info, err := os.Stat(parent)
	if err != nil {
		t.Fatalf("stat parent: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Fatalf("parent mode = %04o, want 0700", got)
	}
}

func TestProtectFilesRestrictsSQLiteArtifacts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tdns.sqlite")
	for _, candidate := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.WriteFile(candidate, []byte("state"), 0o666); err != nil {
			t.Fatalf("write %s: %v", candidate, err)
		}
	}
	if err := ProtectFiles(path); err != nil {
		t.Fatalf("ProtectFiles: %v", err)
	}
	for _, candidate := range []string{path, path + "-wal", path + "-shm"} {
		info, err := os.Stat(candidate)
		if err != nil {
			t.Fatalf("stat %s: %v", candidate, err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("%s mode = %04o, want 0600", candidate, got)
		}
	}
}

func TestResolvePathRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.sqlite")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "tdns.sqlite")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolvePath(link); err == nil {
		t.Fatal("ResolvePath accepted a database symlink")
	}
	directory := filepath.Join(dir, "database")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(directory, "tdns.sqlite")); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolvePath(directory); err == nil {
		t.Fatal("ResolvePath accepted a database symlink inside a configured directory")
	}
}

func TestBootstrapRejectsWritableDatabaseDirectory(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "shared")
	if err := os.Mkdir(directory, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(directory, 0o777); err != nil {
		t.Fatal(err)
	}
	if _, err := Bootstrap(context.Background(), filepath.Join(directory, "tdns.sqlite")); err == nil {
		t.Fatal("Bootstrap accepted a group and world writable database directory")
	}
}

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

	for _, table := range []string{
		"schema_migrations",
		"tdnslog",
		"hosts",
		"labels",
		"members",
		"member_labels",
		"config_overrides",
		"browser_sessions",
		"consumed_browser_codes",
		"browser_session_csrf_tokens",
	} {
		var name string
		if err := conn.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name); err != nil {
			t.Fatalf("expected table %s: %v", table, err)
		}
	}
}
