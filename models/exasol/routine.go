package exasol

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerRoutines() {
	registerFunctions()
	registerTypes()
	registerLanguages()
}

// scriptKind names what a script is, in the words the Function kind uses.
//
// Exasol has four kinds of script and they map onto three of PostgreSQL's
// words. A UDF that takes a set of rows and returns one value is an
// aggregate, and Exasol's own documentation calls it one. Every other UDF is
// a function, whether it returns a value or emits rows. A scripting script
// is a program run with EXECUTE SCRIPT, which is what a procedure is. An
// adapter script is the handler behind a virtual schema, which PostgreSQL
// has no routine kind for, so it keeps Exasol's word.
func scriptKind(p string) string {
	return `CASE WHEN ` + p + `.SCRIPT_TYPE = 'UDF' AND ` + p + `.SCRIPT_INPUT_TYPE = 'SET'` +
		` AND ` + p + `.SCRIPT_RESULT_TYPE = 'RETURNS' THEN 'aggregate'` +
		` WHEN ` + p + `.SCRIPT_TYPE = 'UDF' THEN 'function'` +
		` WHEN ` + p + `.SCRIPT_TYPE = 'SCRIPTING' THEN 'procedure'` +
		` ELSE LOWER(` + p + `.SCRIPT_TYPE) END`
}

// scriptResult says what a script hands back where the catalog records it.
// A UDF that emits rows returns a set of them, and a scripting script
// returns a row count or a table. The type of a returned value is only in
// the text.
func scriptResult(p string) string {
	return `CASE ` + p + `.SCRIPT_RESULT_TYPE WHEN 'EMITS' THEN 'setof record'` +
		` WHEN 'ROWCOUNT' THEN 'rowcount' WHEN 'TABLE' THEN 'table'` +
		` ELSE '' END`
}

// functionFields is the field list Functions and Aggregates share.
var functionFields = []dbmeta.Field{
	{Name: "catalog", Desc: "always empty: an Exasol connection reaches one database"},
	{Name: "schema"}, {Name: "name"},
	{Name: "id", Desc: "the object id. Exasol does not overload a name, so the name identifies the routine on its own"},
	{Name: "kind", Desc: "function for a SQL function and for a UDF script, aggregate for a set UDF that returns one value, procedure for a scripting script run with EXECUTE SCRIPT, and adapter for the handler behind a virtual schema"},
	{Name: "result_type", Desc: "setof record for a UDF that emits rows, and rowcount or table for a scripting script. Absent where the routine returns a single value, because Exasol keeps that type only in the text in source"},
	{Name: "arg_types", Desc: "always absent: Exasol keeps the parameters only in the text in source"},
	{Name: "volatility", Desc: "always empty: Exasol marks no routine as deterministic or otherwise"},
	{Name: "parallel", Desc: "always empty: Exasol marks no parallel safety. A UDF runs in parallel across the cluster by design"},
	{Name: "owner"},
	{Name: "security", Desc: "always empty: Exasol records no definer or invoker rights on a routine"},
	{Name: "access", Desc: "always absent: Privileges reads the grants"},
	{Name: "language", Desc: "SQL for a SQL function, and the script language otherwise, such as LUA, PYTHON3, JAVA or R"},
	{Name: "source", Desc: "the whole CREATE statement, which is what Exasol stores"},
	{Name: "comment", Desc: "from COMMENT ON FUNCTION or COMMENT ON SCRIPT"},
	{Name: "definition", Desc: "the whole CREATE statement, the same as source"},
}

func scanFunction(rows *sql.Rows) (dbmeta.Function, error) {
	var v dbmeta.Function
	err := rows.Scan(dbmeta.NullAsEmpty(&v.Catalog), dbmeta.NullAsEmpty(&v.Schema), dbmeta.NullAsEmpty(&v.Name), &v.ID, dbmeta.NullAsEmpty(&v.Kind),
		&v.ResultType, &v.ArgTypes, dbmeta.NullAsEmpty(&v.Volatility), dbmeta.NullAsEmpty(&v.Parallel), &v.Owner,
		dbmeta.NullAsEmpty(&v.Security), &v.Access, dbmeta.NullAsEmpty(&v.Language), &v.Source, &v.Comment,
		&v.Definition)
	return v, err
}

// scriptArm selects the scripts in one view in the Functions shape.
func scriptArm(view, where string) string {
	return `SELECT '', s.SCRIPT_SCHEMA, s.SCRIPT_NAME` +
		`, CAST(s.SCRIPT_OBJECT_ID AS VARCHAR(20))` +
		`, ` + scriptKind("s") +
		`, ` + scriptResult("s") +
		`, '', '', '', s.SCRIPT_OWNER, '', CAST(NULL AS VARCHAR(1))` +
		`, s.SCRIPT_LANGUAGE, s.SCRIPT_TEXT, s.SCRIPT_COMMENT, s.SCRIPT_TEXT` +
		` FROM ` + view + ` s WHERE ` + where +
		` AND ` + like(`s.SCRIPT_SCHEMA`, `@schema`) +
		` AND ` + like(`s.SCRIPT_NAME`, `@name`)
}

