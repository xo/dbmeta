package libsql

import (
	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The dump of this product. It reads and writes the SQL of SQLite3, so the type
// functions and the style are the ones of the sqlite3 model, which registers first.
// See D280.
func init() {
	dump.RegisterLike(dbmeta.LibSQL, dbmeta.SQLite3, nil, nil)
}
