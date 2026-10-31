package sqlite3

import (
	"database/sql"
	"strings"

	"github.com/xo/dbmeta"
)

// columnAlign right aligns a column whose declared type has an integer or a real
// affinity, or is a decimal or numeric type, and left aligns the rest. SQLite
// keeps the type as the text of the declaration, so no fixed list of names can
// cover it, and mattn/go-sqlite3 and modernc.org/sqlite both report that text
// (D245). The affinity rules are the ones SQLite documents, and BOOLEAN, DATE and
// DATETIME have a numeric affinity but are not numbers.
//
// A column with no declared type, such as an expression, says nothing.
func columnAlign(ct *sql.ColumnType) (dbmeta.Align, bool) {
	name, _, _ := strings.Cut(strings.ToUpper(ct.DatabaseTypeName()), "(")
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return 0, false
	case strings.Contains(name, "INT"):
		return dbmeta.AlignRight, true
	case strings.Contains(name, "CHAR"), strings.Contains(name, "CLOB"), strings.Contains(name, "TEXT"):
		return dbmeta.AlignLeft, true
	case strings.Contains(name, "REAL"), strings.Contains(name, "FLOA"), strings.Contains(name, "DOUB"):
		return dbmeta.AlignRight, true
	case name == "NUMERIC", name == "DECIMAL", name == "DEC", name == "NUMBER":
		return dbmeta.AlignRight, true
	}
	return dbmeta.AlignLeft, true
}
