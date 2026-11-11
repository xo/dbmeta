package questdb

import (
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The statement style of the dump. See D274, D302 and docs/DUMP.md.
//
// QuestDB speaks the wire protocol of PostgreSQL, and its SQL is its own. A table has
// columns and a type for each. It has no NOT NULL, no default, no key, no index that
// a table writes and no schema, so a dump of a table is its columns and its rows.

func init() {
	dump.RegisterStyle(dbmeta.QuestDB, dump.Style{
		Quote:   quoteIdentifier,
		Literal: literal,
		// QuestDB has one schema, and every schema of a snapshot is written into it.
		DefaultSchema: "*",
		OrderFree:     true,
		Column:        column,
		// A number with a point is a double in QuestDB, and the suffix m makes it a decimal.
		DecimalLiteral: func(digits string) string { return digits + "m" },
		// QuestDB has no key and no constraint of any kind.
		RefuseConstraint: refuseConstraint,
		// QuestDB has no binary literal, and no column holds a binary value. A value that
		// reaches a text column is its hexadecimal text.
		Binary:        func(hex string) string { return "'" + hex + "'" },
		MaxParameters: 32767,
	})
}

// quoteIdentifier quotes a name with double quotes.
func quoteIdentifier(name string) string {
	return dbmeta.QuoteIdentifier(name, `"`, `"`)
}

// literal writes a string literal. QuestDB has no backslash escape.
func literal(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// column writes one column of a CREATE TABLE, which is the name and the type. QuestDB
// has no NOT NULL and no default.
func column(c dump.Column, typeSQL string, q func(string) string, _ func(string) string) string {
	return q(c.Name) + " " + typeSQL
}

// refuseConstraint says that QuestDB holds no constraint. A table made there has
// columns, and a key that the source has is not written.
func refuseConstraint(_ dump.Table, c dump.Constraint) string {
	return "QuestDB has no " + c.Kind + ", so " + c.Name + " is not written"
}
