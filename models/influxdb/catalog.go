package influxdb

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// notSystem hides the two schemas InfluxDB 3 keeps for itself: system, which
// holds its caches, its Parquet files and its processing engine, and
// information_schema.
func notSystem(col string) string {
	return `(@with_system OR ` + col + ` NOT IN ('system', 'information_schema'))`
}

func always(query string) dbmeta.Choice { return dbmeta.Choice{{Query: query}} }

func schemaNameSystem(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
		{Name: "with_system", Desc: "include the objects InfluxDB 3 keeps for itself", Default: false},
	}
}

// register registers every statement the model answers.
func register() {
	registerRelations()
	registerRoutines()
	registerServer()
	registerTriggers()
}

// tableType is the word for a table's kind.
const tableType = `CASE t.table_type WHEN 'BASE TABLE' THEN 'table' WHEN 'VIEW' THEN 'view'` +
	` ELSE lower(t.table_type) END`

func registerRelations() {
	// schemata holds iox, where every measurement is, and system. It does
	// not list information_schema, which tables does.
	dbmeta.Schemas.Register(dbmeta.InfluxDB, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.catalog_name AS "catalog"`),
			always(`, s.schema_name AS "name"`),
			always(`, '' AS "owner"`),
			always(`, NULL AS "comment"`),
			always(`FROM information_schema.schemata s`),
			always(`WHERE ` + notSystem("s.schema_name")),
			always(`AND (@name = '' OR s.schema_name LIKE @name)`),
			always(`ORDER BY 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always public, the one catalog DataFusion reports"},
			{Name: "name"},
			{Name: "owner", Desc: "always empty: a schema has no owner"},
			{Name: "comment", Desc: "always absent: InfluxDB 3 has no COMMENT statement"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "with_system", Desc: "include the schemas InfluxDB 3 keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})

	// DataFusion's default schema is the one an unqualified name resolves
	// in, and df_settings records it. There is no current_schema().
	dbmeta.CurrentSchema.Register(dbmeta.InfluxDB, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT MAX(CASE WHEN s.name = 'datafusion.catalog.default_catalog' THEN s.value END) AS "catalog"`),
			always(`, MAX(CASE WHEN s.name = 'datafusion.catalog.default_schema' THEN s.value END) AS "name"`),
			always(`, '' AS "owner"`),
			always(`, NULL AS "comment"`),
			always(`FROM information_schema.df_settings s`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "the default catalog, from df_settings"},
			{Name: "name", Desc: "the default schema, from df_settings, which is iox"},
			{Name: "owner", Desc: "always empty: a schema has no owner"},
			{Name: "comment", Desc: "always absent: InfluxDB 3 has no COMMENT statement"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})

	// A measurement is a table in iox. system holds tables and one view of
	// the server's own, and information_schema holds views.
	dbmeta.Tables.Register(dbmeta.InfluxDB, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			always(`SELECT t.table_catalog AS "catalog"`),
			always(`, t.table_schema AS "schema"`),
			always(`, t.table_name AS "name"`),
			always(`, ` + tableType + ` AS "type"`),
			always(`, NULL AS "comment"`),
			always(`FROM information_schema.tables t`),
			always(`WHERE ` + notSystem("t.table_schema")),
			always(`AND (@schema = '' OR t.table_schema LIKE @schema)`),
			always(`AND (@name = '' OR t.table_name LIKE @name)`),
			always(`AND (@types = '' OR strpos(',' || @types || ',', ',' || ` + tableType + ` || ',') > 0)`),
			always(`ORDER BY 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always public, the one catalog DataFusion reports"},
			{Name: "schema", Desc: "iox for a measurement"},
			{Name: "name"}, {Name: "type"},
			{Name: "comment", Desc: "always absent: InfluxDB 3 has no COMMENT statement"},
		},
		Params: append(schemaNameSystem("relation"), dbmeta.TypesParam()),
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})

	// The columns of a measurement are its tags, its fields and time. A tag
	// reads Dictionary(Int32, Utf8), which is how DataFusion keeps it. The
	// position counts from 0, and the model adds 1, so that the first column
	// is 1 as it is on every other database. Only time is NOT NULL.
	dbmeta.Columns.Register(dbmeta.InfluxDB, &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			always(`SELECT c.table_catalog AS "catalog"`),
			always(`, c.table_schema AS "schema"`),
			always(`, c.table_name AS "table"`),
			always(`, c.column_name AS "name"`),
			always(`, c.ordinal_position + 1 AS "ordinal"`),
			always(`, c.data_type AS "data_type"`),
			always(`, c.is_nullable = 'YES' AS "nullable"`),
			always(`, c.column_default AS "default"`),
			always(`, false AS "primary_key"`),
			always(`, '' AS "identity"`),
			always(`, '' AS "generated"`),
			always(`, NULL AS "comment"`),
			always(`, NULL AS "collation"`),
			always(`FROM information_schema.columns c`),
			always(`WHERE ` + notSystem("c.table_schema")),
			always(`AND (@schema = '' OR c.table_schema LIKE @schema)`),
			always(`AND (@parent = '' OR c.table_name LIKE @parent)`),
			always(`AND (@name = '' OR c.column_name LIKE @name)`),
			always(`ORDER BY 2, 3, 5`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "ordinal", Desc: "the position from 1. DataFusion reports it from 0, and the model adds 1"},
			{Name: "data_type", Desc: "the Arrow type, such as Int64, Float64 or Timestamp(ns). A tag is Dictionary(Int32, Utf8)"},
			{Name: "nullable", Desc: "false for time alone: a point can leave out any tag or field"},
			{Name: "default", Desc: "always absent in practice: a column has no default"},
			{Name: "primary_key", Desc: "always false: InfluxDB 3 has no primary key"},
			{Name: "identity", Desc: "always empty: InfluxDB 3 has no identity column"},
			{Name: "generated", Desc: "always empty: InfluxDB 3 has no generated column"},
			{Name: "comment", Desc: "always absent: InfluxDB 3 has no COMMENT statement"},
			{Name: "collation", Desc: "always absent: InfluxDB 3 has no collation"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
			{Name: "name", Desc: "column name pattern, empty for every column", Default: ""},
			{Name: "with_system", Desc: "include the objects InfluxDB 3 keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			var v dbmeta.Column
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Ordinal, &v.DataType,
				&v.Nullable, &v.Default, &v.PrimaryKey, &v.Identity, &v.Generated, &v.Comment, &v.Collation)
			return v, err
		},
	})
}

