package test

import (
	"fmt"
	"sort"
	"strings"

	"github.com/xo/dbmeta"
)

// Canonical projections of a metadata answer, for comparing databases of
// different families.
//
// # Why this exists and what it must never become
//
// dbmeta reports what a database said. It does not normalize an answer so that
// two databases agree, and a test must not push it to. That is the one rule
// this file could break, so the boundary is mechanical rather than a promise:
//
// Every function here takes a value and returns a new one. None of them takes
// a pointer to a model struct and none of them writes to a field. dbmeta's own
// types are read only from here.
//
// The raw values are asserted before any of this runs. postgres_test.go checks
// that a PostgreSQL column says "integer", sqlite3_test.go checks that a
// SQLite one says "INTEGER", and mysql_test.go checks "int(11)". Those tests
// are the record of what each database actually says, and this file cannot
// weaken them.
//
// canonicalFields below names every field this drops or maps. Widening it is a
// reviewed change, not a quiet one.
//
// # What is portable and what is not
//
// Gemini said no value can be compared across families and DeepSeek said a
// subset can. DeepSeek is right, and the evidence is local: the fault that
// justified the MariaDB comparison was a NULL that would not scan, which is a
// value fault. Names and row counts would have missed it.
//
// The subset is what the SQL standard makes a database record the same way:
// whether a column accepts NULL, where it sits in the table, whether it is in
// the primary key, which columns a constraint covers and in what order, and
// what a foreign key points at. Everything a database is free to spell its own
// way is dropped.

// canonicalFields records every field the canonical projection drops or maps,
// and why. It is checked by TestCanonicalFieldsAreRecorded, so a field cannot
// be quietly added to the drop list.
//
// A dropped field is one a database is free to spell its own way. A mapped
// field is one with a portable meaning and an unportable spelling.
var canonicalFields = map[string]string{
	// dropped, because every family spells a type differently and all are
	// right: integer, INTEGER, int, int(11), INTEGER as an affinity hint
	"Column.DataType": "dropped: a type spelling is per product",
	// dropped, because a default is stored as the database renders it:
	// 'red' on MariaDB, red on MySQL, 'red'::colour on PostgreSQL
	"Column.Default": "dropped: a default is rendered per product",
	// mapped to whether the column has an explicit default other than NULL.
	//
	// MariaDB records the four character string NULL as the default of a
	// nullable column that was declared without one, where a NOT NULL column
	// gets SQL NULL. It is saying that the column defaults to NULL, which is
	// true and is not what every other database says. TestMySQLNullDefault
	// pins the raw behaviour, so this mapping cannot hide it.
	"Column.HasDefault": "mapped: from Column.Default, present and not the literal NULL",
	// dropped, because only PostgreSQL and DuckDB have one and the text is
	// rewritten by the server on MariaDB and MySQL
	"Constraint.Definition": "dropped: a definition is rendered per product",
	// dropped, because DuckDB generates its own name and ignores the one in
	// the DDL, and SQLite names nothing but an index
	"Constraint.Name":             "dropped: a constraint name is generated per product",
	"ConstraintColumn.Constraint": "dropped: for the same reason",
	// dropped, because a catalog is a database on some and a name on others
	"*.Catalog": "dropped: a catalog means a different thing per product",
	// dropped, because SQLite has one schema and calls it main
	"*.Schema": "dropped: SQLite has one schema and it is called main",
	// dropped, because only some databases record one and the fixture sets
	// only two of them
	"*.Comment": "dropped: not every product records a comment on every object",
	// dropped, because only PostgreSQL has an identity column and only
	// PostgreSQL, MySQL and SQLite report a generated one. They are asserted
	// by the tests of the databases that have them.
	"Column.Identity":  "dropped: only PostgreSQL has an identity column",
	"Column.Generated": "dropped: not every product reports a generated column",
	// folded rather than dropped. The columns of a constraint become one
	// joined string in the order the ordinals gave, and the referenced table
	// and columns become one reference string, so that a constraint compares
	// as a whole rather than row by row.
	"ConstraintColumn.Name":         "folded into Columns, in Ordinal order",
	"ConstraintColumn.Ordinal":      "folded into Columns, as the order",
	"ConstraintColumn.ForeignTable": "folded into References",
	"ConstraintColumn.ForeignName":  "folded into References, in Ordinal order",
	// dropped, because a catalog and a schema mean different things per
	// product, which *.Catalog and *.Schema already say for every kind
	"ConstraintColumn.ForeignCatalog": "dropped: a catalog means a different thing per product",
	"ConstraintColumn.ForeignSchema":  "dropped: SQLite has one schema and it is called main",
	// dropped, because the table is on the canonical constraint already
	"ConstraintColumn.Table": "kept, as canonicalConstraint.Table",
}

