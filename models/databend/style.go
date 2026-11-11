package databend

import (
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The statement style of the dump. See D274, D304 and docs/DUMP.md.
//
// A database of Databend is the schema. A table has columns, a default and NOT NULL. Databend
// has no primary key, no unique key, no foreign key and no index that a statement makes.

func init() {
	dump.RegisterStyle(dbmeta.Databend, dump.Style{
		Quote:            quoteIdentifier,
		Literal:          literal,
		DefaultSchema:    "default",
		ExistingSchemas:  []string{"default"},
		CreateSchema:     func(quoted string) string { return "CREATE DATABASE " + quoted },
		OrderFree:        true,
		RefuseConstraint: refuseConstraint,
		Binary:           func(hex string) string { return "unhex('" + hex + "')" },
		ZoneInTimestamps: true,
	})
}

// quoteIdentifier quotes a name with double quotes.
func quoteIdentifier(name string) string {
	return dbmeta.QuoteIdentifier(name, `"`, `"`)
}

// literal writes a string literal. Databend reads a backslash as an escape.
func literal(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// refuseConstraint says that Databend holds no key and no check.
func refuseConstraint(_ dump.Table, c dump.Constraint) string {
	return "Databend has no " + c.Kind + ", so " + c.Name + " is not written"
}