// kinds is one row for each function name, from routines, which has one row
// for each name and return type. A name that is both an aggregate and a
// window function, such as first_value, has two kinds, so aggregate says
// whether one of them is an aggregate.
const kinds = `(SELECT r.specific_name` +
	`, MIN(r.function_type) AS function_type` +
	`, MAX(CASE WHEN r.function_type = 'AGGREGATE' THEN 1 ELSE 0 END) AS aggregate` +
	`, MIN(CASE WHEN r.is_deterministic THEN 1 ELSE 0 END) AS deterministic` +
	`, MIN(r.description) AS description` +
	` FROM information_schema.routines r GROUP BY r.specific_name) k`

// kindOf is the word for a function's kind.
const kindOf = `CASE k.function_type WHEN 'SCALAR' THEN 'func' WHEN 'AGGREGATE' THEN 'agg'` +
	` WHEN 'WINDOW' THEN 'window' ELSE lower(k.function_type) END`

// overloadID names one overload of a function: its name and the number
// DataFusion gives the overload, such as date_bin(1). RoutineParameters
// carries the same id.
const overloadID = `CONCAT(p.specific_name, '(', CAST(p.rid AS VARCHAR), ')')`

// functionStmt builds the statement behind Functions and Aggregates.
//
// parameters has one set of rows for each overload of a function, numbered by
// rid, with the return type as its OUT row, so it gives one row for each
// overload. A function with no parameter rows is read from routines instead,
// with one row for each return type and no id.
func functionStmt(aggregate bool) dbmeta.Stmt {
	kind, filter := kindOf, ``
	if aggregate {
		kind, filter = `'agg'`, ` AND k.aggregate = 1`
	}
	return dbmeta.Stmt{
		always(`SELECT p.specific_catalog AS "catalog"`),
		always(`, p.specific_schema AS "schema"`),
		always(`, p.specific_name AS "name"`),
		always(`, ` + overloadID + ` AS "id"`),
		always(`, ` + kind + ` AS "kind"`),
		always(`, MAX(CASE WHEN p.parameter_mode = 'OUT' THEN p.data_type END) AS "result_type"`),
		always(`, string_agg(CASE WHEN p.parameter_mode = 'IN' THEN p.data_type END, ', '` +
			` ORDER BY p.ordinal_position) AS "arg_types"`),
		always(`, CASE WHEN k.deterministic = 1 THEN 'immutable' ELSE 'volatile' END AS "volatility"`),
		always(`, '' AS "parallel"`),
		always(`, NULL AS "owner"`),
		always(`, '' AS "security"`),
		always(`, NULL AS "access"`),
		always(`, 'internal' AS "language"`),
		always(`, NULL AS "source"`),
		always(`, k.description AS "comment"`),
		always(`, NULL AS "definition"`),
		always(`FROM information_schema.parameters p JOIN ` + kinds + ` ON k.specific_name = p.specific_name`),
		always(`WHERE @with_system` + filter),
		always(`AND (@schema = '' OR p.specific_schema LIKE @schema)`),
		always(`AND (@name = '' OR p.specific_name LIKE @name)`),
		always(`GROUP BY p.specific_catalog, p.specific_schema, p.specific_name, p.rid,` +
			` k.function_type, k.deterministic, k.description`),
		always(`UNION ALL`),
		// DataFusion refuses two columns of one name, and two NULLs are
		// both named NULL, so every column of this arm is named.
		always(`SELECT r.specific_catalog AS "catalog", r.specific_schema AS "schema"` +
			`, r.specific_name AS "name", NULL AS "id"`),
		always(`, ` + kind + ` AS "kind", r.data_type AS "result_type", NULL AS "arg_types"`),
		always(`, CASE WHEN r.is_deterministic THEN 'immutable' ELSE 'volatile' END AS "volatility"`),
		always(`, '' AS "parallel", NULL AS "owner", '' AS "security", NULL AS "access"` +
			`, 'internal' AS "language", NULL AS "source", r.description AS "comment", NULL AS "definition"`),
		always(`FROM information_schema.routines r JOIN ` + kinds + ` ON k.specific_name = r.specific_name`),
		always(`WHERE @with_system` + filter),
		always(`AND NOT EXISTS (SELECT 1 FROM information_schema.parameters x WHERE x.specific_name = r.specific_name)`),
		always(`AND (@schema = '' OR r.specific_schema LIKE @schema)`),
		always(`AND (@name = '' OR r.specific_name LIKE @name)`),
		always(`ORDER BY 3, 4, 6`),
	}
}

