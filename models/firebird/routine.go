package firebird

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// argType is the type of one routine argument.
//
// A PSQL routine records its argument types as internal domains and leaves
// the inline columns NULL, and a legacy external function declared with
// DECLARE EXTERNAL FUNCTION does the opposite. Both still exist, so this
// reads whichever one the row filled.
func argType(arg string) string {
	return `CASE WHEN f.RDB$FIELD_TYPE IS NOT NULL THEN ` + fieldType("f") +
		` ELSE ` + fieldType(arg) + ` END`
}

// routineLanguage names what a routine is written in. Firebird has no
// catalog of languages, so this is what each routine says about itself.
func routineLanguage(p string) string {
	return `CASE WHEN ` + p + `.RDB$ENGINE_NAME IS NOT NULL THEN TRIM(TRAILING FROM ` + p + `.RDB$ENGINE_NAME)` +
		` WHEN ` + p + `.RDB$ENTRYPOINT IS NOT NULL THEN 'UDF'` +
		` ELSE 'PSQL' END`
}

// sqlSecurity reads the SQL SECURITY clause, which arrived in 4.0. Before
// that the clause does not exist and the column is empty, which [dbmeta.Field]
// cannot mark absent because Security is a plain string rather than a
// nullable one. The description says which is which.
func sqlSecurity(p string) dbmeta.Choice {
	return dbmeta.Choice{
		{Min: v4, Query: `, CASE ` + p + `.RDB$SQL_SECURITY WHEN TRUE THEN 'definer'` +
			` WHEN FALSE THEN 'invoker' ELSE 'inherit' END AS "security"`},
		{Query: `, '' AS "security"`},
	}
}

const securityDesc = "definer, invoker, or inherit where the object names no clause" +
	" and takes the database default. Empty on 3.0, which has no SQL SECURITY clause at all"

func registerRoutines() {
	registerFunctions()
	registerRoutineParameters()
	registerDomains()
	registerTypes()
}

