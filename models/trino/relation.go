package trino

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerRelations() {
	// \l. A Trino catalog is a configured connector rather than a store, so
	// this is the list of places the server can reach.
	dbmeta.Databases.Register(dbmeta.Trino, &dbmeta.Binding[dbmeta.Database]{
		Stmt: dbmeta.Stmt{
			always(`SELECT c.catalog_name AS "name"`),
			always(`, '' AS "owner"`),
			always(`, '' AS "encoding"`),
			always(`, '' AS "collate"`),
			always(`, '' AS "ctype"`),
			always(`, CAST(NULL AS varchar) AS "access"`),
			always(`, CAST(NULL AS varchar) AS "tablespace"`),
			always(`, '' AS "size"`),
			// The connector is the one fact a catalog carries beyond its name,
			// and it is what a person wants when asking what a catalog is.
			always(`, c.connector_name AS "comment"`),
			always(`FROM system.metadata.catalogs c`),
			always(`WHERE (@with_system = true OR c.catalog_name NOT IN (` + systemCatalogs + `))`),
			always(`AND (@name = '' OR c.catalog_name LIKE @name)`),
			always(`ORDER BY c.catalog_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "owner", Desc: "always empty: a catalog records no owner"},
			{Name: "encoding", Desc: "always empty: Trino is UTF-8 throughout and stores no per catalog encoding"},
			{Name: "collate", Desc: "always empty: a catalog has no collation"},
			{Name: "ctype", Desc: "always empty: a catalog has no character type"},
			{Name: "access", Desc: "always absent: access is per table and Privileges reads it"},
			{Name: "tablespace", Desc: "always absent: Trino stores nothing of its own"},
			{Name: "size", Desc: "always empty: the size belongs to whatever the connector reaches"},
			{Name: "comment", Desc: "the connector name, which is what a catalog is"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "catalog name pattern, empty for every catalog", Default: ""},
			{Name: "with_system", Desc: "include the catalogs Trino keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Database, error) {
			var v dbmeta.Database
			err := rows.Scan(&v.Name, &v.Owner, &v.Encoding, &v.Collate, &v.CType,
				&v.Access, &v.Tablespace, &v.Size, &v.Comment)
			return v, err
		},
	})

	// \dn. system.jdbc.schemas spans every catalog, which the per catalog
	// information_schema cannot.
	dbmeta.Schemas.Register(dbmeta.Trino, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.table_catalog AS "catalog"`),
			always(`, s.table_schem AS "name"`),
			always(`, '' AS "owner"`),
			always(`, CAST(NULL AS varchar) AS "comment"`),
			always(`FROM system.jdbc.schemas s`),
			always(`WHERE ` + notSystem("s.table_catalog", "s.table_schem")),
			always(`AND (@catalog = '' OR s.table_catalog LIKE @catalog)`),
			always(`AND (@schema = '' OR s.table_schem LIKE @schema)`),
			always(`ORDER BY s.table_catalog, s.table_schem`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"},
			{Name: "name"},
			{
				Name: "owner",
				Desc: "always empty: system.metadata.schemas_authorization holds" +
					" an owner and only for a connector that has one",
			},
			{Name: "comment", Desc: "always absent: Trino stores no schema comment"},
		},
		Params: []dbmeta.Param{
			{Name: "catalog", Desc: "catalog name pattern, empty for every catalog", Default: ""},
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "with_system", Desc: "include the catalogs and schemas Trino keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})

	// \dt and \dv. The comment is a join rather than a column, because
	// system.jdbc.tables has a remarks column and leaves it empty, and
	// system.metadata.table_comments is where a comment actually lands.
	dbmeta.Tables.Register(dbmeta.Trino, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			always(`SELECT t.table_cat AS "catalog"`),
			always(`, t.table_schem AS "schema"`),
			always(`, t.table_name AS "name"`),
			always(`, CASE t.table_type WHEN 'VIEW' THEN 'view' ELSE 'table' END AS "type"`),
			always(`, tc.comment AS "comment"`),
			always(`FROM system.jdbc.tables t`),
			always(`LEFT JOIN system.metadata.table_comments tc` +
				` ON tc.catalog_name = t.table_cat` +
				` AND tc.schema_name = t.table_schem` +
				` AND tc.table_name = t.table_name`),
			always(`WHERE ` + notSystem("t.table_cat", "t.table_schem")),
			always(`AND (@catalog = '' OR t.table_cat LIKE @catalog)`),
			always(`AND (@schema = '' OR t.table_schem LIKE @schema)`),
			always(`AND (@name = '' OR t.table_name LIKE @name)`),
			always(`ORDER BY t.table_cat, t.table_schem, t.table_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{Name: "type", Desc: "table or view. Trino has no other kind of relation"},
			{Name: "comment"},
		},
		Params: catalogSchemaName("table"),
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})

	// \d name. system.jdbc.columns is the only source that carries the column
	// comment: information_schema.columns has no column for it.
	dbmeta.Columns.Register(dbmeta.Trino, &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			always(`SELECT c.table_cat AS "catalog"`),
			always(`, c.table_schem AS "schema"`),
			always(`, c.table_name AS "table"`),
			always(`, c.column_name AS "name"`),
			always(`, c.ordinal_position AS "ordinal"`),
			always(`, c.type_name AS "data_type"`),
			// nullable is the JDBC code: 0 is NO NULLS and 1 is NULLABLE.
			always(`, c.nullable <> 0 AS "nullable"`),
			always(`, c.column_def AS "default"`),
			always(`, false AS "primary_key"`),
			always(`, CAST(NULL AS varchar) AS "identity"`),
			always(`, CAST(NULL AS varchar) AS "generated"`),
			always(`, c.remarks AS "comment"`),
			always(`FROM system.jdbc.columns c`),
			always(`WHERE ` + notSystem("c.table_cat", "c.table_schem")),
			always(`AND (@catalog = '' OR c.table_cat LIKE @catalog)`),
			always(`AND (@schema = '' OR c.table_schem LIKE @schema)`),
			always(`AND (@parent = '' OR c.table_name LIKE @parent)`),
			always(`AND (@name = '' OR c.column_name LIKE @name)`),
			always(`ORDER BY c.table_cat, c.table_schem, c.table_name, c.ordinal_position`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "ordinal"},
			{Name: "data_type", Desc: "the Trino type, such as varchar(255) or array(varchar)"},
			{Name: "nullable"},
			{
				Name: "default",
				Desc: "always absent in practice: Trino parses a default and no" +
					" connector here records one",
			},
			{Name: "primary_key", Desc: "always false: Trino has no primary key"},
			{Name: "identity", Desc: "always absent: Trino has no identity column"},
			{Name: "generated", Desc: "always absent: Trino has no generated column"},
			{Name: "comment"},
		},
		Params: []dbmeta.Param{
			{Name: "catalog", Desc: "catalog name pattern, empty for every catalog", Default: ""},
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
			{Name: "name", Desc: "column name pattern, empty for every column", Default: ""},
			{Name: "with_system", Desc: "include the catalogs and schemas Trino keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			var v dbmeta.Column
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Ordinal,
				&v.DataType, &v.Nullable, &v.Default, &v.PrimaryKey, &v.Identity,
				&v.Generated, &v.Comment)
			return v, err
		},
	})

	// \dv with the definition. This is the one query that cannot span
	// catalogs: no cross catalog source carries a view definition, and the
	// catalog cannot come from a bind parameter, so it reads the
	// information_schema of the session catalog.
	dbmeta.Views.Register(dbmeta.Trino, &dbmeta.Binding[dbmeta.View]{
		Stmt: dbmeta.Stmt{
			always(`SELECT v.table_catalog AS "catalog"`),
			always(`, v.table_schema AS "schema"`),
			always(`, v.table_name AS "name"`),
			always(`, v.view_definition AS "definition"`),
			always(`, CAST(NULL AS varchar) AS "check_option"`),
			always(`, false AS "updatable"`),
			always(`, false AS "insertable"`),
			always(`, tc.comment AS "comment"`),
			always(`FROM information_schema.views v`),
			always(`LEFT JOIN system.metadata.table_comments tc` +
				` ON tc.catalog_name = v.table_catalog` +
				` AND tc.schema_name = v.table_schema` +
				` AND tc.table_name = v.table_name`),
			always(`WHERE ` + notSystem("v.table_catalog", "v.table_schema")),
			always(`AND (@schema = '' OR v.table_schema LIKE @schema)`),
			always(`AND (@name = '' OR v.table_name LIKE @name)`),
			always(`ORDER BY v.table_schema, v.table_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{Name: "definition"},
			{Name: "check_option", Desc: "always absent: Trino has no WITH CHECK OPTION"},
			{Name: "updatable", Desc: "always false: no Trino view can be updated"},
			{Name: "insertable", Desc: "always false: no Trino view accepts an insert"},
			{Name: "comment"},
		},
		Params: []dbmeta.Param{
			{
				Name: "schema",
				Desc: "schema name pattern, empty for every schema. This query" +
					" reads the session catalog only, because no cross catalog" +
					" source carries a view definition",
				Default: "",
			},
			{Name: "name", Desc: "view name pattern, empty for every view", Default: ""},
			{Name: "with_system", Desc: "include the catalogs and schemas Trino keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.View, error) {
			var v dbmeta.View
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Definition,
				&v.CheckOption, &v.Updatable, &v.Insertable, &v.Comment)
			return v, err
		},
	})

	// \dd. Trino records a comment on a table and on a column, and
	// table_comments spans catalogs.
	dbmeta.Comments.Register(dbmeta.Trino, &dbmeta.Binding[dbmeta.Comment]{
		Stmt: dbmeta.Stmt{
			always(`SELECT tc.schema_name AS "schema"`),
			always(`, tc.table_name AS "name"`),
			always(`, 'table' AS "type"`),
			always(`, tc.comment AS "comment"`),
			always(`FROM system.metadata.table_comments tc`),
			always(`WHERE tc.comment IS NOT NULL`),
			always(`AND ` + notSystem("tc.catalog_name", "tc.schema_name")),
			always(`AND (@catalog = '' OR tc.catalog_name LIKE @catalog)`),
			always(`AND (@schema = '' OR tc.schema_name LIKE @schema)`),
			always(`AND (@name = '' OR tc.table_name LIKE @name)`),
			always(`ORDER BY tc.catalog_name, tc.schema_name, tc.table_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{Name: "type", Desc: "always table: a column comment is on the column and Columns reads it"},
			{Name: "comment"},
		},
		Params: catalogSchemaName("table"),
		Scan: func(rows *sql.Rows) (dbmeta.Comment, error) {
			var v dbmeta.Comment
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})
}
