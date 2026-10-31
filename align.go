package dbmeta

import (
	"database/sql"
	"strings"
)

// Align says on which side a client prints the values of a column.
type Align uint8

// The alignments. The zero value is none, which [Dialect.ColumnAlign] returns
// with false for a dialect that records nothing.
const (
	AlignLeft Align = iota + 1
	AlignRight
)

// String satisfies [fmt.Stringer].
func (a Align) String() string {
	switch a {
	case AlignLeft:
		return "left"
	case AlignRight:
		return "right"
	}
	return "none"
}

// RightAligned returns a function for [Info.ColumnAlign] that follows the rule of
// psql: a column of a numeric type prints on the right, and a column of any other
// type prints on the left. The names are the strings that
// [sql.ColumnType.DatabaseTypeName] returns through the drivers of the product,
// and a name matches in any case. A model lists the names that it measured, one
// for each driver that reports a name of its own. See D245.
func RightAligned(names ...string) func(*sql.ColumnType) (Align, bool) {
	align := AlignName(names...)
	return func(ct *sql.ColumnType) (Align, bool) {
		return align(ct.DatabaseTypeName())
	}
}

// AlignName is [RightAligned] for a type name that a model has already taken
// from the column, because the driver wraps it, as YDB does in Optional<Int32>.
// A name matches in any case and without its size, so DECIMAL(10,2) matches
// DECIMAL, and an empty name says nothing.
func AlignName(names ...string) func(string) (Align, bool) {
	right := make(map[string]struct{}, len(names))
	for _, name := range names {
		right[strings.ToUpper(name)] = struct{}{}
	}
	return func(name string) (Align, bool) {
		// A column with no type name, such as an expression in SQLite, says
		// nothing, and a client aligns it by its values.
		if name == "" {
			return 0, false
		}
		// A driver of SQLite, DuckDB and Databend keeps the size in the name, as
		// DECIMAL(10,2), so the match is on the part before the parenthesis.
		name, _, _ = strings.Cut(strings.ToUpper(name), "(")
		if _, ok := right[strings.TrimSpace(name)]; ok {
			return AlignRight, true
		}
		return AlignLeft, true
	}
}

// ColumnAlign returns the alignment of a column of d, from the type that the
// driver reports for it. The second result is false when the model of d records
// no alignment, or no model was built for d, and then a client falls back to
// aligning by the values. See [Info.ColumnAlign].
func (d Dialect) ColumnAlign(ct *sql.ColumnType) (Align, bool) {
	info, ok := d.Info()
	if !ok || info.ColumnAlign == nil {
		return 0, false
	}
	return info.ColumnAlign(ct)
}
