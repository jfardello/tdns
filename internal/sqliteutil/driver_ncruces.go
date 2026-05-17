//go:build sqlite_ncruces || (!sqlite_mattn && !sqlite_modernc)

package sqliteutil

import (
	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

func DriverName() string {
	return "sqlite3"
}
