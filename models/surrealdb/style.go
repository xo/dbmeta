package surrealdb

import (
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The statement style of the dump. SurrealDB is only a source: the style quotes a name and
// writes a string, which a read needs, and has no binary literal, so no dump writes to it. A
// statement reads the database of the connection, so no name has a schema. See D274, D305 and
// docs/DUMP.md.

func init() {
	dump.RegisterStyle(dbmeta.SurrealDB, dump.Style{
		Quote:         quoteIdentifier,
		Literal:       stringLiteral,
		DefaultSchema: "*",
	})
}

// quoteIdentifier quotes a name with backticks.
func quoteIdentifier(name string) string {
	return dbmeta.QuoteIdentifier(name, "`", "`")
}

// stringLiteral writes a string literal. SurrealQL reads a backslash as an escape.
func stringLiteral(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return "'" + strings.ReplaceAll(s, "'", `\'`) + "'"
}
