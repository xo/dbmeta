package clickhouse

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerRelations() {
	// \dn. A ClickHouse database is the only namespace there is.
	dbmeta.Schemas.Register(dbmeta.ClickHouse, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, d.name AS "name"`),
			always(`, '' AS "owner"`),
			always(`, nullIf(d.comment, '') AS "comment"`),
			always(`FROM system.databases d`),
			always(`WHERE ` + notSystem("d.name")),
			always(`AND (@schema = '' OR d.name LIKE @schema)`),
			always(`ORDER BY d.name`),
		},
		Fields: []dbmeta.Field{
			{
				Name: "catalog",
				Desc: "always empty: ClickHouse has one level of namespace and" +
					" nothing above a database",
			},
			{Name: "name"},
			{Name: "owner", Desc: "always empty: a database records no owner"},
			{Name: "comment"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "database name pattern, empty for every database", Default: ""},
			{Name: "with_system", Desc: "include the databases ClickHouse keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})

	// \l. The same rows under the other name, which is what models/mysql does
	// for the same reason: the product has one level and psql has two.
	dbmeta.Databases.Register(dbmeta.ClickHouse, &dbmeta.Binding[dbmeta.Database]{
		Stmt: dbmeta.Stmt{
			always(`SELECT d.name AS "name"`),
			always(`, '' AS "owner"`),
			always(`, 'UTF-8' AS "encoding"`),
			always(`, '' AS "collate"`),
			always(`, '' AS "ctype"`),
			always(`, NULL AS "access"`),
			always(`, NULL AS "tablespace"`),
			always(`, '' AS "size"`),
			always(`, nullIf(d.comment, '') AS "comment"`),
			always(`FROM system.databases d`),
			always(`WHERE ` + notSystem("d.name")),
			always(`AND (@name = '' OR d.name LIKE @name)`),
			always(`ORDER BY d.name`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "owner", Desc: "always empty: a database records no owner"},
			{
				Name: "encoding",
				Desc: "always UTF-8, which is what ClickHouse stores text in and" +
					" is not a per database setting",
			},
			{Name: "collate", Desc: "always empty: a collation is chosen per comparison"},
			{Name: "ctype", Desc: "always empty, for the same reason"},
			{
				Name: "access",
				Desc: "always absent: a grant names a database and the privileges" +
					" query returns it",
			},
			{Name: "tablespace", Desc: "always absent: storage is chosen per table by its policy"},
			{
				Name: "size",
				Desc: "always empty: a size would have to sum system.parts, which is" +
					" a scan that grows with the data rather than with the catalog",
			},
			{Name: "comment"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "database name pattern, empty for every one", Default: ""},
			{Name: "with_system", Desc: "include the databases ClickHouse keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Database, error) {
			var v dbmeta.Database
			err := rows.Scan(&v.Name, &v.Owner, &v.Encoding, &v.Collate, &v.CType,
				&v.Access, &v.Tablespace, &v.Size, &v.Comment)
			return v, err
		},
	})

	// \dt and \dv. The engine says which it is, and a view is a row in
	// system.tables like any other.
	dbmeta.Tables.Register(dbmeta.ClickHouse, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, t.database AS "schema"`),
			always(`, t.name AS "name"`),
			always(`, multiIf(t.engine = 'MaterializedView', 'materialized view'` +
				`, t.engine IN (` + viewEngines + `), 'view'` +
				`, t.is_temporary, 'temporary table', 'table') AS "type"`),
			always(`, nullIf(t.comment, '') AS "comment"`),
			always(`FROM system.tables t`),
			always(`WHERE ` + notSystem("t.database")),
			always(`AND (@schema = '' OR t.database LIKE @schema)`),
			always(`AND (@name = '' OR t.name LIKE @name)`),
			always(`ORDER BY t.database, t.name`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: ClickHouse has nothing above a database"},
			{Name: "schema"}, {Name: "name"},
			{
				Name: "type",
				Desc: "read from the engine, which is what makes a ClickHouse table a view",
			},
			{Name: "comment"},
		},
		Params: schemaNameSystem("table"),
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})

	// \d NAME.
	dbmeta.Columns.Register(dbmeta.ClickHouse, &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, c.database AS "schema"`),
			always(`, c.table AS "table"`),
			always(`, c.name AS "name"`),
			always(`, c.position AS "ordinal"`),
			always(`, c.type AS "data_type"`),
			// Nullable is part of the type rather than a property of the
			// column, so it is read off the type name.
			always(`, startsWith(c.type, 'Nullable(') AS "nullable"`),
			always(`, nullIf(c.default_expression, '') AS "default"`),
			always(`, c.is_in_primary_key AS "primary_key"`),
			always(`, NULL AS "identity"`),
			// DEFAULT is an ordinary default. MATERIALIZED and ALIAS are
			// computed, which is what generated means everywhere else.
			always(`, nullIf(if(c.default_kind IN ('MATERIALIZED', 'ALIAS')` +
				`, c.default_kind, ''), '') AS "generated"`),
			always(`, nullIf(c.comment, '') AS "comment"`),
			always(`FROM system.columns c`),
			always(`WHERE ` + notSystem("c.database")),
			always(`AND (@schema = '' OR c.database LIKE @schema)`),
			always(`AND (@name = '' OR c.table LIKE @name)`),
			always(`ORDER BY c.database, c.table, c.position`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: ClickHouse has nothing above a database"},
			{Name: "schema"}, {Name: "table"}, {Name: "name"}, {Name: "ordinal"},
			{Name: "data_type"},
			{
				Name: "nullable",
				Desc: "read from the type: ClickHouse spells it Nullable(T) rather" +
					" than recording it beside the column",
			},
			{Name: "default"},
			{
				Name: "primary_key",
				Desc: "whether the column is in the primary key, which in ClickHouse" +
					" orders the data and does not make it unique",
			},
			{Name: "identity", Desc: "always absent: ClickHouse has no identity column"},
			{
				Name: "generated",
				Desc: "MATERIALIZED or ALIAS for a computed column, absent for a plain" +
					" one. A DEFAULT is a default and is in that field",
			},
			{Name: "comment"},
		},
		Params: schemaNameSystem("table"),
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			var v dbmeta.Column
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Ordinal,
				&v.DataType, &v.Nullable, &v.Default, &v.PrimaryKey, &v.Identity,
				&v.Generated, &v.Comment)
			return v, err
		},
	})

	// \dv. A view carries its statement, which system.tables keeps whole.
	dbmeta.Views.Register(dbmeta.ClickHouse, &dbmeta.Binding[dbmeta.View]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, t.database AS "schema"`),
			always(`, t.name AS "name"`),
			always(`, if(t.as_select != '', t.as_select, t.create_table_query) AS "definition"`),
			always(`, NULL AS "check_option"`),
			always(`, 0 AS "updatable"`),
			always(`, t.engine = 'MaterializedView' AS "insertable"`),
			always(`, nullIf(t.comment, '') AS "comment"`),
			always(`FROM system.tables t`),
			always(`WHERE t.engine IN (` + viewEngines + `)`),
			always(`AND ` + notSystem("t.database")),
			always(`AND (@schema = '' OR t.database LIKE @schema)`),
			always(`AND (@name = '' OR t.name LIKE @name)`),
			always(`ORDER BY t.database, t.name`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: ClickHouse has nothing above a database"},
			{Name: "schema"}, {Name: "name"},
			{
				Name: "definition",
				Desc: "the SELECT the view stores, or the whole CREATE where the" +
					" server records no separate select",
			},
			{Name: "check_option", Desc: "always absent: ClickHouse has no WITH CHECK OPTION"},
			{Name: "updatable", Desc: "always false: no ClickHouse view can be updated"},
			{
				Name: "insertable",
				Desc: "true for a materialized view, which accepts the inserts that" +
					" feed it. A plain view accepts none",
			},
			{Name: "comment"},
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
