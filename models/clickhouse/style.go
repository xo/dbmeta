package clickhouse

import (
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The statement style of the dump. See D274, D303 and docs/DUMP.md.
//
// A database of ClickHouse is the schema. A table needs an engine and an order, and a
// column that can be NULL has the type Nullable. It has no foreign key, no unique key
// and no index that a statement of another product makes, and the key of a table is the
// order of its rows.

func init() {
	dump.RegisterStyle(dbmeta.ClickHouse, dump.Style{
		Quote:           quoteIdentifier,
		Literal:         literal,
		DefaultSchema:   "default",
		ExistingSchemas: []string{"default"},
		CreateSchema:    func(quoted string) string { return "CREATE DATABASE " + quoted },
		OrderFree:       true,
		// The catalog flags the columns of the primary key and holds no constraint for it.
		PrimaryKeyFromColumns: true,
		Column:                column,
		TableOptions:          engineClause,
		RefuseConstraint:      refuseConstraint,
		// The indexes of ClickHouse skip granules of data and are written in another form.
		ImplicitIndex: func(dump.Table, dump.Index) bool { return true },
		Binary:        func(hex string) string { return "unhex('" + hex + "')" },
		MaxParameters: 65535,
		FloatSpecial: func(v float64) (string, bool) {
			switch {
			case v != v:
				return "nan", true
			case v > 0:
				return "inf", true
			}
			return "-inf", true
		},
	})
}

// quoteIdentifier quotes a name with double quotes, which ClickHouse reads as an identifier.
func quoteIdentifier(name string) string {
	return dbmeta.QuoteIdentifier(name, `"`, `"`)
}

// literal writes a string literal. ClickHouse reads a backslash as an escape.
func literal(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return "'" + strings.ReplaceAll(s, "'", `\'`) + "'"
}

// column writes one column of a CREATE TABLE. A column that can be NULL has the type
// Nullable, except an array, which cannot be. A computed column is MATERIALIZED or
// an ALIAS, and has no other default.
func column(c dump.Column, typeSQL string, q func(string) string, _ func(string) string) string {
	var b strings.Builder
	if c.Nullable && !strings.HasPrefix(typeSQL, "Array(") {
		typeSQL = "Nullable(" + typeSQL + ")"
	}
	b.WriteString(q(c.Name) + " " + typeSQL)
	switch {
	case c.Generated != "" && c.Default != nil:
		kind := "MATERIALIZED"
		if strings.EqualFold(c.Generated, "alias") {
			kind = "ALIAS"
		}
		b.WriteString(" " + kind + " " + *c.Default)
	case c.Default != nil:
		b.WriteString(" DEFAULT " + *c.Default)
	}
	return b.String()
}

// engineClause writes the engine and the order of the rows, which is the key of the
// table. A table with no key, or with a key that can be NULL, has no order.
func engineClause(t dump.Table, _ func(string) string) string {
	order := "tuple()"
	for _, k := range t.Constraints {
		if k.Kind != "primary key" || len(k.Columns) == 0 {
			continue
		}
		nullable := false
		quoted := make([]string, len(k.Columns))
		for i, name := range k.Columns {
			quoted[i] = quoteIdentifier(name)
			for _, c := range t.Columns {
				if c.Name == name && c.Nullable {
					nullable = true
				}
			}
		}
		if !nullable {
			order = "(" + strings.Join(quoted, ", ") + ")"
		}
		break
	}
	return "ENGINE = MergeTree ORDER BY " + order
}

// refuseConstraint says why a constraint is not written. The primary key is the order of
// the table, and ClickHouse holds no other key.
func refuseConstraint(_ dump.Table, c dump.Constraint) string {
	if c.Kind == "primary key" {
		return "the primary key of ClickHouse is the order of the table, so " + c.Name + " is written there"
	}
	return "ClickHouse has no " + c.Kind + ", so " + c.Name + " is not written"
}
