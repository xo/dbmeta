package solr

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// catalogDesc describes the catalog field of every row.
const catalogDesc = "always solr, the one catalog the model reports. Solr names no cluster in SQL"

// schemaDesc describes the schema field of a table or a column.
const schemaDesc = "solr for a collection or an alias, and metadata for the two system tables. Solr itself names the schema of a collection with the address of ZooKeeper, and the model reports solr so that the name does not change with the machine"

// types is the filter on Table.Type. Calcite has no STRPOS here, so the list is
// searched with LIKE. The driver writes each parameter into the statement as a
// literal.
const types = `(@types = ''` +
	` OR ((',' || @types || ',') LIKE '%,table,%' AND t.tableType <> 'SYSTEM TABLE')` +
	` OR ((',' || @types || ',') LIKE '%,system table,%' AND t.tableType = 'SYSTEM TABLE'))`

// schemaFilter matches a pattern against the name that the model reports for a
// row of t. A collection is in solr, and a system table is in metadata.
func schemaFilter(param string) string {
	return `(` + param + ` = ''` +
		` OR (t.tableSchem <> 'metadata' AND 'solr' LIKE ` + param + `)` +
		` OR (t.tableSchem = 'metadata' AND 'metadata' LIKE ` + param + `))`
}

// notSystem hides the two system tables unless with_system is set.
const notSystem = `(@with_system OR t.tableSchem <> 'metadata')`

