package ydb

import (
	"database/sql"
	"strings"

	"github.com/xo/dbmeta"
)

// numeric is the numeric types that the driver of YDB names.
var numeric = dbmeta.AlignName("Int8", "Int16", "Int32", "Int64", "Uint8", "Uint16", "Uint32", "Uint64",
	"Float", "Double", "Decimal")

// columnAlign right aligns a numeric column. The driver names a column that can
// be NULL Optional<Int32>, so the model takes the name out of every Optional
// before it looks the name up. Measured on YDB 26.3 (D245).
func columnAlign(ct *sql.ColumnType) (dbmeta.Align, bool) {
	name := ct.DatabaseTypeName()
	for strings.HasPrefix(name, "Optional<") && strings.HasSuffix(name, ">") {
		name = strings.TrimSuffix(strings.TrimPrefix(name, "Optional<"), ">")
	}
	return numeric(name)
}
