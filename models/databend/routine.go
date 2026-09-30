package databend

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// functionStmt lists the functions a user made, the procedures, and with the
// system objects the functions built into the server. aggregate is the value
// of is_aggregate that a row must have: false for Functions and true for
// Aggregates. A procedure is never an aggregate.
//
// A user defined function records its arguments as a variant, which is a
// list of names for a lambda and a list of types for any other language.
// A procedure records its signature as text.
func functionStmt(aggregate string) dbmeta.Stmt {
	return dbmeta.Stmt{
		always(`SELECT 'default' AS "catalog"`),
		always(`, '' AS "schema"`),
		always(`, f.name AS "name"`),
		always(`, NULL AS "id"`),
		always(`, CASE WHEN ` + aggregate + ` THEN 'agg' ELSE 'func' END AS "kind"`),
		always(`, NULL AS "result_type"`),
		always(`, CAST(f.arguments AS STRING) AS "arg_types"`),
		always(`, '' AS "volatility"`),
		always(`, '' AS "parallel"`),
		always(`, NULL AS "owner"`),
		always(`, '' AS "security"`),
		always(`, NULL AS "access"`),
		always(`, lower(f.language) AS "language"`),
		always(`, f.definition AS "source"`),
		always(`, NULLIF(f.description, '') AS "comment"`),
		always(`, NULL AS "definition"`),
		always(`FROM system.user_functions f`),
		always(`WHERE COALESCE(f.is_aggregate, false) = ` + aggregate),
		always(`AND ` + like("''", "@schema")),
		always(`AND ` + like("f.name", "@name")),
		always(`UNION ALL`),
		always(`SELECT 'default', '', p.name, CAST(p.procedure_id AS STRING), 'proc', NULL`),
		always(`, p.arguments, '', '', NULL, '', NULL, 'sql', NULL, NULLIF(p.comment, ''), NULL`),
		always(`FROM system.procedures p`),
		always(`WHERE NOT ` + aggregate),
		always(`AND ` + like("''", "@schema")),
		always(`AND ` + like("p.name", "@name")),
		always(`UNION ALL`),
		always(`SELECT 'default', 'system', b.name, NULL, CASE WHEN b.is_aggregate THEN 'agg' ELSE 'func' END, NULL`),
		always(`, b.syntax, '', '', NULL, '', NULL, 'internal', NULL, NULLIF(b.description, ''), NULL`),
		always(`FROM system.functions b`),
		always(`WHERE @with_system AND b.is_aggregate = ` + aggregate),
		always(`AND ` + like("'system'", "@schema")),
		always(`AND ` + like("b.name", "@name")),
		// Databend orders a union by a column name and not by a position.
		always(`ORDER BY "schema", "name"`),
	}
}

// functionFields are the fields of Functions and Aggregates.
var functionFields = []dbmeta.Field{
	{Name: "catalog", Desc: "always default, the one catalog a connection reaches"},
	{Name: "schema", Desc: "empty for a function or procedure a user made, which belongs to no database, and system for one built into the server"},
	{Name: "name"},
	{Name: "id", Desc: "the id of a procedure, and absent for a function, which Databend does not overload"},
	{Name: "kind", Desc: "func, agg or proc"},
	{Name: "result_type", Desc: "always absent: the return type is in the arguments or the signature"},
	{Name: "arg_types", Desc: "the arguments as Databend records them: a JSON object for a function a user made, the signature for a procedure, and the syntax for a built in function"},
	{Name: "volatility", Desc: "always empty: Databend does not record it"},
	{Name: "parallel", Desc: "always empty: Databend does not record it"},
	{Name: "owner", Desc: "always absent: system.user_functions records no owner"},
	{Name: "security", Desc: "always empty: Databend does not record it"},
	{Name: "access", Desc: "always absent: a grant is listed only by show_grants for one role"},
	{Name: "language", Desc: "sql, python, javascript or wasm, and internal for a built in function"},
	{Name: "source", Desc: "the definition of a function a user made, and absent for a procedure and a built in function"},
	{Name: "comment"},
	{Name: "definition", Desc: "always absent: Databend keeps the body of a function, which is source, and SHOW CREATE FUNCTION is a statement of its own"},
}

// functionParams are the parameters of Functions and Aggregates.
var functionParams = nameSystem("function",
	"include the functions built into the server, which are reported in the schema system")

func scanFunction(rows *sql.Rows) (dbmeta.Function, error) {
	var v dbmeta.Function
	err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.ID, &v.Kind, &v.ResultType,
		&v.ArgTypes, &v.Volatility, &v.Parallel, &v.Owner, &v.Security, &v.Access,
		&v.Language, &v.Source, &v.Comment, &v.Definition)
	return v, err
}

func registerRoutines() {
	// \df.
	dbmeta.Functions.Register(dbmeta.Databend, &dbmeta.Binding[dbmeta.Function]{
		Stmt:   functionStmt("false"),
		Fields: functionFields,
		Params: functionParams,
		Scan:   scanFunction,
	})
	// \da.
	dbmeta.Aggregates.Register(dbmeta.Databend, &dbmeta.Binding[dbmeta.Function]{
		Stmt:   functionStmt("true"),
		Fields: functionFields,
		Params: functionParams,
		Scan:   scanFunction,
	})
}
