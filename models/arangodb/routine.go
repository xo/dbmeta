package arangodb

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerRoutines() {
	// \df. A user defined AQL function is JavaScript, registered in a
	// database under a name with a namespace, such as dbmeta::full_title.
	// _aqlfunctions holds one document for each, with the name as it was
	// registered, the source and whether it is deterministic. A name is
	// compared without its case, and AQL has no overload. The built in
	// functions are in no collection AQL reads, so with_system adds none.
	dbmeta.Functions.Register(dbmeta.ArangoDB, &dbmeta.Binding[dbmeta.Function]{
		Stmt: dbmeta.Stmt{
			always(`FOR f IN _aqlfunctions`),
			always(`FILTER ` + noSchema),
			always(`FILTER ` + like(`f.name`, `@name`)),
			always(`SORT f.name`),
			always(`RETURN {catalog: '', schema: CURRENT_DATABASE(), name: f.name, id: null, kind: 'func',`),
			always(` result_type: null, arg_types: null,`),
			always(` volatility: f.isDeterministic == true ? 'immutable' : 'volatile',`),
			always(` parallel: '', owner: null, security: '', access: null, language: 'javascript',`),
			always(` source: f.code, comment: null, definition: null}`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: catalogDesc},
			{Name: "schema", Desc: schemaDesc},
			{Name: "name", Desc: "the name with its namespace, such as dbmeta::full_title, as it was registered"},
			{Name: "id", Desc: "always absent: a function has one name and no overload"},
			{Name: "kind", Desc: "always func: AQL has no user defined aggregate or procedure"},
			{Name: "result_type", Desc: "always absent: a JavaScript function declares no type"},
			{
				Name: "arg_types",
				Desc: "always absent: the parameters are in the JavaScript source," +
					" and the catalog does not list them",
			},
			{
				Name: "volatility",
				Desc: "immutable for a function registered as deterministic, which" +
					" the optimizer can run once for constant arguments, and volatile" +
					" for any other",
			},
			{Name: "parallel", Desc: "always empty: AQL has no parallel safety mark"},
			{Name: "owner", Desc: "always absent: a function has no owner"},
			{Name: "security", Desc: "always empty: a function runs as the user of the query"},
			{Name: "access", Desc: "always absent: a function has no grant of its own"},
			{Name: "language", Desc: "always javascript: the one language of a user defined function"},
			{Name: "source", Desc: "the JavaScript source, as the server keeps it"},
			{Name: "comment", Desc: "always absent: a function carries no comment"},
			{Name: "definition", Desc: "always absent: a function is registered through the HTTP API, and there is no statement that makes one"},
		},
		Params: []dbmeta.Param{
			schemaParam,
			{Name: "name", Desc: "function name pattern, with the namespace, empty for every function", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Function, error) {
			var v dbmeta.Function
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.ID, &v.Kind,
				&v.ResultType, &v.ArgTypes, &v.Volatility, &v.Parallel, &v.Owner,
				&v.Security, &v.Access, &v.Language, &v.Source, &v.Comment, &v.Definition)
			return v, err
		},
	})

	// Who the request authenticated as.
	dbmeta.CurrentUser.Register(dbmeta.ArangoDB, &dbmeta.Binding[dbmeta.User]{
		Stmt: dbmeta.Stmt{
			always(`RETURN {name: CURRENT_USER(), session: null}`),
		},
		Fields: []dbmeta.Field{
			{
				Name: "name",
				Desc: "the user of the request. The AQL manual says that" +
					" CURRENT_USER() is null on a server that runs without" +
					" authentication, and the name is then empty",
			},
			{Name: "session", Desc: "always absent: a request carries one user and no session"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.User, error) {
			var v dbmeta.User
			err := rows.Scan(dbmeta.NullAsEmpty(&v.Name), &v.Session)
			return v, err
		},
	})
}
