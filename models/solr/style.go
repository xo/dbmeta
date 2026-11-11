package solr

import (
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The statement style of the dump. Solr is only a source: the style quotes a name and
// writes a string, which a read needs, and has no binary literal, so no dump writes to it.
// See D274, D305 and docs/DUMP.md.

func init() {
	dump.RegisterStyle(dbmeta.Solr, dump.Style{
		Quote:   quoteIdentifier,
		Literal: stringLiteral,
		// A table of the SQL of Solr has no schema.
		DefaultSchema: "*",
		// The catalog lists the query and the score of a search as columns, and a select with no
		// limit cannot read them.
		NestedColumn: func(name string) bool { return name == "_query_" || name == "score" },
	})
}

// quoteIdentifier quotes a name with backticks.
func quoteIdentifier(name string) string {
	return dbmeta.QuoteIdentifier(name, "`", "`")
}

// stringLiteral writes a string literal.
func stringLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
