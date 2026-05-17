//go:build sqlite_mattn && !sqlite_modernc && !sqlite_ncruces

package sqliteutil

import _ "github.com/mattn/go-sqlite3"

func DriverName() string {
	return "sqlite3"
}
