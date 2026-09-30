package dbmeta

import (
	"database/sql"
	"fmt"
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
