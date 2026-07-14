package sqliteutil

import "testing"

func TestDriverName(t *testing.T) {
	if got := DriverName(); got != "sqlite3" {
		t.Fatalf("DriverName() = %q, want %q", got, "sqlite3")
	}
}
