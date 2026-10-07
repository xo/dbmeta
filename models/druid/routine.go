package druid

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// functionStmt builds the statement behind Functions and Aggregates.
//
// ROUTINES has one row for each function name, and no table lists the
// parameters. SIGNATURES holds the overloads as text, one on each line.
func functionStmt(aggregate bool) dbmeta.Stmt {
	kind, filter := `CASE r.IS_AGGREGATOR WHEN 'YES' THEN 'agg' ELSE 'func' END`, ``
	if aggregate {
		kind, filter = `'agg'`, ` AND r.IS_AGGREGATOR = 'YES'`
	}
	return dbmeta.Stmt{
		always(`SELECT r.ROUTINE_CATALOG AS "catalog"`),
		always(`, r.ROUTINE_SCHEMA AS "schema"`),
		always(`, r.ROUTINE_NAME AS "name"`),
		always(`, NULL AS "id"`),
		always(`, ` + kind + ` AS "kind"`),
		always(`, NULL AS "result_type"`),
		always(`, r.SIGNATURES AS "arg_types"`),
		always(`, '' AS "volatility"`),
		always(`, '' AS "parallel"`),
		always(`, NULL AS "owner"`),
		always(`, '' AS "security"`),
		always(`, NULL AS "access"`),
		always(`, 'internal' AS "language"`),
		always(`, NULL AS "source"`),
		always(`, NULL AS "comment"`),
		always(`, NULL AS "definition"`),
		always(`FROM INFORMATION_SCHEMA.ROUTINES r`),
		always(`WHERE @with_system` + filter),
		always(`AND ` + like("r.ROUTINE_SCHEMA", "@schema")),
		always(`AND ` + like("r.ROUTINE_NAME", "@name")),
		always(`ORDER BY 3`),
	}
}

// functionFields describes what both routine queries return.
func functionFields(kind string) []dbmeta.Field {
	return []dbmeta.Field{
		{Name: "catalog", Desc: catalogDesc},
		{Name: "schema", Desc: "always INFORMATION_SCHEMA, where ROUTINES lists every function"},
		{Name: "name"},
		{Name: "id", Desc: "always absent: ROUTINES has one row for each name, and Druid has no parameter table"},
		{Name: "kind", Desc: "func or agg, from IS_AGGREGATOR"},
		{Name: "result_type", Desc: "always absent: ROUTINES records no return type"},
		{Name: "arg_types", Desc: "the signatures of the " + kind + ", one on each line, as Druid spells them, such as 'MAX(<COMPARABLE_TYPE>)'. They hold the argument types and the name, and they are text and not a list"},
		{Name: "volatility", Desc: "always empty: ROUTINES records no volatility"},
		{Name: "parallel", Desc: "always empty: Druid marks no parallel safety"},
		{Name: "owner", Desc: "always absent: a built in " + kind + " has no owner"},
		{Name: "security", Desc: "always empty: a built in " + kind + " has no security mode"},
		{Name: "access", Desc: "always absent: Druid grants nothing on a " + kind},
		{Name: "language", Desc: "always internal: every " + kind + " is built into Druid, which has no user defined function"},
		{Name: "source", Desc: "always absent: a built in " + kind + " has no source text"},
		{Name: "comment", Desc: "always absent: ROUTINES records no description"},
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
	// Druid has no CREATE FUNCTION, so with_system must be set to list any.
	params := func(kind string) []dbmeta.Param {
		return []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
			{
				Name:    "with_system",
				Desc:    "include the functions built into Druid, which are every one there is",
				Default: false,
			},
		}
	}
	dbmeta.Functions.Register(dbmeta.Druid, &dbmeta.Binding[dbmeta.Function]{
		Stmt:   functionStmt(false),
		Fields: functionFields("function"),
		Params: params("function"),
		Scan:   scanFunction,
	})
	dbmeta.Aggregates.Register(dbmeta.Druid, &dbmeta.Binding[dbmeta.Function]{
		Stmt:   functionStmt(true),
		Fields: functionFields("aggregate"),
		Params: params("aggregate"),
		Scan:   scanFunction,
	})
}
