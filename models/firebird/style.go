package firebird

import (
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The statement style of the dump. See D274, D304 and docs/DUMP.md.
//
// Firebird has no schema before release 6, so every table is in the one database. An INSERT
// takes one row, because Firebird has no list of rows after VALUES.

func init() {
	dump.RegisterStyle(dbmeta.Firebird, dump.Style{
		Quote:         quoteIdentifier,
		Literal:       literal,
		DefaultSchema: "*",
		Column:        column,
		Binary:        func(hex string) string { return "x'" + hex + "'" },
		MaxRows:       1,
	})
}

// quoteIdentifier quotes a name with double quotes. Firebird keeps the case of a quoted name
// and folds the case of one that is not quoted.
func quoteIdentifier(name string) string {
	return dbmeta.QuoteIdentifier(name, `"`, `"`)
}

// literal writes a string literal. Firebird has no backslash escape.
func literal(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// column writes one column of a CREATE TABLE. The model gives a default with the word
// DEFAULT in front of it, and Firebird wants the default before NOT NULL.
func column(c dump.Column, typeSQL string, q func(string) string, _ func(string) string) string {
	var b strings.Builder
	b.WriteString(q(c.Name) + " " + typeSQL)
	if c.Default != nil && c.Generated == "" {
		text := strings.TrimSpace(*c.Default)
		if len(text) > 8 && strings.EqualFold(text[:8], "default ") {
			text = strings.TrimSpace(text[8:])
		}
		b.WriteString(" DEFAULT " + text)
	}
	if !c.Nullable {
		b.WriteString(" NOT NULL")
	}
	return b.String()
}
