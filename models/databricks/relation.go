package databricks

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// Databases, schemas, tables, columns and views.

// tableType is the word for the kind of the relation t. Databricks says
// MANAGED, EXTERNAL, VIEW, MATERIALIZED_VIEW, STREAMING_TABLE, FOREIGN and a
// clone of a managed or external table. A clone is a table.
const tableType = "CASE t.table_type WHEN 'MANAGED' THEN 'table' WHEN 'MANAGED_SHALLOW_CLONE' THEN 'table'" +
	" WHEN 'EXTERNAL' THEN 'external table' WHEN 'EXTERNAL_SHALLOW_CLONE' THEN 'external table'" +
	" WHEN 'FOREIGN' THEN 'foreign table'" +
	" ELSE LOWER(REPLACE(t.table_type, '_', ' ')) END"

// tableJoin reads the tags, the row filter and the column masks of the catalog
// once each, and joins them to the table t. A subquery that names t runs for
// each table, and a catalog of thousands of tables then costs the square of
// that. The tags aggregate as g.tags, and the row filter is f.filter_name.
func tableJoin() string {
	return "LEFT JOIN (SELECT catalog_name, schema_name, table_name" +
		", concat_ws(', ', sort_array(collect_list('tag.' || tag_name || '=' || tag_value))) AS tags" +
		" FROM information_schema.table_tags GROUP BY catalog_name, schema_name, table_name) g" +
		" ON g.catalog_name = t.table_catalog AND g.schema_name = t.table_schema AND g.table_name = t.table_name" +
		" LEFT JOIN information_schema.row_filters f" +
		" ON f.table_catalog = t.table_catalog AND f.table_schema = t.table_schema AND f.table_name = t.table_name"
}

