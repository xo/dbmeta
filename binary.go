package dbmeta

import (
	"database/sql"
	"encoding/hex"
	"strings"
)

// BinaryNames returns a function for [Info.IsBinary] that says a column is
// binary when it has one of the named types. The names are the strings that
// [sql.ColumnType.DatabaseTypeName] returns through the drivers of the product,
// and a name matches in any case and without its size. A column with any other
// type, and a column with no name, is not binary. See D258.
func BinaryNames(names ...string) func(*sql.ColumnType) bool {
	return func(ct *sql.ColumnType) bool {
		return hasName(ct.DatabaseTypeName(), names)
	}
}

// hasName says whether name is one of names, in any case and without a size.
func hasName(name string, names []string) bool {
	name, _, _ = strings.Cut(strings.ToUpper(name), "(")
	name = strings.TrimSpace(name)
	for _, n := range names {
		if strings.EqualFold(name, n) {
			return true
		}
	}
	return false
}

// FormatBinary returns the text of the value b of a column in the one form
// that every dialect uses, which is the form that psql prints for bytea: \x
// and the bytes in lower case hex, and \x alone for an empty value. It returns
// false when the column is not binary in this dialect or the dialect records
// nothing, so that a caller leaves the value alone. The caller does not call it
// for a NULL, because it cannot tell one from an empty value. The one
// exception is a value of 16 bytes in a column that the model says is a GUID,
// which is written as a lower case UUID with dashes, in the order of the
// bytes. See D258 and D263.
func (d Dialect) FormatBinary(ct *sql.ColumnType, b []byte) (string, bool) {
	info, ok := d.Info()
	if !ok || info.IsBinary == nil || !info.IsBinary(ct) {
		return "", false
	}
	if len(b) == 16 && info.IsGUID != nil && info.IsGUID(ct) {
		h := hex.EncodeToString(b)
		return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:], true
	}
	return `\x` + hex.EncodeToString(b), true
}
