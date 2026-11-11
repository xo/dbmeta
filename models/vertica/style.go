package vertica

import (
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The statement style of the dump. See D274, D304 and docs/DUMP.md.
//
// Vertica takes a primary key, a unique key and a foreign key, and does not enforce them
// unless they are enabled. It keeps no index that a statement makes.

func init() {
	dump.RegisterStyle(dbmeta.Vertica, dump.Style{
		Quote:           quoteIdentifier,
		Literal:         literal,
		DefaultSchema:   "public",
		ExistingSchemas: []string{"public"},
		CreateSchema:    func(quoted string) string { return "CREATE SCHEMA " + quoted },
		Binary:          func(hex string) string { return "HEX_TO_BINARY('0x" + hex + "')" },
		// A timestamp with a zone reads the offset of a literal.
		ZoneInTimestamps: true,
		// Vertica has no INDEX that a statement makes.
		ImplicitIndex: func(dump.Table, dump.Index) bool { return true },
	})
}

// quoteIdentifier quotes a name with double quotes.
func quoteIdentifier(name string) string {
	return dbmeta.QuoteIdentifier(name, `"`, `"`)
}

// literal writes a string literal. Vertica has no backslash escape in a standard string.
func literal(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
