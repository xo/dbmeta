package exasol

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerSections() {
	// A NOT NULL is a constraint of its own in EXA_ALL_CONSTRAINTS, and
	// Exasol names it SYS_n when the statement gave no name. The column is a
	// row of EXA_ALL_CONSTRAINT_COLUMNS. Exasol has no table inheritance, so
	// the flags that describe inheritance are fixed. A disabled constraint
	// checks no new row but is still validated, so validated follows
	// CONSTRAINT_ENABLED.
	dbmeta.NotNulls.Register(dbmeta.Exasol, &dbmeta.Binding[dbmeta.NotNull]{
		Stmt: dbmeta.Stmt{
			always(`SELECT k.CONSTRAINT_SCHEMA AS "schema"`),
			always(`, k.CONSTRAINT_TABLE AS "table"`),
			always(`, k.CONSTRAINT_NAME AS "name"`),
			always(`, k.COLUMN_NAME AS "column"`),
			always(`, FALSE AS "no_inherit"`),
			always(`, TRUE AS "local"`),
			always(`, FALSE AS "inherited"`),
			always(`, c.CONSTRAINT_ENABLED AS "validated"`),
			always(`FROM EXA_ALL_CONSTRAINT_COLUMNS k`),
			always(`JOIN EXA_ALL_CONSTRAINTS c ON c.CONSTRAINT_SCHEMA = k.CONSTRAINT_SCHEMA`),
			always(`AND c.CONSTRAINT_TABLE = k.CONSTRAINT_TABLE AND c.CONSTRAINT_NAME = k.CONSTRAINT_NAME`),
			always(`WHERE k.CONSTRAINT_TYPE = 'NOT NULL'`),
			always(`AND ` + like(`k.CONSTRAINT_SCHEMA`, `@schema`)),
			always(`AND ` + like(`k.CONSTRAINT_TABLE`, `@parent`)),
			always(`AND ` + like(`k.CONSTRAINT_NAME`, `@name`)),
			always(`ORDER BY 1, 2, 4`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "column", Desc: "from COLUMN_NAME of the constraint columns"},
			{Name: "no_inherit", Desc: "always false: Exasol has no table inheritance"},
			{Name: "local", Desc: "always true, for the same reason"},
			{Name: "inherited", Desc: "always false, for the same reason"},
			{Name: "validated", Desc: "CONSTRAINT_ENABLED: Exasol validates the rows when it enables a constraint"},
		},
		Params: parentAndName("constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.NotNull, error) {
			var v dbmeta.NotNull
			err := rows.Scan(dbmeta.NullAsEmpty(&v.Schema), dbmeta.NullAsEmpty(&v.Table), dbmeta.NullAsEmpty(&v.Name), dbmeta.NullAsEmpty(&v.Column),
				&v.NoInherit, &v.Local, &v.Inherited, &v.Validated)
			return v, err
		},
	})
}
