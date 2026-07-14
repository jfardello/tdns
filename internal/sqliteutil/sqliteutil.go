package sqliteutil

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

const (
	readOnlyMode  = "mode=ro"
	readWriteMode = "mode=rwc"
)

// DSN returns a SQLite file data source name while preserving existing query
// parameters. Shared cache is enabled when the input does not include any.
func DSN(path string) string {
	if !strings.HasPrefix(path, "file:") {
		path = "file:" + path
	}
	if !strings.Contains(path, "?") {
		path += "?cache=shared"
	}
	return path
}

func ReadOnlyDSN(path string) string {
	return addDSNParam(DSN(path), readOnlyMode)
}

func ReadWriteDSN(path string) string {
	return addDSNParam(DSN(path), readWriteMode)
}

func addDSNParam(dsn string, param string) string {
	if strings.Contains(dsn, "?") {
		return dsn + "&" + param
	}
	return dsn + "?" + param
}

func ConfigureConnection(ctx context.Context, db *sql.DB, readWrite bool) error {
	stmts := []string{
		"PRAGMA busy_timeout = 5000;",
		"PRAGMA foreign_keys = ON;",
	}
	if readWrite {
		stmts = append(stmts,
			"PRAGMA journal_mode = WAL;",
			"PRAGMA synchronous = NORMAL;",
		)
	}
	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("configure sqlite connection with %q: %w", stmt, err)
		}
	}
	return nil
}
