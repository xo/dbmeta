package exasol

import (
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The statement style of the dump. See D274, D304 and docs/DUMP.md.
//
// Exasol takes a primary key, a unique key and a foreign key, and does not enforce them
// unless they are enabled. It keeps no index that a statement makes. The connection can
// have no current schema, so every name has its schema.

func init() {
	dump.RegisterStyle(dbmeta.Exasol, dump.Style{
		Quote:        quoteIdentifier,
		Literal:      literal,
		QualifyAll:   true,
		CreateSchema: func(quoted string) string { return "CREATE SCHEMA " + quoted },
		// Exasol has no binary type, so a value that reaches a text column is its hexadecimal text.
		Binary: func(hex string) string { return "'" + hex + "'" },
		// Exasol builds its own indexes.
		ImplicitIndex: func(dump.Table, dump.Index) bool { return true },
	})
}

// quoteIdentifier quotes a name with double quotes. Exasol keeps the case of a quoted name
// and makes the case of one that is not quoted upper.
func quoteIdentifier(name string) string {
	return dbmeta.QuoteIdentifier(name, `"`, `"`)
}

// literal writes a string literal. Exasol has no backslash escape.
func literal(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
