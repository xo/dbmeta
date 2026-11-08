package cockroachdb

import (
	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The dump of CockroachDB. It reads and writes the SQL of PostgreSQL, so the type
// functions and the style are the ones of the postgres model, which registers
// first. See D280.
func init() {
	dump.RegisterLike(dbmeta.CockroachDB, dbmeta.PostgreSQL, nil, nil)
}
