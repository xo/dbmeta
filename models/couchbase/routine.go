package couchbase

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// The bucket and the scope of a function. A global function has neither, and
// its identity says so with the type global.
const (
	funcCatalog = "IFMISSING(f.identity.`bucket`, '')"
	funcSchema  = "IFMISSING(f.identity.`scope`, '')"
)

// funcParams is the parameter names of a function. A function with none has
// the empty list, and a variadic function has no parameters field at all,
// measured on 7.6.12, so a missing list is the one parameter ..., which is
// how CREATE FUNCTION writes it.
const funcParams = "IFMISSING(f.definition.parameters, ['...'])"

func registerRoutines() {
	// \df. A SQL++ function is inline, whose body is one expression, or
	// JavaScript, whose body is an object in a library. Its parameters have
	// names and no types.
	dbmeta.Functions.Register(dbmeta.Couchbase, &dbmeta.Binding[dbmeta.Function]{
		Stmt: dbmeta.Stmt{
			from76("SELECT " + funcCatalog + " AS `catalog`"),
			from76(", " + funcSchema + " AS `schema`"),
			from76(", f.identity.name AS `name`"),
			from76(", NULL AS `id`"),
			from76(", 'function' AS `kind`"),
			from76(", '' AS `result_type`"),
			from76(", CONCAT2(', ', " + funcParams + ") AS `arg_types`"),
			from76(", '' AS `volatility`"),
			from76(", '' AS `parallel`"),
			from76(", '' AS `owner`"),
			from76(", '' AS `security`"),
			from76(", NULL AS `access`"),
			from76(", f.definition.`#language` AS `language`"),
			from76(", IFMISSING(f.definition.text, NULL) AS `source`"),
			from76(", NULL AS `comment`"),
			from76("FROM system:functions f"),
			from76("WHERE " + like(funcSchema, "@schema")),
			from76("AND " + like("f.identity.name", "@name")),
			from76("ORDER BY 1, 2, 3"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "the bucket, and empty for a global function"},
			{Name: "schema", Desc: "the scope, and empty for a global function"},
			{Name: "name"},
			{Name: "id", Desc: "always absent: a function has one name and no id"},
			{Name: "kind", Desc: "always function: SQL++ has no procedure or aggregate of its own"},
			{Name: "result_type", Desc: "always empty: a SQL++ function returns any JSON value and declares no type"},
			{
				Name: "arg_types",
				Desc: "the parameter names, in order. A parameter has no type," +
					" so the names are what psql's argument list would hold. A" +
					" variadic function has the one parameter ...",
			},
			{Name: "volatility", Desc: "always empty: SQL++ marks no volatility"},
			{Name: "parallel", Desc: "always empty: SQL++ has no parallel safety mark"},
			{Name: "owner", Desc: "always empty: a function has no owner"},
			{Name: "security", Desc: "always empty: SQL++ has no SECURITY DEFINER"},
			{Name: "access", Desc: "always absent: privileges returns the roles that run functions"},
			{Name: "language", Desc: "inline or javascript"},
			{
				Name: "source",
				Desc: "the text of an inline function. A JavaScript function's" +
					" body is in a library, which the catalog names and does not hold",
			},
			{Name: "comment", Desc: "always absent: a function carries no comment"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "scope name pattern, empty for every scope", Default: ""},
			{Name: "name", Desc: "function name pattern, empty for every function", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Function, error) {
			var v dbmeta.Function
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.ID, &v.Kind,
				&v.ResultType, &v.ArgTypes, &v.Volatility, &v.Parallel, &v.Owner,
				&v.Security, &v.Access, &v.Language, &v.Source, &v.Comment)
			return v, err
		},
	})

	// The parameters of a function, one row each, in order.
	dbmeta.RoutineParameters.Register(dbmeta.Couchbase, &dbmeta.Binding[dbmeta.RoutineParameter]{
		Stmt: dbmeta.Stmt{
			from76("SELECT " + funcCatalog + " AS `catalog`"),
			from76(", " + funcSchema + " AS `schema`"),
			from76(", f.identity.name AS `routine`"),
			from76(", NULL AS `routine_id`"),
			from76(", p.name AS `name`"),
			from76(", p.pos + 1 AS `ordinal`"),
			from76(", 'IN' AS `mode`"),
			from76(", '' AS `data_type`"),
			from76(", NULL AS `default`"),
			from76("FROM system:functions f"),
			from76("UNNEST ARRAY {\"pos\": i, \"name\": v} FOR i:v IN " + funcParams + " END AS p"),
			from76("WHERE " + like(funcSchema, "@schema")),
			from76("AND " + like("f.identity.name", "@name")),
			from76("ORDER BY 1, 2, 3, 6"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "the bucket, and empty for a global function"},
			{Name: "schema", Desc: "the scope, and empty for a global function"},
			{Name: "routine"},
			{Name: "routine_id", Desc: "always absent: a function has no id"},
			{Name: "name", Desc: "the parameter name, which is ... for a variadic function"},
			{Name: "ordinal", Desc: "the position of the parameter, from 1"},
			{Name: "mode", Desc: "always IN: a SQL++ function returns one value and has no OUT parameter"},
			{Name: "data_type", Desc: "always empty: a parameter has no type"},
			{Name: "default", Desc: "always absent: a parameter has no default"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "scope name pattern, empty for every scope", Default: ""},
			{Name: "name", Desc: "function name pattern, empty for every function", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.RoutineParameter, error) {
			var v dbmeta.RoutineParameter
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Routine, &v.RoutineID, &v.Name,
				&v.Ordinal, &v.Mode, &v.DataType, &v.Default)
			return v, err
		},
	})
}
