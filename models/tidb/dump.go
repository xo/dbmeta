package tidb

import (
	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The dump of TiDB. It reads and writes the SQL of MySQL, so the type functions
// and the style are the ones of the mysql model, which registers first. See D280.
func init() {
	dump.RegisterLike(dbmeta.TiDB, dbmeta.MySQL, nil, nil)
}
