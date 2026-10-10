package spanner

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// Routines, settings, change streams and locality groups.

// streamName is the name of a change stream as a statement writes it, which is
// the name alone in the default schema and schema.name in another. s is the
// alias of the relation, and the columns are those of CHANGE_STREAMS and
// CHANGE_STREAM_TABLES.
func streamName(s string) string {
	return `IF(` + s + `.change_stream_schema = '', ` + s + `.change_stream_name, ` +
		s + `.change_stream_schema || '.' || ` + s + `.change_stream_name)`
}

// streamFlag is true for a change stream that has the option name set to true,
// and false for one that does not set it. o is the aggregate of the options.
func streamFlag(o, name string) string {
	return `COALESCE(` + o + `.` + name + `, FALSE)`
}

func registerExtra() {
	// The IAM principal that the connection signed in as, such as
	// name@project.iam.gserviceaccount.com. Cloud Spanner answers it. Spanner
	// Omni, started with no authentication, refuses it with "the user name is
	// unknown", so on Omni this query is an error and not an answer. The database
	// role that a session names is not readable, so session is absent. See D219.
	dbmeta.CurrentUser.Register(dbmeta.Spanner, &dbmeta.Binding[dbmeta.User]{
		Stmt: dbmeta.Stmt{
			always("SELECT SESSION_USER() AS `name`"),
			always(", CAST(NULL AS STRING) AS `session`"),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "SESSION_USER(), the IAM principal. Spanner Omni with no authentication refuses it"},
			{Name: "session", Desc: "always absent: the database role that a session names with role= is not readable from SQL"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.User, error) {
			var v dbmeta.User
			err := rows.Scan(&v.Name, &v.Session)
			return v, err
		},
	})

	// \df. A routine is a function written with CREATE FUNCTION, which Spanner
	// allows in a named schema only, or the table function that Spanner makes for
	// a change stream, which is named READ_ and the stream. The name is the id,
	// because Spanner does not overload a function.
	dbmeta.Functions.Register(dbmeta.Spanner, &dbmeta.Binding[dbmeta.Function]{
		Stmt: dbmeta.Stmt{
			always("SELECT r.routine_catalog AS `catalog`"),
			always(", r.routine_schema AS `schema`"),
			always(", r.routine_name AS `name`"),
			always(", r.specific_name AS `id`"),
			always(", LOWER(r.routine_type) AS `kind`"),
			always(", COALESCE(r.spanner_type, r.data_type) AS `result_type`"),
			always(", COALESCE(a.arg_types, '') AS `arg_types`"),
			always(", '' AS `volatility`"),
			always(", '' AS `parallel`"),
			always(", CAST(NULL AS STRING) AS `owner`"),
			always(", LOWER(r.security_type) AS `security`"),
			always(", CAST(NULL AS STRING) AS `access`"),
			always(", LOWER(r.routine_body) AS `language`"),
			always(", r.routine_definition AS `source`"),
			always(", CAST(NULL AS STRING) AS `comment`"),
			always(", CAST(NULL AS STRING) AS `definition`"),
			always(", FALSE AS `leakproof`"),
			always(", CAST(NULL AS STRING) AS `prosrc`"),
			always("FROM information_schema.routines r"),
			always("LEFT JOIN (SELECT specific_catalog, specific_schema, specific_name"),
			always(", STRING_AGG(spanner_type, ', ' ORDER BY ordinal_position) AS arg_types"),
			always("FROM information_schema.parameters GROUP BY specific_catalog, specific_schema, specific_name) a"),
			always("ON a.specific_catalog = r.specific_catalog AND a.specific_schema = r.specific_schema"),
			always("AND a.specific_name = r.specific_name"),
			always("WHERE " + notSystem("r.routine_schema")),
			always("AND " + like("r.routine_schema", "@schema")),
			always("AND " + like("r.routine_name", "@name")),
			always("ORDER BY 2, 3"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema", Desc: "empty for the default schema, where Spanner keeps the table function of a change stream"},
			{Name: "name"},
			{Name: "id", Desc: "SPECIFIC_NAME, which is the name, because Spanner does not overload a function"},
			{Name: "kind", Desc: "function for a function written with CREATE FUNCTION, and table function for the function of a change stream"},
			{Name: "result_type", Desc: "SPANNER_TYPE, and DATA_TYPE where that is absent, which is the table type of a change stream function"},
			{Name: "arg_types", Desc: "the Spanner types of the parameters in order, joined by a comma and a space, and empty for none"},
			{Name: "volatility", Desc: "always empty: INFORMATION_SCHEMA does not record it"},
			{Name: "parallel", Desc: "always empty: Spanner has no such property"},
			{Name: "owner", Desc: "always absent: a Spanner function has no owner"},
			{Name: "security", Desc: "SECURITY_TYPE in lower case, invoker or definer"},
			{Name: "access", Desc: "always absent: Privileges reads the grants on a function"},
			{Name: "language", Desc: "ROUTINE_BODY in lower case, which is sql"},
			{Name: "source", Desc: "ROUTINE_DEFINITION, which is the expression of the function and not the whole statement"},
			{Name: "comment", Desc: "always absent"},
			{Name: "definition", Desc: "always absent: INFORMATION_SCHEMA keeps the expression, which is source"},
			{Name: "leakproof", Desc: "always false: Spanner has no such property"},
			{Name: "prosrc", Desc: "always absent: Spanner has no such column"},
		},
		Params: schemaNameSystem("function"),
		Scan: func(rows *sql.Rows) (dbmeta.Function, error) {
			var v dbmeta.Function
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.ID, &v.Kind, &v.ResultType,
				&v.ArgTypes, &v.Volatility, &v.Parallel, &v.Owner, dbmeta.NullAsEmpty(&v.Security),
				&v.Access, dbmeta.NullAsEmpty(&v.Language), &v.Source, &v.Comment,
				&v.Definition, &v.Leakproof, &v.Prosrc)
			return v, err
		},
	})

	dbmeta.RoutineParameters.Register(dbmeta.Spanner, &dbmeta.Binding[dbmeta.RoutineParameter]{
		Stmt: dbmeta.Stmt{
			always("SELECT p.specific_catalog AS `catalog`"),
			always(", p.specific_schema AS `schema`"),
			always(", r.routine_name AS `routine`"),
			always(", p.specific_name AS `routine_id`"),
			always(", p.parameter_name AS `name`"),
			always(", p.ordinal_position AS `ordinal`"),
			always(", 'in' AS `mode`"),
			always(", COALESCE(p.spanner_type, p.data_type) AS `data_type`"),
			always(", p.parameter_default AS `default`"),
			always("FROM information_schema.parameters p"),
			always("JOIN information_schema.routines r ON r.specific_catalog = p.specific_catalog"),
			always("AND r.specific_schema = p.specific_schema AND r.specific_name = p.specific_name"),
			always("WHERE " + notSystem("p.specific_schema")),
			always("AND " + like("p.specific_schema", "@schema")),
			always("AND " + like("r.routine_name", "@parent")),
			always("AND " + like("p.parameter_name", "@name")),
			always("ORDER BY 2, 3, 6"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "routine"},
			{Name: "routine_id", Desc: "SPECIFIC_NAME, which is the name of the routine"},
			{Name: "name"},
			{Name: "ordinal", Desc: "ORDINAL_POSITION, one based"},
			{Name: "mode", Desc: "always in: a Spanner function has input parameters only"},
			{Name: "data_type", Desc: "SPANNER_TYPE, and DATA_TYPE where that is absent"},
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

	// \dconfig. The options of the database, such as optimizer_version. An option
	// that was never set has no row, so its default is not here.
	dbmeta.Settings.Register(dbmeta.Spanner, &dbmeta.Binding[dbmeta.Setting]{
		Stmt: dbmeta.Stmt{
			always("SELECT o.option_name AS `name`"),
			always(", o.option_value AS `value`"),
			always(", o.option_type AS `type`"),
			always(", CAST(NULL AS STRING) AS `context`"),
			always(", CAST(NULL AS STRING) AS `access`"),
			always(", CAST(NULL AS STRING) AS `display`"),
			always("FROM information_schema.database_options o"),
			always("WHERE " + like("o.option_name", "@name")),
			always("ORDER BY 1"),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "value", Desc: "OPTION_VALUE as text, such as TRUE or 8"},
			{Name: "type", Desc: "OPTION_TYPE, which is BOOL, INT64 or STRING"},
			{Name: "context", Desc: "always absent: every option is a property of the database, and ALTER DATABASE sets it"},
			{Name: "access", Desc: "always absent"},
			{Name: "display", Desc: "always absent: Spanner shows a value in one form, which is value"},
		},
		Params: nameOnly("option"),
		Scan: func(rows *sql.Rows) (dbmeta.Setting, error) {
			var v dbmeta.Setting
			err := rows.Scan(&v.Name, &v.Value, &v.Type, &v.Context, &v.Access, &v.Display)
			return v, err
		},
	})

	// \dRp. A change stream is the nearest thing Spanner has to a publication:
	// it names the tables and the columns whose changes it reports, and options
	// choose which kinds of change. The name is the one a statement writes.
	dbmeta.Publications.Register(dbmeta.Spanner, &dbmeta.Binding[dbmeta.Publication]{
		Stmt: dbmeta.Stmt{
			always("SELECT " + streamName("s") + " AS `name`"),
			always(", '' AS `owner`"),
			always(", s.`all` AS `all_tables`"),
			always(", NOT " + streamFlag("o", "exclude_insert") + " AS `insert`"),
			always(", NOT " + streamFlag("o", "exclude_update") + " AS `update`"),
			always(", NOT " + streamFlag("o", "exclude_delete") + " AS `delete`"),
			always(", FALSE AS `truncate`"),
			always(", FALSE AS `via_root`"),
			always(", CAST(NULL AS STRING) AS `comment`"),
			always(", CAST(NULL AS STRING) AS `generated_columns`"),
			always("FROM information_schema.change_streams s"),
			always("LEFT JOIN (SELECT change_stream_schema, change_stream_name"),
			always(", LOGICAL_OR(option_name = 'exclude_insert' AND UPPER(option_value) = 'TRUE') AS exclude_insert"),
			always(", LOGICAL_OR(option_name = 'exclude_update' AND UPPER(option_value) = 'TRUE') AS exclude_update"),
			always(", LOGICAL_OR(option_name = 'exclude_delete' AND UPPER(option_value) = 'TRUE') AS exclude_delete"),
			always("FROM information_schema.change_stream_options GROUP BY change_stream_schema, change_stream_name) o"),
			always("ON o.change_stream_schema = s.change_stream_schema AND o.change_stream_name = s.change_stream_name"),
			always("WHERE " + like("s.change_stream_name", "@name")),
			always("ORDER BY 1"),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the name of a change stream as a statement writes it, which is schema.name outside the default schema"},
			{Name: "owner", Desc: "always empty: a change stream has no owner"},
			{Name: "all_tables", Desc: "the ALL column, true for a stream made FOR ALL"},
			{Name: "insert", Desc: "false when the option exclude_insert is true"},
			{Name: "update", Desc: "false when the option exclude_update is true"},
			{Name: "delete", Desc: "false when the option exclude_delete is true"},
			{Name: "truncate", Desc: "always false: Spanner has no TRUNCATE"},
			{Name: "via_root", Desc: "always false: Spanner has no partitioned table"},
			{Name: "comment", Desc: "always absent"},
			{Name: "generated_columns", Desc: "always absent: Spanner has no such choice"},
		},
		Params: nameOnly("change stream"),
		Scan: func(rows *sql.Rows) (dbmeta.Publication, error) {
			var v dbmeta.Publication
			err := rows.Scan(&v.Name, &v.Owner, &v.AllTables, &v.Insert, &v.Update, &v.Delete,
				&v.Truncate, &v.ViaRoot, &v.Comment, &v.GeneratedColumns)
			return v, err
		},
	})

	// \dRp+. One row for each table a change stream names. A stream made FOR ALL
	// has no row here, because it names no table, and its all_tables is true.
	dbmeta.PublicationTables.Register(dbmeta.Spanner, &dbmeta.Binding[dbmeta.PublicationTable]{
		Stmt: dbmeta.Stmt{
			always("SELECT " + streamName("t") + " AS `publication`"),
			always(", t.table_schema AS `schema`"),
			always(", t.table_name AS `name`"),
			always(", IF(t.all_columns, '', '(' || COALESCE(c.columns, '') || ')') AS `columns`"),
			always(", CAST(NULL AS STRING) AS `where`"),
			always(", 'table' AS `via`"),
			always("FROM information_schema.change_stream_tables t"),
			always("LEFT JOIN (SELECT change_stream_catalog, change_stream_schema, change_stream_name"),
			always(", table_catalog, table_schema, table_name, STRING_AGG(column_name, ', ' ORDER BY column_name) AS columns"),
			always("FROM information_schema.change_stream_columns"),
			always("GROUP BY change_stream_catalog, change_stream_schema, change_stream_name"),
			always(", table_catalog, table_schema, table_name) c"),
			always("ON c.change_stream_catalog = t.change_stream_catalog AND c.change_stream_schema = t.change_stream_schema"),
			always("AND c.change_stream_name = t.change_stream_name AND c.table_catalog = t.table_catalog"),
			always("AND c.table_schema = t.table_schema AND c.table_name = t.table_name"),
			always("WHERE " + like("t.change_stream_name", "@name")),
			always("AND " + like("t.table_name", "@table")),
			always("ORDER BY 1, 2, 3"),
		},
		Fields: []dbmeta.Field{
			{Name: "publication", Desc: "the name of the change stream, as Publications gives it"},
			{Name: "schema", Desc: "the schema of the table, which is empty for the default schema"},
			{Name: "name", Desc: "the table"},
			{Name: "columns", Desc: "empty when the stream tracks every column. For a stream that names columns, the column names in parentheses, as the statement writes them, so () is a stream that tracks the keys of the table and no other column"},
			{Name: "where", Desc: "always absent: a change stream has no row filter"},
			{Name: "via", Desc: "always table: a change stream names a table and never a schema"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "change stream name pattern, empty for every stream", Default: ""},
			{Name: "table", Desc: "table name pattern, empty for every table", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.PublicationTable, error) {
			var v dbmeta.PublicationTable
			err := rows.Scan(&v.Publication, &v.Schema, &v.Name, &v.Columns, &v.Where, &v.Via)
			return v, err
		},
	})

	// \db. A locality group is where Spanner places the data of a column or a
	// table: on solid state or on disk. That is what a tablespace is for.
	dbmeta.Tablespaces.Register(dbmeta.Spanner, &dbmeta.Binding[dbmeta.Tablespace]{
		Stmt: dbmeta.Stmt{
			always("SELECT g.locality_group_name AS `name`"),
			always(", CAST(NULL AS STRING) AS `owner`"),
			always(", CAST(NULL AS STRING) AS `location`"),
			always(", NULLIF(ARRAY_TO_STRING(ARRAY_AGG(g.option_name || '=' || g.option_value"),
			always(" IGNORE NULLS ORDER BY g.option_name), ', '), '') AS `options`"),
			always(", CAST(NULL AS STRING) AS `size`"),
			always(", CAST(NULL AS STRING) AS `access`"),
			always(", CAST(NULL AS STRING) AS `comment`"),
			always("FROM information_schema.locality_group_options g"),
			always("WHERE " + like("g.locality_group_name", "@name")),
			always("GROUP BY g.locality_group_name"),
			always("ORDER BY 1"),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the name of a locality group, and default for the one every column is in until it is moved"},
			{Name: "owner", Desc: "always absent: a locality group has no owner"},
			{Name: "location", Desc: "always absent: a locality group names a kind of storage and no path"},
			{Name: "options", Desc: "name=value joined by a comma and a space, such as storage=hdd. An option that is not set is left out, so the default group has none"},
			{Name: "size", Desc: "always absent: SPANNER_SYS.TABLE_SIZES_STATS_PER_LOCALITY_GROUP_1HOUR holds an hourly size, which D216 leaves out until it is measured"},
			{Name: "access", Desc: "always absent"},
			{Name: "comment", Desc: "always absent"},
		},
		Params: nameOnly("locality group"),
		Scan: func(rows *sql.Rows) (dbmeta.Tablespace, error) {
			var v dbmeta.Tablespace
			err := rows.Scan(&v.Name, &v.Owner, &v.Location, &v.Options, &v.Size, &v.Access, &v.Comment)
			return v, err
		},
	})
}
