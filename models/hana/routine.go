package hana

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerRoutines() {
	registerFunctions()
	registerRoutineParameters()
	registerTypes()
}

// routineArgs lists the input types of a routine, in order.
//
// STRING_AGG takes an ORDER BY in HANA, so the signature is in declaration
// order rather than in whatever order the rows arrive.
func routineArgs(view, key, owner string) string {
	return `(SELECT STRING_AGG(p.DATA_TYPE_NAME, ', ' ORDER BY p.POSITION)` +
		` FROM ` + view + ` p WHERE p.SCHEMA_NAME = ` + owner + `.SCHEMA_NAME` +
		` AND p.` + key + ` = ` + owner + `.` + key +
		` AND p.PARAMETER_TYPE = 'IN')`
}

func registerFunctions() {
	// \df. HANA keeps functions and procedures in two views and both are
	// routines, so this is one statement over both.
	dbmeta.Functions.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.Function]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, f.SCHEMA_NAME AS "schema"`),
			always(`, f.FUNCTION_NAME AS "name"`),
			always(`, CAST(f.FUNCTION_OID AS NVARCHAR(30)) AS "id"`),
			always(`, 'function' AS "kind"`),
			always(`, COALESCE((SELECT STRING_AGG(p.DATA_TYPE_NAME, ', ' ORDER BY p.POSITION)` +
				` FROM SYS.FUNCTION_PARAMETERS p WHERE p.SCHEMA_NAME = f.SCHEMA_NAME` +
				` AND p.FUNCTION_NAME = f.FUNCTION_NAME AND p.PARAMETER_TYPE = 'RETURN'), '')` +
				` AS "result_type"`),
			always(`, COALESCE(` + routineArgs(`SYS.FUNCTION_PARAMETERS`, `FUNCTION_NAME`, `f`) + `, '') AS "arg_types"`),
			always(`, CASE WHEN f.IS_DETERMINISTIC = 'TRUE' THEN 'immutable'` +
				` ELSE 'volatile' END AS "volatility"`),
			always(`, '' AS "parallel"`),
			always(`, f.OWNER_NAME AS "owner"`),
			always(`, LOWER(f.SQL_SECURITY) AS "security"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "access"`),
			always(`, f.FUNCTION_TYPE AS "language"`),
			always(`, f.DEFINITION AS "source"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "comment"`),
			always(`FROM SYS.FUNCTIONS f`),
			always(`WHERE ` + notSystem(`f.SCHEMA_NAME`)),
			always(`AND ` + like(`f.SCHEMA_NAME`, `@schema`)),
			always(`AND ` + like(`f.FUNCTION_NAME`, `@name`)),
			always(`UNION ALL`),
			always(`SELECT '', r.SCHEMA_NAME, r.PROCEDURE_NAME`),
			always(`, CAST(r.PROCEDURE_OID AS NVARCHAR(30))`),
			always(`, 'procedure'`),
			// A procedure returns result sets rather than a value, so the
			// result type is the word and the columns are parameters with
			// mode out.
			always(`, CASE WHEN r.OUTPUT_PARAMETER_COUNT > 0 OR r.RESULT_SET_COUNT > 0` +
				` THEN 'record' ELSE '' END`),
			always(`, COALESCE(` + routineArgs(`SYS.PROCEDURE_PARAMETERS`, `PROCEDURE_NAME`, `r`) + `, '')`),
			always(`, CASE WHEN r.IS_DETERMINISTIC = 'TRUE' THEN 'immutable' ELSE 'volatile' END`),
			always(`, ''`),
			always(`, r.OWNER_NAME`),
			always(`, LOWER(r.SQL_SECURITY)`),
			always(`, CAST(NULL AS NVARCHAR(1))`),
			always(`, r.PROCEDURE_TYPE`),
			always(`, r.DEFINITION`),
			always(`, CAST(NULL AS NVARCHAR(1))`),
			always(`FROM SYS.PROCEDURES r`),
			always(`WHERE ` + notSystem(`r.SCHEMA_NAME`)),
			always(`AND ` + like(`r.SCHEMA_NAME`, `@schema`)),
			always(`AND ` + like(`r.PROCEDURE_NAME`, `@name`)),
			always(`ORDER BY 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: a connection reaches one tenant database"},
			{Name: "schema"}, {Name: "name"},
			{Name: "id", Desc: "the object id, which is unique across the database"},
			{Name: "kind", Desc: "function or procedure"},
			{Name: "result_type", Desc: "the return type of a function. For a procedure it is record where the procedure returns output parameters or result sets, and empty where it returns none"},
			{Name: "arg_types", Desc: "the input types, comma separated and in declaration order"},
			{Name: "volatility", Desc: "immutable for a routine declared DETERMINISTIC, and volatile otherwise"},
			{Name: "parallel", Desc: "always empty: HANA marks no parallel safety on a routine"},
			{Name: "owner"},
			{Name: "security", Desc: "definer or invoker, from SQL_SECURITY"},
			{Name: "access", Desc: "always absent: Privileges reads the grants"},
			{Name: "language", Desc: "what the routine is written in, from FUNCTION_TYPE or PROCEDURE_TYPE: SQLSCRIPT2 for SQLScript, and BUILTIN, LIVECACHE or AFLLANG for the rest"},
			{Name: "source", Desc: "the CREATE text, which HANA stores in full"},
			{Name: "comment", Desc: "always absent: COMMENT ON has no routine form"},
		},
		Params: schemaAndName("routine"),
		Scan: func(rows *sql.Rows) (dbmeta.Function, error) {
			var v dbmeta.Function
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.ID, &v.Kind,
				&v.ResultType, &v.ArgTypes, &v.Volatility, &v.Parallel, &v.Owner,
				&v.Security, &v.Access, &v.Language, &v.Source, &v.Comment)
			return v, err
		},
	})
}

