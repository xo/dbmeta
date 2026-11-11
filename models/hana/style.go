package hana

import (
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The statement style of the dump. See D274, D304 and docs/DUMP.md.
//
// SAP HANA takes a primary key, a unique key, a foreign key and a check. An INSERT takes one
// row, because HANA has no list of rows after VALUES. Every name has its schema, because the
// schema of a connection is the user that logged in.

func init() {
	dump.RegisterStyle(dbmeta.HANA, dump.Style{
		Quote:        quoteIdentifier,
		Literal:      literal,
		QualifyAll:   true,
		CreateSchema: func(quoted string) string { return "CREATE SCHEMA " + quoted },
		Binary:       func(hex string) string { return "HEXTOBIN('" + hex + "')" },
		MaxRows:      1,
		SelectColumn: selectDecimal,
	})
}

// quoteIdentifier quotes a name with double quotes. HANA keeps the case of a quoted name
// and makes the case of one that is not quoted upper.
func quoteIdentifier(name string) string {
	return dbmeta.QuoteIdentifier(name, `"`, `"`)
}

// literal writes a string literal. HANA has no backslash escape.
func literal(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// selectDecimal reads a DECIMAL as text. The driver gives it as a value of its own type that
// no other product reads, and the text is the digits.
func selectDecimal(_ *dump.Snapshot, _ string, c dump.Column, quoted string) (string, bool) {
	if c.Type.Kind != dump.Decimal {
		return "", false
	}
	return "CAST(" + quoted + " AS NVARCHAR(100))", true
}
