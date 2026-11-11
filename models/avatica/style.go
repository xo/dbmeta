package avatica

import (
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The statement style of the dump. See D274, D304 and docs/DUMP.md.
//
// The standalone Avatica server stands in front of HSQLDB, which takes the SQL of the
// standard. Phoenix speaks the same protocol and has no model here, so it has no style.

func init() {
	dump.RegisterStyle(dbmeta.Avatica, dump.Style{
		Quote:           quoteIdentifier,
		Literal:         literal,
		DefaultSchema:   "PUBLIC",
		ExistingSchemas: []string{"PUBLIC"},
		CreateSchema:    func(quoted string) string { return "CREATE SCHEMA " + quoted },
		Binary:          func(hex string) string { return "X'" + hex + "'" },
		// HSQLDB lists each NOT NULL of a column as a check constraint.
		ImplicitConstraint: func(kind, definition string) bool {
			return kind == "check" && strings.HasSuffix(strings.ToUpper(definition), " IS NOT NULL")
		},
	})
}

// quoteIdentifier quotes a name with double quotes. HSQLDB keeps the case of a quoted name
// and makes the case of one that is not quoted upper.
func quoteIdentifier(name string) string {
	return dbmeta.QuoteIdentifier(name, `"`, `"`)
}

// literal writes a string literal. HSQLDB has no backslash escape.
func literal(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
