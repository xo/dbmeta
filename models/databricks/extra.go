package databricks

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// Routines, comments, partitions, privileges, policies, collations, foreign
// servers, the current schema and the current user.

// routineParams is a relation that holds each parameter of a routine, and each
// column of the table that a table function returns, which Databricks keeps in
// ROUTINE_COLUMNS. The columns of the result follow the parameters, so a result
// column takes the position of the last parameter and counts on from there.
const routineParams = "(SELECT p.specific_catalog AS catalog, p.specific_schema AS schema, p.specific_name AS routine" +
	", p.parameter_name AS name, p.ordinal_position + 1 AS ordinal" +
	", IF(p.is_result = 'YES', 'return', LOWER(p.parameter_mode)) AS mode" +
	", p.full_data_type AS data_type, p.parameter_default AS default_value" +
	" FROM information_schema.parameters p" +
	" UNION ALL SELECT c.specific_catalog, c.specific_schema, c.specific_name, c.column_name" +
	", c.ordinal_position + 1 + COALESCE(n.total, 0), 'table', c.full_data_type, CAST(NULL AS STRING)" +
	" FROM information_schema.routine_columns c" +
	" LEFT JOIN (SELECT specific_catalog, specific_schema, specific_name, COUNT(*) AS total" +
	" FROM information_schema.parameters GROUP BY specific_catalog, specific_schema, specific_name) n" +
	" ON n.specific_catalog = c.specific_catalog AND n.specific_schema = c.specific_schema" +
	" AND n.specific_name = c.specific_name)"

