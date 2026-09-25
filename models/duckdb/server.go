package duckdb

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// Functions, types, collations, settings and extensions.

func registerServer() {
	registerFunctions()
	registerTypes()
	registerSettings()
}

// functionStmt builds the statement behind \df and \da. kindFilter narrows to
// one function_type, or is empty for every kind.
func functionStmt(kindFilter string) dbmeta.Stmt {
	stmt := dbmeta.Stmt{
		always(`SELECT f.database_name AS "catalog"`),
		always(`, f.schema_name AS "schema"`),
		always(`, f.function_name AS "name"`),
		// A name can be overloaded, so the oid is what RoutineParameters
		// joins on, the same as on PostgreSQL.
		always(`, f.function_oid::VARCHAR AS "id"`),
		always(`, CASE f.function_type WHEN 'scalar' THEN 'func'` +
			` WHEN 'aggregate' THEN 'agg' WHEN 'table' THEN 'table'` +
			` WHEN 'macro' THEN 'macro' WHEN 'pragma' THEN 'pragma'` +
			` ELSE f.function_type END AS "kind"`),
		always(`, COALESCE(f.return_type, '') AS "result_type"`),
		always(`, COALESCE(LIST_REDUCE(f.parameter_types, (a, b) -> a || ', ' || b), '') AS "arg_types"`),
		always(`, CASE WHEN f.has_side_effects THEN 'volatile' ELSE '' END AS "volatility"`),
		always(`, '' AS "parallel"`),
		always(`, '' AS "owner"`),
		always(`, '' AS "security"`),
		always(`, NULL AS "access"`),
		always(`, CASE WHEN f.internal THEN 'c' ELSE 'sql' END AS "language"`),
		// A macro keeps its body. A compiled function does not have one.
		always(`, f.macro_definition AS "source"`),
		always(`, COALESCE(f.comment, f.description) AS "comment"`),
		always(`FROM duckdb_functions() f`),
		always(`WHERE ` + internalOf("f")),
	}
	if kindFilter != "" {
		stmt = append(stmt, always(`AND f.function_type = '`+kindFilter+`'`))
	}
	return append(stmt,
		always(`AND (@schema = '' OR f.schema_name LIKE @schema)`),
		always(`AND (@name = '' OR f.function_name LIKE @name)`),
		always(`ORDER BY 2, 3, 4`),
	)
}

func functionFields() []dbmeta.Field {
	return []dbmeta.Field{
		{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
		{Name: "id", Desc: "the function oid, because DuckDB overloads a name"},
		{Name: "kind", Desc: "func, agg, table, macro or pragma"},
		{Name: "result_type", Desc: "empty where DuckDB records none, which is normal for a macro"},
		{
			Name: "arg_types",
			Desc: "the parameter types joined with a comma. RoutineParameters has them as rows",
		},
		{
			Name: "volatility",
			Desc: "volatile for a function with side effects, and empty otherwise. DuckDB records no finer grade",
		},
		{Name: "parallel", Desc: "always empty: DuckDB has no parallel safety marking"},
		{Name: "owner", Desc: "always empty: DuckDB has no users"},
		{Name: "security", Desc: "always empty, for the same reason"},
		{Name: "access", Desc: "always absent: DuckDB has no grants"},
		{Name: "language", Desc: "c for a function built into the library, sql for a macro"},
		{Name: "source", Desc: "the body of a macro, and absent for a compiled function"},
		{Name: "comment", Desc: "the comment, or the description DuckDB ships for a built in"},
	}
}

func scanFunction(rows *sql.Rows) (dbmeta.Function, error) {
	var v dbmeta.Function
	err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.ID, &v.Kind, &v.ResultType,
		&v.ArgTypes, &v.Volatility, &v.Parallel, &v.Owner, &v.Security, &v.Access,
		&v.Language, &v.Source, &v.Comment)
	return v, err
}

