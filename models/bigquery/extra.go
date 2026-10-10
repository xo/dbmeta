package bigquery

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// Routines, comments, partitions, the current user and databases.

// routineStmt is the statement of Functions and Aggregates. where is the
// condition that tells the two apart. BigQuery names the kind of a routine
// FUNCTION, TABLE FUNCTION, PROCEDURE or AGGREGATE FUNCTION.
func routineStmt(where string) dbmeta.Stmt {
	return dbmeta.Stmt{
		always("SELECT r.routine_catalog AS `catalog`"),
		always(", r.routine_schema AS `schema`"),
		always(", r.routine_name AS `name`"),
		always(", r.specific_name AS `id`"),
		always(", LOWER(r.routine_type) AS `kind`"),
		always(", r.data_type AS `result_type`"),
		always(", COALESCE(a.arg_types, '') AS `arg_types`"),
		always(", IF(r.is_deterministic = 'YES', 'immutable', '') AS `volatility`"),
		always(", '' AS `parallel`"),
		always(", CAST(NULL AS STRING) AS `owner`"),
		always(", LOWER(r.security_type) AS `security`"),
		always(", CAST(NULL AS STRING) AS `access`"),
		always(", LOWER(COALESCE(r.external_language, r.routine_body)) AS `language`"),
		always(", r.routine_definition AS `source`"),
		always(", o.description AS `comment`"),
		always(", r.ddl AS `definition`"),
		always(", FALSE AS `leakproof`"),
		always(", CAST(NULL AS STRING) AS `prosrc`"),
		always("FROM INFORMATION_SCHEMA.ROUTINES r"),
		always("LEFT JOIN (SELECT specific_catalog, specific_schema, specific_name"),
		always(", STRING_AGG(data_type, ', ' ORDER BY ordinal_position) AS arg_types"),
		always("FROM INFORMATION_SCHEMA.PARAMETERS WHERE is_result = 'NO'"),
		always("GROUP BY specific_catalog, specific_schema, specific_name) a"),
		always("ON a.specific_catalog = r.specific_catalog AND a.specific_schema = r.specific_schema"),
		always("AND a.specific_name = r.specific_name"),
		always("LEFT JOIN (SELECT specific_catalog, specific_schema, specific_name"),
		always(", MAX(" + unquote("option_value") + ") AS description"),
		always("FROM INFORMATION_SCHEMA.ROUTINE_OPTIONS WHERE option_name = 'description'"),
		always("GROUP BY specific_catalog, specific_schema, specific_name) o"),
		always("ON o.specific_catalog = r.specific_catalog AND o.specific_schema = r.specific_schema"),
		always("AND o.specific_name = r.specific_name"),
		always("WHERE " + where),
		always("AND " + like("r.routine_schema", "@schema")),
		always("AND " + like("r.routine_name", "@name")),
		always("ORDER BY 2, 3"),
	}
}

// routineFields declares the columns of routineStmt.
func routineFields(kind string) []dbmeta.Field {
	return []dbmeta.Field{
		{Name: "catalog", Desc: "the project"},
		{Name: "schema", Desc: "the dataset"},
		{Name: "name"},
		{Name: "id", Desc: "SPECIFIC_NAME, which is the name, because BigQuery does not overload a routine"},
		{Name: "kind", Desc: kind},
		{Name: "result_type", Desc: "DATA_TYPE, the type of the value. Absent for a procedure and for a table function, which BigQuery reports no type for"},
		{Name: "arg_types", Desc: "the types of the parameters in order, joined by a comma and a space, and empty for none"},
		{Name: "volatility", Desc: "immutable when IS_DETERMINISTIC is YES, and empty otherwise. BigQuery says it for a function written in JavaScript only"},
		{Name: "parallel", Desc: "always empty: BigQuery has no such property"},
		{Name: "owner", Desc: "always absent: a BigQuery routine has no owner"},
		{Name: "security", Desc: "SECURITY_TYPE in lower case, which is empty for every routine that was seen"},
		{Name: "access", Desc: "always absent: BigQuery grants access on a dataset or a routine by IAM, which INFORMATION_SCHEMA does not list for a dataset principal"},
		{Name: "language", Desc: "EXTERNAL_LANGUAGE in lower case, such as javascript, and ROUTINE_BODY in lower case, which is sql, where there is none"},
		{Name: "source", Desc: "ROUTINE_DEFINITION, which is the body of the routine and not the whole statement"},
		{Name: "comment", Desc: "the description option of the routine, as plain text. Absent when none is set"},
		{Name: "definition", Desc: "DDL, which is the whole CREATE statement"},
		{Name: "leakproof", Desc: "always false: BigQuery has no such property"},
		{Name: "prosrc", Desc: "always absent: BigQuery has no such column"},
	}
}

