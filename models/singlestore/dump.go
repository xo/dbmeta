package singlestore

import (
	"slices"
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The dump of SingleStore. It reads and writes the SQL of MySQL, so the type
// functions and the style are the ones of the mysql model, which registers first.
//
// SingleStore has no foreign key. It shards a table by its primary key, and it
// refuses a unique key that does not hold every column of the shard key. The dump
// leaves such a constraint out and says so, because writing a unique key as an
// ordinary index changes what the table allows without a word. See D280.
func init() {
	dump.RegisterLike(dbmeta.MemSQL, dbmeta.MySQL, nil, func(s *dump.Style) {
		s.RefuseConstraint = refuseUnique
	})
}

// refuseUnique returns the reason that a constraint is not written, or the empty
// string. A table with no primary key has no shard key to hold.
func refuseUnique(t dump.Table, c dump.Constraint) string {
	switch c.Kind {
	case "foreign key":
		return "SingleStore has no foreign key, so " + c.Name + " is not written"
	case "unique":
	default:
		return ""
	}
	for _, k := range t.Constraints {
		if k.Kind != "primary key" {
			continue
		}
		if k.Name == c.Name {
			return ""
		}
		if !slices.ContainsFunc(k.Columns, func(col string) bool { return !slices.Contains(c.Columns, col) }) {
			return ""
		}
		return "SingleStore shards by the primary key (" + strings.Join(k.Columns, ", ") +
			") and refuses a unique key that does not hold every column of it, so " + c.Name + " is not written"
	}
	return ""
}
