package duckdb

import (
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The statement style of the dump. See D274 and docs/DUMP.md.

func init() {
	dump.RegisterStyle(dbmeta.DuckDB, dump.Style{
		Quote:                quoteIdentifier,
		Literal:              literal,
		DefaultSchema:        "main",
		ExistingSchemas:      []string{"main"},
		CreateSchema:         func(quoted string) string { return "CREATE SCHEMA " + quoted },
		InlineConstraints:    true,
		AnonymousConstraints: true,
		// DuckDB refuses ON DELETE and ON UPDATE other than the default.
		NoForeignKeyActions: true,
		Sequence:            sequence,
		Comment:             comment,
		View:                definitionStatement,
	})
}

// quoteIdentifier quotes a name with double quotes.
func quoteIdentifier(name string) string {
	return dbmeta.QuoteIdentifier(name, `"`, `"`)
}

// literal writes a string literal. DuckDB has no backslash escape.
func literal(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// definitionStatement returns the statement of the source, because DuckDB keeps
// the whole CREATE VIEW and not only the query.
func definitionStatement(_ string, v dump.View) string {
	return strings.TrimRight(strings.TrimSpace(v.Definition), "; \n\t")
}

// sequence writes CREATE SEQUENCE with the bounds that the catalog gives.
func sequence(quoted string, s dump.Sequence) string {
	var b strings.Builder
	b.WriteString("CREATE SEQUENCE " + quoted)
	for _, f := range []struct{ word, value string }{
		{"INCREMENT BY", s.Increment}, {"MINVALUE", s.Minimum}, {"MAXVALUE", s.Maximum}, {"START WITH", s.Start},
	} {
		if f.value != "" {
			b.WriteString(" " + f.word + " " + f.value)
		}
	}
	if s.Cycles {
		b.WriteString(" CYCLE")
	}
	return b.String()
}

// comment writes COMMENT ON.
func comment(target, quotedName, text string, lit func(string) string) string {
	return "COMMENT ON " + target + " " + quotedName + " IS " + lit(text)
}
