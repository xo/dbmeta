package sqlite3

import (
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The statement style of the dump. See D274 and docs/DUMP.md.

func init() {
	dump.RegisterStyle(dbmeta.SQLite3, dump.Style{
		Quote:   quoteIdentifier,
		Literal: literal,
		// SQLite has one schema, and every schema of a snapshot is written into it.
		DefaultSchema:     "*",
		InlineConstraints: true,
		OrderFree:         true,
		TableText:         true,
		// A view can read another view, and SQLite has no catalog that says so.
		RoutinesAfterTables:  true,
		SequenceValue:        sequenceValue,
		AnonymousConstraints: true,
		SelectColumn:         selectTime,
		ValueLiteral:         timeText,
		// The tables can be written in any order, and the rows can be loaded in any
		// order when the checks are off, and checked at the end. The pragma has no
		// effect inside a transaction, so it comes before one. See docs/DUMP.md.
		Binary:           func(hex string) string { return "X'" + hex + "'" },
		ZoneInTimestamps: true,
		// A statement takes 32766 variables from release 3.32 and 999 before.
		MaxRows:        500,
		MaxParameters:  999,
		BeforeData:     []string{"PRAGMA foreign_keys = OFF"},
		AfterData:      []string{"PRAGMA foreign_keys = ON", "PRAGMA foreign_key_check"},
		CheckAfterData: "PRAGMA foreign_key_check",
		ImplicitIndex:  func(_ dump.Table, ix dump.Index) bool { return strings.HasPrefix(ix.Name, "sqlite_autoindex_") },
		View:           definitionStatement,
	})
}

// quoteIdentifier quotes a name with double quotes.
func quoteIdentifier(name string) string {
	return dbmeta.QuoteIdentifier(name, `"`, `"`)
}

// literal writes a string literal. SQLite has no backslash escape.
func literal(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// definitionStatement returns the statement of the source, because SQLite keeps
// the whole CREATE VIEW and not only the query.
func definitionStatement(_ string, v dump.View) string {
	return strings.TrimRight(strings.TrimSpace(v.Definition), "; \n\t")
}

// sequenceValue sets the counter of a table that uses AUTOINCREMENT, which the rows set
// to the largest rowid. A table that has no row has no counter yet.
func sequenceValue(quoted string, s dump.Sequence, lit func(string) string) string {
	return "UPDATE sqlite_sequence SET seq = " + s.Last + " WHERE name = " + lit(strings.Trim(quoted, `"`))
}

// timeKind says that a column is read by the driver as a time, which SQLite does not have.
func timeKind(k dump.Kind) bool {
	return k == dump.Date || k == dump.TimeOfDay || k == dump.LocalTimestamp || k == dump.Timestamp
}

// selectTime reads a column of a time kind as the text that SQLite holds. A column of that
// kind holds whatever text it was given, and the driver turns the text into a time in the
// zone of the machine, which moves the value. See D300.
func selectTime(_ *dump.Snapshot, _ string, c dump.Column, quoted string) (string, bool) {
	if !timeKind(c.Type.Kind) {
		return "", false
	}
	return "CAST(" + quoted + " AS TEXT)", true
}

// timeText writes the text of a time column as it was read, so that a copy to SQLite holds
// the text of the source and not a time that was formatted again.
func timeText(t dump.Type, v any) (string, bool) {
	if !timeKind(t.Kind) {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	return literal(s), true
}
