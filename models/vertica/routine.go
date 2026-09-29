package vertica

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerRoutines() {
	registerFunctions()
	registerTypes()
	registerTriggers()
}

// functionKind names what a routine is, in the words the Function kind uses.
//
// v_catalog.user_functions says User Defined and then the kind. A scalar
// function and a transform, which returns rows, are both functions, an
// aggregate is an aggregate, an analytic function is a window function, and a
// stored procedure is a procedure. A parser, a source and a filter are the
// pieces a COPY statement loads through, which PostgreSQL has no routine kind
// for, so they keep Vertica's word.
func functionKind(col string) string {
	return text(`CASE ` + col +
		` WHEN 'User Defined Function' THEN 'function'` +
		` WHEN 'User Defined Transform' THEN 'function'` +
		` WHEN 'User Defined Aggregate' THEN 'aggregate'` +
		` WHEN 'User Defined Analytic' THEN 'window'` +
		` WHEN 'Stored Procedure' THEN 'procedure'` +
		` ELSE LOWER(REPLACE(` + col + `, 'User Defined ', '')) END`)
}

// functionID identifies a routine by its name and its argument types.
// Vertica overloads a name across argument types and records no object id, so
// the signature is what tells two apart.
func functionID(p string) string {
	return p + `.schema_name || '.' || ` + p + `.function_name || '(' || ` +
		p + `.function_argument_type || ')'`
}

// functionFields is the field list Functions and Aggregates share.
var functionFields = []dbmeta.Field{
	{Name: "catalog", Desc: "always empty: a Vertica connection reaches one database"},
	{Name: "schema"}, {Name: "name"},
	{Name: "id", Desc: "the name and argument types, such as public.approximate_median(Float), because Vertica overloads a name and records no object id for a routine"},
	{Name: "kind", Desc: "function for a scalar function or a transform, aggregate, window for an analytic function, procedure, or parser, source or filter for the pieces a COPY statement loads through"},
	{Name: "result_type", Desc: "the return type as Vertica writes it, such as Integer, and empty where the routine returns nothing a type describes"},
	{Name: "arg_types", Desc: "the argument types, comma separated, as Vertica writes them"},
	{Name: "volatility", Desc: "immutable, stable or volatile, and empty where the routine does not declare one"},
	{Name: "parallel", Desc: "always empty: Vertica marks no parallel safety"},
	{Name: "owner", Desc: "the owner. 7.2 does not record one for a function, so it is absent there"},
	{Name: "security", Desc: "definer or invoker for a stored procedure on 25.1, and empty otherwise"},
	{Name: "access", Desc: "always absent: Privileges reads the grants"},
	{Name: "language", Desc: "PL/vSQL for a stored procedure on 25.1, and empty otherwise: a function records the class and library it comes from rather than the language they were written in"},
	{Name: "source", Desc: "the definition Vertica records: the class and library for a function from a library, and absent for a stored procedure"},
	{Name: "comment", Desc: "from COMMENT ON FUNCTION"},
}

func scanFunction(rows *sql.Rows) (dbmeta.Function, error) {
	var v dbmeta.Function
	err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.ID, &v.Kind,
		&v.ResultType, &v.ArgTypes, &v.Volatility, &v.Parallel, &v.Owner,
		&v.Security, &v.Access, &v.Language, &v.Source, &v.Comment)
	return v, err
}

// functionStmt is the statement Functions and Aggregates share, with the
// kinds of routine each one reads.
func functionStmt(where string) dbmeta.Stmt {
	return dbmeta.Stmt{
		always(`SELECT '' AS "catalog"`),
		always(`, f.schema_name AS "schema"`),
		always(`, f.function_name AS "name"`),
		always(`, ` + functionID("f") + ` AS "id"`),
		always(`, ` + functionKind("f.procedure_type") + ` AS "kind"`),
		always(`, f.function_return_type AS "result_type"`),
		always(`, f.function_argument_type AS "arg_types"`),
		always(`, LOWER(f.volatility) AS "volatility"`),
		always(`, '' AS "parallel"`),
		since(v91, `, f.owner AS "owner"`, `, CAST(NULL AS VARCHAR) AS "owner"`),
		// A stored procedure is in user_procedures as well, which carries
		// what user_functions does not.
		since(v251, `, COALESCE(LOWER(p.security), '') AS "security"`, `, '' AS "security"`),
		always(`, CAST(NULL AS VARCHAR) AS "access"`),
		since(v251, `, COALESCE(p.language, '') AS "language"`, `, '' AS "language"`),
		always(`, NULLIF(f.function_definition, '') AS "source"`),
		always(`, NULLIF(f.comment, '') AS "comment"`),
		always(`FROM v_catalog.user_functions f`),
		since(v251, `LEFT JOIN v_catalog.user_procedures p ON f.procedure_type = 'Stored Procedure'`+
			` AND p.schema_name = f.schema_name AND p.procedure_name = f.function_name`+
			` AND p.procedure_arguments = f.function_argument_type`, ``),
		always(`WHERE ` + where),
		always(`AND ` + notSystem(`f.schema_name`)),
		always(`AND ` + like(`f.schema_name`, `@schema`)),
		always(`AND ` + like(`f.function_name`, `@name`)),
		always(`ORDER BY 2, 3, 4`),
	}
}

