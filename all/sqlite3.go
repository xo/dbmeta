//go:build (!no_base || sqlite3) && !no_sqlite3

package all

import _ "github.com/xo/dbmeta/models/sqlite3"