func registerExtra() {
	// The principal that the connection signed in as. A user is its email, a
	// service principal is its application id, and both are what current_user()
	// reports.
	dbmeta.CurrentUser.Register(dbmeta.Databricks, &dbmeta.Binding[dbmeta.User]{
		Stmt: dbmeta.Stmt{
			always("SELECT current_user() AS `name`"),
			always(", NULLIF(session_user(), current_user()) AS `session`"),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "current_user(), which is the email of a user and the application id of a service principal, not the display name"},
			{Name: "session", Desc: "session_user() when it differs from the current user, and absent otherwise. It is absent for every session that was seen"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.User, error) {
			var v dbmeta.User
			err := rows.Scan(&v.Name, &v.Session)
			return v, err
		},
	})

	// The schema that an unqualified name resolves in, and the catalog it is in.
	dbmeta.CurrentSchema.Register(dbmeta.Databricks, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always("SELECT current_catalog() AS `catalog`"),
			always(", current_schema() AS `name`"),
			always(", '' AS `owner`"),
			always(", CAST(NULL AS STRING) AS `comment`"),
			always(", CAST(NULL AS STRING) AS `access`"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "the catalog of the session"},
			{Name: "name", Desc: "current_schema(), which is default when the connection names none"},
			{Name: "owner", Desc: "always empty, as it is in Schemas"},
			{Name: "comment", Desc: "always absent, as it is in Schemas"},
			{Name: "access", Desc: "always absent, as it is in Schemas"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment, &v.Access)
			return v, err
		},
	})

	// \df. A function, a table function and a procedure. Databricks does not
	// overload a function, so the name is the identity. A user defined
	// aggregate function does not exist.
	dbmeta.Functions.Register(dbmeta.Databricks, &dbmeta.Binding[dbmeta.Function]{
		Stmt: dbmeta.Stmt{
			always("SELECT r.routine_catalog AS `catalog`"),
			always(", r.routine_schema AS `schema`"),
			always(", r.routine_name AS `name`"),
			always(", r.specific_name AS `id`"),
			always(", LOWER(r.routine_type) AS `kind`"),
			always(", NULLIF(r.full_data_type, 'NULL') AS `result_type`"),
			always(", COALESCE(a.arg_types, '') AS `arg_types`"),
			always(", IF(LOWER(r.is_deterministic) IN ('true', 'yes'), 'immutable', '') AS `volatility`"),
			always(", '' AS `parallel`"),
			always(", r.routine_owner AS `owner`"),
			always(", LOWER(r.security_type) AS `security`"),
			always(", p.acl AS `access`"),
			always(", LOWER(COALESCE(r.external_language, r.routine_body)) AS `language`"),
			always(", r.routine_definition AS `source`"),
			always(", r.comment AS `comment`"),
			always(", CAST(NULL AS STRING) AS `definition`"),
			always(", FALSE AS `leakproof`"),
			always(", CAST(NULL AS STRING) AS `prosrc`"),
			always("FROM information_schema.routines r"),
			always("LEFT JOIN (SELECT specific_catalog, specific_schema, specific_name"),
			always(", concat_ws(', ', transform(array_sort(collect_list(struct(ordinal_position AS o, full_data_type AS t))), x -> x.t)) AS arg_types"),
			always("FROM information_schema.parameters WHERE is_result = 'NO'"),
			always("GROUP BY specific_catalog, specific_schema, specific_name) a"),
			always("ON a.specific_catalog = r.specific_catalog AND a.specific_schema = r.specific_schema"),
			always("AND a.specific_name = r.specific_name"),
			always("LEFT JOIN " + grants("routine_privileges", "specific_catalog", "specific_schema", "specific_name") + " p"),
			always("ON p.specific_catalog = r.specific_catalog AND p.specific_schema = r.specific_schema"),
			always("AND p.specific_name = r.specific_name"),
			always("WHERE " + notSystem("r.routine_schema")),
			always("AND " + like("r.routine_schema", "@schema")),
			always("AND " + like("r.routine_name", "@name")),
			always("ORDER BY 2, 3"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{Name: "id", Desc: "SPECIFIC_NAME, which is the name, because Databricks does not overload a function"},
			{Name: "kind", Desc: "function or procedure. A table function is a function whose result type is a table"},
			{Name: "result_type", Desc: "FULL_DATA_TYPE, the type of the value, such as bigint. For a table function it is the list of its columns, such as (book_id BIGINT, title STRING). Absent for a procedure"},
			{Name: "arg_types", Desc: "the types of the parameters in order, joined by a comma and a space, and empty for none. A procedure lists its OUT parameters too"},
			{Name: "volatility", Desc: "immutable when IS_DETERMINISTIC is true, and empty otherwise"},
			{Name: "parallel", Desc: "always empty: Databricks has no such property"},
			{Name: "owner", Desc: "ROUTINE_OWNER"},
			{Name: "security", Desc: "SECURITY_TYPE in lower case, which is definer or invoker"},
			{Name: "access", Desc: "the grants on the routine, a line for each, as grantee=PRIVILEGE/grantor. Absent when the routine has no grant"},
			{Name: "language", Desc: "EXTERNAL_LANGUAGE in lower case, such as python, and ROUTINE_BODY in lower case, which is sql, where there is none"},
			{Name: "source", Desc: "ROUTINE_DEFINITION, which is the body of the routine and not the whole statement"},
			{Name: "comment", Desc: "the comment of the routine. Absent when none is set"},
			{Name: "definition", Desc: "always absent: INFORMATION_SCHEMA keeps the body and not the CREATE statement"},
			{Name: "leakproof", Desc: "always false: Databricks has no such property"},
			{Name: "prosrc", Desc: "always absent: Databricks has no such column"},
		},
		Params: schemaName("routine"),
		Scan: func(rows *sql.Rows) (dbmeta.Function, error) {
			var v dbmeta.Function
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.ID, &v.Kind, &v.ResultType,
				&v.ArgTypes, &v.Volatility, &v.Parallel, &v.Owner, dbmeta.NullAsEmpty(&v.Security),
				&v.Access, dbmeta.NullAsEmpty(&v.Language), &v.Source, &v.Comment,
				&v.Definition, &v.Leakproof, &v.Prosrc)
			return v, err
		},
	})

	// The parameters of a routine, and the columns of a table function.
	dbmeta.RoutineParameters.Register(dbmeta.Databricks, &dbmeta.Binding[dbmeta.RoutineParameter]{
		Stmt: dbmeta.Stmt{
			always("SELECT p.catalog AS `catalog`"),
			always(", p.schema AS `schema`"),
			always(", p.routine AS `routine`"),
			always(", p.routine AS `routine_id`"),
			always(", p.name AS `name`"),
			always(", p.ordinal AS `ordinal`"),
			always(", p.mode AS `mode`"),
			always(", p.data_type AS `data_type`"),
			always(", p.default_value AS `default`"),
			always("FROM " + routineParams + " p"),
			always("WHERE " + notSystem("p.schema")),
			always("AND " + like("p.schema", "@schema")),
			always("AND " + like("p.routine", "@parent")),
			always("AND " + like("p.name", "@name")),
			always("ORDER BY 2, 3, 6"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "routine"},
			{Name: "routine_id", Desc: "SPECIFIC_NAME, which is the name of the routine"},
			{Name: "name", Desc: "PARAMETER_NAME, or the name of the column for a table function"},
			{Name: "ordinal", Desc: "ORDINAL_POSITION plus one, because PARAMETERS counts from zero. The columns of a table function follow the parameters"},
			{Name: "mode", Desc: "in or out, from PARAMETER_MODE, and table for a column of the result of a table function"},
			{Name: "data_type", Desc: "FULL_DATA_TYPE, as a statement spells the type"},
			{Name: "default", Desc: "PARAMETER_DEFAULT, the text of the default, and absent when there is none"},
		},
		Params: schemaParentName("routine", "parameter"),
		Scan: func(rows *sql.Rows) (dbmeta.RoutineParameter, error) {
			var v dbmeta.RoutineParameter
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Routine, &v.RoutineID, &v.Name,
				&v.Ordinal, &v.Mode, &v.DataType, &v.Default)
			return v, err
		},
	})

	// \dd. The comment of a relation, of a schema, of a routine and of a volume.
	// The comment of a column is in Columns.
	dbmeta.Comments.Register(dbmeta.Databricks, &dbmeta.Binding[dbmeta.Comment]{
		Stmt: dbmeta.Stmt{
			always("SELECT c.schema AS `schema`"),
			always(", c.name AS `name`"),
			always(", c.type AS `type`"),
			always(", c.comment AS `comment`"),
			always("FROM (SELECT t.table_schema AS schema, t.table_name AS name, " + tableType + " AS type, t.comment AS comment"),
			always("FROM information_schema.tables t"),
			always("UNION ALL SELECT schema_name, schema_name, 'schema', comment FROM information_schema.schemata"),
			always("UNION ALL SELECT routine_schema, routine_name, LOWER(routine_type), comment FROM information_schema.routines"),
			always("UNION ALL SELECT volume_schema, volume_name, 'volume', comment FROM information_schema.volumes) c"),
			always("WHERE c.comment IS NOT NULL"),
			always("AND " + notSystem("c.schema")),
			always("AND " + like("c.schema", "@schema")),
			always("AND " + like("c.name", "@name")),
			always("ORDER BY 1, 2"),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "the schema, and for a schema the name of the schema"},
			{Name: "name"},
			{Name: "type", Desc: "the type of the object, as Tables spells it for a relation, and schema, function, procedure or volume"},
			{Name: "comment"},
		},
		Params: schemaName("object"),
		Scan: func(rows *sql.Rows) (dbmeta.Comment, error) {
			var v dbmeta.Comment
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})

	// \dP. A table that is partitioned by the value of a column. COLUMNS gives
	// the position of each partition column. A table clustered by liquid
	// clustering has no partition column, and its clustering columns are only in
	// DESCRIBE DETAIL.
	dbmeta.PartitionedTables.Register(dbmeta.Databricks, &dbmeta.Binding[dbmeta.PartitionedTable]{
		Stmt: dbmeta.Stmt{
			always("SELECT c.table_schema AS `schema`"),
			always(", c.table_name AS `name`"),
			always(", t.table_owner AS `owner`"),
			always(", " + tableType + " AS `type`"),
			always(", '' AS `parent`"),
			always(", 'list' AS `strategy`"),
			always(", c.column_name AS `expression`"),
			always(", c.comment AS `comment`"),
			always(", CAST(NULL AS STRING) AS `table`"),
			always(", CAST(NULL AS STRING) AS `access_method`"),
			always(", CAST(NULL AS BIGINT) AS `direct_size`"),
			always(", CAST(NULL AS BIGINT) AS `total_size`"),
			always(", CAST(NULL AS BOOLEAN) AS `parent_visible`"),
			always(", CAST(NULL AS BOOLEAN) AS `table_visible`"),
			always("FROM information_schema.columns c JOIN information_schema.tables t"),
			always("ON t.table_catalog = c.table_catalog AND t.table_schema = c.table_schema"),
			always("AND t.table_name = c.table_name"),
			always("WHERE c.partition_index IS NOT NULL"),
			always("AND " + notSystem("c.table_schema")),
			always("AND " + like("c.table_schema", "@schema")),
			always("AND " + like("c.table_name", "@name")),
			always("ORDER BY 1, 2, c.partition_index"),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{Name: "owner", Desc: "TABLE_OWNER"},
			{Name: "type", Desc: "the type of the table, as Tables spells it"},
			{Name: "parent", Desc: "always empty: a partition of Delta is a directory of files and not a table"},
			{Name: "strategy", Desc: "always list: a table is partitioned by the value of a column"},
			{Name: "expression", Desc: "the partition column. A table partitioned by two columns has two rows, in the order of PARTITION_INDEX. A table with liquid clustering has none"},
			{Name: "comment", Desc: "the comment of the partition column. Absent when none is set"},
			{Name: "table", Desc: "always absent: Databricks has no partitioned index"},
			{Name: "access_method", Desc: "always absent"},
			{Name: "direct_size", Desc: "always absent: only DESCRIBE DETAIL reports the size"},
			{Name: "total_size", Desc: "always absent, for the same reason"},
			{Name: "parent_visible", Desc: "always absent: Databricks has no search path"},
			{Name: "table_visible", Desc: "always absent, for the same reason"},
		},
		Params: schemaName("partitioned table"),
		Scan: func(rows *sql.Rows) (dbmeta.PartitionedTable, error) {
			var v dbmeta.PartitionedTable
			err := rows.Scan(&v.Schema, &v.Name, dbmeta.NullAsEmpty(&v.Owner), &v.Type, &v.Parent, &v.Strategy,
				&v.Expression, &v.Comment, &v.Table, &v.AccessMethod, &v.DirectSize,
				&v.TotalSize, &v.ParentVisible, &v.TableVisible)
			return v, err
		},
	})

	// \dp. The grants on a table, a view or a materialized view, and the row filter
	// and the column masks of the table.
	dbmeta.Privileges.Register(dbmeta.Databricks, &dbmeta.Binding[dbmeta.Privilege]{
		Stmt: dbmeta.Stmt{
			always("SELECT t.table_schema AS `schema`"),
			always(", t.table_name AS `name`"),
			always(", " + tableType + " AS `type`"),
			always(", a.acl AS `access`"),
			always(", CAST(NULL AS STRING) AS `column_access`"),
			always(", NULLIF(concat_ws(chr(10), IF(f.filter_name IS NULL, NULL,"),
			always("'row filter ' || f.filter_name || ' on (' || f.target_columns || ')'), m.masks), '') AS `policies`"),
			always("FROM information_schema.tables t"),
			always("LEFT JOIN " + grants("table_privileges", "table_catalog", "table_schema", "table_name") + " a"),
			always("ON a.table_catalog = t.table_catalog AND a.table_schema = t.table_schema AND a.table_name = t.table_name"),
			always("LEFT JOIN information_schema.row_filters f"),
			always("ON f.table_catalog = t.table_catalog AND f.table_schema = t.table_schema AND f.table_name = t.table_name"),
			always("LEFT JOIN (SELECT table_catalog, table_schema, table_name"),
			always(", concat_ws(chr(10), sort_array(collect_list('column mask ' || mask_name || ' on ' || column_name))) AS masks"),
			always("FROM information_schema.column_masks GROUP BY table_catalog, table_schema, table_name) m"),
			always("ON m.table_catalog = t.table_catalog AND m.table_schema = t.table_schema AND m.table_name = t.table_name"),
			always("WHERE " + notSystem("t.table_schema")),
			always("AND " + like("t.table_schema", "@schema")),
			always("AND " + like("t.table_name", "@name")),
			always("ORDER BY 1, 2"),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{Name: "type", Desc: "the type of the relation, as Tables spells it"},
			{Name: "access", Desc: "the grants on the relation, a line for each, as grantee=PRIVILEGE/grantor. A grant that the relation inherits from the schema or the catalog says so. Absent when there is none, which means the owner alone holds it. A grantee is a name, a group or an application id"},
			{Name: "column_access", Desc: "always absent: Databricks has no grant on a column"},
			{Name: "policies", Desc: "the row filter, as row filter name on (columns), and a line for each column mask, as column mask name on column. Absent when there is none"},
		},
		Params: schemaName("relation"),
		Scan: func(rows *sql.Rows) (dbmeta.Privilege, error) {
			var v dbmeta.Privilege
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Access, &v.ColumnAccess, &v.Policies)
			return v, err
		},
	})

	// \d+ with a row security policy. A row filter and a column mask are the two
	// policies that Unity Catalog has. A row filter hides rows from every
	// statement that reads the table, so its command is all, and a column mask
	// changes the value that a read returns, so its command is select.
	dbmeta.Policies.Register(dbmeta.Databricks, &dbmeta.Binding[dbmeta.Policy]{
		Stmt: dbmeta.Stmt{
			always("SELECT p.schema AS `schema`"),
			always(", p.tbl AS `table`"),
			always(", p.name AS `name`"),
			always(", p.command AS `command`"),
			always(", TRUE AS `permissive`"),
			always(", CAST(NULL AS STRING) AS `roles`"),
			always(", p.using_text AS `using`"),
			always(", CAST(NULL AS STRING) AS `with_check`"),
			always(", CAST(NULL AS STRING) AS `comment`"),
			always(", CAST(NULL AS BOOLEAN) AS `enabled`"),
			always("FROM (SELECT table_schema AS schema, table_name AS tbl, filter_name AS name, 'all' AS command"),
			always(", filter_name || ' (' || target_columns || ')' AS using_text"),
			always("FROM information_schema.row_filters"),
			always("UNION ALL SELECT table_schema, table_name, mask_name, 'select'"),
			always(", mask_name || ' (' || column_name || IF(using_columns IS NULL OR using_columns = '', '', ', ' || using_columns) || ')'"),
			always("FROM information_schema.column_masks) p"),
			always("WHERE " + notSystem("p.schema")),
			always("AND " + like("p.schema", "@schema")),
			always("AND " + like("p.tbl", "@parent")),
			always("AND " + like("p.name", "@name")),
			always("ORDER BY 1, 2, 3"),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"},
			{Name: "name", Desc: "the full name of the function, such as workspace.dbmeta.region_filter, because a row filter and a column mask have no name of their own"},
			{Name: "command", Desc: "all for a row filter and select for a column mask"},
			{Name: "permissive", Desc: "always true: Databricks has no restrictive policy"},
			{Name: "roles", Desc: "always absent: a policy applies to every principal, and the function decides who is excepted"},
			{Name: "using", Desc: "the function and the columns it takes, such as workspace.dbmeta.region_filter (region). For a column mask it is the function, the masked column and the USING columns"},
			{Name: "with_check", Desc: "always absent: Databricks has no check on a write"},
			{Name: "comment", Desc: "always absent"},
			{Name: "enabled", Desc: "always absent: a row filter and a column mask have no switch"},
		},
		Params: schemaParentName("table", "policy"),
		Scan: func(rows *sql.Rows) (dbmeta.Policy, error) {
			var v dbmeta.Policy
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Command, &v.Permissive,
				&v.Roles, &v.Using, &v.WithCheck, &v.Comment, &v.Enabled)
			return v, err
		},
	})

	// \dO. The collations that collations() lists, which are the built in ones
	// and the ones that ICU gives.
	dbmeta.Collations.Register(dbmeta.Databricks, &dbmeta.Binding[dbmeta.Collation]{
		Stmt: dbmeta.Stmt{
			always("SELECT LOWER(c.schema) AS `schema`"),
			always(", c.name AS `name`"),
			always(", IF(c.icu_version IS NULL, 'builtin', 'icu') AS `provider`"),
			always(", CAST(NULL AS STRING) AS `collate`"),
			always(", CAST(NULL AS STRING) AS `ctype`"),
			always(", NULLIF(concat_ws(', ', c.language, c.country), '') AS `locale`"),
			always(", c.case_sensitivity = 'CASE_SENSITIVE' AND c.accent_sensitivity = 'ACCENT_SENSITIVE'"),
			always("AND c.pad_attribute = 'NO_PAD' AS `deterministic`"),
			always(", LOWER(concat_ws(', ', c.accent_sensitivity, c.case_sensitivity, c.pad_attribute)) AS `comment`"),
			always(", CAST(NULL AS STRING) AS `rules`"),
			always("FROM collations() c"),
			always("WHERE " + like("LOWER(c.schema)", "@schema")),
			always("AND " + like("c.name", "@name")),
			always("ORDER BY 1, 2"),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "SCHEMA in lower case, which is builtin for every collation that was seen"},
			{Name: "name", Desc: "NAME as Databricks writes it, such as UTF8_LCASE, UNICODE_CI or af_CI, which is the code of a language and its options"},
			{Name: "provider", Desc: "builtin for a collation with no ICU version, and icu for the rest"},
			{Name: "collate", Desc: "always absent: a Databricks collation is named by its language, so the language is in locale"},
			{Name: "ctype", Desc: "always absent, for the same reason"},
			{Name: "locale", Desc: "LANGUAGE and COUNTRY joined by a comma and a space, such as Afrikaans. Absent for a collation that has neither"},
			{Name: "deterministic", Desc: "true when the collation is case sensitive, accent sensitive and has no padding rule, because then two strings are equal only if their bytes are"},
			{Name: "comment", Desc: "ACCENT_SENSITIVITY, CASE_SENSITIVITY and PAD_ATTRIBUTE in lower case, joined by a comma and a space"},
			{Name: "rules", Desc: "always absent: Databricks has no custom collation rules"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "collation schema pattern, empty for every schema", Default: ""},
			{Name: "name", Desc: "collation name pattern, which is case sensitive, and empty for every collation", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Collation, error) {
			var v dbmeta.Collation
			err := rows.Scan(&v.Schema, &v.Name, &v.Provider, &v.Collate, &v.CType, &v.Locale,
				&v.Deterministic, &v.Comment, &v.Rules)
			return v, err
		},
	})

	// \des. A connection of Lakehouse Federation, which CREATE SERVER also makes.
	// It is a connection to another database, such as PostgreSQL or Snowflake.
	dbmeta.ForeignServers.Register(dbmeta.Databricks, &dbmeta.Binding[dbmeta.ForeignServer]{
		Stmt: dbmeta.Stmt{
			always("SELECT c.connection_name AS `name`"),
			always(", c.connection_owner AS `owner`"),
			always(", c.connection_type AS `wrapper`"),
			always(", CAST(NULL AS STRING) AS `type`"),
			always(", CAST(NULL AS STRING) AS `version`"),
			always(", CAST(NULL AS STRING) AS `access`"),
			always(", CAST(c.connection_options AS STRING) AS `options`"),
			always(", c.comment AS `comment`"),
			always("FROM system.information_schema.connections c"),
			always("WHERE " + like("c.connection_name", "@name")),
			always("ORDER BY 1"),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "CONNECTION_NAME"},
			{Name: "owner", Desc: "CONNECTION_OWNER"},
			{Name: "wrapper", Desc: "CONNECTION_TYPE, such as POSTGRESQL, which is the kind of database the connection reaches"},
			{Name: "type", Desc: "always absent: a connection has no second type"},
			{Name: "version", Desc: "always absent: a connection has no version"},
			{Name: "access", Desc: "always absent: CONNECTION_PRIVILEGES lists the grants, and the model does not read them"},
			{Name: "options", Desc: "CONNECTION_OPTIONS as text. Databricks hides a secret in it"},
			{Name: "comment", Desc: "the comment of the connection. Absent when none is set"},
		},
		Params: nameOnly("connection"),
		Scan: func(rows *sql.Rows) (dbmeta.ForeignServer, error) {
			var v dbmeta.ForeignServer
			err := rows.Scan(&v.Name, dbmeta.NullAsEmpty(&v.Owner), dbmeta.NullAsEmpty(&v.Wrapper), &v.Type,
				&v.Version, &v.Access, &v.Options, &v.Comment)
			return v, err
		},
	})
}
