package influxql

import (
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The statement style of the dump. InfluxQL is only a source: the style quotes a name and
// writes a string, which a read needs, and has no binary literal, so no dump writes to it. A
// statement reads the database of the connection, so no name has a schema. See D274, D305
// and docs/DUMP.md.

func init() {
	dump.RegisterStyle(dbmeta.InfluxQL, dump.Style{
		Quote:         quoteIdentifier,
		Literal:       stringLiteral,
		DefaultSchema: "*",
		// The driver gives the name of the series before the time and the other columns.
		LeadingColumns: 1,
	})
}

// quoteIdentifier quotes a name with double quotes.
func quoteIdentifier(name string) string {
	return dbmeta.QuoteIdentifier(name, `"`, `"`)
}

// stringLiteral writes a string literal. InfluxQL reads a backslash as an escape.
func stringLiteral(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return "'" + strings.ReplaceAll(s, "'", `\'`) + "'"
}
