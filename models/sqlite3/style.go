package sqlite3

import (
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The statement style of the dump. See D274 and docs/DUMP.md.

func init() {
	dump.RegisterStyle(dbmeta.SQLite3, dump.Style{
		Quote:                quoteIdentifier,
		Literal:              literal,
		DefaultSchema:        "main",
		InlineConstraints:    true,
		OrderFree:            true,
		AnonymousConstraints: true,
		ImplicitIndex:        func(_ dump.Table, ix dump.Index) bool { return strings.HasPrefix(ix.Name, "sqlite_autoindex_") },
		View:                 definitionStatement,
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