func registerFunctions() {
	// \df. Firebird keeps functions and procedures in two tables and both are
	// routines, so this is one statement over both.
	//
	// A member of a package is left out, the way models/oracle leaves one
	// out: it is not callable by name on its own, and the package is the
	// object a caller names.
	dbmeta.Functions.Register(dbmeta.Firebird, &dbmeta.Binding[dbmeta.Function]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, ` + noSchema),
			always(`, TRIM(TRAILING FROM fn.RDB$FUNCTION_NAME) AS "name"`),
			always(`, CAST(fn.RDB$FUNCTION_ID AS VARCHAR(20)) AS "id"`),
			always(`, 'function' AS "kind"`),
			always(`, COALESCE((SELECT ` + argType("a") + ` FROM RDB$FUNCTION_ARGUMENTS a` +
				` LEFT JOIN RDB$FIELDS f ON f.RDB$FIELD_NAME = a.RDB$FIELD_SOURCE` +
				` WHERE a.RDB$FUNCTION_NAME = fn.RDB$FUNCTION_NAME` +
				` AND a.RDB$PACKAGE_NAME IS NOT DISTINCT FROM fn.RDB$PACKAGE_NAME` +
				` AND a.RDB$ARGUMENT_POSITION = fn.RDB$RETURN_ARGUMENT), '') AS "result_type"`),
			always(`, COALESCE((SELECT LIST(t.ty, ', ') FROM (` +
				`SELECT ` + argType("a") + ` AS ty FROM RDB$FUNCTION_ARGUMENTS a` +
				` LEFT JOIN RDB$FIELDS f ON f.RDB$FIELD_NAME = a.RDB$FIELD_SOURCE` +
				` WHERE a.RDB$FUNCTION_NAME = fn.RDB$FUNCTION_NAME` +
				` AND a.RDB$PACKAGE_NAME IS NOT DISTINCT FROM fn.RDB$PACKAGE_NAME` +
				` AND a.RDB$ARGUMENT_POSITION <> fn.RDB$RETURN_ARGUMENT` +
				` ORDER BY a.RDB$ARGUMENT_POSITION) t), '') AS "arg_types"`),
			always(`, CASE WHEN COALESCE(fn.RDB$DETERMINISTIC_FLAG, 0) = 1` +
				` THEN 'immutable' ELSE 'volatile' END AS "volatility"`),
			always(`, '' AS "parallel"`),
			always(`, TRIM(TRAILING FROM COALESCE(fn.RDB$OWNER_NAME, '')) AS "owner"`),
			sqlSecurity("fn"),
			always(`, CAST(NULL AS VARCHAR(1)) AS "access"`),
			always(`, ` + routineLanguage("fn") + ` AS "language"`),
			always(`, fn.RDB$FUNCTION_SOURCE AS "source"`),
			always(`, fn.RDB$DESCRIPTION AS "comment"`),
			always(`FROM RDB$FUNCTIONS fn`),
			always(`WHERE ` + userObject(`fn.RDB$SYSTEM_FLAG`) + ` AND fn.RDB$PACKAGE_NAME IS NULL`),
			always(`AND ` + like(`''`, `@schema`) + ``),
			always(`AND ` + like(`fn.RDB$FUNCTION_NAME`, `@name`) + ``),
			always(`UNION ALL`),
			always(`SELECT '', ''`),
			always(`, TRIM(TRAILING FROM pr.RDB$PROCEDURE_NAME)`),
			always(`, CAST(pr.RDB$PROCEDURE_ID AS VARCHAR(20))`),
			always(`, 'procedure'`),
			// A procedure returns a row set rather than a value, so the
			// result type is the word and the columns are in
			// RoutineParameters with mode out.
			always(`, CASE WHEN COALESCE(pr.RDB$PROCEDURE_OUTPUTS, 0) > 0` +
				` THEN 'record' ELSE '' END`),
			always(`, COALESCE((SELECT LIST(t.ty, ', ') FROM (` +
				`SELECT ` + fieldType("f") + ` AS ty FROM RDB$PROCEDURE_PARAMETERS p` +
				` JOIN RDB$FIELDS f ON f.RDB$FIELD_NAME = p.RDB$FIELD_SOURCE` +
				` WHERE p.RDB$PROCEDURE_NAME = pr.RDB$PROCEDURE_NAME` +
				` AND p.RDB$PACKAGE_NAME IS NOT DISTINCT FROM pr.RDB$PACKAGE_NAME` +
				` AND p.RDB$PARAMETER_TYPE = 0` +
				` ORDER BY p.RDB$PARAMETER_NUMBER) t), '')`),
			always(`, 'volatile'`),
			always(`, ''`),
			always(`, TRIM(TRAILING FROM COALESCE(pr.RDB$OWNER_NAME, ''))`),
			sqlSecurity("pr"),
			always(`, CAST(NULL AS VARCHAR(1))`),
			always(`, ` + routineLanguage("pr")),
			always(`, pr.RDB$PROCEDURE_SOURCE`),
			always(`, pr.RDB$DESCRIPTION`),
			always(`FROM RDB$PROCEDURES pr`),
			always(`WHERE ` + userObject(`pr.RDB$SYSTEM_FLAG`) + ` AND pr.RDB$PACKAGE_NAME IS NULL`),
			always(`AND ` + like(`''`, `@schema`) + ``),
			always(`AND ` + like(`pr.RDB$PROCEDURE_NAME`, `@name`) + ``),
			always(`ORDER BY 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: Firebird has no level above the database"},
			{Name: "schema", Desc: schemaDesc},
			{Name: "name"},
			{Name: "id", Desc: "the numeric id, which is unique within its table. Firebird does not overload a name, so the name identifies the routine on its own"},
			{Name: "kind", Desc: "function or procedure"},
			{Name: "result_type", Desc: "the return type of a function. For a procedure it is record where the procedure returns columns, and empty where it returns none"},
			{Name: "arg_types", Desc: "the input types, comma separated. Firebird's LIST aggregate does not promise an order, so RoutineParameters is the form to read when the order matters"},
			{Name: "volatility", Desc: "immutable for a function declared DETERMINISTIC, and volatile otherwise. A procedure is always volatile: Firebird has no such clause for one"},
			{Name: "parallel", Desc: "always empty: Firebird has no parallel safety marking on a routine"},
			{Name: "owner", Desc: "from RDB$OWNER_NAME, the user that created it"},
			{Name: "security", Desc: securityDesc},
			{Name: "access", Desc: "always absent: Privileges reads the grants"},
			{Name: "language", Desc: "PSQL for a routine written in Firebird's own language, the engine name for an external one such as UDR, and UDF for a legacy external function"},
			{Name: "source", Desc: "the body, absent for an external routine, which has no body in the database"},
			{Name: "comment"},
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
	// The parameters of every routine, in declaration order. A function
	// records its return value as an argument like any other, at the
	// position RDB$RETURN_ARGUMENT names, so it arrives here with mode
	// return.
	dbmeta.RoutineParameters.Register(dbmeta.Firebird, &dbmeta.Binding[dbmeta.RoutineParameter]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, ` + noSchema),
			always(`, TRIM(TRAILING FROM fn.RDB$FUNCTION_NAME) AS "routine"`),
			always(`, CAST(fn.RDB$FUNCTION_ID AS VARCHAR(20)) AS "routine_id"`),
			always(`, TRIM(TRAILING FROM a.RDB$ARGUMENT_NAME) AS "name"`),
			always(`, CAST(a.RDB$ARGUMENT_POSITION AS BIGINT) AS "ordinal"`),
			always(`, CASE WHEN a.RDB$ARGUMENT_POSITION = fn.RDB$RETURN_ARGUMENT` +
				` THEN 'return' ELSE 'in' END AS "mode"`),
			always(`, ` + argType("a") + ` AS "data_type"`),
			always(`, a.RDB$DEFAULT_SOURCE AS "default"`),
			always(`FROM RDB$FUNCTION_ARGUMENTS a`),
			always(`JOIN RDB$FUNCTIONS fn ON fn.RDB$FUNCTION_NAME = a.RDB$FUNCTION_NAME` +
				` AND fn.RDB$PACKAGE_NAME IS NOT DISTINCT FROM a.RDB$PACKAGE_NAME`),
			always(`LEFT JOIN RDB$FIELDS f ON f.RDB$FIELD_NAME = a.RDB$FIELD_SOURCE`),
			always(`WHERE ` + userObject(`fn.RDB$SYSTEM_FLAG`) + ` AND fn.RDB$PACKAGE_NAME IS NULL`),
			always(`AND ` + like(`''`, `@schema`) + ``),
			always(`AND ` + like(`fn.RDB$FUNCTION_NAME`, `@parent`) + ``),
			always(`UNION ALL`),
			always(`SELECT '', ''`),
			always(`, TRIM(TRAILING FROM pr.RDB$PROCEDURE_NAME)`),
			always(`, CAST(pr.RDB$PROCEDURE_ID AS VARCHAR(20))`),
			always(`, TRIM(TRAILING FROM p.RDB$PARAMETER_NAME)`),
			always(`, CAST(p.RDB$PARAMETER_NUMBER + 1 AS BIGINT)`),
			always(`, CASE p.RDB$PARAMETER_TYPE WHEN 0 THEN 'in' ELSE 'out' END`),
			always(`, ` + fieldType("pf")),
			always(`, p.RDB$DEFAULT_SOURCE`),
			always(`FROM RDB$PROCEDURE_PARAMETERS p`),
			always(`JOIN RDB$PROCEDURES pr ON pr.RDB$PROCEDURE_NAME = p.RDB$PROCEDURE_NAME` +
				` AND pr.RDB$PACKAGE_NAME IS NOT DISTINCT FROM p.RDB$PACKAGE_NAME`),
			always(`JOIN RDB$FIELDS pf ON pf.RDB$FIELD_NAME = p.RDB$FIELD_SOURCE`),
			always(`WHERE ` + userObject(`pr.RDB$SYSTEM_FLAG`) + ` AND pr.RDB$PACKAGE_NAME IS NULL`),
			always(`AND ` + like(`''`, `@schema`) + ``),
			always(`AND ` + like(`pr.RDB$PROCEDURE_NAME`, `@parent`) + ``),
			always(`ORDER BY 3, 6`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: Firebird has no level above the database"},
			{Name: "schema", Desc: schemaDesc},
			{Name: "routine"},
			{Name: "routine_id", Desc: "the numeric id of the routine. Firebird does not overload a name, so grouping by the name alone is safe here"},
			{Name: "name", Desc: "absent for the return value of a function declared without one"},
			{Name: "ordinal", Desc: "the declaration position. A function's return value is at the position RDB$RETURN_ARGUMENT names, which is zero, and its inputs start at one. A procedure's parameters count from one in each direction"},
			{Name: "mode", Desc: "in, out or return. Firebird has no INOUT parameter and no variadic one"},
			{Name: "data_type"},
			{Name: "default", Desc: "the source text, which begins with the word DEFAULT"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern. Firebird has no schemas", Default: ""},
			{Name: "parent", Desc: "routine name pattern, empty for every routine", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.RoutineParameter, error) {
			var v dbmeta.RoutineParameter
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Routine, &v.RoutineID, &v.Name,
				&v.Ordinal, &v.Mode, &v.DataType, &v.Default)
			return v, err
		},
	})
}

// collationOf names the collation of a row that carries a character set and
// a collation id, which is how RDB$FIELDS records one.
const collationOf = `(SELECT TRIM(TRAILING FROM c.RDB$COLLATION_NAME) FROM RDB$COLLATIONS c` +
	` WHERE c.RDB$CHARACTER_SET_ID = f.RDB$CHARACTER_SET_ID` +
	` AND c.RDB$COLLATION_ID = COALESCE(f.RDB$COLLATION_ID, 0))`

func registerDomains() {
	// \dD. A Firebird domain is a first class object and it shares
	// RDB$FIELDS with the internal rows that hold every column's type, so
	// the filter is the name: an internal one is called RDB$ and a number.
	dbmeta.Domains.Register(dbmeta.Firebird, &dbmeta.Binding[dbmeta.Domain]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, ` + noSchema),
			always(`, TRIM(TRAILING FROM f.RDB$FIELD_NAME) AS "name"`),
			always(`, ` + fieldType("f") + ` AS "data_type"`),
			always(`, COALESCE(` + collationOf + `, '') AS "collation"`),
			always(`, COALESCE(f.RDB$NULL_FLAG, 0) = 0 AS "nullable"`),
			always(`, f.RDB$DEFAULT_SOURCE AS "default"`),
			always(`, COALESCE(f.RDB$VALIDATION_SOURCE, '') AS "constraints"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "access"`),
			always(`, f.RDB$DESCRIPTION AS "comment"`),
			always(`FROM RDB$FIELDS f`),
			always(`WHERE ` + userObject(`f.RDB$SYSTEM_FLAG`)),
			always(`AND f.RDB$FIELD_NAME NOT STARTING WITH 'RDB$'`),
			always(`AND ` + like(`''`, `@schema`) + ``),
			always(`AND ` + like(`f.RDB$FIELD_NAME`, `@name`) + ``),
			always(`ORDER BY f.RDB$FIELD_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: Firebird has no level above the database"},
			{Name: "schema", Desc: schemaDesc},
			{Name: "name"},
			{Name: "data_type"},
			{Name: "collation", Desc: "empty where the domain is not a character type, which records no collation"},
			{Name: "nullable"},
			{Name: "default", Desc: "the source text, which begins with the word DEFAULT"},
			{Name: "constraints", Desc: "the CHECK text, from RDB$VALIDATION_SOURCE. Empty where the domain has none"},
			{Name: "access", Desc: "always absent: Firebird grants nothing on a domain"},
			{Name: "comment"},
		},
		Params: schemaAndName("domain"),
		Scan: func(rows *sql.Rows) (dbmeta.Domain, error) {
			var v dbmeta.Domain
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.DataType, &v.Collation,
				&v.Nullable, &v.Default, &v.Constraints, &v.Access, &v.Comment)
			return v, err
		},
	})
}

func registerTypes() {
	// \dT. RDB$TYPES is the engine's own enumeration of everything it names
	// by a number, and the rows for RDB$FIELD_TYPE are the data types. They
	// belong to the engine rather than to the database, the way Trino's are.
	dbmeta.Types.Register(dbmeta.Firebird, &dbmeta.Binding[dbmeta.Type]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, ` + noSchema),
			always(`, TRIM(TRAILING FROM t.RDB$TYPE_NAME) AS "name"`),
			always(`, CAST(t.RDB$TYPE AS VARCHAR(10)) AS "internal"`),
			always(`, 'base' AS "kind"`),
			always(`, '' AS "elements"`),
			always(`, '' AS "owner"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "access"`),
			always(`, t.RDB$DESCRIPTION AS "comment"`),
			always(`FROM RDB$TYPES t`),
			always(`WHERE t.RDB$FIELD_NAME = 'RDB$FIELD_TYPE'`),
			always(`AND ` + like(`''`, `@schema`) + ``),
			always(`AND ` + like(`t.RDB$TYPE_NAME`, `@name`) + ``),
			always(`ORDER BY t.RDB$TYPE_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: Firebird has no level above the database"},
			{Name: "schema", Desc: "always empty: a type belongs to the engine rather than to a namespace"},
			{Name: "name", Desc: "the engine's own spelling, such as VARYING for VARCHAR and LONG for INTEGER"},
			{Name: "internal", Desc: "the numeric type code, which is what RDB$FIELDS stores"},
			{Name: "kind", Desc: "always base: every Firebird type is built in and there is no CREATE TYPE"},
			{Name: "elements", Desc: "always empty: Firebird has no enumerated type"},
			{Name: "owner", Desc: "always empty: a built in type has no owner"},
			{Name: "access", Desc: "always absent: a type carries no grant"},
			{Name: "comment", Desc: "always absent in practice: the engine writes no description on its own rows"},
		},
		Params: schemaAndName("type"),
		Scan: func(rows *sql.Rows) (dbmeta.Type, error) {
			var v dbmeta.Type
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Internal, &v.Kind,
				&v.Elements, &v.Owner, &v.Access, &v.Comment)
			return v, err
		},
	})
}
