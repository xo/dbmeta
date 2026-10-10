package bigquery

import (
	"database/sql"
	"fmt"

	"github.com/xo/dbmeta"
)

// Tables, columns, indexes and views.

// tableType is the word for the kind of the relation t. BigQuery says BASE
// TABLE, VIEW, MATERIALIZED VIEW, EXTERNAL, CLONE and SNAPSHOT.
const tableType = `CASE t.table_type WHEN 'BASE TABLE' THEN 'table'` +
	` WHEN 'EXTERNAL' THEN 'external table' ELSE LOWER(t.table_type) END`

// stored is true for a relation that holds data of its own. A view holds none,
// and an external table keeps its data outside BigQuery.
const stored = `t.table_type IN ('BASE TABLE', 'CLONE', 'SNAPSHOT', 'MATERIALIZED VIEW')`

// tableJoin reads the options, the clustering columns and the legacy size table
// of the dataset once each, and joins them to the table t. A subquery that names
// t runs for each table, and a dataset of thousands of tables then costs the
// square of that.
//
// `__TABLES__` is the one place that holds the size and the row count of every
// table in one read. TABLE_STORAGE has them too, but it is a view of the region
// and needs a permission on the project. The options aggregate as o.options and
// the description as o.description.
func tableJoin() string {
	return `LEFT JOIN (SELECT table_catalog, table_schema, table_name` +
		`, MAX(IF(option_name = 'description', ` + unquote("option_value") + `, NULL)) AS description` +
		`, STRING_AGG(IF(option_name = 'description', NULL, option_name || '=' || option_value)` +
		`, ', ' ORDER BY option_name) AS options` +
		` FROM INFORMATION_SCHEMA.TABLE_OPTIONS GROUP BY table_catalog, table_schema, table_name) o` +
		` ON o.table_catalog = t.table_catalog AND o.table_schema = t.table_schema AND o.table_name = t.table_name` +
		` LEFT JOIN (SELECT table_catalog, table_schema, table_name` +
		`, STRING_AGG(column_name, ', ' ORDER BY clustering_ordinal_position) AS columns` +
		` FROM INFORMATION_SCHEMA.COLUMNS WHERE clustering_ordinal_position IS NOT NULL` +
		` GROUP BY table_catalog, table_schema, table_name) cl` +
		` ON cl.table_catalog = t.table_catalog AND cl.table_schema = t.table_schema AND cl.table_name = t.table_name` +
		` LEFT JOIN __TABLES__ m ON m.project_id = t.table_catalog AND m.dataset_id = t.table_schema` +
		` AND m.table_id = t.table_name`
}

// tableOptions is Table.Options. Each part is name=value. The first is the
// table that a clone or a snapshot copies, the next is the columns the table is
// clustered by, and the last is the options of the table with the description
// left out, which tableJoin aggregates as o.options.
const tableOptions = `NULLIF(ARRAY_TO_STRING(` +
	`[IF(t.base_table_name IS NULL, NULL, 'base_table=' || t.base_table_schema || '.' || t.base_table_name)` +
	`, 'cluster_by=' || cl.columns, o.options], ', '), '')`

// indexOptionsJoin reads the options of every search index or every vector
// index once, as tableJoin does. The verb is the name of the options view.
const indexOptionsJoin = `LEFT JOIN (SELECT index_catalog, index_schema, table_name, index_name` +
	`, STRING_AGG(option_name || '=' || option_value, ', ' ORDER BY option_name) AS options` +
	` FROM INFORMATION_SCHEMA.%s GROUP BY index_catalog, index_schema, table_name, index_name) o` +
	` ON o.index_catalog = i.index_catalog AND o.index_schema = i.index_schema` +
	` AND o.table_name = i.table_name AND o.index_name = i.index_name`

// indexes is a relation of every search index and every vector index, with the
// columns of the index as the statement needs them. kind is the type word, and
// vector indexes have an index_type option that is the access method.
func indexes() string {
	return `(SELECT i.index_catalog AS catalog, i.index_schema AS schema, i.table_name AS table_name` +
		`, i.index_name AS index_name, 'search' AS type, i.index_status AS status, i.ddl AS ddl` +
		`, i.total_storage_bytes AS size, o.options AS options, CAST(NULL AS STRING) AS using_name` +
		` FROM INFORMATION_SCHEMA.SEARCH_INDEXES i ` + fmt.Sprintf(indexOptionsJoin, "SEARCH_INDEX_OPTIONS") +
		` UNION ALL SELECT i.index_catalog, i.index_schema, i.table_name, i.index_name, 'vector', i.index_status, i.ddl` +
		`, i.total_storage_bytes, o.options, v.option_value` +
		` FROM INFORMATION_SCHEMA.VECTOR_INDEXES i ` + fmt.Sprintf(indexOptionsJoin, "VECTOR_INDEX_OPTIONS") +
		` LEFT JOIN INFORMATION_SCHEMA.VECTOR_INDEX_OPTIONS v ON v.index_catalog = i.index_catalog` +
		` AND v.index_schema = i.index_schema AND v.table_name = i.table_name AND v.index_name = i.index_name` +
		` AND v.option_name = 'index_type')`
}