func scanFunction(rows *sql.Rows) (dbmeta.Function, error) {
	var v dbmeta.Function
	err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.ID, &v.Kind, &v.ResultType,
		&v.ArgTypes, &v.Volatility, &v.Parallel, &v.Owner, dbmeta.NullAsEmpty(&v.Security),
		&v.Access, dbmeta.NullAsEmpty(&v.Language), &v.Source, &v.Comment,
		&v.Definition, &v.Leakproof, &v.Prosrc)
	return v, err
}

func registerExtra() {
	// The IAM principal that the connection signed in as, such as
	// name@project.iam.gserviceaccount.com.
	dbmeta.CurrentUser.Register(dbmeta.BigQuery, &dbmeta.Binding[dbmeta.User]{
		Stmt: dbmeta.Stmt{
			always("SELECT SESSION_USER() AS `name`"),
			always(", CAST(NULL AS STRING) AS `session`"),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "SESSION_USER(), the IAM principal, such as the email of a service account"},
			{Name: "session", Desc: "always absent: a BigQuery session has no second user"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.User, error) {
			var v dbmeta.User
			err := rows.Scan(&v.Name, &v.Session)
			return v, err
		},
	})

	// \l. A project is the database. BigQuery has no statement that lists the
	// projects, so the one row is the project that the session runs in, which
	// @@project_id names.
	dbmeta.Databases.Register(dbmeta.BigQuery, &dbmeta.Binding[dbmeta.Database]{
		Stmt: dbmeta.Stmt{
			always("SELECT d.name AS `name`"),
			always(", '' AS `owner`"),
			always(", '' AS `encoding`"),
			always(", '' AS `collate`"),
			always(", '' AS `ctype`"),
			always(", CAST(NULL AS STRING) AS `access`"),
			always(", CAST(NULL AS STRING) AS `tablespace`"),
			always(", CAST(NULL AS STRING) AS `size`"),
			always(", CAST(NULL AS STRING) AS `comment`"),
			always("FROM (SELECT @@project_id AS name) d"),
			always("WHERE " + like("d.name", "@name")),
			always("ORDER BY 1"),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the project that the session runs in. It is the only row, because BigQuery lists no other project"},
			{Name: "owner", Desc: "always empty: a project is owned through IAM"},
			{Name: "encoding", Desc: "always empty: BigQuery reports no encoding"},
			{Name: "collate", Desc: "always empty: a project has no collation"},
			{Name: "ctype", Desc: "always empty, for the same reason"},
			{Name: "access", Desc: "always absent"},
			{Name: "tablespace", Desc: "always absent: BigQuery has no tablespace"},
			{Name: "size", Desc: "always absent: BigQuery reports no size for a project"},
			{Name: "comment", Desc: "always absent"},
		},
		Params: nameOnly("project"),
		Scan: func(rows *sql.Rows) (dbmeta.Database, error) {
			var v dbmeta.Database
			err := rows.Scan(&v.Name, &v.Owner, &v.Encoding, &v.Collate, &v.CType,
				&v.Access, &v.Tablespace, &v.Size, &v.Comment)
			return v, err
		},
	})

	// \df. A function, a table function and a procedure. A user defined aggregate
	// function is in Aggregates.
	dbmeta.Functions.Register(dbmeta.BigQuery, &dbmeta.Binding[dbmeta.Function]{
		Stmt:   routineStmt("r.routine_type <> 'AGGREGATE FUNCTION'"),
		Fields: routineFields("function, table function or procedure"),
		Params: schemaName("routine"),
		Scan:   scanFunction,
	})

	// \da. A user defined aggregate function, which CREATE AGGREGATE FUNCTION makes.
	dbmeta.Aggregates.Register(dbmeta.BigQuery, &dbmeta.Binding[dbmeta.Function]{
		Stmt:   routineStmt("r.routine_type = 'AGGREGATE FUNCTION'"),
		Fields: routineFields("always aggregate function"),
		Params: schemaName("aggregate"),
		Scan:   scanFunction,
	})

	// The parameters of a routine. A function has no mode on its parameters, and
	// every one of them is an input. A procedure has IN, OUT or INOUT. The row of
	// ordinal zero is the value that a function returns.
	dbmeta.RoutineParameters.Register(dbmeta.BigQuery, &dbmeta.Binding[dbmeta.RoutineParameter]{
		Stmt: dbmeta.Stmt{
			always("SELECT p.specific_catalog AS `catalog`"),
			always(", p.specific_schema AS `schema`"),
			always(", r.routine_name AS `routine`"),
			always(", p.specific_name AS `routine_id`"),
			always(", p.parameter_name AS `name`"),
			always(", p.ordinal_position AS `ordinal`"),
			always(", IF(p.is_result = 'YES', 'return', COALESCE(LOWER(p.parameter_mode), 'in')) AS `mode`"),
			always(", p.data_type AS `data_type`"),
			always(", p.parameter_default AS `default`"),
			always("FROM INFORMATION_SCHEMA.PARAMETERS p"),
			always("JOIN INFORMATION_SCHEMA.ROUTINES r ON r.specific_catalog = p.specific_catalog"),
			always("AND r.specific_schema = p.specific_schema AND r.specific_name = p.specific_name"),
			always("WHERE " + like("p.specific_schema", "@schema")),
			always("AND " + like("r.routine_name", "@parent")),
			always("AND " + like("p.parameter_name", "@name")),
			always("ORDER BY 2, 3, 6"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "routine"},
			{Name: "routine_id", Desc: "SPECIFIC_NAME, which is the name of the routine"},
			{Name: "name", Desc: "PARAMETER_NAME, which is absent for the returned value"},
			{Name: "ordinal", Desc: "ORDINAL_POSITION, one based, and zero for the value that a function returns"},
			{Name: "mode", Desc: "in, out or inout for a procedure. A function has no mode, so its parameters are in, and the value it returns is return"},
			{Name: "data_type", Desc: "DATA_TYPE as BigQuery writes it"},
			{Name: "default", Desc: "PARAMETER_DEFAULT, which is absent for every parameter that was seen"},
		},
		Params: schemaParentName("routine", "parameter"),
		Scan: func(rows *sql.Rows) (dbmeta.RoutineParameter, error) {
			var v dbmeta.RoutineParameter
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Routine, &v.RoutineID, &v.Name,
				&v.Ordinal, &v.Mode, &v.DataType, &v.Default)
			return v, err
		},
	})

	// \dd. The description of a table, a view and the rest of the relations, and of
	// a routine. A description of a column is in Columns. The description of a
	// dataset is in SCHEMATA_OPTIONS, which needs a permission on the project.
	dbmeta.Comments.Register(dbmeta.BigQuery, &dbmeta.Binding[dbmeta.Comment]{
		Stmt: dbmeta.Stmt{
			always("SELECT c.schema AS `schema`"),
			always(", c.name AS `name`"),
			always(", c.type AS `type`"),
			always(", c.comment AS `comment`"),
			always("FROM (SELECT o.table_schema AS schema, o.table_name AS name, " + tableType + " AS type"),
			always(", " + unquote("o.option_value") + " AS comment"),
			always("FROM INFORMATION_SCHEMA.TABLE_OPTIONS o JOIN INFORMATION_SCHEMA.TABLES t"),
			always("ON t.table_catalog = o.table_catalog AND t.table_schema = o.table_schema AND t.table_name = o.table_name"),
			always("WHERE o.option_name = 'description'"),
			always("UNION ALL SELECT o.specific_schema, o.specific_name, LOWER(r.routine_type)"),
			always(", " + unquote("o.option_value")),
			always("FROM INFORMATION_SCHEMA.ROUTINE_OPTIONS o JOIN INFORMATION_SCHEMA.ROUTINES r"),
			always("ON r.specific_catalog = o.specific_catalog AND r.specific_schema = o.specific_schema"),
			always("AND r.specific_name = o.specific_name WHERE o.option_name = 'description') c"),
			always("WHERE " + like("c.schema", "@schema")),
			always("AND " + like("c.name", "@name")),
			always("ORDER BY 1, 2"),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "the dataset"},
			{Name: "name"},
			{Name: "type", Desc: "the type of the object, as Tables spells it for a relation, and function, table function, aggregate function or procedure for a routine"},
			{Name: "comment", Desc: "the description option, as plain text"},
		},
		Params: schemaName("object"),
		Scan: func(rows *sql.Rows) (dbmeta.Comment, error) {
			var v dbmeta.Comment
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})

	// \dP. A table partitioned by a date, a timestamp or a number. BigQuery keeps
	// the clause in the DDL of the table, and INFORMATION_SCHEMA has no column for
	// it, so the expression is the text after PARTITION BY. Every BigQuery
	// partition covers a range, which is a day, an hour, a month, a year or an
	// interval of a number, so the strategy is range.
	dbmeta.PartitionedTables.Register(dbmeta.BigQuery, &dbmeta.Binding[dbmeta.PartitionedTable]{
		Stmt: dbmeta.Stmt{
			always("SELECT t.table_schema AS `schema`"),
			always(", t.table_name AS `name`"),
			always(", '' AS `owner`"),
			always(", " + tableType + " AS `type`"),
			always(", '' AS `parent`"),
			always(", 'range' AS `strategy`"),
			always(", REGEXP_EXTRACT(t.ddl, r'\\nPARTITION BY ([^\\n]+)') AS `expression`"),
			always(", o.description AS `comment`"),
			always(", CAST(NULL AS STRING) AS `table`"),
			always(", CAST(NULL AS STRING) AS `access_method`"),
			always(", m.size_bytes AS `direct_size`"),
			always(", m.size_bytes AS `total_size`"),
			always(", CAST(NULL AS BOOL) AS `parent_visible`"),
			always(", CAST(NULL AS BOOL) AS `table_visible`"),
			always("FROM INFORMATION_SCHEMA.TABLES t"),
			always("LEFT JOIN (SELECT table_catalog, table_schema, table_name"),
			always(", MAX(" + unquote("option_value") + ") AS description"),
			always("FROM INFORMATION_SCHEMA.TABLE_OPTIONS WHERE option_name = 'description'"),
			always("GROUP BY table_catalog, table_schema, table_name) o"),
			always("ON o.table_catalog = t.table_catalog AND o.table_schema = t.table_schema"),
			always("AND o.table_name = t.table_name"),
			always("LEFT JOIN __TABLES__ m ON m.project_id = t.table_catalog AND m.dataset_id = t.table_schema"),
			always("AND m.table_id = t.table_name"),
			always("WHERE REGEXP_CONTAINS(t.ddl, r'\\nPARTITION BY ')"),
			always("AND " + like("t.table_schema", "@schema")),
			always("AND " + like("t.table_name", "@name")),
			always("ORDER BY 1, 2"),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "the dataset"},
			{Name: "name"},
			{Name: "owner", Desc: "always empty: a BigQuery table has no owner"},
			{Name: "type", Desc: "the type of the table, as Tables spells it"},
			{Name: "parent", Desc: "always empty: a BigQuery partition is not a table of its own"},
			{Name: "strategy", Desc: "always range: a partition covers a range of dates, of times or of an integer"},
			{Name: "expression", Desc: "what follows PARTITION BY in the DDL of the table, such as sold_on or DATE_TRUNC(sold_on, MONTH)"},
			{Name: "comment", Desc: "the description of the table, as plain text. Absent when none is set"},
			{Name: "table", Desc: "always absent: BigQuery has no partitioned index"},
			{Name: "access_method", Desc: "always absent"},
			{Name: "direct_size", Desc: "SIZE_BYTES of the legacy __TABLES__ table, which is the same as total_size, because BigQuery has one level of partitions"},
			{Name: "total_size", Desc: "SIZE_BYTES of the legacy __TABLES__ table"},
			{Name: "parent_visible", Desc: "always absent: BigQuery has no search path"},
			{Name: "table_visible", Desc: "always absent, for the same reason"},
		},
		Params: schemaName("partitioned table"),
		Scan: func(rows *sql.Rows) (dbmeta.PartitionedTable, error) {
			var v dbmeta.PartitionedTable
			err := rows.Scan(&v.Schema, &v.Name, &v.Owner, &v.Type, &v.Parent, &v.Strategy,
				&v.Expression, &v.Comment, &v.Table, &v.AccessMethod, &v.DirectSize,
				&v.TotalSize, &v.ParentVisible, &v.TableVisible)
			return v, err
		},
	})

	// The partitions of a table. A partition has an id, such as 20260101 for a day,
	// and a table decorator names it, as sales$20260101. A table that is not
	// partitioned has one row with no id, and it is left out. An id that a row
	// cannot be put under is `__NULL__` or `__UNPARTITIONED__`, and BigQuery keeps those
	// rows in a partition of that name, so they are partitions here too.
	dbmeta.Partitions.Register(dbmeta.BigQuery, &dbmeta.Binding[dbmeta.Partition]{
		Stmt: dbmeta.Stmt{
			always("SELECT p.table_schema AS `schema`"),
			always(", p.table_name AS `table`"),
			always(", p.table_schema AS `partition_schema`"),
			always(", p.table_name || '$' || p.partition_id AS `partition`"),
			always(", 'partition' AS `type`"),
			always(", p.partition_id AS `bound`"),
			always(", CAST(NULL AS STRING) AS `constraint`"),
			always(", FALSE AS `partitioned`"),
			always(", FALSE AS `detach_pending`"),
			always(", CAST(NULL AS BOOL) AS `table_visible`"),
			always(", CAST(NULL AS BOOL) AS `partition_visible`"),
			always("FROM INFORMATION_SCHEMA.PARTITIONS p"),
			always("WHERE p.partition_id IS NOT NULL"),
			always("AND " + like("p.table_schema", "@schema")),
			always("AND " + like("p.table_name", "@parent")),
			always("AND " + like("p.table_schema", "@partition_schema")),
			always("AND " + like("p.table_name || '$' || p.partition_id", "@name")),
			always("ORDER BY 1, 2, 4"),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "the dataset of the table"},
			{Name: "table"},
			{Name: "partition_schema", Desc: "the dataset of the partition, which is the dataset of the table"},
			{Name: "partition", Desc: "the table, a dollar sign and the partition id, which is how a statement names a partition"},
			{Name: "type", Desc: "always partition"},
			{Name: "bound", Desc: "PARTITION_ID, such as 20260101 for a day, 202601 for a month, or __NULL__"},
			{Name: "constraint", Desc: "always absent: BigQuery keeps no constraint for a partition"},
			{Name: "partitioned", Desc: "always false: a partition has no partitions"},
			{Name: "detach_pending", Desc: "always false: BigQuery has no DETACH"},
			{Name: "table_visible", Desc: "always absent: BigQuery has no search path"},
			{Name: "partition_visible", Desc: "always absent, for the same reason"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "dataset name pattern of the table, empty for every dataset", Default: ""},
			{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
			{Name: "partition_schema", Desc: "dataset name pattern of the partition, empty for every dataset", Default: ""},
			{Name: "name", Desc: "partition name pattern, empty for every partition", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Partition, error) {
			var v dbmeta.Partition
			err := rows.Scan(&v.Schema, &v.Table, &v.PartitionSchema, &v.Partition, &v.Type,
				&v.Bound, &v.Constraint, &v.Partitioned, &v.DetachPending,
				&v.TableVisible, &v.PartitionVisible)
			return v, err
		},
	})
}
