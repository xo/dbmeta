package neo4j

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// functionColumns is the select list of a function row, read from the
// variables name, kind, result, args, description, builtin and roles.
var functionColumns = "'' AS `catalog`, '' AS `schema`, name AS `name`, NULL AS `id`, kind AS `kind`" +
	", result AS `result_type`, " + argTypes + " AS `arg_types`" +
	", '' AS `volatility`, '' AS `parallel`, NULL AS `owner`, '' AS `security`" +
	", CASE WHEN roles IS NULL THEN NULL ELSE " + join("roles", ", ") + " END AS `access`" +
	", CASE WHEN kind = 'proc' THEN '' WHEN builtin THEN 'internal' ELSE 'java' END AS `language`" +
	", NULL AS `source`, description AS `comment`, NULL AS `definition`"

// argTypes is the types of the arguments in args, joined by commas.
var argTypes = join("[a IN args | a.type]", ", ")

// functionFields describes what Functions and Aggregates return.
func functionFields(kind string) []dbmeta.Field {
	return []dbmeta.Field{
		{Name: "catalog", Desc: "always empty: a " + kind + " belongs to the server and not to a database"},
		{Name: "schema", Desc: "always empty, for the same reason. The namespace is part of the name, such as db in db.labels"},
		{Name: "name", Desc: "the whole name, with its namespace"},
		{Name: "id", Desc: "always absent: Neo4j does not overload a name, so the name identifies the " + kind},
		{Name: "kind", Desc: "func, agg for an aggregating function, or proc for a procedure"},
		{
			Name: "result_type",
			Desc: "the type a function returns, and for a procedure each column it yields," +
				" as name :: TYPE joined by commas",
		},
		{Name: "arg_types", Desc: "the types of the arguments, in order, joined by commas"},
		{Name: "volatility", Desc: "always empty: Neo4j marks no volatility"},
		{Name: "parallel", Desc: "always empty: Neo4j marks no parallel safety"},
		{Name: "owner", Desc: "always absent: a " + kind + " has no owner"},
		{Name: "security", Desc: "always empty: Neo4j has no security mode for a " + kind},
		{
			Name: "access",
			Desc: "the roles that can run it, joined by commas. Absent for a user who may not" +
				" see the roles, because SHOW FUNCTIONS and SHOW PROCEDURES then report none",
		},
		{
			Name: "language",
			Desc: "internal for a built in function, java for a user defined one, which is a" +
				" Java plugin, and empty for a procedure, because SHOW PROCEDURES does not say" +
				" which are built in",
		},
		{Name: "source", Desc: "always absent: a " + kind + " is compiled Java and Neo4j keeps no source"},
		{Name: "comment", Desc: "the description Neo4j ships for it"},
		{Name: "definition", Desc: "always absent, for the same reason as source"},
	}
}

