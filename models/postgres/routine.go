package postgres

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// Routines, types and operators.

func init() {
	registerFunctions()
	registerAggregates()
	registerTypes()
	registerDomains()
	registerOperators()
}

// functionStmt builds the statement behind \df and \da, from
// describeFunctions. kindFilter narrows to one kind, or is empty for all.
//
// Release 11 replaced pg_proc.proisagg and pg_proc.proiswindow with the single
// prokind column, which is where all eight of the version gates in
// describeFunctions come from. Both alternatives produce the same kind names,
// so the column set does not change.
func functionStmt(kindFilter string) dbmeta.Stmt {
	kindOld := `, CASE WHEN p.proisagg THEN 'agg'` +
		` WHEN p.proiswindow THEN 'window'` +
		` WHEN p.prorettype = 'pg_catalog.trigger'::pg_catalog.regtype THEN 'trigger'` +
		` ELSE 'func' END AS "kind"`
	kindNew := `, CASE p.prokind WHEN 'a' THEN 'agg' WHEN 'w' THEN 'window'` +
		` WHEN 'p' THEN 'proc'` +
		` WHEN 'f' THEN CASE WHEN p.prorettype = 'pg_catalog.trigger'::pg_catalog.regtype` +
		` THEN 'trigger' ELSE 'func' END` +
		` ELSE p.prokind::text END AS "kind"`
	stmt := dbmeta.Stmt{
		{{Query: `SELECT current_database() AS "catalog"`}},
		{{Query: `, n.nspname AS "schema"`}},
		{{Query: `, p.proname AS "name"`}},
		// The oid, so that RoutineParameters can be joined back. PostgreSQL
		// overloads a name, so the name alone does not identify a routine.
		{{Query: `, p.oid::text AS "id"`}},
		{
			{Query: kindOld},
			{Min: v11, Query: kindNew},
		},
		{{Query: `, pg_catalog.pg_get_function_result(p.oid) AS "result_type"`}},
		{{Query: `, pg_catalog.pg_get_function_arguments(p.oid) AS "arg_types"`}},
		{{Query: `, CASE p.provolatile WHEN 'i' THEN 'immutable' WHEN 's' THEN 'stable'` +
			` WHEN 'v' THEN 'volatile' ELSE '' END AS "volatility"`}},
		{{Query: `, CASE p.proparallel WHEN 'r' THEN 'restricted' WHEN 's' THEN 'safe'` +
			` WHEN 'u' THEN 'unsafe' ELSE '' END AS "parallel"`}},
		{{Query: `, pg_catalog.pg_get_userbyid(p.proowner) AS "owner"`}},
		{{Query: `, CASE WHEN p.prosecdef THEN 'definer' ELSE 'invoker' END AS "security"`}},
		{{Query: `, pg_catalog.array_to_string(p.proacl, E'\n') AS "access"`}},
		{{Query: `, l.lanname AS "language"`}},
		{{Query: `, CASE WHEN l.lanname IN ('internal', 'c') THEN p.prosrc END AS "source"`}},
		{{Query: `, pg_catalog.obj_description(p.oid, 'pg_proc') AS "comment"`}},
		{{Query: `FROM pg_catalog.pg_proc p`}},
		{{Query: `LEFT JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace`}},
		{{Query: `LEFT JOIN pg_catalog.pg_language l ON l.oid = p.prolang`}},
		{{Query: `WHERE (@with_system OR (n.nspname <> 'pg_catalog' AND n.nspname <> 'information_schema'))`}},
	}
	if kindFilter != "" {
		stmt = append(stmt, dbmeta.Choice{
			{Query: `AND p.proisagg`},
			{Min: v11, Query: `AND p.prokind = '` + kindFilter + `'`},
		})
	}
	return append(stmt,
		dbmeta.Choice{{Query: `AND (@schema = '' OR n.nspname LIKE @schema)`}},
		dbmeta.Choice{{Query: `AND (@name = '' OR p.proname LIKE @name)`}},
		dbmeta.Choice{{Query: `ORDER BY 2, 3, 6`}},
	)
}

func functionFields() []dbmeta.Field {
	return fields("catalog", "schema", "name", "id", "kind", "result_type", "arg_types",
		"volatility", "parallel", "owner", "security", "access", "language", "source", "comment")
}

