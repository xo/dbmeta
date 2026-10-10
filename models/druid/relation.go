package druid

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// catalogDesc describes the catalog field of every row.
const catalogDesc = "always druid, the one catalog Druid SQL reports"

// tableType is the word for a table's kind. TABLE is a datasource or a
// lookup, and SYSTEM_TABLE is a table of INFORMATION_SCHEMA or sys.
const tableType = `CASE t.TABLE_TYPE WHEN 'TABLE' THEN 'table' WHEN 'SYSTEM_TABLE' THEN 'system table'` +
	` ELSE LOWER(t.TABLE_TYPE) END`

// types is the filter on Table.Type. Druid plans STRPOS only when its second
// argument is a constant, so the list is searched for each type in turn, and
// each type is a test of TABLE_TYPE. A type that Druid has and this does not
// name is never matched by a list.
const types = `(CAST(@types AS VARCHAR) = ''` +
	` OR (STRPOS(',' || CAST(@types AS VARCHAR) || ',', ',table,') > 0 AND t.TABLE_TYPE = 'TABLE')` +
	` OR (STRPOS(',' || CAST(@types AS VARCHAR) || ',', ',system table,') > 0 AND t.TABLE_TYPE = 'SYSTEM_TABLE')` +
	` OR (STRPOS(',' || CAST(@types AS VARCHAR) || ',', ',view,') > 0 AND t.TABLE_TYPE = 'VIEW'))`