func registerRelations() {
	// \l. The catalogs that the principal can see. A catalog of the INFORMATION_SCHEMA
	// of the session lists only the catalog that the session is in, so the
	// statement reads the one of the system catalog, which lists them all.
	dbmeta.Databases.Register(dbmeta.Databricks, &dbmeta.Binding[dbmeta.Database]{
		Stmt: dbmeta.Stmt{
			always("SELECT c.catalog_name AS `name`"),
			always(", c.catalog_owner AS `owner`"),
			always(", '' AS `encoding`"),
			always(", '' AS `collate`"),
			always(", '' AS `ctype`"),
			always(", CAST(NULL AS STRING) AS `access`"),
			always(", CAST(NULL AS STRING) AS `tablespace`"),
			always(", CAST(NULL AS STRING) AS `size`"),
			always(", c.comment AS `comment`"),
			always("FROM system.information_schema.catalogs c"),
			always("WHERE " + like("c.catalog_name", "@name")),
			always("ORDER BY 1"),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the catalog. The system catalog lists every catalog that the principal can see, including system and samples"},
			{Name: "owner", Desc: "CATALOG_OWNER, which is a group, a user or the text System user"},
			{Name: "encoding", Desc: "always empty: Databricks reports no encoding"},
			{Name: "collate", Desc: "always empty: CATALOG_PROPERTIES holds the default collation, and it is not supported by INFORMATION_SCHEMA"},
			{Name: "ctype", Desc: "always empty, for the same reason"},
			{Name: "access", Desc: "always absent: CATALOG_PRIVILEGES lists the grants, and the model reads them for tables only"},
			{Name: "tablespace", Desc: "always absent: Databricks has no tablespace"},
			{Name: "size", Desc: "always absent: Databricks reports no size for a catalog"},
			{Name: "comment", Desc: "the comment of the catalog. Absent when none is set"},
		},
		Params: nameOnly("catalog"),
		Scan: func(rows *sql.Rows) (dbmeta.Database, error) {
			var v dbmeta.Database
			err := rows.Scan(&v.Name, dbmeta.NullAsEmpty(&v.Owner), &v.Encoding, &v.Collate, &v.CType,
				&v.Access, &v.Tablespace, &v.Size, &v.Comment)
			return v, err
		},
	})

	// \dn.
	dbmeta.Schemas.Register(dbmeta.Databricks, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always("SELECT s.catalog_name AS `catalog`"),
			always(", s.schema_name AS `name`"),
			always(", s.schema_owner AS `owner`"),
			always(", s.comment AS `comment`"),
			always(", a.acl AS `access`"),
			always("FROM information_schema.schemata s"),
			always("LEFT JOIN " + grants("schema_privileges", "catalog_name", "schema_name") + " a"),
			always("ON a.catalog_name = s.catalog_name AND a.schema_name = s.schema_name"),
			always("WHERE " + notSystem("s.schema_name")),
			always("AND " + like("s.schema_name", "@name")),
			always("ORDER BY 1, 2"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"},
			{Name: "name"},
			{Name: "owner", Desc: "SCHEMA_OWNER, which is a group, a user, an application id for a service principal, or the text System user"},
			{Name: "comment", Desc: "the comment of the schema. Absent when none is set"},
			{Name: "access", Desc: "the grants on the schema, a line for each, as grantee=PRIVILEGE/grantor, with the privilege USE_SCHEMA written with an underscore. A grant that the schema inherits from the catalog says so. Absent when the schema has no grant"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "with_system", Desc: systemDesc, Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, dbmeta.NullAsEmpty(&v.Owner), &v.Comment, &v.Access)
			return v, err
		},
	})

	// \dt, \dv and \dm. TABLES holds the comment and the owner and says no more.
	// The size, the row count and the clustering columns are in DESCRIBE DETAIL.
	dbmeta.Tables.Register(dbmeta.Databricks, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			always("SELECT t.table_catalog AS `catalog`"),
			always(", t.table_schema AS `schema`"),
			always(", t.table_name AS `name`"),
			always(", " + tableType + " AS `type`"),
			always(", t.comment AS `comment`"),
			always(", t.table_owner AS `owner`"),
			always(", IF(t.table_type = 'VIEW', NULL, 'permanent') AS `persistence`"),
			always(", IF(t.data_source_format = 'UNKNOWN_DATA_SOURCE_FORMAT', NULL, LOWER(t.data_source_format)) AS `access_method`"),
			always(", CAST(NULL AS BIGINT) AS `size`"),
			always(", CAST(NULL AS BIGINT) AS `rows`"),
			always(", NULLIF(concat_ws(', ', IF(t.storage_path = '', NULL, 'path=' || t.storage_path), g.tags), '') AS `options`"),
			always(", f.filter_name IS NOT NULL AS `row_security`"),
			always(", CAST(NULL AS BOOLEAN) AS `row_security_forced`"),
			always("FROM information_schema.tables t"),
			always(tableJoin()),
			always("WHERE " + notSystem("t.table_schema")),
			always("AND " + like("t.table_schema", "@schema")),
			always("AND " + like("t.table_name", "@name")),
			always("AND (@types = '' OR " + dbmeta.InList("@types", tableType) + ")"),
			always("ORDER BY 2, 3"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"},
			{Name: "schema"},
			{Name: "name"},
			{Name: "type", Desc: "table, external table, foreign table, view, materialized view or streaming table. A managed table and a clone of one are both table, and an external table and a clone of one are both external table. A materialized view makes a hidden table and an event log table in the same schema, and both are listed as tables"},
			{Name: "comment", Desc: "the comment of the table. Absent when none is set"},
			{Name: "owner", Desc: "TABLE_OWNER, which is a user, a group or an application id"},
			{Name: "persistence", Desc: "permanent for every relation that stores data, which is every one but a view. Databricks lists no temporary table here"},
			{Name: "access_method", Desc: "DATA_SOURCE_FORMAT in lower case, such as delta. Absent for a view and a materialized view, which report no format"},
			{Name: "size", Desc: "always absent: only DESCRIBE DETAIL reports the size, and it is a statement for one table"},
			{Name: "rows", Desc: "always absent, for the same reason"},
			{Name: "options", Desc: "path=STORAGE_PATH for a table that has one, and tag.name=value for each tag of the table, joined by a comma and a space. Absent when there is none"},
			{Name: "row_security", Desc: "whether a row filter is set on the table"},
			{Name: "row_security_forced", Desc: "always absent: a row filter always applies"},
		},
		Params: append(schemaName("table"), dbmeta.TypesParam()),
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment, &v.Owner,
				&v.Persistence, &v.AccessMethod, &v.Size, &v.Rows, &v.Options,
				&v.RowSecurity, &v.RowSecurityForced)
			return v, err
		},
	})

	// \d name. The ordinal of COLUMNS starts at zero, and the model adds one.
	dbmeta.Columns.Register(dbmeta.Databricks, &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			always("SELECT c.table_catalog AS `catalog`"),
			always(", c.table_schema AS `schema`"),
			always(", c.table_name AS `table`"),
			always(", c.column_name AS `name`"),
			always(", c.ordinal_position + 1 AS `ordinal`"),
			always(", c.full_data_type AS `data_type`"),
			always(", c.is_nullable = 'YES' AS `nullable`"),
			always(", c.column_default AS `default`"),
			always(", k.column_name IS NOT NULL AS `primary_key`"),
			always(", IF(c.is_identity = 'YES', LOWER(c.identity_generation), NULL) AS `identity`"),
			always(", IF(c.is_generated IN ('ALWAYS', 'YES'), 'stored', NULL) AS `generated`"),
			always(", c.comment AS `comment`"),
			always(", CAST(NULL AS STRING) AS `collation`"),
			always(", CAST(NULL AS STRING) AS `storage`"),
			always(", CAST(NULL AS STRING) AS `compression`"),
			always(", CAST(NULL AS BIGINT) AS `stats_target`"),
			always("FROM information_schema.columns c"),
			always("LEFT JOIN (SELECT u.table_catalog, u.table_schema, u.table_name, u.column_name"),
			always("FROM information_schema.key_column_usage u JOIN information_schema.table_constraints s"),
			always("ON s.constraint_catalog = u.constraint_catalog AND s.constraint_schema = u.constraint_schema"),
			always("AND s.constraint_name = u.constraint_name WHERE s.constraint_type = 'PRIMARY KEY') k"),
			always("ON k.table_catalog = c.table_catalog AND k.table_schema = c.table_schema"),
			always("AND k.table_name = c.table_name AND k.column_name = c.column_name"),
			always("WHERE " + notSystem("c.table_schema")),
			always("AND " + like("c.table_schema", "@schema")),
			always("AND " + like("c.table_name", "@parent")),
			always("AND " + like("c.column_name", "@name")),
			always("ORDER BY 2, 3, 5"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "ordinal", Desc: "ORDINAL_POSITION plus one, because COLUMNS counts from zero. The partition columns are in the order that the table was made with, and not last"},
			{Name: "data_type", Desc: "FULL_DATA_TYPE, which spells the type as a statement does, such as bigint, decimal(10,2) or array<string>. DATA_TYPE is the short name, such as LONG, and is not read"},
			{Name: "nullable", Desc: "IS_NULLABLE. A NOT NULL column is not nullable"},
			{Name: "default", Desc: "COLUMN_DEFAULT. It is absent for every column that was seen, including a column made with DEFAULT, because INFORMATION_SCHEMA does not report a default of a Delta table. Only SHOW CREATE TABLE does"},
			{Name: "primary_key", Desc: "whether the column is part of the primary key, which Databricks does not enforce"},
			{Name: "identity", Desc: "always or by default for an identity column. It is absent for every column that was seen, including a column made with GENERATED ALWAYS AS IDENTITY, because IS_IDENTITY says NO for it"},
			{Name: "generated", Desc: "stored for a generated column. It is absent for every column that was seen, including one made with GENERATED ALWAYS AS, because IS_GENERATED says NO for it"},
			{Name: "comment", Desc: "the comment of the column. Absent when none is set"},
			{Name: "collation", Desc: "always absent: COLUMNS has no collation column on the release that was measured"},
			{Name: "storage", Desc: "always absent: Databricks has no storage choice for a column"},
			{Name: "compression", Desc: "always absent: Databricks has no compression choice for a column"},
			{Name: "stats_target", Desc: "always absent: Databricks has no statistics target"},
		},
		Params: schemaParentName("table", "column"),
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			var v dbmeta.Column
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Ordinal,
				&v.DataType, &v.Nullable, &v.Default, &v.PrimaryKey, &v.Identity,
				&v.Generated, &v.Comment, &v.Collation, &v.Storage, &v.Compression,
				&v.StatsTarget)
			return v, err
		},
	})

	// \dv and \sv. A materialized view is a view here as well, and VIEWS says so.
	dbmeta.Views.Register(dbmeta.Databricks, &dbmeta.Binding[dbmeta.View]{
		Stmt: dbmeta.Stmt{
			always("SELECT v.table_catalog AS `catalog`"),
			always(", v.table_schema AS `schema`"),
			always(", v.table_name AS `name`"),
			always(", v.view_definition AS `definition`"),
			always(", IF(v.check_option = 'NONE', NULL, LOWER(v.check_option)) AS `check_option`"),
			always(", v.is_updatable = 'YES' AS `updatable`"),
			always(", v.is_insertable_into = 'YES' AS `insertable`"),
			always(", t.comment AS `comment`"),
			always("FROM information_schema.views v"),
			always("JOIN information_schema.tables t ON t.table_catalog = v.table_catalog"),
			always("AND t.table_schema = v.table_schema AND t.table_name = v.table_name"),
			always("WHERE " + notSystem("v.table_schema")),
			always("AND " + like("v.table_schema", "@schema")),
			always("AND " + like("v.table_name", "@name")),
			always("ORDER BY 2, 3"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{Name: "definition", Desc: "VIEW_DEFINITION, the query as it was written"},
			{Name: "check_option", Desc: "CHECK_OPTION in lower case. Databricks says NONE for every view that was seen, and the model reports that as absent"},
			{Name: "updatable", Desc: "IS_UPDATABLE, which is false for every view that was seen"},
			{Name: "insertable", Desc: "IS_INSERTABLE_INTO, which is false for every view that was seen"},
			{Name: "comment", Desc: "the comment of the view, from TABLES. Absent when none is set"},
		},
		Params: schemaName("view"),
		Scan: func(rows *sql.Rows) (dbmeta.View, error) {
			var v dbmeta.View
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Definition, &v.CheckOption,
				&v.Updatable, &v.Insertable, &v.Comment)
			return v, err
		},
	})
}