// functionFields describes what both routine queries return.
func functionFields(kind string) []dbmeta.Field {
	return []dbmeta.Field{
		{Name: "catalog"},
		{Name: "schema", Desc: "always iox, where DataFusion lists its functions"},
		{Name: "name"},
		{Name: "id", Desc: "the name and the number of the overload, such as date_bin(1), which RoutineParameters joins on. Absent for a " + kind + " with no parameter rows, which is listed once for each return type"},
		{Name: "kind", Desc: "func, agg or window. A name that is both an aggregate and a window function reads agg"},
		{Name: "result_type", Desc: "the Arrow type the overload returns"},
		{Name: "arg_types", Desc: "the Arrow types of the arguments, in order. Absent where DataFusion lists no parameters"},
		{Name: "volatility", Desc: "immutable for a deterministic " + kind + ", and volatile otherwise"},
		{Name: "parallel", Desc: "always empty: DataFusion marks no parallel safety"},
		{Name: "owner", Desc: "always absent: a built in " + kind + " has no owner"},
		{Name: "security", Desc: "always empty: a built in " + kind + " has no security mode"},
		{Name: "access", Desc: "always absent: InfluxDB 3 Core grants nothing on a " + kind},
		{Name: "language", Desc: "always internal: every " + kind + " is built into DataFusion"},
		{Name: "source", Desc: "always absent: a built in " + kind + " has no source text"},
		{Name: "comment", Desc: "the description DataFusion ships for it"},
		{Name: "definition", Desc: "always absent, for the same reason as source"},
	}
}

func scanFunction(rows *sql.Rows) (dbmeta.Function, error) {
	var v dbmeta.Function
	err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.ID, &v.Kind, &v.ResultType, &v.ArgTypes,
		&v.Volatility, &v.Parallel, &v.Owner, &v.Security, &v.Access, &v.Language, &v.Source,
		&v.Comment, &v.Definition)
	return v, err
}

