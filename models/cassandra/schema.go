package cassandra

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerSchema() {
	// \dn. A keyspace is Cassandra's namespace and the only one it has.
	dbmeta.Schemas.Register(dbmeta.Cassandra, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT (text)'' AS "catalog"`),
			always(`, keyspace_name AS "name"`),
			always(`, (text)'' AS "owner"`),
			always(`, (text)NULL AS "comment"`),
			always(`FROM system_schema.keyspaces`),
		},
		Fields: []dbmeta.Field{
			{
				Name: "catalog",
				Desc: "always empty: the cluster is the thing above a keyspace" +
					" and it is in system.local, which no one statement can reach" +
					" from here, because CQL has no join",
			},
			{Name: "name"},
			{Name: "owner", Desc: "always empty: a keyspace has no owner in the catalog"},
			{Name: "comment", Desc: "always absent: a keyspace carries no comment"},
		},
		Params: filters("keyspace"),
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, pad{})
			return v, err
		},
	})

	// \dt and \dv together, which is what Tables returns everywhere.
	//
	// A materialized view is not in system_schema.tables. It is in
	// system_schema.views and CQL has no UNION, so this returns tables only
	// and Views returns the rest. That is a difference from every other
	// model, where one query answers both.
	dbmeta.Tables.Register(dbmeta.Cassandra, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			always(`SELECT (text)'' AS "catalog"`),
			always(`, keyspace_name AS "schema"`),
			always(`, table_name AS "name"`),
			always(`, (text)'table' AS "type"`),
			always(`, comment`),
			always(`FROM system_schema.tables`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: Cassandra has nothing above a keyspace"},
			{Name: "schema"}, {Name: "name"},
			{
				Name: "type",
				Desc: "always table: a materialized view is in system_schema.views" +
					" and CQL has no UNION, so the views query returns those",
			},
			{Name: "comment"},
		},
		Params: filters("table"),
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})

	// \d NAME.
	//
	// Two fields are read from one column. CQL has no CASE, so nullability
	// and the primary key flag cannot be computed in the statement, and kind
	// carries both: a column of kind partition_key or clustering is in the
	// primary key, and the primary key is the only thing in Cassandra that
	// cannot be null. Scan does the mapping. See D62.
	dbmeta.Columns.Register(dbmeta.Cassandra, &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			always(`SELECT (text)'' AS "catalog"`),
			always(`, keyspace_name AS "schema"`),
			always(`, table_name AS "table"`),
			always(`, column_name AS "name"`),
			always(`, position AS "ordinal"`),
			always(`, type AS "data_type"`),
			always(`, kind AS "nullable"`),
			always(`, (text)NULL AS "default"`),
			always(`, kind AS "primary_key"`),
			always(`, (text)NULL AS "identity"`),
			always(`, (text)NULL AS "generated"`),
			always(`, (text)NULL AS "comment"`),
			always(`FROM system_schema.columns`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: Cassandra has nothing above a keyspace"},
			{Name: "schema"}, {Name: "table"}, {Name: "name"},
			{
				Name: "ordinal",
				Desc: "the position within the primary key, and -1 for a column" +
					" outside it: Cassandra records no declaration order for a" +
					" table's columns and the catalog holds them alphabetically",
			},
			{Name: "data_type"},
			{
				Name: "nullable",
				Desc: "read from the column kind: only a primary key column" +
					" cannot be null in Cassandra",
			},
			{
				Name: "default",
				Desc: "always absent: CQL has no DEFAULT",
			},
			{Name: "primary_key", Desc: "read from the column kind"},
			{Name: "identity", Desc: "always absent: CQL has no identity column"},
			{Name: "generated", Desc: "always absent: CQL has no generated column"},
			{Name: "comment", Desc: "always absent: Cassandra records no column comment"},
		},
		Params: filters("table"),
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			// Both hold the column kind. The statement selects it twice,
			// once under each name, so the names say which field each copy
			// feeds rather than pretending to hold a boolean.
			var (
				v                 dbmeta.Column
				nullKind, keyKind string
			)
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Ordinal,
				&v.DataType, &nullKind, pad{}, &keyKind, pad{},
				pad{}, pad{})
			v.Nullable = !isKey(nullKind)
			v.PrimaryKey = isKey(keyKind)
			return v, err
		},
	})

	// \dm. A materialized view is Cassandra's only view.
	dbmeta.Views.Register(dbmeta.Cassandra, &dbmeta.Binding[dbmeta.View]{
		Stmt: dbmeta.Stmt{
			always(`SELECT (text)'' AS "catalog"`),
			always(`, keyspace_name AS "schema"`),
			always(`, view_name AS "name"`),
			always(`, where_clause AS "definition"`),
			always(`, (text)NULL AS "check_option"`),
			always(`, (boolean)false AS "updatable"`),
			always(`, (boolean)false AS "insertable"`),
			always(`, comment`),
			always(`FROM system_schema.views`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: Cassandra has nothing above a keyspace"},
			{Name: "schema"}, {Name: "name"},
			{
				Name: "definition",
				Desc: "the WHERE clause of the view, which is the only part of" +
					" the statement the catalog keeps",
			},
			{Name: "check_option", Desc: "always absent: CQL has no WITH CHECK OPTION"},
			{
				Name: "updatable",
				Desc: "always false: a materialized view is maintained from its" +
					" base table and cannot be written to",
			},
			{Name: "insertable", Desc: "always false, for the same reason as updatable"},
			{Name: "comment"},
		},
		Params: filters("view"),
		Scan: func(rows *sql.Rows) (dbmeta.View, error) {
			var v dbmeta.View
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Definition,
				pad{}, &v.Updatable, &v.Insertable, &v.Comment)
			return v, err
		},
	})
}