func scanFunction(rows *sql.Rows) (dbmeta.Function, error) {
	var v dbmeta.Function
	err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.ID, &v.Kind, &v.ResultType, &v.ArgTypes,
		&v.Volatility, &v.Parallel, &v.Owner, &v.Security, &v.Access, &v.Language,
		&v.Source, &v.Comment)
	return v, err
}

// registerFunctions backs \df.
func registerFunctions() {
	dbmeta.Functions.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.Function]{
		Stmt:   functionStmt(""),
		Fields: functionFields(),
		Params: schemaNameSystem("function"),
		Scan:   scanFunction,
	})
}

// registerAggregates backs \da, from describeAggregates. It is the function
// query narrowed to one kind, which is how psql writes it too.
func registerAggregates() {
	dbmeta.Aggregates.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.Function]{
		Stmt:   functionStmt("a"),
		Fields: functionFields(),
		Params: schemaNameSystem("aggregate"),
		Scan:   scanFunction,
	})
}

// registerTypes backs \dT, from describeTypes.
func registerTypes() {
	dbmeta.Types.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.Type]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT current_database() AS "catalog"`}},
			{{Query: `, n.nspname AS "schema"`}},
			{{Query: `, pg_catalog.format_type(t.oid, NULL) AS "name"`}},
			{{Query: `, t.typname AS "internal"`}},
			{{Query: `, CASE t.typtype WHEN 'b' THEN 'base' WHEN 'c' THEN 'composite'` +
				` WHEN 'd' THEN 'domain' WHEN 'e' THEN 'enum' WHEN 'p' THEN 'pseudo'` +
				` WHEN 'r' THEN 'range' WHEN 'm' THEN 'multirange'` +
				` ELSE t.typtype::text END AS "kind"`}},
			{{Query: `, COALESCE((SELECT pg_catalog.string_agg(e.enumlabel, ', '` +
				` ORDER BY e.enumsortorder) FROM pg_catalog.pg_enum e` +
				` WHERE e.enumtypid = t.oid), '') AS "elements"`}},
			{{Query: `, pg_catalog.pg_get_userbyid(t.typowner) AS "owner"`}},
			{{Query: `, pg_catalog.array_to_string(t.typacl, E'\n') AS "access"`}},
			{{Query: `, pg_catalog.obj_description(t.oid, 'pg_type') AS "comment"`}},
			{{Query: `FROM pg_catalog.pg_type t`}},
			{{Query: `LEFT JOIN pg_catalog.pg_namespace n ON n.oid = t.typnamespace`}},
			// leave out the composite types that back a table, and the array
			// type that every scalar type creates
			{{Query: `WHERE (t.typrelid = 0 OR (SELECT c.relkind = 'c' FROM pg_catalog.pg_class c WHERE c.oid = t.typrelid))`}},
			{{Query: `AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_type el WHERE el.oid = t.typelem AND el.typarray = t.oid)`}},
			{{Query: `AND (@with_system OR (n.nspname <> 'pg_catalog' AND n.nspname <> 'information_schema'))`}},
			{{Query: `AND (@schema = '' OR n.nspname LIKE @schema)`}},
			{{Query: `AND (@name = '' OR t.typname LIKE @name OR pg_catalog.format_type(t.oid, NULL) LIKE @name)`}},
			{{Query: `ORDER BY 2, 3`}},
		},
		Fields: fields("catalog", "schema", "name", "internal", "kind", "elements", "owner", "access", "comment"),
		Params: schemaNameSystem("type"),
		Scan: func(rows *sql.Rows) (dbmeta.Type, error) {
			var v dbmeta.Type
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Internal, &v.Kind,
				&v.Elements, &v.Owner, &v.Access, &v.Comment)
			return v, err
		},
	})
}

// registerDomains backs \dD, from listDomains.
func registerDomains() {
	dbmeta.Domains.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.Domain]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT current_database() AS "catalog"`}},
			{{Query: `, n.nspname AS "schema"`}},
			{{Query: `, t.typname AS "name"`}},
			{{Query: `, pg_catalog.format_type(t.typbasetype, t.typtypmod) AS "data_type"`}},
			{{Query: `, COALESCE((SELECT c.collname FROM pg_catalog.pg_collation c, pg_catalog.pg_type bt` +
				` WHERE c.oid = t.typcollation AND bt.oid = t.typbasetype` +
				` AND t.typcollation <> bt.typcollation), '') AS "collation"`}},
			{{Query: `, NOT t.typnotnull AS "nullable"`}},
			{{Query: `, t.typdefault AS "default"`}},
			{{Query: `, COALESCE((SELECT pg_catalog.string_agg(pg_catalog.pg_get_constraintdef(r.oid, true), ' '` +
				` ORDER BY r.conname) FROM pg_catalog.pg_constraint r WHERE t.oid = r.contypid), '') AS "constraints"`}},
			{{Query: `, pg_catalog.array_to_string(t.typacl, E'\n') AS "access"`}},
			{{Query: `, pg_catalog.obj_description(t.oid, 'pg_type') AS "comment"`}},
			{{Query: `FROM pg_catalog.pg_type t`}},
			{{Query: `LEFT JOIN pg_catalog.pg_namespace n ON n.oid = t.typnamespace`}},
			{{Query: `WHERE t.typtype = 'd'`}},
			{{Query: `AND (@with_system OR (n.nspname <> 'pg_catalog' AND n.nspname <> 'information_schema'))`}},
			{{Query: `AND (@schema = '' OR n.nspname LIKE @schema)`}},
			{{Query: `AND (@name = '' OR t.typname LIKE @name)`}},
			{{Query: `ORDER BY 2, 3`}},
		},
		Fields: fields("catalog", "schema", "name", "data_type", "collation", "nullable",
			"default", "constraints", "access", "comment"),
		Params: schemaNameSystem("domain"),
		Scan: func(rows *sql.Rows) (dbmeta.Domain, error) {
			var v dbmeta.Domain
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.DataType, &v.Collation,
				&v.Nullable, &v.Default, &v.Constraints, &v.Access, &v.Comment)
			return v, err
		},
	})
}

