package surrealdb

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// functions is every function of the database. A function is called as
// fn::name, and INFO names it without the fn::.
const functions = "(INFO FOR DB STRUCTURE).functions"

func registerRoutines() {
	// \df. A function is written in SurrealQL, and its parameters and its
	// result have types. The definition is the DEFINE FUNCTION statement
	// that INFO FOR DB returns without STRUCTURE.
	dbmeta.Functions.Register(dbmeta.SurrealDB, &dbmeta.Binding[dbmeta.Function]{
		Stmt: from3("SELECT access, arg_types, " + currentNS + " AS catalog, comment, definition, NULL AS id" +
			", 'function' AS kind, 'surrealql' AS language, name, NULL AS owner, '' AS parallel, result_type" +
			", " + currentDB + " AS schema, '' AS security, source, '' AS volatility" +
			" FROM array::map(" + functions + ", |$x| {name: $x.name, comment: $x.comment, source: $x.block," +
			" result_type: $x.returns, access: " + permission("$x.permissions") + "," +
			" arg_types: array::join(array::map($x.args, |$a| $a[1]), ', ')," +
			" definition: (INFO FOR DB).functions[$x.name]})" +
			" WHERE " + like(currentDB, "@schema") +
			" AND " + like("name", "@name") +
			" ORDER BY name"),
		Fields: []dbmeta.Field{
			{Name: "access", Desc: "the PERMISSIONS of the function, as FULL, NONE or WHERE and an expression, which a record user is held to"},
			{Name: "arg_types", Desc: "the types of the parameters, in order, joined by commas"},
			{Name: "catalog", Desc: "the namespace"},
			{Name: "comment"},
			{Name: "definition", Desc: "the DEFINE FUNCTION statement"},
			{Name: "id", Desc: "always absent: a function has one name and no id"},
			{Name: "kind", Desc: "always function: SurrealQL has no procedure and no aggregate of its own"},
			{Name: "language", Desc: "always surrealql"},
			{Name: "name", Desc: "the function, without the fn:: that a call writes before it"},
			{Name: "owner", Desc: "always absent: a function has no owner"},
			{Name: "parallel", Desc: "always empty: SurrealQL marks no parallel safety"},
			{Name: "result_type", Desc: "the type after ->, and absent for a function that declares none"},
			{Name: "schema", Desc: "the database"},
			{Name: "security", Desc: "always empty: SurrealQL has no SECURITY DEFINER"},
			{Name: "source", Desc: "the body of the function, between braces"},
			{Name: "volatility", Desc: "always empty: SurrealQL marks no volatility"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "database name pattern, empty for the database of the connection", Default: ""},
			{Name: "name", Desc: "function name pattern, empty for every function", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Function, error) {
			var v dbmeta.Function
			err := rows.Scan(&v.Access, &v.ArgTypes, &v.Catalog, &v.Comment, &v.Definition,
				&v.ID, &v.Kind, &v.Language, &v.Name, &v.Owner, &v.Parallel, &v.ResultType,
				&v.Schema, &v.Security, &v.Source, &v.Volatility)
			return v, err
		},
	})

	// The parameters of a function, one row each, in order.
	dbmeta.RoutineParameters.Register(dbmeta.SurrealDB, &dbmeta.Binding[dbmeta.RoutineParameter]{
		Stmt: from3("SELECT " + currentNS + " AS catalog, data_type, NULL AS default, 'IN' AS mode, name" +
			", ordinal, routine, NULL AS routine_id, " + currentDB + " AS schema" +
			" FROM array::flatten(array::map(" + functions + ", |$f| array::map($f.args, |$a, $n|" +
			" {routine: $f.name, name: $a[0], data_type: $a[1], ordinal: $n + 1})))" +
			" WHERE " + like(currentDB, "@schema") +
			" AND " + like("routine", "@parent") +
			" AND " + like("name", "@name") +
			" ORDER BY routine, ordinal"),
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "the namespace"},
			{Name: "data_type", Desc: "the type of the parameter"},
			{Name: "default", Desc: "always absent: a parameter has no default"},
			{Name: "mode", Desc: "always IN: a function returns one value and has no OUT parameter"},
			{Name: "name", Desc: "the parameter, without the $ that the body writes before it"},
			{Name: "ordinal", Desc: "the position of the parameter, from 1"},
			{Name: "routine", Desc: "the function, without the fn::"},
			{Name: "routine_id", Desc: "always absent: a function has no id"},
			{Name: "schema", Desc: "the database"},
		},
		Params: []dbmeta.Param{
			{Name: "parent", Desc: "function name pattern, empty for every function", Default: ""},
			{Name: "schema", Desc: "database name pattern, empty for the database of the connection", Default: ""},
			{Name: "name", Desc: "parameter name pattern, empty for every parameter", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.RoutineParameter, error) {
			var v dbmeta.RoutineParameter
			err := rows.Scan(&v.Catalog, &v.DataType, &v.Default, &v.Mode, &v.Name,
				&v.Ordinal, &v.Routine, &v.RoutineID, &v.Schema)
			return v, err
		},
	})
}
