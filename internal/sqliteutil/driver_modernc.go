//go:build sqlite_modernc && !sqlite_mattn && !sqlite_ncruces

package sqliteutil

import _ "modernc.org/sqlite"

func DriverName() string {
	return "sqlite"
}
