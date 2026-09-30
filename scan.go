package dbmeta

import (
	"database/sql"
	"fmt"
	"strconv"
)

// NullAsEmpty returns a scanner that reads a text column into a plain string,
// and reads NULL as the empty string.
//
// It is for a product that cannot store the empty string. Oracle and Exasol
// both read one as NULL, so a statement that selects ” for a field that is
// empty, or wraps a column in NVL(x, ”), gets NULL back, and a NULL cannot
// be scanned into a string. A model for such a product scans each plain
// string field that can be empty through this. D87 found it on Exasol, and
// D127 put the one copy here, because two models need it.
//
// This is not the COALESCE docs/NULLS.md forbids. That rule protects a NULL
// that means something different from the empty string. A field whose type is
// sql.Null keeps its NULL, and is never scanned through this. A plain string
// field is one the object model says is never absent, and on these products
// NULL is the only way to write it empty.
func NullAsEmpty(p *string) sql.Scanner { return nullAsEmpty{p} }

type nullAsEmpty struct{ p *string }

// Scan satisfies sql.Scanner.
func (n nullAsEmpty) Scan(v any) error {
	switch t := v.(type) {
	case nil:
		*n.p = ""
	case string:
		*n.p = t
	case []byte:
		*n.p = string(t)
	default:
		*n.p = fmt.Sprint(t)
	}
	return nil
}

// NumberAsBool returns a scanner that reads a boolean column into a bool. It
// reads a number as a bool too, where zero is false and anything else is
// true, and it refuses NULL, as database/sql does.
//
// It is for a driver that reports a boolean as a number of a type
// database/sql does not convert. SQLite has no boolean type and returns 0 or
// 1, and github.com/rqlite/gorqlite decodes rqlite's JSON and gives every
// number as a float64, which database/sql converts to an integer and never
// to a bool. The sqlite3 model scans each boolean through this, so that rqlite
// shares its statements. See D148.
func NumberAsBool(p *bool) sql.Scanner { return numberAsBool{p} }

// NullNumberAsBool is [NumberAsBool] for a field that can be absent. It reads
// NULL as absent.
func NullNumberAsBool(p *sql.Null[bool]) sql.Scanner { return nullNumberAsBool{p} }

type numberAsBool struct{ p *bool }

// Scan satisfies sql.Scanner.
func (n numberAsBool) Scan(v any) error {
	if v == nil {
		return fmt.Errorf("reading NULL into a bool: %w", ErrInvalidBool)
	}
	b, err := asBool(v)
	if err != nil {
		return err
	}
	*n.p = b
	return nil
}

type nullNumberAsBool struct{ p *sql.Null[bool] }

// Scan satisfies sql.Scanner.
func (n nullNumberAsBool) Scan(v any) error {
	if v == nil {
		*n.p = sql.Null[bool]{}
		return nil
	}
	b, err := asBool(v)
	if err != nil {
		return err
	}
	*n.p = sql.Null[bool]{V: b, Valid: true}
	return nil
}

// asBool reads a bool, a number, or the text of either.
func asBool(v any) (bool, error) {
	switch t := v.(type) {
	case bool:
		return t, nil
	case int64:
		return t != 0, nil
	case float64:
		return t != 0, nil
	case []byte:
		return textBool(string(t))
	case string:
		return textBool(t)
	}
	return false, fmt.Errorf("reading %T into a bool: %w", v, ErrInvalidBool)
}

// textBool reads the text of a bool or of a number.
func textBool(s string) (bool, error) {
	if b, err := strconv.ParseBool(s); err == nil {
		return b, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return false, fmt.Errorf("reading %q into a bool: %w", s, ErrInvalidBool)
	}
	return f != 0, nil
}