func registerRelations() {
	// The schema solr always exists. The row comes from the one system table
	// that is always there, so that a server with no collection still lists it,
	// and the schema metadata is added with with_system.
	dbmeta.Schemas.Register(dbmeta.Solr, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT ` + text("solr") + ` AS ` + al("catalog")),
			always(`, ` + text("solr") + ` AS ` + al("name")),
			always(`, ` + null + ` AS ` + al("owner")),
			always(`, ` + null + ` AS ` + al("comment")),
			always(`FROM metadata.TABLES s WHERE s.tableName = 'TABLES' AND ` + like("'solr'", "@name")),
			always(`UNION ALL`),
			always(`SELECT ` + text("solr") + ` AS ` + al("catalog")),
			always(`, ` + text("metadata") + ` AS ` + al("name")),
			always(`, ` + null + ` AS ` + al("owner")),
			always(`, ` + null + ` AS ` + al("comment")),
			always(`FROM metadata.TABLES s WHERE s.tableName = 'TABLES' AND @with_system AND ` + like("'metadata'", "@name")),
			always(`ORDER BY 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: catalogDesc},
			{Name: "name", Desc: "solr, and metadata with with_system"},
			{Name: "owner", Desc: "always empty: a schema has no owner"},
			{Name: "comment", Desc: "always absent: Solr has no COMMENT statement"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "schema name pattern, empty for every schema", Default: ""},
			system,
		},
		Scan: scanSchema,
	})

	// An unqualified name resolves to a collection, and the model reports that
	// schema as solr.
	dbmeta.CurrentSchema.Register(dbmeta.Solr, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT ` + text("solr") + ` AS ` + al("catalog")),
			always(`, ` + text("solr") + ` AS ` + al("name")),
			always(`, ` + null + ` AS ` + al("owner")),
			always(`, ` + null + ` AS ` + al("comment")),
			always(`FROM metadata.TABLES s WHERE s.tableName = 'TABLES'`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: catalogDesc},
			{Name: "name", Desc: "always solr, the schema where an unqualified name resolves"},
			{Name: "owner", Desc: "always empty: a schema has no owner"},
			{Name: "comment", Desc: "always absent: Solr has no COMMENT statement"},
		},
		Scan: scanSchema,
	})

	// metadata.TABLES lists a collection, an alias of a collection, and the two
	// system tables. SQL cannot tell an alias from a collection. The statement
	// reads every collection of the cluster, and a filter does not prune it.
	dbmeta.Tables.Register(dbmeta.Solr, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			always(`SELECT ` + text("solr") + ` AS ` + al("catalog")),
			always(`, ` + schemaOf("t") + ` AS ` + al("schema")),
			always(`, t.tableName AS ` + al("name")),
			always(`, CASE t.tableType WHEN 'SYSTEM TABLE' THEN ` + text("system table") + ` ELSE ` + text("table") + ` END AS ` + al("type")),
			always(`, t.remarks AS ` + al("comment")),
			always(`FROM metadata.TABLES t`),
			always(`WHERE ` + notSystem),
			always(`AND ` + schemaFilter("@schema")),
			always(`AND ` + like("t.tableName", "@name")),
			always(`AND ` + types),
			always(`ORDER BY 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: catalogDesc},
			{Name: "schema", Desc: schemaDesc},
			{Name: "name", Desc: "the collection or the alias"},
			{Name: "type", Desc: "table, or system table for the two tables of metadata. An alias is a table, because SQL does not tell it from a collection"},
			{Name: "comment", Desc: "always absent: remarks is NULL, and Solr has no comment"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "name", Desc: "table name pattern, empty for every table", Default: ""},
			system,
			dbmeta.TypesParam(),
		},
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})

	// metadata.COLUMNS lists the fields of every collection. Every column
	// reads nullable and none reads a key, because the SQL module reports
	// nothing else, whatever the schema says. The type is the JDBC type code,
	// which the statement turns into the name.
	dbmeta.Columns.Register(dbmeta.Solr, &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			always(`SELECT ` + text("solr") + ` AS ` + al("catalog")),
			always(`, ` + schemaOf("t") + ` AS ` + al("schema")),
			always(`, t.tableName AS ` + al("table")),
			always(`, t.columnName AS ` + al("name")),
			always(`, t.ordinalPosition AS ` + al("ordinal")),
			always(`, CASE t.dataType WHEN 12 THEN ` + text("VARCHAR") + ` WHEN -5 THEN ` + text("BIGINT") +
				` WHEN 8 THEN ` + text("DOUBLE") + ` WHEN 93 THEN ` + text("TIMESTAMP") + ` WHEN 2000 THEN ` + text("ANY") +
				` ELSE CAST(t.typeName AS VARCHAR) END AS ` + al("data_type")),
			always(`, t.isNullable = 'YES' AS ` + al("nullable")),
			always(`, t.columnDef AS ` + al("default")),
			always(`, false AS ` + al("primary_key")),
			always(`, t.isAutoincrement AS ` + al("identity")),
			always(`, t.isGeneratedcolumn AS ` + al("generated")),
			always(`, t.remarks AS ` + al("comment")),
			always(`, ` + null + ` AS ` + al("collation")),
			always(`FROM metadata.COLUMNS t`),
			always(`WHERE ` + notSystem),
			always(`AND ` + schemaFilter("@schema")),
			always(`AND ` + like("t.tableName", "@parent")),
			always(`AND ` + like("t.columnName", "@name")),
			always(`ORDER BY 2, 3, 5`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: catalogDesc},
			{Name: "schema", Desc: schemaDesc},
			{Name: "table"},
			{Name: "name", Desc: "the field. Solr adds _nest_path_, _root_, _text_, _version_, _query_ and score to the fields that the schema holds"},
			{Name: "ordinal", Desc: "the position from 1, in the order the SQL module lists the fields"},
			{Name: "data_type", Desc: "VARCHAR, BIGINT, DOUBLE, TIMESTAMP or ANY, from the JDBC type of the SQL module. A boolean field reads VARCHAR, and a multi-valued field reads ANY"},
			{Name: "nullable", Desc: "always true: the SQL module reports every field nullable, the unique key included"},
			{Name: "default", Desc: "always absent: Solr has no DEFAULT clause in SQL, and columnDef is NULL"},
			{Name: "primary_key", Desc: "always false: SQL does not report the unique key of the schema"},
			{Name: "identity", Desc: "always empty: Solr has no identity column"},
			{Name: "generated", Desc: "always empty: Solr has no generated column"},
			{Name: "comment", Desc: "always absent: remarks is NULL, and Solr has no comment"},
			{Name: "collation", Desc: "always absent: Solr has no collation in SQL"},
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

// scanSchema reads a schema row.
func scanSchema(rows *sql.Rows) (dbmeta.Schema, error) {
	var v dbmeta.Schema
	err := rows.Scan(&v.Catalog, &v.Name, dbmeta.NullAsEmpty(&v.Owner), &v.Comment)
	return v, err
}