func registerRelations() {
	// \d, \dt, \dv, \dm. A clone and a snapshot keep the word BigQuery gives
	// them, because a caller that treats them as tables is right about reading
	// them, and a snapshot cannot be written to.
	dbmeta.Tables.Register(dbmeta.BigQuery, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			always("SELECT t.table_catalog AS `catalog`"),
			always(", t.table_schema AS `schema`"),
			always(", t.table_name AS `name`"),
			always(", " + tableType + " AS `type`"),
			always(", o.description AS `comment`"),
			always(", CAST(NULL AS STRING) AS `owner`"),
			always(", IF(" + stored + ", 'permanent', NULL) AS `persistence`"),
			always(", CAST(NULL AS STRING) AS `access_method`"),
			always(", IF(" + stored + ", m.size_bytes, NULL) AS `size`"),
			always(", IF(" + stored + ", m.row_count, NULL) AS `rows`"),
			always(", " + tableOptions + " AS `options`"),
			always(", CAST(NULL AS BOOL) AS `row_security`"),
			always(", CAST(NULL AS BOOL) AS `row_security_forced`"),
			always("FROM INFORMATION_SCHEMA.TABLES t"),
			always(tableJoin()),
			always("WHERE " + like("t.table_schema", "@schema")),
			always("AND " + like("t.table_name", "@name")),
			always("AND (@types = '' OR " + dbmeta.InList("@types", tableType) + ")"),
			always("ORDER BY 2, 3"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "the project"},
			{Name: "schema", Desc: "the dataset"},
			{Name: "name"},
			{Name: "type", Desc: "table, view, materialized view, external table, clone or snapshot. BigQuery says EXTERNAL for a table whose data is outside BigQuery"},
			{Name: "comment", Desc: "the description option of the table, as plain text. Absent when none is set"},
			{Name: "owner", Desc: "always absent: a BigQuery table has no owner, and access is by IAM"},
			{Name: "persistence", Desc: "permanent for a table, a clone, a snapshot and a materialized view, and absent for a view and an external table. BigQuery has no temporary table in INFORMATION_SCHEMA"},
			{Name: "access_method", Desc: "always absent: BigQuery has no access method"},
			{Name: "size", Desc: "SIZE_BYTES of the legacy __TABLES__ table, which is the logical bytes. Absent for a view and an external table. TABLE_STORAGE needs a permission on the project and is not read"},
			{Name: "rows", Desc: "ROW_COUNT of the legacy __TABLES__ table, which is exact and not an estimate. Absent for a view and an external table"},
			{
				Name: "options",
				Desc: "name=value joined by a comma and a space. base_table for a clone or a snapshot, cluster_by for the clustering columns, and the options of TABLE_OPTIONS with the description left out, such as expiration_timestamp. The value is the text that BigQuery writes, quotes included. Absent when none applies",
			},
			{Name: "row_security", Desc: "always absent: a row access policy has no view that INFORMATION_SCHEMA offers a dataset principal"},
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

	// \d name. The description of a column is in COLUMN_FIELD_PATHS, on the row
	// whose field path is the column itself.
	dbmeta.Columns.Register(dbmeta.BigQuery, &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			always("SELECT c.table_catalog AS `catalog`"),
			always(", c.table_schema AS `schema`"),
			always(", c.table_name AS `table`"),
			always(", c.column_name AS `name`"),
			always(", c.ordinal_position AS `ordinal`"),
			always(", c.data_type AS `data_type`"),
			always(", c.is_nullable = 'YES' AS `nullable`"),
			always(", " + nullText("c.column_default") + " AS `default`"),
			always(", k.column_name IS NOT NULL AS `primary_key`"),
			always(", IF(c.is_identity = 'YES', LOWER(c.identity_generation), NULL) AS `identity`"),
			always(", IF(c.is_generated = 'ALWAYS', IF(c.is_stored = 'YES', 'stored', 'virtual'), NULL) AS `generated`"),
			always(", f.description AS `comment`"),
			always(", " + nullText("c.collation_name") + " AS `collation`"),
			always(", CAST(NULL AS STRING) AS `storage`"),
			always(", CAST(NULL AS STRING) AS `compression`"),
			always(", CAST(NULL AS INT64) AS `stats_target`"),
			always("FROM INFORMATION_SCHEMA.COLUMNS c"),
			always("LEFT JOIN (SELECT table_catalog, table_schema, table_name, column_name, description"),
			always("FROM INFORMATION_SCHEMA.COLUMN_FIELD_PATHS WHERE field_path = column_name) f"),
			always("ON f.table_catalog = c.table_catalog AND f.table_schema = c.table_schema"),
			always("AND f.table_name = c.table_name AND f.column_name = c.column_name"),
			always("LEFT JOIN (SELECT u.table_catalog, u.table_schema, u.table_name, u.column_name"),
			always("FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE u JOIN INFORMATION_SCHEMA.TABLE_CONSTRAINTS s"),
			always("ON s.constraint_catalog = u.constraint_catalog AND s.constraint_schema = u.constraint_schema"),
			always("AND s.constraint_name = u.constraint_name WHERE s.constraint_type = 'PRIMARY KEY') k"),
			always("ON k.table_catalog = c.table_catalog AND k.table_schema = c.table_schema"),
			always("AND k.table_name = c.table_name AND k.column_name = c.column_name"),
			always("WHERE " + like("c.table_schema", "@schema")),
			always("AND " + like("c.table_name", "@parent")),
			always("AND " + like("c.column_name", "@name")),
			always("ORDER BY 2, 3, 5"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "ordinal", Desc: "ORDINAL_POSITION, one based"},
			{Name: "data_type", Desc: "DATA_TYPE as BigQuery writes it, such as STRING, INT64 or ARRAY<FLOAT64>"},
			{Name: "nullable", Desc: "IS_NULLABLE. A column of mode REQUIRED is not nullable, and so is a repeated column, because an array is never NULL"},
			{Name: "default", Desc: "COLUMN_DEFAULT as BigQuery writes the expression. BigQuery reports the text NULL for a column with no default, and the model turns that into an absent value"},
			{Name: "primary_key", Desc: "whether the column is part of the primary key, which BigQuery does not enforce"},
			{Name: "identity", Desc: "always or by default for an identity column, and absent otherwise"},
			{Name: "generated", Desc: "stored or virtual for a generated column, and absent otherwise. BigQuery refused every generated column that the fixture tried, so the answer is not measured"},
			{Name: "comment", Desc: "the description of the column, as plain text. Absent when none is set"},
			{Name: "collation", Desc: "COLLATION_NAME. BigQuery reports the text NULL for a column with none, and the model turns that into an absent value"},
			{Name: "storage", Desc: "always absent: BigQuery has no storage choice for a column"},
			{Name: "compression", Desc: "always absent: BigQuery has no compression choice for a column"},
			{Name: "stats_target", Desc: "always absent: BigQuery has no statistics target"},
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

	// \di. BigQuery has two kinds of index, a search index and a vector index.
	// The primary key is a constraint and no index backs it.
	dbmeta.Indexes.Register(dbmeta.BigQuery, &dbmeta.Binding[dbmeta.Index]{
		Stmt: dbmeta.Stmt{
			always("SELECT i.catalog AS `catalog`"),
			always(", i.schema AS `schema`"),
			always(", i.table_name AS `table`"),
			always(", i.index_name AS `name`"),
			always(", i.type AS `type`"),
			always(", FALSE AS `unique`"),
			always(", FALSE AS `primary`"),
			always(", CAST(NULL AS STRING) AS `comment`"),
			always(", CAST(NULL AS STRING) AS `owner`"),
			always(", 'permanent' AS `persistence`"),
			always(", i.size AS `size`"),
			always(", CAST(NULL AS STRING) AS `predicate`"),
			always(", i.status = 'ACTIVE' AS `valid`"),
			always(", CAST(NULL AS BOOL) AS `clustered`"),
			always(", CAST(NULL AS BOOL) AS `replica_identity`"),
			always(", CAST(NULL AS BOOL) AS `deferrable`"),
			always(", CAST(NULL AS BOOL) AS `initially_deferred`"),
			always(", i.options AS `options`"),
			always(", i.ddl AS `definition`"),
			always(", i.using_name AS `using`"),
			always(", CAST(NULL AS STRING) AS `constraint_type`"),
			always(", CAST(NULL AS STRING) AS `constraint_definition`"),
			always(", CAST(NULL AS BOOL) AS `constraint_period`"),
			always(", CAST(NULL AS BOOL) AS `table_visible`"),
			always("FROM " + indexes() + " i"),
			always("WHERE " + like("i.schema", "@schema")),
			always("AND " + like("i.table_name", "@parent")),
			always("AND " + like("i.index_name", "@name")),
			always("ORDER BY 2, 3, 4"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "type", Desc: "search or vector, which are the two kinds of index that BigQuery has"},
			{Name: "unique", Desc: "always false: a BigQuery index is never unique"},
			{Name: "primary", Desc: "always false: a primary key is a constraint and has no index"},
			{Name: "comment", Desc: "always absent: BigQuery keeps no description for an index"},
			{Name: "owner", Desc: "always absent: a BigQuery index has no owner"},
			{Name: "persistence", Desc: "always permanent"},
			{Name: "size", Desc: "TOTAL_STORAGE_BYTES of the index. It is 0 for an index that is disabled, as an index on a small table is"},
			{Name: "predicate", Desc: "always absent: a BigQuery index covers every row"},
			{Name: "valid", Desc: "whether INDEX_STATUS is ACTIVE. An index on a table below the size threshold is TEMPORARILY DISABLED, and BigQuery does not use it"},
			{Name: "clustered", Desc: "always absent: BigQuery has no CLUSTER of an index"},
			{Name: "replica_identity", Desc: "always absent: BigQuery has no logical replication"},
			{Name: "deferrable", Desc: "always absent: no constraint owns a BigQuery index"},
			{Name: "initially_deferred", Desc: "always absent, for the same reason"},
			{Name: "options", Desc: "name=value joined by a comma and a space, from SEARCH_INDEX_OPTIONS or VECTOR_INDEX_OPTIONS, such as analyzer or distance_type. The value is the text that BigQuery writes"},
			{Name: "definition", Desc: "DDL, which is the whole CREATE statement of the index"},
			{Name: "using", Desc: "the index_type option of a vector index, such as IVF, and absent for a search index"},
			{Name: "constraint_type", Desc: "always absent: no constraint owns a BigQuery index"},
			{Name: "constraint_definition", Desc: "always absent, for the same reason"},
			{Name: "constraint_period", Desc: "always absent, for the same reason"},
			{Name: "table_visible", Desc: "always absent: BigQuery has no search path"},
		},
		Params: schemaParentName("table", "index"),
		Scan: func(rows *sql.Rows) (dbmeta.Index, error) {
			var v dbmeta.Index
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Type, &v.Unique,
				&v.Primary, &v.Comment, &v.Owner, &v.Persistence, &v.Size, &v.Predicate,
				&v.Valid, &v.Clustered, &v.ReplicaIdentity, &v.Deferrable,
				&v.InitiallyDeferred, &v.Options, &v.Definition, &v.Using,
				&v.ConstraintType, &v.ConstraintDefinition, &v.ConstraintPeriod,
				&v.TableVisible)
			return v, err
		},
	})

	// \dv and \sv. VIEW_DEFINITION is the query, as it was written. A
	// materialized view is not in VIEWS, and Tables reads it.
	dbmeta.Views.Register(dbmeta.BigQuery, &dbmeta.Binding[dbmeta.View]{
		Stmt: dbmeta.Stmt{
			always("SELECT v.table_catalog AS `catalog`"),
			always(", v.table_schema AS `schema`"),
			always(", v.table_name AS `name`"),
			always(", v.view_definition AS `definition`"),
			always(", v.check_option AS `check_option`"),
			always(", CAST(NULL AS BOOL) AS `updatable`"),
			always(", t.is_insertable_into = 'YES' AS `insertable`"),
			always(", o.description AS `comment`"),
			always("FROM INFORMATION_SCHEMA.VIEWS v"),
			always("JOIN INFORMATION_SCHEMA.TABLES t ON t.table_catalog = v.table_catalog"),
			always("AND t.table_schema = v.table_schema AND t.table_name = v.table_name"),
			always("LEFT JOIN (SELECT table_catalog, table_schema, table_name"),
			always(", MAX(" + unquote("option_value") + ") AS description"),
			always("FROM INFORMATION_SCHEMA.TABLE_OPTIONS WHERE option_name = 'description'"),
			always("GROUP BY table_catalog, table_schema, table_name) o"),
			always("ON o.table_catalog = v.table_catalog AND o.table_schema = v.table_schema"),
			always("AND o.table_name = v.table_name"),
			always("WHERE " + like("v.table_schema", "@schema")),
			always("AND " + like("v.table_name", "@name")),
			always("ORDER BY 2, 3"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{Name: "definition", Desc: "VIEW_DEFINITION, the query as it was written"},
			{Name: "check_option", Desc: "CHECK_OPTION, which is absent for every view that was seen"},
			{Name: "updatable", Desc: "always absent: INFORMATION_SCHEMA does not say whether a view can be updated"},
			{Name: "insertable", Desc: "IS_INSERTABLE_INTO of the view, which is false for every view"},
			{Name: "comment", Desc: "the description of the view, as plain text. Absent when none is set"},
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
