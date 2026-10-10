package athena

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// tableType is the word for the kind of the relation t. Athena says BASE TABLE
// for a Hive table, an external table and an Iceberg table alike, and VIEW for
// a view.
const tableType = `CASE t.table_type WHEN 'VIEW' THEN 'view' ELSE 'table' END`

func registerRelations() {
	// \l. The catalog is the one place that Athena reads, and SHOW CATALOGS is a
	// syntax error. INFORMATION_SCHEMA names the catalog of each schema.
	dbmeta.Databases.Register(dbmeta.Athena, &dbmeta.Binding[dbmeta.Database]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.catalog_name AS "name"`),
			always(`, '' AS "owner"`),
			always(`, '' AS "encoding"`),
			always(`, '' AS "collate"`),
			always(`, '' AS "ctype"`),
			always(`, CAST(NULL AS varchar) AS "access"`),
			always(`, CAST(NULL AS varchar) AS "tablespace"`),
			always(`, CAST(NULL AS varchar) AS "size"`),
			always(`, CAST(NULL AS varchar) AS "comment"`),
			always(`FROM (SELECT DISTINCT catalog_name FROM information_schema.schemata) s`),
			always(`WHERE ` + like("s.catalog_name", "@name")),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the data catalog, which is awsdatacatalog for the Glue Data Catalog. Athena can attach more catalogs, and the session reads only the one it is in"},
			{Name: "owner", Desc: "always empty: a catalog has no owner here"},
			{Name: "encoding", Desc: "always empty: Athena reports no encoding"},
			{Name: "collate", Desc: "always empty: a catalog has no collation"},
			{Name: "ctype", Desc: "always empty, for the same reason"},
			{Name: "access", Desc: "always absent"},
			{Name: "tablespace", Desc: "always absent: Athena has no tablespace"},
			{Name: "size", Desc: "always absent: Athena reports no size for a catalog"},
			{Name: "comment", Desc: "always absent"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "catalog name pattern, empty for every catalog", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Database, error) {
			var v dbmeta.Database
			err := rows.Scan(&v.Name, &v.Owner, &v.Encoding, &v.Collate, &v.CType,
				&v.Access, &v.Tablespace, &v.Size, &v.Comment)
			return v, err
		},
	})

	// \dn. A Glue database. The session sees only the Glue databases that its
	// principal can read.
	dbmeta.Schemas.Register(dbmeta.Athena, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.catalog_name AS "catalog"`),
			always(`, s.schema_name AS "name"`),
			always(`, '' AS "owner"`),
			always(`, CAST(NULL AS varchar) AS "comment"`),
			always(`FROM information_schema.schemata s`),
			always(`WHERE ` + notSystem("s.schema_name")),
			always(`AND ` + like("s.schema_name", "@schema")),
			always(`ORDER BY 1, 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"},
			{Name: "name", Desc: "the Glue database"},
			{Name: "owner", Desc: "always empty: SCHEMATA has no owner column, and a Glue database has an owner that only the Glue API reports"},
			{Name: "comment", Desc: "always absent: SCHEMATA has no description column, and the description of a Glue database is in the Glue API"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "Glue database name pattern, empty for every database", Default: ""},
			{Name: "with_system", Desc: "include information_schema, which Athena lists as a schema", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})

	// \dt and \dv. TABLES has four columns, and none of them holds a comment, an
	// owner, a size or a row count, so Table has only its names and its type.
	dbmeta.Tables.Register(dbmeta.Athena, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			always(`SELECT t.table_catalog AS "catalog"`),
			always(`, t.table_schema AS "schema"`),
			always(`, t.table_name AS "name"`),
			always(`, ` + tableType + ` AS "type"`),
			always(`, CAST(NULL AS varchar) AS "comment"`),
			always(`FROM information_schema.tables t`),
			always(`WHERE ` + notSystem("t.table_schema")),
			always(`AND ` + like("t.table_schema", "@schema")),
			always(`AND ` + like("t.table_name", "@name")),
			always(`AND (@types = '' OR ` + dbmeta.InList("@types", tableType) + `)`),
			always(`ORDER BY 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"},
			{Name: "schema", Desc: "the Glue database"},
			{Name: "name"},
			{Name: "type", Desc: "table or view. A Hive table, an external table, a table made by CTAS and an Iceberg table are all table, because TABLES says BASE TABLE for each"},
			{Name: "comment", Desc: "always absent: TABLES has no comment column. SHOW CREATE TABLE and SHOW TBLPROPERTIES hold it, and a SELECT cannot read them"},
		},
		Params: append(schemaName("table"), dbmeta.TypesParam()),
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})

	// \d name. COLUMNS carries the comment of a column, and a partition column of
	// a Hive table has the extra_info partition key and the last positions.
	dbmeta.Columns.Register(dbmeta.Athena, &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			always(`SELECT c.table_catalog AS "catalog"`),
			always(`, c.table_schema AS "schema"`),
			always(`, c.table_name AS "table"`),
			always(`, c.column_name AS "name"`),
			always(`, c.ordinal_position AS "ordinal"`),
			always(`, c.data_type AS "data_type"`),
			always(`, c.is_nullable = 'YES' AS "nullable"`),
			always(`, c.column_default AS "default"`),
			always(`, false AS "primary_key"`),
			always(`, CAST(NULL AS varchar) AS "identity"`),
			always(`, CAST(NULL AS varchar) AS "generated"`),
			always(`, c.comment AS "comment"`),
			always(`, CAST(NULL AS varchar) AS "collation"`),
			always(`FROM information_schema.columns c`),
			always(`WHERE ` + notSystem("c.table_schema")),
			always(`AND ` + like("c.table_schema", "@schema")),
			always(`AND ` + like("c.table_name", "@parent")),
			always(`AND ` + like("c.column_name", "@name")),
			always(`ORDER BY 2, 3, 5`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "ordinal", Desc: "ORDINAL_POSITION, one based. The partition columns of a Hive table come after the others"},
			{Name: "data_type", Desc: "DATA_TYPE as Athena writes it, such as varchar, integer or timestamp(6). Athena reports a Glue string as varchar and an int as integer"},
			{Name: "nullable", Desc: "IS_NULLABLE, which is always YES, because a Glue column has no NOT NULL"},
			{Name: "default", Desc: "always absent in practice: a Glue column has no default"},
			{Name: "primary_key", Desc: "always false: Athena has no primary key"},
			{Name: "identity", Desc: "always absent: Athena has no identity column"},
			{Name: "generated", Desc: "always absent: Athena has no generated column"},
			{Name: "comment", Desc: "the comment of the column. A column that has none is absent in a table that Athena made, and empty in a table that another service made, such as an external table of Redshift Spectrum, because Glue holds the empty text there"},
			{Name: "collation", Desc: "always absent: Athena has no collation"},
		},
		Params: schemaParentName("table", "column"),
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			var v dbmeta.Column
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Ordinal,
				&v.DataType, &v.Nullable, &v.Default, &v.PrimaryKey, &v.Identity,
				&v.Generated, &v.Comment, &v.Collation)
			return v, err
		},
	})

	// \dv with the definition. VIEWS holds the definition of a Presto view as
	// Athena writes it back, which is formatted SQL.
	dbmeta.Views.Register(dbmeta.Athena, &dbmeta.Binding[dbmeta.View]{
		Stmt: dbmeta.Stmt{
			always(`SELECT v.table_catalog AS "catalog"`),
			always(`, v.table_schema AS "schema"`),
			always(`, v.table_name AS "name"`),
			always(`, v.view_definition AS "definition"`),
			always(`, CAST(NULL AS varchar) AS "check_option"`),
			always(`, false AS "updatable"`),
			always(`, false AS "insertable"`),
			always(`, CAST(NULL AS varchar) AS "comment"`),
			always(`FROM information_schema.views v`),
			always(`WHERE ` + notSystem("v.table_schema")),
			always(`AND ` + like("v.table_schema", "@schema")),
			always(`AND ` + like("v.table_name", "@name")),
			always(`ORDER BY 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{Name: "definition", Desc: "VIEW_DEFINITION, the SELECT that the view runs, which Athena reformats onto many lines"},
			{Name: "check_option", Desc: "always absent: Athena has no WITH CHECK OPTION"},
			{Name: "updatable", Desc: "always false: no Athena view can be updated"},
			{Name: "insertable", Desc: "always false: no Athena view accepts an insert"},
			{Name: "comment", Desc: "always absent: a view has no comment in any relation that a SELECT reads"},
		},
		Params: schemaName("view"),
		Scan: func(rows *sql.Rows) (dbmeta.View, error) {
			var v dbmeta.View
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Definition,
				&v.CheckOption, &v.Updatable, &v.Insertable, &v.Comment)
			return v, err
		},
	})
}
