package oracle

import (
	"database/sql"
	"reflect"

	"github.com/xo/dbmeta"
)

// columnAlign right aligns a column that go-ora scans into a number, and left
// aligns the rest. go-ora reports no name for any type, because its
// DatabaseTypeName returns the empty string, so the Go type that it scans a
// column into is the only fact it gives. It says int64 or float64 for NUMBER and
// float32 or float64 for BINARY_FLOAT and BINARY_DOUBLE (D245).
//
// A type for which go-ora names no Go type says nothing, and a client aligns it
// by its values.
func columnAlign(ct *sql.ColumnType) (dbmeta.Align, bool) {
	t := ct.ScanType()
	if t == nil {
		return 0, false
	}
	switch t.Kind() {
	case reflect.Int64, reflect.Float32, reflect.Float64:
		return dbmeta.AlignRight, true
	}
	return dbmeta.AlignLeft, true
}
