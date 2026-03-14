package sqliteutil

import (
	"context"
	"database/sql"
	"fmt"
)

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