func registerRoutineParameters() {
	dbmeta.RoutineParameters.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.RoutineParameter]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, p.SCHEMA_NAME AS "schema"`),
			always(`, p.FUNCTION_NAME AS "routine"`),
			always(`, CAST(p.FUNCTION_OID AS NVARCHAR(30)) AS "routine_id"`),
			always(`, p.PARAMETER_NAME AS "name"`),
			always(`, p.POSITION AS "ordinal"`),
			always(`, CASE p.PARAMETER_TYPE WHEN 'IN' THEN 'in' WHEN 'OUT' THEN 'out'` +
				` WHEN 'INOUT' THEN 'inout' WHEN 'RETURN' THEN 'return'` +
				` ELSE LOWER(p.PARAMETER_TYPE) END AS "mode"`),
			always(`, ` + hanaType("p") + ` AS "data_type"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "default"`),
			always(`FROM SYS.FUNCTION_PARAMETERS p`),
			always(`WHERE ` + notSystem(`p.SCHEMA_NAME`)),
			always(`AND ` + like(`p.SCHEMA_NAME`, `@schema`)),
			always(`AND ` + like(`p.FUNCTION_NAME`, `@parent`)),
			always(`UNION ALL`),
			always(`SELECT '', q.SCHEMA_NAME, q.PROCEDURE_NAME`),
			always(`, CAST(q.PROCEDURE_OID AS NVARCHAR(30))`),
			always(`, q.PARAMETER_NAME, q.POSITION`),
			always(`, CASE q.PARAMETER_TYPE WHEN 'IN' THEN 'in' WHEN 'OUT' THEN 'out'` +
				` WHEN 'INOUT' THEN 'inout' ELSE LOWER(q.PARAMETER_TYPE) END`),
			always(`, ` + hanaType("q")),
			always(`, CAST(NULL AS NVARCHAR(1))`),
			always(`FROM SYS.PROCEDURE_PARAMETERS q`),
			always(`WHERE ` + notSystem(`q.SCHEMA_NAME`)),
			always(`AND ` + like(`q.SCHEMA_NAME`, `@schema`)),
			always(`AND ` + like(`q.PROCEDURE_NAME`, `@parent`)),
			always(`ORDER BY 2, 3, 6`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: a connection reaches one tenant database"},
			{Name: "schema"}, {Name: "routine"},
			{Name: "routine_id", Desc: "the object id. HANA does not overload a name, so the name identifies the routine on its own"},
			{Name: "name"},
			{Name: "ordinal", Desc: "from POSITION, which counts from one in each direction"},
			{Name: "mode", Desc: "in, out, inout or return"},
			{Name: "data_type"},
			{Name: "default", Desc: "always absent: HANA records that a parameter has a default in HAS_DEFAULT_VALUE and does not keep the value"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "parent", Desc: "routine name pattern, empty for every routine", Default: ""},
			{Name: "with_system", Desc: "include the schemas SAP HANA keeps for itself", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.RoutineParameter, error) {
			var v dbmeta.RoutineParameter
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Routine, &v.RoutineID, &v.Name,
				&v.Ordinal, &v.Mode, &v.DataType, &v.Default)
			return v, err
		},
	})
}

func registerTypes() {
	// \dT. SYS.DATA_TYPES is the engine's type list, which belongs to the
	// engine rather than to a schema, the way Trino's and Firebird's do.
	dbmeta.Types.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.Type]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, '' AS "schema"`),
			always(`, t.TYPE_NAME AS "name"`),
			always(`, CAST(t.TYPE_ID AS NVARCHAR(10)) AS "internal"`),
			always(`, 'base' AS "kind"`),
			always(`, '' AS "elements"`),
			always(`, '' AS "owner"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "access"`),
			always(`, CAST(NULL AS NVARCHAR(1)) AS "comment"`),
			always(`FROM SYS.DATA_TYPES t`),
			always(`WHERE ` + like(`t.TYPE_NAME`, `@name`)),
			always(`ORDER BY t.TYPE_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: a type belongs to the engine"},
			{Name: "schema", Desc: "always empty: a type belongs to the engine rather than to a schema"},
			{Name: "name"},
			{Name: "internal", Desc: "the numeric type id, which is what the catalog stores on a column"},
			{Name: "kind", Desc: "always base: every HANA type is built in and there is no CREATE TYPE for a scalar"},
			{Name: "elements", Desc: "always empty: HANA has no enumerated type"},
			{Name: "owner", Desc: "always empty: a built in type has no owner"},
			{Name: "access", Desc: "always absent: a type carries no grant"},
			{Name: "comment", Desc: "always absent: the engine writes no description on its own types"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "type name pattern, empty for every type", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Type, error) {
			var v dbmeta.Type
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Internal, &v.Kind,
				&v.Elements, &v.Owner, &v.Access, &v.Comment)
			return v, err
		},
	})
}