// canonicalColumn is what every database must agree on about a column.
type canonicalColumn struct {
	Table      string
	Name       string
	Ordinal    int
	Nullable   bool
	PrimaryKey bool
	HasDefault bool
}

func (c canonicalColumn) String() string {
	return fmt.Sprintf("ordinal=%d nullable=%v primary_key=%v has_default=%v",
		c.Ordinal, c.Nullable, c.PrimaryKey, c.HasDefault)
}

func (c canonicalColumn) key() string { return c.Table + "." + c.Name }

// canonicalizeColumn returns a new value. It does not write to v.
func canonicalizeColumn(v dbmeta.Column) canonicalColumn {
	return canonicalColumn{
		Table:      v.Table,
		Name:       v.Name,
		Ordinal:    v.Ordinal,
		Nullable:   v.Nullable,
		PrimaryKey: v.PrimaryKey,
		// The value is per product and whether there is one is not. The
		// literal NULL is MariaDB spelling "no default" its own way, and
		// canonicalFields records it.
		HasDefault: v.Default.Valid && !strings.EqualFold(v.Default.V, "NULL"),
	}
}

// canonicalConstraint is what every database must agree on about a constraint.
//
// It is keyed by the table, the kind and the columns rather than by the name,
// because the name is generated differently everywhere. Two constraints of the
// same kind on the same columns of the same table are the same constraint.
type canonicalConstraint struct {
	Table string
	Type  string
	// Columns is the constraint's columns in order, joined.
	Columns string
	// References is the table and columns a foreign key points at, and empty
	// for anything else.
	References string
}

func (c canonicalConstraint) String() string {
	if c.References == "" {
		return c.key()
	}
	return c.key() + " -> " + c.References
}

func (c canonicalConstraint) key() string {
	return c.Table + " " + c.Type + " (" + c.Columns + ")"
}

// canonicalizeConstraints turns constraint columns into one value per
// constraint. It reads and returns new values.
func canonicalizeConstraints(cols []dbmeta.ConstraintColumn, kinds map[string]string) []canonicalConstraint {
	type group struct {
		table, ftable string
		cols, fcols   []string
	}
	byKey := map[string]*group{}
	var order []string
	for _, c := range cols {
		key := c.Table + "\x00" + c.Constraint
		g, ok := byKey[key]
		if !ok {
			g = &group{table: c.Table}
			byKey[key] = g
			order = append(order, key)
		}
		g.cols = append(g.cols, c.Name)
		if c.ForeignTable.Valid {
			g.ftable = c.ForeignTable.V
			g.fcols = append(g.fcols, c.ForeignName.V)
		}
	}
	out := make([]canonicalConstraint, 0, len(order))
	for _, key := range order {
		g := byKey[key]
		kind := kinds[key]
		if kind == "" {
			continue
		}
		c := canonicalConstraint{
			Table:   g.table,
			Type:    kind,
			Columns: strings.Join(g.cols, ", "),
		}
		if g.ftable != "" {
			c.References = g.ftable + "(" + strings.Join(g.fcols, ", ") + ")"
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].key() < out[j].key() })
	return out
}
