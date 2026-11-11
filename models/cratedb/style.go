package cratedb

import (
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The statement style of the dump. See D274, D302 and docs/DUMP.md.
//
// CrateDB speaks the wire protocol of PostgreSQL, and its SQL is its own. It has a
// primary key and a check, and no foreign key, no unique key, no secondary index
// and no schema to create: a schema exists when a table names it.

func init() {
	dump.RegisterStyle(dbmeta.CrateDB, dump.Style{
		Quote:             quoteIdentifier,
		Literal:           literal,
		DefaultSchema:     "doc",
		InlineConstraints: true,
		OrderFree:         true,
		Column:            column,
		RefuseConstraint:  refuseConstraint,
		NestedColumn:      func(name string) bool { return strings.Contains(name, "['") },
		// CrateDB has no binary type, so no column holds a binary value. A value that
		// reaches a text column is its hexadecimal text.
		Binary:        func(hex string) string { return "'" + hex + "'" },
		MaxParameters: 32767,
	})
}

// quoteIdentifier quotes a name with double quotes.
func quoteIdentifier(name string) string {
	return dbmeta.QuoteIdentifier(name, `"`, `"`)
}

// literal writes a string literal. CrateDB has no backslash escape.
func literal(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// column writes one column of a CREATE TABLE. A generated column is written with its
// expression, and CrateDB takes no default beside it.
func column(c dump.Column, typeSQL string, q func(string) string, _ func(string) string) string {
	var b strings.Builder
	b.WriteString(q(c.Name) + " " + typeSQL)
	switch {
	case c.Generated != "" && c.Default != nil:
		b.WriteString(" GENERATED ALWAYS AS (" + *c.Default + ")")
	case c.Default != nil:
		b.WriteString(" DEFAULT " + *c.Default)
	}
	if !c.Nullable {
		b.WriteString(" NOT NULL")
	}
	return b.String()
}

// refuseConstraint returns the reason that CrateDB cannot hold a constraint. It has a
// primary key and a check. It has no foreign key and no unique key.
func refuseConstraint(_ dump.Table, c dump.Constraint) string {
	switch c.Kind {
	case "foreign key":
		return "CrateDB has no foreign key, so " + c.Name + " is not written"
	case "unique":
		return "CrateDB has no unique key, so " + c.Name + " is not written"
	}
	return ""
}