func registerFunctions() {
	// \df. Exasol keeps SQL functions and scripts in two views and both
	// are routines, so this is one statement over both. The system scripts
	// are a third view of the same shape.
	dbmeta.Functions.Register(dbmeta.Exasol, &dbmeta.Binding[dbmeta.Function]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, f.FUNCTION_SCHEMA AS "schema"`),
			always(`, f.FUNCTION_NAME AS "name"`),
			always(`, CAST(f.FUNCTION_OBJECT_ID AS VARCHAR(20)) AS "id"`),
			always(`, 'function' AS "kind"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "result_type"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "arg_types"`),
			always(`, '' AS "volatility"`),
			always(`, '' AS "parallel"`),
			always(`, f.FUNCTION_OWNER AS "owner"`),
			always(`, '' AS "security"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "access"`),
			always(`, 'SQL' AS "language"`),
			always(`, f.FUNCTION_TEXT AS "source"`),
			always(`, f.FUNCTION_COMMENT AS "comment"`),
			always(`, f.FUNCTION_TEXT AS "definition"`),
			always(`FROM EXA_ALL_FUNCTIONS f`),
			always(`WHERE ` + like(`f.FUNCTION_SCHEMA`, `@schema`)),
			always(`AND ` + like(`f.FUNCTION_NAME`, `@name`)),
			always(`UNION ALL`),
			always(scriptArm(`EXA_ALL_SCRIPTS`, `TRUE`)),
			always(`UNION ALL`),
			always(scriptArm(`EXA_SYS_SCRIPTS`, system)),
			always(`ORDER BY 2, 3`),
		},
		Fields: functionFields,
		Params: schemaAndName("routine"),
		Scan:   scanFunction,
	})

	// \da. A set UDF that returns one value is Exasol's user defined
	// aggregate.
	dbmeta.Aggregates.Register(dbmeta.Exasol, &dbmeta.Binding[dbmeta.Function]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, s.SCRIPT_SCHEMA AS "schema"`),
			always(`, s.SCRIPT_NAME AS "name"`),
			always(`, CAST(s.SCRIPT_OBJECT_ID AS VARCHAR(20)) AS "id"`),
			always(`, 'aggregate' AS "kind"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "result_type"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "arg_types"`),
			always(`, '' AS "volatility"`),
			always(`, '' AS "parallel"`),
			always(`, s.SCRIPT_OWNER AS "owner"`),
			always(`, '' AS "security"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "access"`),
			always(`, s.SCRIPT_LANGUAGE AS "language"`),
			always(`, s.SCRIPT_TEXT AS "source"`),
			always(`, s.SCRIPT_COMMENT AS "comment"`),
			always(`, s.SCRIPT_TEXT AS "definition"`),
			always(`FROM EXA_ALL_SCRIPTS s`),
			always(`WHERE s.SCRIPT_TYPE = 'UDF' AND s.SCRIPT_INPUT_TYPE = 'SET'`),
			always(`AND s.SCRIPT_RESULT_TYPE = 'RETURNS'`),
			always(`AND ` + like(`s.SCRIPT_SCHEMA`, `@schema`)),
			always(`AND ` + like(`s.SCRIPT_NAME`, `@name`)),
			always(`ORDER BY 2, 3`),
		},
		Fields: functionFields,
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "name", Desc: "aggregate name pattern, empty for every one", Default: ""},
		},
		Scan: scanFunction,
	})
}

func registerTypes() {
	// \dT. EXA_SQL_TYPES is the engine's type list, which belongs to the
	// engine rather than to a schema.
	dbmeta.Types.Register(dbmeta.Exasol, &dbmeta.Binding[dbmeta.Type]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, '' AS "schema"`),
			always(`, t.TYPE_NAME AS "name"`),
			always(`, CAST(t.TYPE_ID AS VARCHAR(20)) AS "internal"`),
			always(`, 'base' AS "kind"`),
			always(`, '' AS "elements"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "owner"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "access"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "comment"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "size"`),
			always(`FROM EXA_SQL_TYPES t`),
			always(`WHERE ` + like(`t.TYPE_NAME`, `@name`)),
			always(`ORDER BY t.TYPE_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: a type belongs to the engine"},
			{Name: "schema", Desc: "always empty: a type belongs to the engine rather than to a schema"},
			{Name: "name"},
			{Name: "internal", Desc: "the JDBC type number Exasol reports for the type"},
			{Name: "kind", Desc: "always base: every Exasol type is built in and there is no CREATE TYPE"},
			{Name: "elements", Desc: "always empty: Exasol has no enumerated type"},
			{Name: "owner", Desc: "always absent: a built in type has no owner"},
			{Name: "access", Desc: "always absent: a type carries no grant"},
			{Name: "comment", Desc: "always absent: the engine writes no description on its own types"},
			{Name: "size", Desc: "always absent: EXA_SQL_TYPES records a precision and not an internal length"},
		},
		Params: nameOnly("type"),
		Scan: func(rows *sql.Rows) (dbmeta.Type, error) {
			var v dbmeta.Type
			err := rows.Scan(dbmeta.NullAsEmpty(&v.Catalog), dbmeta.NullAsEmpty(&v.Schema), dbmeta.NullAsEmpty(&v.Name), dbmeta.NullAsEmpty(&v.Internal), dbmeta.NullAsEmpty(&v.Kind),
				dbmeta.NullAsEmpty(&v.Elements), &v.Owner, &v.Access, &v.Comment, &v.Size)
			return v, err
		},
	})
}

