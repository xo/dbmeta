package arangodb

import (
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The statement style of the dump. ArangoDB is only a source: the style quotes a name and
// writes a string, which a read needs, and has no binary literal, so no dump writes to it. Its
// query language is AQL, so the statement that reads a collection is a FOR with a RETURN. A
// statement reads the database of the connection, so no name has a schema. See D274, D305 and
// docs/DUMP.md.

func init() {
	dump.RegisterStyle(dbmeta.ArangoDB, dump.Style{
		Quote:         quoteIdentifier,
		Literal:       stringLiteral,
		DefaultSchema: "*",
		SelectAll:     selectAll,
	})
}

// quoteIdentifier quotes a name with backticks.
func quoteIdentifier(name string) string {
	return dbmeta.QuoteIdentifier(name, "`", "`")
}

// stringLiteral writes a string literal. AQL reads a backslash as an escape.
func stringLiteral(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return "'" + strings.ReplaceAll(s, "'", `\'`) + "'"
}

// selectAll reads every document of a collection as a row of the columns.
func selectAll(quotedTable string, quotedColumns []string) string {
	fields := make([]string, len(quotedColumns))
	for i, c := range quotedColumns {
		fields[i] = c + ": d." + c
	}
	return "FOR d IN " + quotedTable + " RETURN {" + strings.Join(fields, ", ") + "}"
}
