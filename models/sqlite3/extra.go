package sqlite3

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// The kinds psql has no command for. SQLite answers three of the six: the
// columns of a constraint, the definition of a view, and the schema an
// unqualified name resolves in.
//
// It answers none of the other three and cannot. It has no routine with named
// parameters, because a function is compiled C and SQLite publishes only its
// argument count. It has no enumerated type. And sqlite_stat1 holds one text
// string per index rather than anything about a column's values, so there is
// nothing to report for column statistics. See docs/COVERAGE.md.

func registerExtra() {
	registerConstraintColumns()
	registerViews()
	registerCurrentSchema()
}

// registerConstraintColumns is Constraints in parts.
//
// Three pragmas again, joined by UNION ALL, matching the three kinds
// Constraints reports. A check constraint has no columns here for the same
// reason it has no row there: SQLite keeps it only as DDL text.
func registerConstraintColumns() {
	dbmeta.ConstraintColumns.Register(dbmeta.SQLite3, &dbmeta.Binding[dbmeta.ConstraintColumn]{
		Stmt: dbmeta.Stmt{
			// the primary key. pk is the position within the key, counting
			// from one, which is the ordinal this kind wants.
			always(`SELECT '' AS "catalog"`),
			always(`, ` + mainSchema),
			always(`, m.name AS "table"`),
			always(`, 'pk_' || m.name AS "constraint"`),
			always(`, c.name AS "name"`),
			always(`, c.pk AS "ordinal"`),
			always(`, NULL AS "foreign_catalog"`),
			always(`, NULL AS "foreign_schema"`),
			always(`, NULL AS "foreign_table"`),
			always(`, NULL AS "foreign_name"`),
			always(`FROM sqlite_schema m JOIN pragma_table_xinfo(m.name) c`),
			always(`WHERE m.type = 'table' AND c.pk > 0`),
			always(`AND ` + notSystem),
			always(`AND (@schema = '' OR 'main' LIKE @schema)`),
			always(`AND (@parent = '' OR m.name LIKE @parent)`),
			always(`AND (@name = '' OR ('pk_' || m.name) LIKE @name)`),

			// every unique index, which is how SQLite records a UNIQUE clause
			always(`UNION ALL`),
			always(`SELECT '', 'main', m.name, i.name, x.name, x.seqno + 1`),
			always(`, NULL, NULL, NULL, NULL`),
			always(`FROM sqlite_schema m JOIN pragma_index_list(m.name) i`),
			always(`JOIN pragma_index_xinfo(i.name) x`),
			always(`WHERE m.type = 'table' AND i."unique" = 1 AND i.origin <> 'pk' AND x.key = 1`),
			always(`AND ` + notSystem),
			always(`AND (@schema = '' OR 'main' LIKE @schema)`),
			always(`AND (@parent = '' OR m.name LIKE @parent)`),
			always(`AND (@name = '' OR i.name LIKE @name)`),

			// every foreign key. seq is the position within a composite key,
			// counting from zero.
			always(`UNION ALL`),
			always(`SELECT '', 'main', m.name, 'fk_' || m.name || '_' || f.id, f."from", f.seq + 1`),
			always(`, '', 'main', f."table", f."to"`),
			always(`FROM sqlite_schema m JOIN pragma_foreign_key_list(m.name) f`),
			always(`WHERE m.type = 'table'`),
			always(`AND ` + notSystem),
			always(`AND (@schema = '' OR 'main' LIKE @schema)`),
			always(`AND (@parent = '' OR m.name LIKE @parent)`),
			always(`AND (@name = '' OR ('fk_' || m.name || '_' || f.id) LIKE @name)`),
			always(`ORDER BY 3, 4, 6`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: SQLite has no catalog above a schema"},
			{Name: "schema"}, {Name: "table"},
			{
				Name: "constraint",
				Desc: "named the way Constraints names it, so the two join",
			},
			{Name: "name", Desc: "the column the constraint is on"},
			{Name: "ordinal", Desc: "one based position within the constraint"},
			{Name: "foreign_catalog", Desc: "always empty for a foreign key and absent otherwise"},
			{Name: "foreign_schema", Desc: "always main: SQLite cannot reference another database"},
			{Name: "foreign_table"},
			{
				Name: "foreign_name",
				Desc: "absent when the key points at the primary key of the other table without naming it",
			},
		},
		Params: schemaParentName("constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.ConstraintColumn, error) {
			var v dbmeta.ConstraintColumn
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Constraint, &v.Name,
				&v.Ordinal, &v.ForeignCatalog, &v.ForeignSchema, &v.ForeignTable, &v.ForeignName)
			return v, err
		},
	})
}

// registerViews answers what a view selects.
//
// sqlite_schema.sql holds the CREATE VIEW statement as it was written, which
// is the whole statement rather than the select alone. SQLite stores nothing
// else about a view.
func registerViews() {
	dbmeta.Views.Register(dbmeta.SQLite3, &dbmeta.Binding[dbmeta.View]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, ` + mainSchema),
			always(`, m.name AS "name"`),
			always(`, m.sql AS "definition"`),
			always(`, NULL AS "check_option"`),
			// A SQLite view is read only unless a trigger makes it writable,
			// and that is a trigger rather than a property of the view.
			always(`, FALSE AS "updatable"`),
			always(`, FALSE AS "insertable"`),
			always(`, NULL AS "comment"`),
			always(`FROM sqlite_schema m`),
			always(`WHERE m.type = 'view'`),
			always(`AND ` + notSystem),
			always(`AND (@schema = '' OR 'main' LIKE @schema)`),
			always(`AND (@name = '' OR m.name LIKE @name)`),
			always(`ORDER BY m.name`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty"},
			{Name: "schema", Desc: "always main: sqlite_schema describes the main database"},
			{Name: "name"},
			{
				Name: "definition",
				Desc: "the whole CREATE VIEW statement as written, which is all SQLite keeps",
			},
			{Name: "check_option", Desc: "always absent: SQLite has no check option"},
			{
				Name: "updatable",
				Desc: "always false: a SQLite view is read only unless an INSTEAD OF trigger makes it writable, which is a property of the trigger",
			},
			{Name: "insertable", Desc: "always false, for the same reason"},
			{Name: "comment", Desc: "always absent"},
		},
		Params: schemaNameSystem("view"),
		Scan: func(rows *sql.Rows) (dbmeta.View, error) {
			var v dbmeta.View
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Definition,
				&v.CheckOption, &v.Updatable, &v.Insertable, &v.Comment)
			return v, err
		},
	})
}

// registerCurrentSchema answers where an unqualified name resolves.
//
// SQLite resolves an unqualified name in temp first and then in main, and it
// has no statement that reports which. main is the answer for every name that
// is not in a temporary object, so main is what this reports. It is the one
// kind here whose answer is a constant.
func registerCurrentSchema() {
	dbmeta.CurrentSchema.Register(dbmeta.SQLite3, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, d.name AS "name"`),
			always(`, '' AS "owner"`),
			always(`, NULL AS "comment"`),
			always(`FROM pragma_database_list d`),
			always(`WHERE d.name = 'main'`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty"},
			{
				Name: "name",
				Desc: "always main. SQLite looks in temp first and reports nothing about that, so main is the answer for anything not temporary",
			},
			{Name: "owner", Desc: "always empty: SQLite has no users"},
			{Name: "comment", Desc: "always absent"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})
}
