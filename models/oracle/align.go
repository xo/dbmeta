package oracle

import (
	"database/sql"
	"reflect"

	"github.com/xo/dbmeta"
)

// nameAlign aligns by the name of a type, for the driver that reports one.
var nameAlign = dbmeta.AlignName("NUMBER", "FLOAT", "BINARY_FLOAT", "BINARY_DOUBLE")

// columnAlign right aligns a numeric column and left aligns the rest.
//
// github.com/xo/oracle reports the name of every type, and it scans NUMBER and
// FLOAT into a decimal type, so the name is the fact to read. go-ora reports no
// name, because its DatabaseTypeName returns the empty string, so for it the Go
// type that it scans a column into is the only fact. It says int64 or float64
// for NUMBER and float32 or float64 for BINARY_FLOAT and BINARY_DOUBLE (D245).
//
// A type for which the driver gives neither a name nor a Go type says nothing,
// and a client aligns it by its values.
func columnAlign(ct *sql.ColumnType) (dbmeta.Align, bool) {
	if name := ct.DatabaseTypeName(); name != "" {
		return nameAlign(name)
	}
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

// nameBinary says that a column has a binary type of Oracle.
var nameBinary = dbmeta.BinaryNames("RAW", "LONG RAW", "BLOB")

// isBinary says that a RAW, LONG RAW or BLOB column is binary. The driver
// github.com/xo/oracle reports the name of the type. go-ora reports none, and
// for it a column that scans into bytes is binary, because no other type of
// Oracle does (D258).
func isBinary(ct *sql.ColumnType) bool {
	if ct.DatabaseTypeName() != "" {
		return nameBinary(ct)
	}
	t := ct.ScanType()
	return t != nil && t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8
}