func registerFunctions() {
	dbmeta.Functions.Register(dbmeta.DuckDB, &dbmeta.Binding[dbmeta.Function]{
		Stmt:   functionStmt(""),
		Fields: functionFields(),
		Params: schemaNameSystem("function"),
		Scan:   scanFunction,
	})

	// \da. DuckDB marks an aggregate in the same catalog, so this is the
	// function query narrowed, which is how psql writes it too.
	dbmeta.Aggregates.Register(dbmeta.DuckDB, &dbmeta.Binding[dbmeta.Function]{
		Stmt:   functionStmt("aggregate"),
		Fields: functionFields(),
		Params: schemaNameSystem("aggregate"),
		Scan:   scanFunction,
	})

	// The parameters of a function are two lists in the same row, the names
	// and the types, in the same order.
	dbmeta.RoutineParameters.Register(dbmeta.DuckDB, &dbmeta.Binding[dbmeta.RoutineParameter]{
		Stmt: dbmeta.Stmt{
			always(`SELECT f.database_name AS "catalog"`),
			always(`, f.schema_name AS "schema"`),
			always(`, f.function_name AS "routine"`),
			always(`, f.function_oid::VARCHAR AS "routine_id"`),
			always(`, NULLIF(e.param, '') AS "name"`),
			always(`, e.ordinality AS "ordinal"`),
			always(`, 'in' AS "mode"`),
			always(`, COALESCE(f.parameter_types[e.ordinality], '') AS "data_type"`),
			always(`, NULL AS "default"`),
			always(`FROM duckdb_functions() f,` +
				` unnest(f.parameters) WITH ORDINALITY AS e(param, ordinality)`),
			always(`WHERE ` + internalOf("f")),
			always(`AND (@schema = '' OR f.schema_name LIKE @schema)`),
			always(`AND (@parent = '' OR f.function_name LIKE @parent)`),
			always(`AND (@name = '' OR e.param LIKE @name)`),
			always(`ORDER BY 2, 3, 4, 6`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "routine"},
			{Name: "routine_id", Desc: "the function oid, because DuckDB overloads a name"},
			{Name: "name", Desc: "absent where the parameter is unnamed"},
			{Name: "ordinal", Desc: "one based position in the declaration"},
			{
				Name: "mode",
				Desc: "always in: DuckDB has no output parameter, and a return value is Function.ResultType rather than a row",
			},
			{Name: "data_type"},
			{Name: "default", Desc: "always absent: DuckDB records no parameter default"},
		},
		Params: schemaParentName("parameter"),
		Scan: func(rows *sql.Rows) (dbmeta.RoutineParameter, error) {
			var v dbmeta.RoutineParameter
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Routine, &v.RoutineID, &v.Name,
				&v.Ordinal, &v.Mode, &v.DataType, &v.Default)
			return v, err
		},
	})
}

