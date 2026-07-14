package sqliteutil

import _ "github.com/ncruces/go-sqlite3/driver"

func DriverName() string {
	return "sqlite3"
}
