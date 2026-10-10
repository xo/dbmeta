package vertica

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerRelations() {
	registerTables()
	registerColumns()
	registerProjections()
	registerConstraints()
}

// tableType is the word for the kind of the table t.
var tableType = text(`CASE WHEN t.is_temp_table THEN 'temporary table'` +
	` WHEN t.table_definition <> '' THEN 'external table'` +
	` WHEN t.is_flextable THEN 'flex table'` +
	` ELSE 'table' END`)

// viewType is the word for the kind of the view v.
var viewType = text(`CASE WHEN v.is_system_view THEN 'system view' ELSE 'view' END`)

// inTypes is the condition that item is one of the types in @types, and
// holds when @types is empty.
func inTypes(item string) string {
	return `(@types = '' OR ` + dbmeta.InList(`@types`, item) + `)`
}

func registerTables() {
	// \dn.
	dbmeta.Schemas.Register(dbmeta.Vertica, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, s.schema_name AS "name"`),
			always(`, s.schema_owner AS "owner"`),
			always(`, ` + comment("SCHEMA", "''", "s.schema_name") + ` AS "comment"`),
			always(`FROM v_catalog.schemata s`),
			always(`WHERE (@with_system OR NOT s.is_system_schema)`),
			always(`AND ` + like(`s.schema_name`, `@name`)),
			always(`ORDER BY s.schema_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: a Vertica connection reaches one database and has no catalog level"},
			{Name: "name"},
			{Name: "owner"},
			{Name: "comment", Desc: "from COMMENT ON SCHEMA"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "with_system", Desc: "include the schemas Vertica keeps for itself, such as v_catalog and v_monitor", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})

	// \dt and \dv. Tables and views are in two views, and the system
	// tables in a third, so this is one statement over all three and the
	// type column separates them.
	dbmeta.Tables.Register(dbmeta.Vertica, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, t.table_schema AS "schema"`),
			always(`, t.table_name AS "name"`),
			always(`, ` + tableType + ` AS "type"`),
			always(`, ` + comment("TABLE", "t.table_schema", "t.table_name") + ` AS "comment"`),
			always(`, t.owner_name AS "owner"`),
			always(`, ` + text(`CASE WHEN t.is_temp_table THEN 'temporary' ELSE 'permanent' END`) + ` AS "persistence"`),
			always(`, ` + text(`CASE WHEN t.partition_expression <> '' THEN 'partition_by=' || t.partition_expression END`) + ` AS "options"`),
			always(`FROM v_catalog.tables t`),
			always(`WHERE ` + notSystem(`t.table_schema`)),
			always(`AND ` + like(`t.table_schema`, `@schema`)),
			always(`AND ` + like(`t.table_name`, `@name`)),
			always(`AND ` + inTypes(tableType)),
			always(`UNION ALL`),
			always(`SELECT '', v.table_schema, v.table_name`),
			always(`, ` + viewType),
			always(`, ` + comment("VIEW", "v.table_schema", "v.table_name")),
			always(`, v.owner_name, CAST(NULL AS VARCHAR), CAST(NULL AS VARCHAR)`),
			always(`FROM v_catalog.views v`),
			always(`WHERE ` + notSystem(`v.table_schema`)),
			always(`AND ` + like(`v.table_schema`, `@schema`)),
			always(`AND ` + like(`v.table_name`, `@name`)),
			always(`AND ` + inTypes(viewType)),
			always(`UNION ALL`),
			always(`SELECT '', y.table_schema, y.table_name, 'system table', y.table_description`),
			always(`, CAST(NULL AS VARCHAR), CAST(NULL AS VARCHAR), CAST(NULL AS VARCHAR)`),
			always(`FROM v_catalog.system_tables y`),
			always(`WHERE @with_system`),
			always(`AND ` + like(`y.table_schema`, `@schema`)),
			always(`AND ` + like(`y.table_name`, `@name`)),
			always(`AND ` + inTypes(`'system table'`)),
			always(`ORDER BY 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: a Vertica connection reaches one database"},
			{Name: "schema"},
			{Name: "name"},
			{Name: "type", Desc: "table, temporary table, external table, flex table, view, system view or system table. An external table reads its rows from files at query time, and a flex table stores semi structured data in a map"},
			{Name: "comment", Desc: "from COMMENT ON TABLE or COMMENT ON VIEW, and the engine's own description for a system table"},
			{Name: "owner", Desc: "owner_name of the table or the view, and absent for a system table"},
			{Name: "persistence", Desc: "permanent, or temporary for a global or local temporary table. Absent for a view and a system table"},
			{Name: "options", Desc: "partition_by=expression for a partitioned table, and absent for the rest"},
		},
		Params: append(schemaAndName("table"), dbmeta.TypesParam()),
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment,
				&v.Owner, &v.Persistence, &v.Options)
			return v, err
		},
	})

	// \dv in full.
	dbmeta.Views.Register(dbmeta.Vertica, &dbmeta.Binding[dbmeta.View]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, v.table_schema AS "schema"`),
			always(`, v.table_name AS "name"`),
			always(`, v.view_definition AS "definition"`),
			always(`, 'none' AS "check_option"`),
			always(`, FALSE AS "updatable"`),
			always(`, FALSE AS "insertable"`),
			always(`, ` + comment("VIEW", "v.table_schema", "v.table_name") + ` AS "comment"`),
			always(`FROM v_catalog.views v`),
			always(`WHERE ` + notSystem(`v.table_schema`)),
			always(`AND ` + like(`v.table_schema`, `@schema`)),
			always(`AND ` + like(`v.table_name`, `@name`)),
			always(`ORDER BY v.table_schema, v.table_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: a Vertica connection reaches one database"},
			{Name: "schema"}, {Name: "name"},
			{Name: "definition", Desc: "the query the view selects, as Vertica rewrote it with every name qualified"},
			{Name: "check_option", Desc: "always none: Vertica has no WITH CHECK OPTION"},
			{Name: "updatable", Desc: "always false: a Vertica view cannot be the target of INSERT, UPDATE or DELETE"},
			{Name: "insertable", Desc: "always false, for the same reason"},
			{Name: "comment", Desc: "from COMMENT ON VIEW"},
		},
		Params: schemaAndName("view"),
		Scan: func(rows *sql.Rows) (dbmeta.View, error) {
			var v dbmeta.View
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Definition,
				&v.CheckOption, &v.Updatable, &v.Insertable, &v.Comment)
			return v, err
		},
	})

	// \dP. A table is partitioned by an expression over its columns.
	dbmeta.PartitionedTables.Register(dbmeta.Vertica, &dbmeta.Binding[dbmeta.PartitionedTable]{
		Stmt: dbmeta.Stmt{
			always(`SELECT t.table_schema AS "schema"`),
			always(`, t.table_name AS "name"`),
			always(`, t.owner_name AS "owner"`),
			always(`, 'table' AS "type"`),
			always(`, '' AS "parent"`),
			always(`, 'expression' AS "strategy"`),
			always(`, t.partition_expression AS "expression"`),
			always(`, ` + comment("TABLE", "t.table_schema", "t.table_name") + ` AS "comment"`),
			always(`FROM v_catalog.tables t`),
			always(`WHERE t.partition_expression <> ''`),
			always(`AND ` + notSystem(`t.table_schema`)),
			always(`AND ` + like(`t.table_schema`, `@schema`)),
			always(`AND ` + like(`t.table_name`, `@name`)),
			always(`ORDER BY t.table_schema, t.table_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"}, {Name: "owner"},
			{Name: "type", Desc: "always table: Vertica partitions a table and nothing else"},
			{Name: "parent", Desc: "always empty: a Vertica partition is not a table of its own"},
			{Name: "strategy", Desc: "always expression: Vertica partitions by the value of an expression and records no range, list or hash kind"},
			{Name: "expression", Desc: "the PARTITION BY expression, with each column qualified by its table, such as archive.filed_year"},
			{Name: "comment", Desc: "the table's comment"},
		},
		Params: schemaAndName("table"),
		Scan: func(rows *sql.Rows) (dbmeta.PartitionedTable, error) {
			var v dbmeta.PartitionedTable
			err := rows.Scan(&v.Schema, &v.Name, &v.Owner, &v.Type, &v.Parent,
				&v.Strategy, &v.Expression, &v.Comment)
			return v, err
		},
	})

	// \ds. An identity column is backed by a sequence of its own, which is
	// listed here and names the table it serves.
	dbmeta.Sequences.Register(dbmeta.Vertica, &dbmeta.Binding[dbmeta.Sequence]{
		Stmt: dbmeta.Stmt{
			always(`SELECT q.sequence_schema AS "schema"`),
			always(`, q.sequence_name AS "name"`),
			always(`, 'INTEGER' AS "data_type"`),
			always(`, CAST(NULL AS INTEGER) AS "start"`),
			always(`, q.minimum AS "minimum"`),
			always(`, q.maximum AS "maximum"`),
			always(`, q.increment_by AS "increment"`),
			always(`, q.allow_cycle AS "cycles"`),
			always(`, ` + text(`CASE WHEN q.identity_table_name IS NULL THEN ''`+
				` ELSE q.sequence_schema || '.' || q.identity_table_name END`) + ` AS "owned_by"`),
			always(`, ` + comment("SEQUENCE", "q.sequence_schema", "q.sequence_name") + ` AS "comment"`),
			always(`FROM v_catalog.sequences q`),
			always(`WHERE ` + notSystem(`q.sequence_schema`)),
			always(`AND ` + like(`q.sequence_schema`, `@schema`)),
			always(`AND ` + like(`q.sequence_name`, `@name`)),
			always(`ORDER BY q.sequence_schema, q.sequence_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{Name: "data_type", Desc: "always INTEGER: every Vertica sequence is a 64 bit integer"},
			{Name: "start", Desc: "always absent: v_catalog.sequences records the current value and not the one the sequence started from"},
			{Name: "minimum"}, {Name: "maximum"}, {Name: "increment"},
			{Name: "cycles", Desc: "from allow_cycle"},
			{Name: "owned_by", Desc: "the table an identity column's sequence serves, schema qualified, and empty for a sequence of its own. Vertica records the table and not the column"},
			{Name: "comment", Desc: "from COMMENT ON SEQUENCE"},
		},
		Params: schemaAndName("sequence"),
		Scan: func(rows *sql.Rows) (dbmeta.Sequence, error) {
			var v dbmeta.Sequence
			err := rows.Scan(&v.Schema, &v.Name, &v.DataType, &v.Start, &v.Minimum,
				&v.Maximum, &v.Increment, &v.Cycles, &v.OwnedBy, &v.Comment)
			return v, err
		},
	})

	// Every comment in the database, which Vertica keeps in one place. A
	// column comment is left out, because it needs two names to identify it
	// and this kind has one, and Columns carries it.
	dbmeta.Comments.Register(dbmeta.Vertica, &dbmeta.Binding[dbmeta.Comment]{
		Stmt: dbmeta.Stmt{
			always(`SELECT m.object_schema AS "schema"`),
			always(`, m.object_name AS "name"`),
			always(`, LOWER(m.object_type) AS "type"`),
			always(`, m.comment AS "comment"`),
			always(`FROM v_catalog.comments m`),
			always(`WHERE m.object_type <> 'COLUMN'`),
			always(`AND ` + like(`m.object_name`, `@name`)),
			always(`ORDER BY 3, 1, 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "empty for an object that belongs to the database, such as a schema"},
			{Name: "name"},
			{Name: "type", Desc: "the kind of object, lower cased, such as schema, table, view, sequence, projection or function"},
			{Name: "comment"},
		},
		Params: nameOnly("object"),
		Scan: func(rows *sql.Rows) (dbmeta.Comment, error) {
			var v dbmeta.Comment
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})
}

func registerColumns() {
	// The columns of a table or a view, which Vertica keeps in two views of
	// the same shape.
	dbmeta.Columns.Register(dbmeta.Vertica, &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, c.table_schema AS "schema"`),
			always(`, c.table_name AS "table"`),
			always(`, c.column_name AS "name"`),
			always(`, c.ordinal_position AS "ordinal"`),
			always(`, c.data_type AS "data_type"`),
			always(`, c.is_nullable AS "nullable"`),
			// The catalog writes an empty string for a column with no
			// default, and NULL for an identity column. Both are the absence
			// of a default expression.
			always(`, NULLIF(c.column_default, '') AS "default"`),
			always(`, CASE WHEN k.column_name IS NULL THEN FALSE ELSE TRUE END AS "primary_key"`),
			always(`, ` + text(`CASE WHEN c.is_identity THEN 'always' ELSE '' END`) + ` AS "identity"`),
			since(v91, `, `+text(`CASE WHEN c.column_set_using <> '' THEN 'set using' ELSE '' END`)+` AS "generated"`,
				`, '' AS "generated"`),
			since(v101, `, (SELECT m.comment FROM v_catalog.comments m WHERE m.object_type = 'COLUMN'`+
				` AND m.object_schema = c.table_schema AND m.object_name = c.table_name`+
				` AND m.child_object = c.column_name) AS "comment"`,
				`, CAST(NULL AS VARCHAR) AS "comment"`),
			always(`, CAST(NULL AS VARCHAR) AS "collation"`),
			always(`FROM v_catalog.columns c`),
			always(`LEFT JOIN v_catalog.primary_keys k ON k.table_schema = c.table_schema` +
				` AND k.table_name = c.table_name AND k.column_name = c.column_name`),
			always(`WHERE ` + notSystem(`c.table_schema`)),
			always(`AND ` + like(`c.table_schema`, `@schema`)),
			always(`AND ` + like(`c.table_name`, `@parent`)),
			always(`AND ` + like(`c.column_name`, `@name`)),
			always(`UNION ALL`),
			always(`SELECT '', w.table_schema, w.table_name, w.column_name, w.ordinal_position`),
			always(`, w.data_type, TRUE, CAST(NULL AS VARCHAR), FALSE, '', ''`),
			since(v101, `, (SELECT m.comment FROM v_catalog.comments m WHERE m.object_type = 'COLUMN'`+
				` AND m.object_schema = w.table_schema AND m.object_name = w.table_name`+
				` AND m.child_object = w.column_name)`,
				`, CAST(NULL AS VARCHAR)`),
			always(`, CAST(NULL AS VARCHAR)`),
			always(`FROM v_catalog.view_columns w`),
			always(`WHERE ` + notSystem(`w.table_schema`)),
			always(`AND ` + like(`w.table_schema`, `@schema`)),
			always(`AND ` + like(`w.table_name`, `@parent`)),
			always(`AND ` + like(`w.column_name`, `@name`)),
			always(`ORDER BY 2, 3, 5`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: a Vertica connection reaches one database"},
			{Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "ordinal", Desc: "from ordinal_position, which counts from one"},
			{Name: "data_type", Desc: "the type as Vertica writes it, such as varchar(128) or numeric(12,2)"},
			{Name: "nullable", Desc: "true for a view column, which records no nullability, the way PostgreSQL reports one"},
			{Name: "default", Desc: "the default expression, such as 'plain' with its quotes, and absent where there is none"},
			{Name: "primary_key", Desc: "always false for a view column: Vertica records no key on a view"},
			{Name: "identity", Desc: "always for an IDENTITY or AUTO_INCREMENT column, which refuses an explicit value, and empty otherwise"},
			{Name: "generated", Desc: "set using for a column Vertica refreshes from a query with SET USING, and empty otherwise. 7.2 has no such column, so it is empty there"},
			{Name: "comment", Desc: "from COMMENT ON COLUMN. Before 10.1 a comment belongs to a projection column rather than to the table's, so it is absent here"},
			{Name: "collation", Desc: "always absent: Vertica has no collation on a column"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "parent", Desc: "table or view name pattern, empty for every one", Default: ""},
			{Name: "name", Desc: "column name pattern, empty for every column", Default: ""},
			{Name: "with_system", Desc: "include the schemas Vertica keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			var v dbmeta.Column
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Ordinal,
				&v.DataType, &v.Nullable, &v.Default, &v.PrimaryKey, &v.Identity,
				&v.Generated, &v.Comment, &v.Collation)
			return v, err
		},
	})
}

// row91 is a fragment of a statement that needs 9.1, the oldest release that
// has a row access policy.
func row91(query string) dbmeta.Choice { return dbmeta.Choice{{Min: v91, Query: query}} }

func registerProjections() {
	// A row access policy filters the rows a user reads. The catalog has no
	// name for one and no list of roles: the expression decides who it applies
	// to, with a call such as ENABLED_ROLE. A column policy is not a row
	// policy and is not listed. A disabled policy is listed, and Enabled says
	// that it is disabled (D211). The column is_policy_enabled holds the words
	// Enabled and Disabled.
	dbmeta.Policies.Register(dbmeta.Vertica, &dbmeta.Binding[dbmeta.Policy]{
		Stmt: dbmeta.Stmt{
			row91(`SELECT SPLIT_PART(a.table_name, '.', 1) AS "schema"`),
			row91(`, SPLIT_PART(a.table_name, '.', 2) AS "table"`),
			row91(`, ` + text(`a.access_policy_oid`) + ` AS "name"`),
			row91(`, 'all' AS "command"`),
			row91(`, TRUE AS "permissive"`),
			row91(`, CAST(NULL AS VARCHAR) AS "roles"`),
			row91(`, a.expression AS "using"`),
			row91(`, CAST(NULL AS VARCHAR) AS "with_check"`),
			row91(`, CAST(NULL AS VARCHAR) AS "comment"`),
			row91(`, CASE a.is_policy_enabled WHEN 'Enabled' THEN TRUE WHEN 'Disabled' THEN FALSE END AS "enabled"`),
			row91(`FROM v_catalog.access_policy a`),
			row91(`WHERE a.policy_type = 'Row policy'`),
			row91(`AND ` + notSystem(`SPLIT_PART(a.table_name, '.', 1)`)),
			row91(`AND ` + like(`SPLIT_PART(a.table_name, '.', 1)`, `@schema`)),
			row91(`AND ` + like(`SPLIT_PART(a.table_name, '.', 2)`, `@parent`)),
			row91(`AND ` + like(text(`a.access_policy_oid`), `@name`)),
			row91(`ORDER BY 1, 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Min: v91}, {Name: "table", Min: v91},
			{Name: "name", Desc: "the object id of the policy, because Vertica gives a policy no name", Min: v91},
			{Name: "command", Desc: "always all: a row access policy filters every read and write of the table", Min: v91},
			{Name: "permissive", Desc: "always true: Vertica has no restrictive policy", Min: v91},
			{Name: "roles", Desc: "always absent: the expression names the roles it applies to", Min: v91},
			{Name: "using", Desc: "the expression, such as (a > 1)", Min: v91},
			{Name: "with_check", Desc: "always absent: a Vertica row policy has one expression", Min: v91},
			{Name: "comment", Desc: "always absent", Min: v91},
			{Name: "enabled", Desc: "false for a policy that ALTER ACCESS POLICY ... DISABLE switched off, which restricts nothing", Min: v91},
		},
		Params: parentAndName("policy"),
		Scan: func(rows *sql.Rows) (dbmeta.Policy, error) {
			var v dbmeta.Policy
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Command, &v.Permissive,
				&v.Roles, &v.Using, &v.WithCheck, &v.Comment, &v.Enabled)
			return v, err
		},
	})

	// The partitions of a table, from the storage containers of its
	// projections. A partition is a value of the PARTITION BY expression, and
	// a table has one row for each value that holds data. A partition has no
	// table, so the type is partition. The rows of a partition are in
	// partitions.ros_row_count and the Partition kind has no field for them.
	dbmeta.Partitions.Register(dbmeta.Vertica, &dbmeta.Binding[dbmeta.Partition]{
		Stmt: dbmeta.Stmt{
			always(`SELECT j.projection_schema AS "schema"`),
			always(`, j.anchor_table_name AS "table"`),
			always(`, j.projection_schema AS "partition_schema"`),
			always(`, ` + text(`x.partition_key`) + ` AS "partition"`),
			always(`, 'partition' AS "type"`),
			always(`, ` + text(`x.partition_key`) + ` AS "bound"`),
			always(`, CAST(NULL AS VARCHAR) AS "constraint"`),
			always(`, FALSE AS "partitioned"`),
			always(`, FALSE AS "detach_pending"`),
			always(`, CAST(NULL AS BOOLEAN) AS "table_visible"`),
			always(`, CAST(NULL AS BOOLEAN) AS "partition_visible"`),
			always(`FROM v_monitor.partitions x`),
			always(`JOIN v_catalog.projections j ON j.projection_id = x.projection_id`),
			always(`WHERE ` + notSystem(`j.projection_schema`)),
			always(`AND ` + like(`j.projection_schema`, `@schema`)),
			always(`AND ` + like(`j.anchor_table_name`, `@parent`)),
			always(`AND ` + like(`j.projection_schema`, `@partition_schema`)),
			always(`AND ` + like(`x.partition_key`, `@name`)),
			always(`GROUP BY j.projection_schema, j.anchor_table_name, x.partition_key`),
			always(`ORDER BY 1, 2, 4`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "schema of the partitioned table"},
			{Name: "table", Desc: "the partitioned table, which is the anchor table of the projections that hold the partition"},
			{Name: "partition_schema", Desc: "the same schema: a partition is in no schema of its own"},
			{Name: "partition", Desc: "the partition key, which is the value of the PARTITION BY expression, such as 2025"},
			{Name: "type", Desc: "always partition: a Vertica partition is not a table"},
			{Name: "bound", Desc: "the same value as the partition key"},
			{Name: "constraint", Desc: "always absent: Vertica makes no constraint of a partition value"},
			{Name: "partitioned", Desc: "always false: a partition has no partitions of its own"},
			{Name: "detach_pending", Desc: "always false: Vertica has no concurrent detach"},
			{Name: "table_visible", Desc: "always absent: Vertica reports no search path for a table"},
			{Name: "partition_visible", Desc: "always absent, for the same reason"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern of the table, empty for every schema", Default: ""},
			{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
			{Name: "partition_schema", Desc: "schema name pattern of the partition, empty for every schema", Default: ""},
			{Name: "name", Desc: "partition key pattern, empty for every partition", Default: ""},
			{Name: "with_system", Desc: "include the schemas Vertica keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Partition, error) {
			var v dbmeta.Partition
			err := rows.Scan(&v.Schema, &v.Table, &v.PartitionSchema, &v.Partition, &v.Type,
				&v.Bound, &v.Constraint, &v.Partitioned, &v.DetachPending,
				&v.TableVisible, &v.PartitionVisible)
			return v, err
		},
	})

	// \di. Vertica has no index. A projection is a stored, sorted and
	// segmented copy of some or all of a table's columns, and it is what
	// the optimizer chooses between the way another database chooses an
	// index, so it is what this reports. Every table gets a superprojection
	// holding every column the first time rows are loaded into it.
	dbmeta.Indexes.Register(dbmeta.Vertica, &dbmeta.Binding[dbmeta.Index]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, p.projection_schema AS "schema"`),
			always(`, p.anchor_table_name AS "table"`),
			always(`, p.projection_name AS "name"`),
			always(`, ` + text(`CASE WHEN p.is_super_projection THEN 'superprojection'`+
				` WHEN p.is_aggregate_projection THEN 'aggregate projection'`+
				` ELSE 'projection' END`) + ` AS "type"`),
			always(`, FALSE AS "unique"`),
			always(`, p.is_key_constraint_projection AS "primary"`),
			always(`, ` + comment("PROJECTION", "p.projection_schema", "p.projection_name") + ` AS "comment"`),
			always(`, (SELECT CAST(SUM(s.used_bytes) AS BIGINT) FROM v_monitor.projection_storage s` +
				` WHERE s.projection_id = p.projection_id) AS "size"`),
			always(`, p.is_up_to_date AS "valid"`),
			always(`, ` + text(`CASE WHEN p.is_segmented THEN 'segmented_by=' || p.segment_expression`+
				` ELSE 'unsegmented' END`) + ` AS "options"`),
			always(`FROM v_catalog.projections p`),
			always(`WHERE ` + notSystem(`p.projection_schema`)),
			always(`AND ` + like(`p.projection_schema`, `@schema`)),
			always(`AND ` + like(`p.anchor_table_name`, `@parent`)),
			always(`AND ` + like(`p.projection_name`, `@name`)),
			always(`ORDER BY p.projection_schema, p.anchor_table_name, p.projection_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: a Vertica connection reaches one database"},
			{Name: "schema"},
			{Name: "table", Desc: "the anchor table, whose columns the projection stores"},
			{Name: "name", Desc: "the projection. A projection on a cluster of more than one node is replicated or segmented, and Vertica names each copy with a _b0 or _b1 suffix"},
			{Name: "type", Desc: "superprojection, aggregate projection or projection. A superprojection holds every column of its table"},
			{Name: "unique", Desc: "always false: a projection enforces nothing. A key is checked by its constraint, when it is enabled"},
			{Name: "primary", Desc: "true for a projection Vertica made to enforce an enabled key constraint"},
			{Name: "comment", Desc: "from COMMENT ON PROJECTION"},
			{
				Name: "size",
				Desc: "the bytes of the projection on every node, from projection_storage. The rows of the projection are in the same view and Index has no field for them. Absent for a projection that holds no row yet",
			},
			{Name: "valid", Desc: "is_up_to_date: false for a projection that holds the data of an earlier time and that the optimizer does not use until it is refreshed"},
			{Name: "options", Desc: "segmented_by=expression, or unsegmented for a projection that every node holds in full"},
		},
		Params: parentAndName("projection"),
		Scan: func(rows *sql.Rows) (dbmeta.Index, error) {
			var v dbmeta.Index
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Type,
				&v.Unique, &v.Primary, &v.Comment, &v.Size, &v.Valid, &v.Options)
			return v, err
		},
	})

	// The columns of each projection, in the order it stores them.
	dbmeta.IndexColumns.Register(dbmeta.Vertica, &dbmeta.Binding[dbmeta.IndexColumn]{
		Stmt: dbmeta.Stmt{
			always(`SELECT c.table_schema AS "schema"`),
			always(`, c.table_name AS "table"`),
			always(`, c.projection_name AS "index"`),
			always(`, c.projection_column_name AS "name"`),
			always(`, c.column_position + 1 AS "ordinal"`),
			always(`, CASE WHEN c.is_expression THEN c.column_expression END AS "expression"`),
			always(`, CASE WHEN c.order_by_type LIKE 'DESC%' THEN TRUE ELSE FALSE END AS "descending"`),
			always(`FROM v_catalog.projection_columns c`),
			always(`WHERE ` + notSystem(`c.table_schema`)),
			always(`AND ` + like(`c.table_schema`, `@schema`)),
			always(`AND ` + like(`c.table_name`, `@parent`)),
			always(`AND ` + like(`c.projection_name`, `@name`)),
			always(`ORDER BY 1, 2, 3, 5`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"},
			{Name: "table", Desc: "the anchor table"},
			{Name: "index", Desc: "the projection"},
			{Name: "name", Desc: "the column as the projection names it, which is the table's column name unless the projection renamed it"},
			{Name: "ordinal", Desc: "the column's position in the projection, counting from one. The sort order is a separate list, and a column the projection stores and does not sort by is still here"},
			{Name: "expression", Desc: "the expression a projection column computes, and absent for a plain column"},
			{Name: "descending", Desc: "true for a column the projection sorts descending"},
		},
		Params: parentAndName("projection"),
		Scan: func(rows *sql.Rows) (dbmeta.IndexColumn, error) {
			var v dbmeta.IndexColumn
			err := rows.Scan(&v.Schema, &v.Table, &v.Index, &v.Name, &v.Ordinal,
				&v.Expression, &v.Descending)
			return v, err
		},
	})
}

// constraintKind spells out the one letter Vertica records for a kind.
func constraintKind(col string) string {
	return text(`CASE ` + col + ` WHEN 'p' THEN 'primary key' WHEN 'f' THEN 'foreign key'` +
		` WHEN 'u' THEN 'unique' WHEN 'c' THEN 'check' WHEN 'n' THEN 'not null'` +
		` ELSE ` + col + ` END`)
}

func registerConstraints() {
	// The constraints on a table. v_catalog.table_constraints holds the
	// primary, foreign, unique and check constraints. A NOT NULL is in
	// constraint_columns alone, under the one name C_NOTNULL for every
	// column, and it is not a constraint of its own here, the way it is not
	// in PostgreSQL.
	dbmeta.Constraints.Register(dbmeta.Vertica, &dbmeta.Binding[dbmeta.Constraint]{
		Stmt: dbmeta.Stmt{
			always(`SELECT t.table_schema AS "schema"`),
			always(`, t.table_name AS "table"`),
			always(`, k.constraint_name AS "name"`),
			always(`, ` + constraintKind("k.constraint_type") + ` AS "type"`),
			since(v91, `, CASE WHEN k.constraint_type = 'c' THEN k.predicate END AS "definition"`,
				`, CAST(NULL AS VARCHAR) AS "definition"`),
			always(`, FALSE AS "deferrable"`),
			always(`, FALSE AS "deferred"`),
			always(`, CAST(NULL AS VARCHAR) AS "comment"`),
			always(`, k.is_enabled AS "enforced"`),
			always(`FROM v_catalog.table_constraints k`),
			always(`JOIN v_catalog.tables t ON t.table_id = k.table_id`),
			always(`WHERE ` + notSystem(`t.table_schema`)),
			always(`AND ` + like(`t.table_schema`, `@schema`)),
			always(`AND ` + like(`t.table_name`, `@parent`)),
			always(`AND ` + like(`k.constraint_name`, `@name`)),
			always(`ORDER BY 1, 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "type", Desc: "primary key, foreign key, unique or check. A key is recorded whether or not it is enabled, and Vertica checks one only when it is"},
			{Name: "definition", Desc: "the CHECK predicate, and absent for every other kind. A CHECK constraint arrived in 9.1"},
			{Name: "deferrable", Desc: "always false: Vertica defers no constraint"},
			{Name: "deferred", Desc: "always false, for the same reason"},
			{Name: "comment", Desc: "always absent: COMMENT ON has no constraint form"},
			{Name: "enforced", Desc: "is_enabled: Vertica checks a key only when it is enabled, and it is disabled unless the statement said ENABLED or a setting asks for it. A check constraint is always enabled, and a foreign key has no value, so it is absent"},
		},
		Params: parentAndName("constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.Constraint, error) {
			var v dbmeta.Constraint
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Type, &v.Definition,
				&v.Deferrable, &v.Deferred, &v.Comment, &v.Enforced)
			return v, err
		},
	})

	// The columns of each constraint. A primary and a foreign key record
	// the position of each column in their own views, and a unique or check
	// constraint records none, so its columns are numbered in the table's
	// column order.
	dbmeta.ConstraintColumns.Register(dbmeta.Vertica, &dbmeta.Binding[dbmeta.ConstraintColumn]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, k.table_schema AS "schema"`),
			always(`, k.table_name AS "table"`),
			always(`, k.constraint_name AS "constraint"`),
			always(`, k.column_name AS "name"`),
			always(`, k.ordinal_position AS "ordinal"`),
			always(`, CAST(NULL AS VARCHAR) AS "foreign_catalog"`),
			always(`, CAST(NULL AS VARCHAR) AS "foreign_schema"`),
			always(`, CAST(NULL AS VARCHAR) AS "foreign_table"`),
			always(`, CAST(NULL AS VARCHAR) AS "foreign_name"`),
			always(`FROM v_catalog.primary_keys k`),
			always(`WHERE ` + notSystem(`k.table_schema`)),
			always(`AND ` + like(`k.table_schema`, `@schema`)),
			always(`AND ` + like(`k.table_name`, `@parent`)),
			always(`AND ` + like(`k.constraint_name`, `@name`)),
			always(`UNION ALL`),
			always(`SELECT '', f.table_schema, f.table_name, f.constraint_name, f.column_name`),
			always(`, f.ordinal_position, '', f.reference_table_schema, f.reference_table_name`),
			always(`, f.reference_column_name`),
			always(`FROM v_catalog.foreign_keys f`),
			always(`WHERE ` + notSystem(`f.table_schema`)),
			always(`AND ` + like(`f.table_schema`, `@schema`)),
			always(`AND ` + like(`f.table_name`, `@parent`)),
			always(`AND ` + like(`f.constraint_name`, `@name`)),
			always(`UNION ALL`),
			always(`SELECT '', u.table_schema, u.table_name, u.constraint_name, u.column_name`),
			always(`, ROW_NUMBER() OVER (PARTITION BY u.constraint_id ORDER BY c.ordinal_position)`),
			always(`, CAST(NULL AS VARCHAR), CAST(NULL AS VARCHAR), CAST(NULL AS VARCHAR)`),
			always(`, CAST(NULL AS VARCHAR)`),
			always(`FROM v_catalog.constraint_columns u`),
			always(`JOIN v_catalog.columns c ON c.table_id = u.table_id AND c.column_name = u.column_name`),
			always(`WHERE u.constraint_type IN ('u', 'c')`),
			always(`AND ` + notSystem(`u.table_schema`)),
			always(`AND ` + like(`u.table_schema`, `@schema`)),
			always(`AND ` + like(`u.table_name`, `@parent`)),
			always(`AND ` + like(`u.constraint_name`, `@name`)),
			always(`ORDER BY 2, 3, 4, 6`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: a Vertica connection reaches one database"},
			{Name: "schema"}, {Name: "table"}, {Name: "constraint"}, {Name: "name"},
			{Name: "ordinal", Desc: "the column's position in a primary or foreign key, counting from one. A unique or check constraint records no position, so its columns are numbered in the table's column order"},
			{Name: "foreign_catalog", Desc: "empty for a foreign key and absent for every other kind"},
			{Name: "foreign_schema", Desc: "absent unless this is a foreign key"},
			{Name: "foreign_table", Desc: "absent unless this is a foreign key"},
			{Name: "foreign_name", Desc: "absent unless this is a foreign key"},
		},
		Params: parentAndName("constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.ConstraintColumn, error) {
			var v dbmeta.ConstraintColumn
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Constraint, &v.Name,
				&v.Ordinal, &v.ForeignCatalog, &v.ForeignSchema, &v.ForeignTable, &v.ForeignName)
			return v, err
		},
	})
}