func registerRoutines() {
	// Every function is built into DataFusion, and InfluxDB 3 has no
	// CREATE FUNCTION, so with_system must be set to list any.
	params := func(kind string) []dbmeta.Param {
		out := schemaNameSystem(kind)
		out[2].Desc = "include the functions built into DataFusion, which are every one there is"
		return out
	}
	dbmeta.Functions.Register(dbmeta.InfluxDB, &dbmeta.Binding[dbmeta.Function]{
		Stmt:   functionStmt(false),
		Fields: functionFields("function"),
		Params: params("function"),
		Scan:   scanFunction,
	})
	dbmeta.Aggregates.Register(dbmeta.InfluxDB, &dbmeta.Binding[dbmeta.Function]{
		Stmt:   functionStmt(true),
		Fields: functionFields("aggregate"),
		Params: params("aggregate"),
		Scan:   scanFunction,
	})

	// The OUT row of an overload is its return value, which reads mode
	// return and ordinal 0, as it does on every other database.
	dbmeta.RoutineParameters.Register(dbmeta.InfluxDB, &dbmeta.Binding[dbmeta.RoutineParameter]{
		Stmt: dbmeta.Stmt{
			always(`SELECT p.specific_catalog AS "catalog"`),
			always(`, p.specific_schema AS "schema"`),
			always(`, p.specific_name AS "routine"`),
			always(`, ` + overloadID + ` AS "routine_id"`),
			always(`, p.parameter_name AS "name"`),
			always(`, CASE WHEN p.parameter_mode = 'OUT' THEN 0 ELSE p.ordinal_position END AS "ordinal"`),
			always(`, CASE WHEN p.parameter_mode = 'OUT' THEN 'return'` +
				` WHEN p.is_variadic THEN 'variadic' ELSE lower(p.parameter_mode) END AS "mode"`),
			always(`, p.data_type AS "data_type"`),
			always(`, NULL AS "default"`),
			always(`FROM information_schema.parameters p`),
			always(`WHERE (@schema = '' OR p.specific_schema LIKE @schema)`),
			always(`AND (@parent = '' OR p.specific_name LIKE @parent)`),
			always(`AND (@name = '' OR p.parameter_name LIKE @name)`),
			always(`ORDER BY 3, 4, 6`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "routine"},
			{Name: "routine_id", Desc: "the name and the number of the overload, the same value Function.ID carries"},
			{Name: "name", Desc: "absent for the return value, and for a parameter DataFusion does not name"},
			{Name: "ordinal", Desc: "one based, and zero for the return value"},
			{Name: "mode", Desc: "in, variadic, or return for the return value"},
			{Name: "data_type", Desc: "the Arrow type"},
			{Name: "default", Desc: "always absent: a parameter has no default"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "parent", Desc: "routine name pattern, empty for every routine", Default: ""},
			{Name: "name", Desc: "parameter name pattern, empty for every parameter", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.RoutineParameter, error) {
			var v dbmeta.RoutineParameter
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Routine, &v.RoutineID, &v.Name,
				&v.Ordinal, &v.Mode, &v.DataType, &v.Default)
			return v, err
		},
	})
}

func registerServer() {
	// df_settings holds DataFusion's settings for the session, with a
	// description of each.
	dbmeta.Settings.Register(dbmeta.InfluxDB, &dbmeta.Binding[dbmeta.Setting]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.name AS "name"`),
			always(`, s.value AS "value"`),
			always(`, NULL AS "type"`),
			always(`, NULL AS "context"`),
			always(`, NULL AS "access"`),
			always(`, NULL AS "display"`),
			always(`FROM information_schema.df_settings s`),
			always(`WHERE (@name = '' OR s.name LIKE @name)`),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the DataFusion setting, such as datafusion.execution.batch_size"},
			{Name: "value", Desc: "the value, and absent where it is not set"},
			{Name: "type", Desc: "always absent: df_settings records no type"},
			{Name: "context", Desc: "always absent: df_settings records no context"},
			{Name: "access", Desc: "always absent: a setting has no grant"},
			{Name: "display", Desc: "always absent: DataFusion shows a value in one form, which is value"},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "setting name pattern, empty for every setting", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.Setting, error) {
			var v dbmeta.Setting
			err := rows.Scan(&v.Name, &v.Value, &v.Type, &v.Context, &v.Access, &v.Display)
			return v, err
		},
	})
}