func registerRelations() {
	// SCHEMATA lists druid, where the datasources are, INFORMATION_SCHEMA,
	// lookup, sys and view. Its owner and character set columns are NULL.
	dbmeta.Schemas.Register(dbmeta.Druid, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.CATALOG_NAME AS "catalog"`),
			always(`, s.SCHEMA_NAME AS "name"`),
			always(`, s.SCHEMA_OWNER AS "owner"`),
			always(`, NULL AS "comment"`),
			always(`FROM INFORMATION_SCHEMA.SCHEMATA s`),
			always(`WHERE ` + notSystem("s.SCHEMA_NAME")),
			always(`AND ` + like("s.SCHEMA_NAME", "@name")),
			always(`ORDER BY 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: catalogDesc},
			{Name: "name"},
			{Name: "owner", Desc: "always empty: a schema has no owner, and SCHEMATA holds NULL"},
			{Name: "comment", Desc: "always absent: Druid has no COMMENT statement"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "schema name pattern, empty for every schema", Default: ""},
			system,
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, dbmeta.NullAsEmpty(&v.Owner), &v.Comment)
			return v, err
		},
	})

	// Druid resolves an unqualified name in the schema druid. No function or
	// setting returns it, and SCHEMATA names it.
	dbmeta.CurrentSchema.Register(dbmeta.Druid, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.CATALOG_NAME AS "catalog"`),
			always(`, s.SCHEMA_NAME AS "name"`),
			always(`, s.SCHEMA_OWNER AS "owner"`),
			always(`, NULL AS "comment"`),
			always(`FROM INFORMATION_SCHEMA.SCHEMATA s`),
			always(`WHERE s.SCHEMA_NAME = 'druid'`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: catalogDesc},
			{Name: "name", Desc: "always druid, the schema where an unqualified name resolves"},
			{Name: "owner", Desc: "always empty: a schema has no owner, and SCHEMATA holds NULL"},
			{Name: "comment", Desc: "always absent: Druid has no COMMENT statement"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, dbmeta.NullAsEmpty(&v.Owner), &v.Comment)
			return v, err
		},
	})

	// A datasource is a table. The tables of INFORMATION_SCHEMA and sys are
	// system tables, and a lookup is a table in the schema lookup.
	dbmeta.Tables.Register(dbmeta.Druid, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			always(`SELECT t.TABLE_CATALOG AS "catalog"`),
			always(`, t.TABLE_SCHEMA AS "schema"`),
			always(`, t.TABLE_NAME AS "name"`),
			always(`, ` + tableType + ` AS "type"`),
			always(`, NULL AS "comment"`),
			always(`, CASE WHEN t.IS_JOINABLE IS NULL THEN NULL ELSE 'joinable=' || t.IS_JOINABLE || ', broadcast=' || t.IS_BROADCAST END AS "options"`),
			always(`FROM INFORMATION_SCHEMA.TABLES t`),
			always(`WHERE ` + notSystem("t.TABLE_SCHEMA")),
			always(`AND ` + like("t.TABLE_SCHEMA", "@schema")),
			always(`AND ` + like("t.TABLE_NAME", "@name")),
			always(`AND ` + types),
			always(`ORDER BY 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: catalogDesc},
			{Name: "schema", Desc: "druid for a datasource"},
			{Name: "name"},
			{Name: "type", Desc: "table, system table or view"},
			{Name: "comment", Desc: "always absent: Druid has no COMMENT statement"},
			{Name: "options", Desc: "IS_JOINABLE and IS_BROADCAST of the row, such as joinable=NO, broadcast=NO"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "name", Desc: "table name pattern, empty for every table", Default: ""},
			system,
			dbmeta.TypesParam(),
		},
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment, &v.Options)
			return v, err
		},
	})

	// COLUMN_DEFAULT is the empty string where a column has no default, so
	// NULLIF turns it into the NULL that means nothing there. Druid has no
	// DEFAULT clause, so no column has one.
	dbmeta.Columns.Register(dbmeta.Druid, &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			always(`SELECT c.TABLE_CATALOG AS "catalog"`),
			always(`, c.TABLE_SCHEMA AS "schema"`),
			always(`, c.TABLE_NAME AS "table"`),
			always(`, c.COLUMN_NAME AS "name"`),
			always(`, c.ORDINAL_POSITION AS "ordinal"`),
			always(`, c.DATA_TYPE AS "data_type"`),
			always(`, c.IS_NULLABLE = 'YES' AS "nullable"`),
			always(`, NULLIF(c.COLUMN_DEFAULT, '') AS "default"`),
			always(`, false AS "primary_key"`),
			always(`, '' AS "identity"`),
			always(`, '' AS "generated"`),
			always(`, NULL AS "comment"`),
			always(`, c.COLLATION_NAME AS "collation"`),
			always(`FROM INFORMATION_SCHEMA.COLUMNS c`),
			always(`WHERE ` + notSystem("c.TABLE_SCHEMA")),
			always(`AND ` + like("c.TABLE_SCHEMA", "@schema")),
			always(`AND ` + like("c.TABLE_NAME", "@parent")),
			always(`AND ` + like("c.COLUMN_NAME", "@name")),
			always(`ORDER BY 2, 3, 5`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: catalogDesc}, {Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "ordinal", Desc: "the position from 1, with __time first"},
			{Name: "data_type", Desc: "the SQL type, such as BIGINT, VARCHAR or TIMESTAMP. A column of a complex type reads OTHER"},
			{Name: "nullable", Desc: "false for __time alone: every other column of a datasource can be NULL"},
			{Name: "default", Desc: "always absent: Druid has no DEFAULT clause, and COLUMN_DEFAULT is empty"},
			{Name: "primary_key", Desc: "always false: Druid has no primary key"},
			{Name: "identity", Desc: "always empty: Druid has no identity column"},
			{Name: "generated", Desc: "always empty: Druid has no generated column"},
			{Name: "comment", Desc: "always absent: Druid has no COMMENT statement"},
			{Name: "collation", Desc: "the collation name Druid SQL reports for a string column, which is the one of its Calcite planner, and absent for any other type"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
			{Name: "name", Desc: "column name pattern, empty for every column", Default: ""},
			system,
		},
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			var v dbmeta.Column
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Ordinal, &v.DataType,
				&v.Nullable, &v.Default, &v.PrimaryKey, &v.Identity, &v.Generated, &v.Comment, &v.Collation)
			return v, err
		},
	})
}
