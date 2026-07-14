package sqliteutil

import (
	"database/sql"
	"testing"
)

func TestDriverName(t *testing.T) {
	if got := DriverName(); got != "sqlite3" {
		t.Fatalf("DriverName() = %q, want %q", got, "sqlite3")
	}
}

func TestDriverProvidesSQLiteVersion(t *testing.T) {
	db, err := sql.Open(DriverName(), ":memory:")
	if err != nil {
		t.Fatalf("open in-memory database: %v", err)
	}
	defer db.Close()

	var version string
	if err := db.QueryRow("SELECT sqlite_version()").Scan(&version); err != nil {
		t.Fatalf("query SQLite version: %v", err)
	}
	if version == "" {
		t.Fatal("SQLite version is empty")
	}
}