// registerOperators backs \do, from describeOperators.
//
// A prefix operator has no left type and a postfix operator has no right type,
// so both are reported as an empty string rather than as NULL.
func registerOperators() {
	dbmeta.Operators.Register(dbmeta.PostgreSQL, &dbmeta.Binding[dbmeta.Operator]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT n.nspname AS "schema"`}},
			{{Query: `, o.oprname AS "name"`}},
			{{Query: `, CASE WHEN o.oprkind = 'l' THEN '' ELSE pg_catalog.format_type(o.oprleft, NULL) END AS "left_type"`}},
			{{Query: `, CASE WHEN o.oprkind = 'r' THEN '' ELSE pg_catalog.format_type(o.oprright, NULL) END AS "right_type"`}},
			{{Query: `, pg_catalog.format_type(o.oprresult, NULL) AS "result_type"`}},
			{{Query: `, o.oprcode::text AS "function"`}},
			{{Query: `, COALESCE(pg_catalog.obj_description(o.oid, 'pg_operator'),` +
				` pg_catalog.obj_description(o.oprcode, 'pg_proc')) AS "comment"`}},
			{{Query: `FROM pg_catalog.pg_operator o`}},
			{{Query: `LEFT JOIN pg_catalog.pg_namespace n ON n.oid = o.oprnamespace`}},
			{{Query: `WHERE (@with_system OR (n.nspname <> 'pg_catalog' AND n.nspname <> 'information_schema'))`}},
			{{Query: `AND (@schema = '' OR n.nspname LIKE @schema)`}},
			{{Query: `AND (@name = '' OR o.oprname LIKE @name)`}},
			{{Query: `ORDER BY 1, 2, 3, 4`}},
		},
		Fields: fields("schema", "name", "left_type", "right_type", "result_type", "function", "comment"),
		Params: schemaNameSystem("operator"),
		Scan: func(rows *sql.Rows) (dbmeta.Operator, error) {
			var v dbmeta.Operator
			err := rows.Scan(&v.Schema, &v.Name, &v.LeftType, &v.RightType,
				&v.ResultType, &v.Function, &v.Comment)
			return v, err
		},
	})
}
