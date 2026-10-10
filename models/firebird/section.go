package firebird

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerSections() {
	// A NOT NULL on a column of a table is a constraint of its own in
	// RDB$RELATION_CONSTRAINTS, and Firebird names it INTEG_n when the person
	// gave no name. RDB$CHECK_CONSTRAINTS maps it to a row that holds the name
	// of the column. A NOT NULL that a domain carries is not a constraint and
	// is not here. Firebird has no table inheritance, so the three flags that
	// describe inheritance are fixed, and a NOT NULL is always validated,
	// because Firebird cannot add one without checking the rows.
	dbmeta.NotNulls.Register(dbmeta.Firebird, &dbmeta.Binding[dbmeta.NotNull]{
		Stmt: dbmeta.Stmt{
			always(`SELECT ` + noSchema),
			always(`, TRIM(TRAILING FROM rc.RDB$RELATION_NAME) AS "table"`),
			always(`, TRIM(TRAILING FROM rc.RDB$CONSTRAINT_NAME) AS "name"`),
			always(`, TRIM(TRAILING FROM cc.RDB$TRIGGER_NAME) AS "column"`),
			always(`, FALSE AS "no_inherit"`),
			always(`, TRUE AS "local"`),
			always(`, FALSE AS "inherited"`),
			always(`, TRUE AS "validated"`),
			always(`FROM RDB$RELATION_CONSTRAINTS rc`),
			always(`JOIN RDB$RELATIONS r ON r.RDB$RELATION_NAME = rc.RDB$RELATION_NAME`),
			always(`JOIN RDB$CHECK_CONSTRAINTS cc ON cc.RDB$CONSTRAINT_NAME = rc.RDB$CONSTRAINT_NAME`),
			always(`WHERE rc.RDB$CONSTRAINT_TYPE = 'NOT NULL'`),
			always(`AND ` + userObject(`r.RDB$SYSTEM_FLAG`)),
			always(`AND ` + like(`''`, `@schema`)),
			always(`AND ` + like(`rc.RDB$RELATION_NAME`, `@parent`)),
			always(`AND ` + like(`rc.RDB$CONSTRAINT_NAME`, `@name`)),
			always(`ORDER BY rc.RDB$RELATION_NAME, rc.RDB$CONSTRAINT_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: schemaDesc},
			{Name: "table"}, {Name: "name"},
			{Name: "column", Desc: "from the row of RDB$CHECK_CONSTRAINTS that implements the constraint"},
			{Name: "no_inherit", Desc: "always false: Firebird has no table inheritance"},
			{Name: "local", Desc: "always true, for the same reason"},
			{Name: "inherited", Desc: "always false, for the same reason"},
			{Name: "validated", Desc: "always true: Firebird checks the rows before it adds a NOT NULL"},
		},
		Params: parentAndName("constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.NotNull, error) {
			var v dbmeta.NotNull
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Column, &v.NoInherit,
				&v.Local, &v.Inherited, &v.Validated)
			return v, err
		},
	})
}
