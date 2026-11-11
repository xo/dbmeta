package spanner

import (
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The statement style of the dump. See D274, D303 and docs/DUMP.md.
//
// A table of Spanner has a primary key, written after the columns, and the key cannot
// be empty for more than one row. The default schema has the name that is the empty
// string, and a named schema is made with CREATE SCHEMA. A dump of a basic table leaves out
// every constraint and index that is not the key.

func init() {
	dump.RegisterStyle(dbmeta.Spanner, dump.Style{
		Quote:            quoteIdentifier,
		Literal:          literal,
		DefaultSchema:    "",
		CreateSchema:     func(quoted string) string { return "CREATE SCHEMA " + quoted },
		OrderFree:        true,
		Column:           column,
		SelectColumn:     selectNumeric,
		TableOptions:     keyClause,
		RefuseConstraint: refuseConstraint,
		ImplicitIndex:    func(dump.Table, dump.Index) bool { return true },
		Binary:           func(hex string) string { return "FROM_HEX('" + hex + "')" },
		// A timestamp literal with no offset is read in the time zone of the session.
		ZoneInTimestamps: true,
		TimeLiteral:      timeLiteral,
		MaxParameters:    950,
		MaxRows:          100,
	})
}

// quoteIdentifier quotes a name with backticks.
func quoteIdentifier(name string) string {
	return dbmeta.QuoteIdentifier(name, "`", "`")
}

// literal writes a string literal. GoogleSQL reads a backslash as an escape.
func literal(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return "'" + strings.ReplaceAll(s, "'", `\'`) + "'"
}

// column writes one column of a CREATE TABLE: the type, NOT NULL, and then the default
// or the expression of a generated column.
func column(c dump.Column, typeSQL string, q func(string) string, _ func(string) string) string {
	var b strings.Builder
	b.WriteString(q(c.Name) + " " + typeSQL)
	if !c.Nullable {
		b.WriteString(" NOT NULL")
	}
	switch {
	case c.Generated != "" && c.Default != nil:
		b.WriteString(" AS (" + *c.Default + ") STORED")
	case c.Default != nil:
		b.WriteString(" DEFAULT (" + *c.Default + ")")
	}
	return b.String()
}

// keyClause writes the primary key, which Spanner wants after the closing bracket. A
// table that has none gets an empty key, which holds one row, so a second row fails.
func keyClause(t dump.Table, _ func(string) string) string {
	for _, k := range t.Constraints {
		if k.Kind != "primary key" || len(k.Columns) == 0 {
			continue
		}
		quoted := make([]string, len(k.Columns))
		for i, name := range k.Columns {
			quoted[i] = quoteIdentifier(name)
		}
		return "PRIMARY KEY (" + strings.Join(quoted, ", ") + ")"
	}
	return "PRIMARY KEY ()"
}

// refuseConstraint says why a constraint is not written. The primary key is written
// after the columns, and a basic dump writes no other constraint.
func refuseConstraint(_ dump.Table, c dump.Constraint) string {
	if c.Kind == "primary key" {
		return "the primary key of Spanner is written after the columns, so " + c.Name + " is written there"
	}
	return "a basic dump of Spanner writes no " + c.Kind + ", so " + c.Name + " is not written"
}

// selectNumeric reads a NUMERIC as text. The driver gives it as a rational number that no
// other product reads, and the text is the digits.
func selectNumeric(_ *dump.Snapshot, _ string, c dump.Column, quoted string) (string, bool) {
	if c.Type.Kind != dump.Decimal {
		return "", false
	}
	return "CAST(" + quoted + " AS STRING)", true
}

// timeLiteral writes a timestamp with no zone as an instant in UTC. The text of a TIMESTAMP
// with no offset is read in the time zone of the session.
func timeLiteral(kind dump.Kind, quoted string) string {
	if kind == dump.LocalTimestamp && strings.HasSuffix(quoted, "'") {
		return "TIMESTAMP " + strings.TrimSuffix(quoted, "'") + "+00'"
	}
	return quoted
}
