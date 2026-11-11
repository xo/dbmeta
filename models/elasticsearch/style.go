package elasticsearch

import (
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The statement style of the dump. Elasticsearch is only a source: the style quotes a name and
// writes a string, which a read needs, and has no binary literal, so no dump writes to it.
// See D274, D305 and docs/DUMP.md.

func init() {
	dump.RegisterStyle(dbmeta.Elasticsearch, dump.Style{
		Quote:   quoteIdentifier,
		Literal: stringLiteral,
		// A table of the SQL of the product has no schema.
		DefaultSchema: "*",
	})
}

// quoteIdentifier quotes a name with double quotes.
func quoteIdentifier(name string) string {
	return dbmeta.QuoteIdentifier(name, `"`, `"`)
}

// stringLiteral writes a string literal.
func stringLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