// languageItems splits the SCRIPT_LANGUAGES parameter into one row per
// language, as ALIAS=URL.
//
// The parameter is one string of space separated entries, such as
// PYTHON3=builtin_python3 JAVA=builtin_java. CONNECT BY LEVEL counts the
// entries off, and REGEXP_SUBSTR takes the nth. The parameter holds a few
// entries whatever the size of the catalog, so the plan is bounded.
//
// The parameter is selected first and the generator runs over that one row.
// CONNECT BY runs before WHERE, so generating over EXA_PARAMETERS itself
// multiplied every level by every parameter and returned 182 rows for four
// languages.
const languageItems = `SELECT REGEXP_SUBSTR(p.V, '[^ ]+', 1, LEVEL) AS ITEM` +
	` FROM (SELECT TRIM(SESSION_VALUE) AS V FROM EXA_PARAMETERS` +
	` WHERE PARAMETER_NAME = 'SCRIPT_LANGUAGES') p` +
	` CONNECT BY LEVEL <= LENGTH(p.V) - LENGTH(REPLACE(p.V, ' ', '')) + 1`

func registerLanguages() {
	// \dL. Lua is built into the engine. Every other language is a script
	// language container, and SCRIPT_LANGUAGES maps the alias a CREATE
	// SCRIPT names onto the container that runs it.
	dbmeta.Languages.Register(dbmeta.Exasol, &dbmeta.Binding[dbmeta.Language]{
		Stmt: dbmeta.Stmt{
			always(`SELECT 'LUA' AS "name"`),
			always(`, '' AS "owner"`),
			always(`, TRUE AS "trusted"`),
			always(`, TRUE AS "internal"`),
			always(`, '' AS "handler"`),
			always(`, '' AS "validator"`),
			always(`, '' AS "inline"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "access"`),
			always(`, CAST(NULL AS VARCHAR(1)) AS "comment"`),
			always(`FROM DUAL WHERE ` + like(`'LUA'`, `@name`)),
			always(`UNION ALL`),
			always(`SELECT SUBSTR(l.ITEM, 1, INSTR(l.ITEM, '=') - 1)`),
			always(`, '', TRUE, FALSE`),
			always(`, SUBSTR(l.ITEM, INSTR(l.ITEM, '=') + 1)`),
			always(`, '', '', CAST(NULL AS VARCHAR(1)), CAST(NULL AS VARCHAR(1))`),
			always(`FROM (` + languageItems + `) l`),
			always(`WHERE INSTR(l.ITEM, '=') > 0`),
			always(`AND ` + like(`SUBSTR(l.ITEM, 1, INSTR(l.ITEM, '=') - 1)`, `@name`)),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "LUA, which is built in, and each alias SCRIPT_LANGUAGES defines, such as PYTHON3"},
			{Name: "owner", Desc: "always empty: a language belongs to the database"},
			{Name: "trusted", Desc: "always true: any user with CREATE SCRIPT can write a script in any language, which is what PostgreSQL means by trusted"},
			{Name: "internal", Desc: "true for LUA, which runs inside the engine, and false for a language that runs in a container"},
			{Name: "handler", Desc: "the container that runs the language, from SCRIPT_LANGUAGES, such as builtin_python3. Empty for LUA"},
			{Name: "validator", Desc: "always empty: Exasol names no validator"},
			{Name: "inline", Desc: "always empty: Exasol has no inline code block"},
			{Name: "access", Desc: "always absent: a language carries no grant"},
			{Name: "comment", Desc: "always absent: COMMENT ON has no language form"},
		},
		Params: nameOnly("language"),
		Scan: func(rows *sql.Rows) (dbmeta.Language, error) {
			var v dbmeta.Language
			err := rows.Scan(dbmeta.NullAsEmpty(&v.Name), dbmeta.NullAsEmpty(&v.Owner), &v.Trusted, &v.Internal, dbmeta.NullAsEmpty(&v.Handler),
				dbmeta.NullAsEmpty(&v.Validator), dbmeta.NullAsEmpty(&v.Inline), &v.Access, &v.Comment)
			return v, err
		},
	})
}
