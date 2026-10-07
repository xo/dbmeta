package drill

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerRoutines() {
	// sys.functions has one row for each overload. Every function that ships
	// with Drill has the source built-in, and a function from a JAR has the
	// name of the JAR. Drill has no aggregate marker, so every row is a func.
	dbmeta.Functions.Register(dbmeta.Drill, &dbmeta.Binding[dbmeta.Function]{
		Stmt: dbmeta.Stmt{
			always("SELECT 'DRILL' AS `catalog`"),
			always(", 'sys' AS `schema`"),
			always(", f.name AS `name`"),
			always(", NULL AS `id`"),
			always(", 'func' AS `kind`"),
			always(", f.returnType AS `result_type`"),
			always(", f.signature AS `arg_types`"),
			always(", '' AS `volatility`"),
			always(", '' AS `parallel`"),
			always(", NULL AS `owner`"),
			always(", '' AS `security`"),
			always(", NULL AS `access`"),
			always(", 'java' AS `language`"),
			always(", f.source AS `source`"),
			always(", NULL AS `comment`"),
			always(", NULL AS `definition`"),
			always("FROM sys.functions f"),
			always("WHERE (@with_system OR f.source <> 'built-in')"),
			always("AND " + like("'sys'", "@schema")),
			always("AND " + like("f.name", "@name")),
			always("ORDER BY 3, 7"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: catalogDesc},
			{Name: "schema", Desc: "always sys, where Drill lists every function. A function has no schema of its own"},
			{Name: "name"},
			{Name: "id", Desc: "always absent: Drill has no identifier for an overload"},
			{Name: "kind", Desc: "always func: sys.functions has no marker for an aggregate"},
			{Name: "result_type", Desc: "the return type of this overload"},
			{Name: "arg_types", Desc: "the argument types of this overload, as Drill spells them, such as BIGINT-OPTIONAL,VARCHAR-REQUIRED. There is one row for each overload, and the text is empty for a function with no argument"},
			{Name: "volatility", Desc: "always empty: sys.functions records no volatility"},
			{Name: "parallel", Desc: "always empty: Drill marks no parallel safety"},
			{Name: "owner", Desc: "always absent: a function has no owner"},
			{Name: "security", Desc: "always empty: a function has no security mode"},
			{Name: "access", Desc: "always absent: Drill grants nothing on a function"},
			{Name: "language", Desc: "always java: a built in function and a user defined function are both Java classes"},
			{Name: "source", Desc: "built-in for a function that ships with Drill, and the name of the JAR for any other"},
			{Name: "comment", Desc: "always absent: sys.functions records no description"},
			{Name: "definition", Desc: "always absent: Drill keeps no source text of a function"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "name", Desc: "function name pattern, empty for every function", Default: ""},
			{
				Name:    "with_system",
				Desc:    "include the functions that ship with Drill. Drill has no CREATE FUNCTION and a JAR adds the others, so without this a fresh server lists none",
				Default: false,
			},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Function, error) {
			var v dbmeta.Function
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.ID, &v.Kind, &v.ResultType, &v.ArgTypes,
				&v.Volatility, &v.Parallel, &v.Owner, &v.Security, &v.Access, &v.Language, &v.Source,
				&v.Comment, &v.Definition)
			return v, err
		},
	})
}