func registerTypes() {
	// \dT.
	dbmeta.Types.Register(dbmeta.DuckDB, &dbmeta.Binding[dbmeta.Type]{
		Stmt: dbmeta.Stmt{
			always(`SELECT y.database_name AS "catalog"`),
			always(`, y.schema_name AS "schema"`),
			always(`, y.type_name AS "name"`),
			always(`, y.type_name AS "internal"`),
			always(`, LOWER(y.logical_type) AS "kind"`),
			always(`, COALESCE(LIST_REDUCE(y.labels, (a, b) -> a || ', ' || b), '') AS "elements"`),
			always(`, '' AS "owner"`),
			always(`, NULL AS "access"`),
			always(`, y.comment AS "comment"`),
			always(`FROM duckdb_types() y`),
			always(`WHERE ` + internalOf("y")),
			always(`AND (@schema = '' OR y.schema_name LIKE @schema)`),
			always(`AND (@name = '' OR y.type_name LIKE @name)`),
			always(`ORDER BY 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{Name: "internal", Desc: "the same as the name: DuckDB has no separate internal name"},
			{Name: "kind", Desc: "the logical type, such as enum or struct"},
			{Name: "elements", Desc: "the labels of an enum, joined. EnumValues has them as rows"},
			{Name: "owner", Desc: "always empty: DuckDB has no users"},
			{Name: "access", Desc: "always absent: DuckDB has no grants"},
			{Name: "comment"},
		},
		Params: schemaNameSystem("type"),
		Scan: func(rows *sql.Rows) (dbmeta.Type, error) {
			var v dbmeta.Type
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Internal, &v.Kind,
				&v.Elements, &v.Owner, &v.Access, &v.Comment)
			return v, err
		},
	})

	dbmeta.EnumValues.Register(dbmeta.DuckDB, &dbmeta.Binding[dbmeta.EnumValue]{
		Stmt: dbmeta.Stmt{
			always(`SELECT y.database_name AS "catalog"`),
			always(`, y.schema_name AS "schema"`),
			always(`, y.type_name AS "enum"`),
			always(`, e.label AS "label"`),
			always(`, e.ordinality AS "ordinal"`),
			always(`FROM duckdb_types() y, unnest(y.labels) WITH ORDINALITY AS e(label, ordinality)`),
			always(`WHERE ` + internalOf("y")),
			always(`AND (@schema = '' OR y.schema_name LIKE @schema)`),
			always(`AND (@name = '' OR y.type_name LIKE @name)`),
			always(`ORDER BY 2, 3, 5`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"},
			{Name: "enum", Desc: "the enumerated type this label belongs to"},
			{Name: "label"},
			{Name: "ordinal", Desc: "one based position in the declaration"},
		},
		Params: schemaNameSystem("enum"),
		Scan: func(rows *sql.Rows) (dbmeta.EnumValue, error) {
			var v dbmeta.EnumValue
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Enum, &v.Label, &v.Ordinal)
			return v, err
		},
	})

	// \dO. DuckDB ships the ICU collations and names nothing else about them.
	dbmeta.Collations.Register(dbmeta.DuckDB, &dbmeta.Binding[dbmeta.Collation]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "schema"`),
			always(`, c.collname AS "name"`),
			always(`, NULL AS "provider"`),
			always(`, c.collname AS "collate"`),
			always(`, c.collname AS "ctype"`),
			always(`, NULL AS "locale"`),
			always(`, TRUE AS "deterministic"`),
			always(`, NULL AS "comment"`),
			always(`FROM pragma_collations() c`),
			always(`WHERE (@name = '' OR c.collname LIKE @name)`),
			always(`ORDER BY 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "always empty: a DuckDB collation belongs to the library"},
			{Name: "name"},
			{Name: "provider", Desc: "always absent: DuckDB does not name the provider"},
			{Name: "collate"}, {Name: "ctype"},
			{Name: "locale", Desc: "always absent: the name is the locale"},
			{Name: "deterministic", Desc: "always true"},
			{Name: "comment", Desc: "always absent"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "collation name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Collation, error) {
			var v dbmeta.Collation
			err := rows.Scan(&v.Schema, &v.Name, &v.Provider, &v.Collate, &v.CType,
				&v.Locale, &v.Deterministic, &v.Comment)
			return v, err
		},
	})
}

func registerSettings() {
	// \dconfig. DuckDB publishes the value and the scope, which is more than
	// most databases here manage.
	dbmeta.Settings.Register(dbmeta.DuckDB, &dbmeta.Binding[dbmeta.Setting]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.name AS "name"`),
			always(`, s.value AS "value"`),
			always(`, s.input_type AS "type"`),
			always(`, LOWER(s.scope) AS "context"`),
			always(`, NULL AS "access"`),
			always(`FROM duckdb_settings() s`),
			always(`WHERE (@name = '' OR s.name LIKE @name)`),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"}, {Name: "value"},
			{Name: "type", Desc: "the type the setting accepts, such as VARCHAR or BOOLEAN"},
			{Name: "context", Desc: "global or local"},
			{Name: "access", Desc: "always absent: DuckDB has no grants"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "setting name pattern, empty for every setting", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Setting, error) {
			var v dbmeta.Setting
			err := rows.Scan(&v.Name, &v.Value, &v.Type, &v.Context, &v.Access)
			return v, err
		},
	})

	// \dx. A DuckDB extension is installed and loaded separately, which is
	// closer to a PostgreSQL extension than anything else here manages.
	dbmeta.Extensions.Register(dbmeta.DuckDB, &dbmeta.Binding[dbmeta.Extension]{
		Stmt: dbmeta.Stmt{
			always(`SELECT e.extension_name AS "name"`),
			always(`, COALESCE(e.extension_version, '') AS "version"`),
			always(`, '' AS "schema"`),
			always(`, e.description AS "comment"`),
			always(`FROM duckdb_extensions() e`),
			always(`WHERE (@with_system OR e.loaded)`),
			always(`AND (@name = '' OR e.extension_name LIKE @name)`),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "version", Desc: "empty for an extension that is installed and not loaded"},
			{Name: "schema", Desc: "always empty: a DuckDB extension is not in a schema"},
			{Name: "comment", Desc: "the description DuckDB ships"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "extension name pattern, empty for every one", Default: ""},
			{
				Name:    "with_system",
				Desc:    "include the extensions that are installed and not loaded",
				Default: false,
			},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Extension, error) {
			var v dbmeta.Extension
			err := rows.Scan(&v.Name, &v.Version, &v.Schema, &v.Comment)
			return v, err
		},
	})

	dbmeta.CurrentSchema.Register(dbmeta.DuckDB, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_database() AS "catalog"`),
			always(`, current_schema() AS "name"`),
			always(`, '' AS "owner"`),
			always(`, NULL AS "comment"`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "the attached database in use"},
			{Name: "name", Desc: "the schema an unqualified name resolves in"},
			{Name: "owner", Desc: "always empty: DuckDB has no users"},
			{Name: "comment", Desc: "always absent"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})
}