// functionParams is what Functions and Aggregates take.
func functionParams(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern. A " + kind + " has no schema, so only a pattern that matches the empty string finds one", Default: ""},
		{Name: "name", Desc: kind + " name pattern, with its namespace, empty for every one", Default: ""},
		{
			Name: "with_system",
			Desc: "include the built in functions. A procedure is always included, because" +
				" SHOW PROCEDURES does not say which are built in",
			Default: false,
		},
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
	// \df. SHOW FUNCTIONS and SHOW PROCEDURES, joined, which needs
	// 2026.05.
	dbmeta.Functions.Register(dbmeta.Neo4j, &dbmeta.Binding[dbmeta.Function]{
		Stmt: composed("CALL () {" +
			" SHOW FUNCTIONS YIELD name, description, isBuiltIn, argumentDescription, returnDescription," +
			" aggregating, rolesExecution" +
			" RETURN name, CASE WHEN aggregating THEN 'agg' ELSE 'func' END AS kind," +
			" returnDescription AS result, argumentDescription AS args, description," +
			" isBuiltIn AS builtin, rolesExecution AS roles" +
			" UNION ALL" +
			" SHOW PROCEDURES YIELD name, description, argumentDescription, returnDescription, rolesExecution" +
			" RETURN name, 'proc' AS kind," +
			" " + join("[o IN returnDescription | o.name + ' :: ' + o.type]", ", ") + " AS result," +
			" argumentDescription AS args, description, false AS builtin, rolesExecution AS roles }" +
			" WITH * WHERE (@with_system OR NOT builtin)" +
			" AND " + like("''", "@schema") + " AND " + like("name", "@name") +
			" RETURN " + functionColumns +
			" ORDER BY `name`, `kind`"),
		Fields: functionFields("function"),
		Params: functionParams("function"),
		Scan:   scanFunction,
	})

	// \da. SHOW FUNCTIONS alone, which every release reads.
	dbmeta.Aggregates.Register(dbmeta.Neo4j, &dbmeta.Binding[dbmeta.Function]{
		Stmt: both("SHOW FUNCTIONS YIELD name, description, isBuiltIn, argumentDescription," +
			" returnDescription, aggregating, rolesExecution" +
			" WHERE aggregating AND (@with_system OR NOT isBuiltIn)" +
			" AND " + like("''", "@schema") + " AND " + like("name", "@name") +
			" RETURN '' AS `catalog`, '' AS `schema`, name AS `name`, NULL AS `id`, 'agg' AS `kind`" +
			", returnDescription AS `result_type`" +
			", " + join("[a IN argumentDescription | a.type]", ", ") + " AS `arg_types`" +
			", '' AS `volatility`, '' AS `parallel`, NULL AS `owner`, '' AS `security`" +
			", CASE WHEN rolesExecution IS NULL THEN NULL" +
			" ELSE " + join("rolesExecution", ", ") + " END AS `access`" +
			", CASE WHEN isBuiltIn THEN 'internal' ELSE 'java' END AS `language`" +
			", NULL AS `source`, description AS `comment`, NULL AS `definition`" +
			" ORDER BY `name`"),
		Fields: functionFields("aggregate"),
		Params: functionParams("aggregate"),
		Scan:   scanFunction,
	})

	// The arguments of each function and procedure. A function's return
	// value reads mode return and ordinal 0. A procedure yields columns,
	// which read mode table and follow its arguments, as a column of a
	// returned table does on PostgreSQL.
	dbmeta.RoutineParameters.Register(dbmeta.Neo4j, &dbmeta.Binding[dbmeta.RoutineParameter]{
		Stmt: composed("CALL () {" +
			" SHOW FUNCTIONS YIELD name, isBuiltIn, argumentDescription AS args, returnDescription" +
			" UNWIND range(0, size(args)) AS i" +
			" RETURN name AS routine, isBuiltIn AS builtin," +
			" CASE WHEN i = 0 THEN NULL ELSE args[i - 1].name END AS pname, i AS ordinal," +
			" CASE WHEN i = 0 THEN 'return' ELSE 'in' END AS mode," +
			" CASE WHEN i = 0 THEN returnDescription ELSE args[i - 1].type END AS dtype," +
			" CASE WHEN i = 0 THEN NULL ELSE args[i - 1]['default'] END AS dflt" +
			" UNION ALL" +
			" SHOW PROCEDURES YIELD name, argumentDescription, returnDescription" +
			" WITH name, argumentDescription + returnDescription AS ps, size(argumentDescription) AS n" +
			" UNWIND range(1, size(ps)) AS i" +
			" RETURN name AS routine, false AS builtin, ps[i - 1].name AS pname, i AS ordinal," +
			" CASE WHEN i <= n THEN 'in' ELSE 'table' END AS mode, ps[i - 1].type AS dtype," +
			" ps[i - 1]['default'] AS dflt }" +
			" WITH * WHERE (@with_system OR NOT builtin)" +
			" AND " + like("''", "@schema") + " AND " + like("routine", "@parent") + " AND " + like("pname", "@name") +
			" RETURN '' AS `catalog`, '' AS `schema`, routine AS `routine`, NULL AS `routine_id`" +
			", pname AS `name`, ordinal AS `ordinal`, mode AS `mode`, dtype AS `data_type`, dflt AS `default`" +
			" ORDER BY `routine`, `ordinal`"),
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: a routine belongs to the server and not to a database"},
			{Name: "schema", Desc: "always empty, for the same reason"},
			{Name: "routine", Desc: "the whole name of the function or the procedure, with its namespace"},
			{Name: "routine_id", Desc: "always absent: Neo4j does not overload a name"},
			{Name: "name", Desc: "the argument or the column, and absent for the return value of a function"},
			{Name: "ordinal", Desc: "one based, and zero for the return value of a function"},
			{Name: "mode", Desc: "in, return for the return value of a function, or table for a column a procedure yields"},
			{Name: "data_type", Desc: "the Cypher type, such as STRING or LIST<ANY>"},
			{
				Name: "default",
				Desc: "the default of an optional argument, as Neo4j writes it, such as" +
					" DefaultParameterValue{value={}, type=MAP}",
			},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern. A routine has no schema, so only a pattern that matches the empty string finds one", Default: ""},
			{Name: "parent", Desc: "routine name pattern, with its namespace, empty for every routine", Default: ""},
			{Name: "name", Desc: "argument name pattern, empty for every argument", Default: ""},
			{
				Name: "with_system",
				Desc: "include the arguments of the built in functions. Those of a procedure" +
					" are always included, because SHOW PROCEDURES does not say which are built in",
				Default: false,
			},
		},
		Scan: func(rows *sql.Rows) (dbmeta.RoutineParameter, error) {
			var v dbmeta.RoutineParameter
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Routine, &v.RoutineID, &v.Name,
				&v.Ordinal, &v.Mode, &v.DataType, &v.Default)
			return v, err
		},
	})
}