func registerFunctions() {
	// \df. user_functions lists every routine that is not built in: the
	// functions a library provides, the packages Vertica installs being
	// the most of them, and from 25.1 the stored procedures.
	dbmeta.Functions.Register(dbmeta.Vertica, &dbmeta.Binding[dbmeta.Function]{
		Stmt:   functionStmt(`TRUE`),
		Fields: functionFields,
		Params: schemaAndName("routine"),
		Scan:   scanFunction,
	})

	// \da.
	dbmeta.Aggregates.Register(dbmeta.Vertica, &dbmeta.Binding[dbmeta.Function]{
		Stmt:   functionStmt(`f.procedure_type = 'User Defined Aggregate'`),
		Fields: functionFields,
		Params: schemaAndName("aggregate"),
		Scan:   scanFunction,
	})
}

func registerTypes() {
	// \dT. v_catalog.types is the engine's scalar type list.
	dbmeta.Types.Register(dbmeta.Vertica, &dbmeta.Binding[dbmeta.Type]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, '' AS "schema"`),
			always(`, t.type_name AS "name"`),
			always(`, CAST(t.type_id AS VARCHAR) AS "internal"`),
			always(`, 'base' AS "kind"`),
			always(`, '' AS "elements"`),
			always(`, CAST(NULL AS VARCHAR) AS "owner"`),
			always(`, CAST(NULL AS VARCHAR) AS "access"`),
			always(`, CAST(NULL AS VARCHAR) AS "comment"`),
			always(`FROM v_catalog.types t`),
			always(`WHERE ` + like(`t.type_name`, `@name`)),
			always(`ORDER BY t.type_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: a type belongs to the engine"},
			{Name: "schema", Desc: "always empty: a type belongs to the engine rather than to a schema"},
			{Name: "name"},
			{Name: "internal", Desc: "the numeric type id, which is what the catalog stores on a column"},
			{Name: "kind", Desc: "always base: every type here is built in and there is no CREATE TYPE. From 10.1 the list includes array and set types, such as Array[Int8], which PostgreSQL's catalog also records as base types"},
			{Name: "elements", Desc: "always empty: Vertica has no enumerated type"},
			{Name: "owner", Desc: "always absent: a built in type has no owner"},
			{Name: "access", Desc: "always absent: a type carries no grant"},
			{Name: "comment", Desc: "always absent: the engine writes no description on its own types"},
		},
		Params: nameOnly("type"),
		Scan: func(rows *sql.Rows) (dbmeta.Type, error) {
			var v dbmeta.Type
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Internal, &v.Kind,
				&v.Elements, &v.Owner, &v.Access, &v.Comment)
			return v, err
		},
	})
}

func registerTriggers() {
	// A Vertica trigger runs a stored procedure on a schedule rather than
	// on a change to a table, so it names no table. It is measured on 25.1
	// and absent on 10.1.
	dbmeta.Triggers.Register(dbmeta.Vertica, &dbmeta.Binding[dbmeta.Trigger]{
		Stmt: dbmeta.Stmt{
			{{Min: v251, Query: `SELECT t.schema_name AS "schema"`}},
			always(`, '' AS "table"`),
			always(`, t.trigger_name AS "name"`),
			always(`, ` + text(`CASE WHEN t.enabled THEN 'enabled' ELSE 'disabled' END`) + ` AS "enabled"`),
			always(`, 'EXECUTE PROCEDURE ' || t.procedure_name || '(' || t.procedure_args || ')' AS "definition"`),
			always(`, CAST(NULL AS VARCHAR) AS "comment"`),
			always(`FROM v_catalog.stored_proc_triggers t`),
			always(`WHERE ` + notSystem(`t.schema_name`)),
			always(`AND ` + like(`t.schema_name`, `@schema`)),
			always(`AND ` + like(`t.trigger_name`, `@name`)),
			always(`ORDER BY 1, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"},
			{Name: "table", Desc: "always empty: a Vertica trigger fires on a schedule and is attached to no table"},
			{Name: "name"},
			{Name: "enabled"},
			{Name: "definition", Desc: "the procedure it runs and the arguments it passes. The schedule is an object of its own and is not recorded on the trigger"},
			{Name: "comment", Desc: "always absent: COMMENT ON has no trigger form"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "name", Desc: "trigger name pattern, empty for every one", Default: ""},
			{Name: "with_system", Desc: "include the schemas Vertica keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Trigger, error) {
			var v dbmeta.Trigger
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Enabled, &v.Definition, &v.Comment)
			return v, err
		},
	})
}
