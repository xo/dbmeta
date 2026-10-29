package maxcompute

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// Databases, schemas, tables, columns and views.

// tableType is the word for the kind of the relation t. TABLES says
// MANAGED_TABLE, EXTERNAL_TABLE, VIRTUAL_VIEW or MATERIALIZED_VIEW.
const tableType = "CASE t.table_type WHEN 'MANAGED_TABLE' THEN 'table' WHEN 'EXTERNAL_TABLE' THEN 'external table'" +
	" WHEN 'VIRTUAL_VIEW' THEN 'view' WHEN 'MATERIALIZED_VIEW' THEN 'materialized view'" +
	" ELSE LOWER(REPLACE(t.table_type, '_', ' ')) END"

// isView is true for a view and a materialized view.
const isView = "t.table_type IN ('VIRTUAL_VIEW', 'MATERIALIZED_VIEW')"

// catalog is the value that every view of INFORMATION_SCHEMA reports in the
// column for the catalog, for a table, a role, a function and a grant alike.
const catalog = "odps"

// project is a relation of one row that holds the name of the project of the
// session. MaxCompute has two levels, a project and its tables, so TABLES puts
// the project in the column for the schema.
const project = "(SELECT current_project() AS name) p"

func registerRelations() {
	// \l. Every view reports the catalog odps. The text is a constant of the
	// model, because no relation lists it and an empty project has no row to read
	// it from.
	dbmeta.Databases.Register(dbmeta.MaxCompute, &dbmeta.Binding[dbmeta.Database]{
		Stmt: dbmeta.Stmt{
			always("SELECT '" + catalog + "' AS `name`"),
			always(", '' AS `owner`"),
			always(", '' AS `encoding`"),
			always(", '' AS `collate`"),
			always(", '' AS `ctype`"),
			always(", CAST(NULL AS STRING) AS `access`"),
			always(", CAST(NULL AS STRING) AS `tablespace`"),
			always(", CAST(NULL AS STRING) AS `size`"),
			always(", CAST(NULL AS STRING) AS `comment`"),
			always("FROM (SELECT 1 AS one) d"),
			always("WHERE " + like("'"+catalog+"'", "@name")),
			last("ORDER BY `name`"),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "always odps, which every view reports in the column for the catalog. The projects are the schemas"},
			{Name: "owner", Desc: "always empty: no view reports the owner of a project"},
			{Name: "encoding", Desc: "always empty: MaxCompute reports no encoding"},
			{Name: "collate", Desc: "always empty: a project has no collation"},
			{Name: "ctype", Desc: "always empty, for the same reason"},
			{Name: "access", Desc: "always absent"},
			{Name: "tablespace", Desc: "always absent: MaxCompute has no tablespace"},
			{Name: "size", Desc: "always absent: no view reports the size of a project"},
			{Name: "comment", Desc: "always absent: no view reports the comment of a project"},
		},
		Params: nameOnly("catalog"),
		Scan: func(rows *sql.Rows) (dbmeta.Database, error) {
			var v dbmeta.Database
			err := rows.Scan(&v.Name, &v.Owner, &v.Encoding, &v.Collate, &v.CType,
				&v.Access, &v.Tablespace, &v.Size, &v.Comment)
			return v, err
		},
	})

	// \dn. A project that is not schema enabled has no schema inside it, and TABLES
	// names the project in the column for the schema. The project that the
	// connection names is the one schema.
	dbmeta.Schemas.Register(dbmeta.MaxCompute, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always("SELECT '" + catalog + "' AS `catalog`"),
			always(", p.name AS `name`"),
			always(", '' AS `owner`"),
			always(", CAST(NULL AS STRING) AS `comment`"),
			always("FROM " + project),
			always("WHERE " + like("p.name", "@schema")),
			last("ORDER BY `name`"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always odps"},
			{Name: "name", Desc: "the project that the connection names, from current_project(). MaxCompute lists no other project, and a project that is not schema enabled has two levels, so TABLES puts the project in the column for the schema"},
			{Name: "owner", Desc: "always empty: no view reports the owner of a project"},
			{Name: "comment", Desc: "always absent"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "project name pattern, empty for every project", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})

	// \dt, \dv and \dm. TABLES holds the owner, the comment, the size in bytes, the
	// life cycle and the clustering, and no count of rows.
	dbmeta.Tables.Register(dbmeta.MaxCompute, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			always("SELECT t.table_catalog AS `catalog`"),
			always(", t.table_schema AS `schema`"),
			always(", t.table_name AS `name`"),
			always(", " + tableType + " AS `type`"),
			always(", t.table_comment AS `comment`"),
			always(", t.owner_name AS `owner`"),
			always(", IF(" + isView + ", NULL, 'permanent') AS `persistence`"),
			always(", CAST(NULL AS STRING) AS `access_method`"),
			always(", IF(" + isView + " OR t.data_length < 0, NULL, t.data_length) AS `size`"),
			always(", CAST(NULL AS BIGINT) AS `rows`"),
			always(", NULLIF(SUBSTR(CONCAT(IF(t.life_cycle > 0, CONCAT(', lifecycle=', t.life_cycle), '')"),
			always(", IF(t.cluster_type IS NULL, '', CONCAT(', cluster_type=', t.cluster_type))"),
			always(", IF(t.number_buckets > 0, CONCAT(', number_buckets=', t.number_buckets), '')), 3), '') AS `options`"),
			always(", CAST(NULL AS BOOLEAN) AS `row_security`"),
			always(", CAST(NULL AS BOOLEAN) AS `row_security_forced`"),
			always("FROM information_schema.tables t"),
			always("WHERE " + like("t.table_schema", "@schema")),
			always("AND " + like("t.table_name", "@name")),
			always("AND (@types = '' OR " + dbmeta.InList("@types", tableType) + ")"),
			last("ORDER BY t.table_schema, t.table_name"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always odps"},
			{Name: "schema", Desc: "the project, which TABLES reports in the column for the schema"},
			{Name: "name"},
			{Name: "type", Desc: "table, external table, view or materialized view, from TABLE_TYPE"},
			{Name: "comment", Desc: "TABLE_COMMENT. Absent when none is set"},
			{Name: "owner", Desc: "OWNER_NAME"},
			{Name: "persistence", Desc: "permanent for a relation that stores data, and absent for a view. MaxCompute has no temporary table"},
			{Name: "access_method", Desc: "always absent: MaxCompute has no table access method"},
			{Name: "size", Desc: "DATA_LENGTH, the bytes of the table. Absent for a view, and for a partitioned table, where DATA_LENGTH is NULL and Partitions holds the size of each partition"},
			{Name: "rows", Desc: "always absent: no view reports a count of rows"},
			{Name: "options", Desc: "lifecycle=LIFE_CYCLE in days, cluster_type=CLUSTER_TYPE and number_buckets=NUMBER_BUCKETS, joined by a comma and a space, for each that the table sets. A transactional table is clustered by hash into 16 buckets. Absent when none is set"},
			{Name: "row_security", Desc: "always absent: MaxCompute has no row level security"},
			{Name: "row_security_forced", Desc: "always absent, for the same reason"},
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

	// \d name. COLUMNS numbers the partition columns from one again, after the
	// other columns, so the model puts the partition columns last and counts on.
	// The window reads the columns of each table whole, and the filters on the
	// column name apply after it, so a filter does not change a number.
	dbmeta.Columns.Register(dbmeta.MaxCompute, &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			always("SELECT c.table_catalog AS `catalog`"),
			always(", c.table_schema AS `schema`"),
			always(", c.table_name AS `table`"),
			always(", c.column_name AS `name`"),
			always(", c.pos AS `ordinal`"),
			always(", c.data_type AS `data_type`"),
			always(", c.is_nullable AS `nullable`"),
			always(", c.column_default AS `default`"),
			always(", FALSE AS `primary_key`"),
			always(", CAST(NULL AS STRING) AS `identity`"),
			always(", CAST(NULL AS STRING) AS `generated`"),
			always(", c.column_comment AS `comment`"),
			always(", CAST(NULL AS STRING) AS `collation`"),
			always("FROM (SELECT x.table_catalog, x.table_schema, x.table_name, x.column_name, x.data_type, x.is_nullable"),
			always(", x.column_default, x.column_comment, row_number() OVER (PARTITION BY x.table_schema, x.table_name"),
			always("ORDER BY x.is_partition_key, x.ordinal_position) AS pos"),
			always("FROM information_schema.columns x"),
			always("WHERE " + like("x.table_schema", "@schema")),
			always("AND " + like("x.table_name", "@parent") + ") c"),
			always("WHERE " + like("c.column_name", "@name")),
			last("ORDER BY c.table_schema, c.table_name, c.pos"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always odps"}, {Name: "schema", Desc: "the project"}, {Name: "table"}, {Name: "name"},
			{Name: "ordinal", Desc: "the place of the column in the table, from one, with the partition columns after the others, as DESC shows them. COLUMNS counts the partition columns from one again, and the model counts on from the last other column"},
			{Name: "data_type", Desc: "DATA_TYPE as MaxCompute writes it, in lower case, such as bigint, string or decimal(10,2)"},
			{Name: "nullable", Desc: "IS_NULLABLE. A column made with NOT NULL is not nullable, and so is a column of the primary key"},
			{Name: "default", Desc: "COLUMN_DEFAULT, the value as text without its quotes. Absent when none is set"},
			{Name: "primary_key", Desc: "always false: COLUMNS has no primary key column, and only a transactional table has a key"},
			{Name: "identity", Desc: "always absent: MaxCompute has no identity column"},
			{Name: "generated", Desc: "always absent: MaxCompute has no generated column"},
			{Name: "comment", Desc: "COLUMN_COMMENT. It is the empty text for a column that has no comment, because COLUMNS holds the empty text there"},
			{Name: "collation", Desc: "always absent: MaxCompute has no collation"},
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

	// \dv and \sv. TABLES holds the text of a view in VIEW_ORIGINAL_TEXT.
	dbmeta.Views.Register(dbmeta.MaxCompute, &dbmeta.Binding[dbmeta.View]{
		Stmt: dbmeta.Stmt{
			always("SELECT t.table_catalog AS `catalog`"),
			always(", t.table_schema AS `schema`"),
			always(", t.table_name AS `name`"),
			always(", t.view_original_text AS `definition`"),
			always(", CAST(NULL AS STRING) AS `check_option`"),
			always(", FALSE AS `updatable`"),
			always(", FALSE AS `insertable`"),
			always(", t.table_comment AS `comment`"),
			always("FROM information_schema.tables t"),
			always("WHERE " + isView),
			always("AND " + like("t.table_schema", "@schema")),
			always("AND " + like("t.table_name", "@name")),
			last("ORDER BY t.table_schema, t.table_name"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always odps"}, {Name: "schema", Desc: "the project"}, {Name: "name"},
			{Name: "definition", Desc: "VIEW_ORIGINAL_TEXT, the SELECT of the view as it was written. A materialized view is a view here as well"},
			{Name: "check_option", Desc: "always absent: MaxCompute has no WITH CHECK OPTION"},
			{Name: "updatable", Desc: "always false: no MaxCompute view can be updated"},
			{Name: "insertable", Desc: "always false: no MaxCompute view accepts an insert"},
			{Name: "comment", Desc: "TABLE_COMMENT of the view. Absent when none is set"},
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
