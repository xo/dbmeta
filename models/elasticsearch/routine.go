package elasticsearch

import (
	"context"
	"database/sql"
	"iter"

	"github.com/xo/dbmeta"
)

// functionFields describes what Functions and Aggregates return.
func functionFields(kind string) []dbmeta.Field {
	return []dbmeta.Field{
		{Name: "catalog", Desc: "always empty: a function of SQL belongs to no cluster"},
		{Name: "schema", Desc: "always empty: Elasticsearch has no schema"},
		{Name: "name"},
		{Name: "id", Desc: "always absent: SHOW FUNCTIONS has one row for each name, and a name has no overload to tell apart"},
		{Name: "kind", Desc: "agg for an aggregate, and func for every other " + kind + ": a scalar, a conditional, a grouping function and the score. SHOW FUNCTIONS spells them SCALAR, CONDITIONAL, GROUPING and SCORE, and no field here holds that word"},
		{Name: "result_type", Desc: "always absent: SHOW FUNCTIONS records no return type"},
		{Name: "arg_types", Desc: "always absent: SHOW FUNCTIONS records no argument type"},
		{Name: "volatility", Desc: "always empty: SHOW FUNCTIONS records no volatility"},
		{Name: "parallel", Desc: "always empty: Elasticsearch marks no parallel safety"},
		{Name: "owner", Desc: "always absent: a built in " + kind + " has no owner"},
		{Name: "security", Desc: "always empty: a built in " + kind + " has no security mode"},
		{Name: "access", Desc: "always absent: Elasticsearch grants nothing on a " + kind},
		{Name: "language", Desc: "always internal: every " + kind + " is built into Elasticsearch, which has no user defined function in SQL"},
		{Name: "source", Desc: "always absent: a built in " + kind + " has no source text"},
		{Name: "comment", Desc: "always absent: SHOW FUNCTIONS records no description"},
		{Name: "definition", Desc: "always absent, for the same reason as source"},
	}
}

// functionParams are the parameters of Functions and Aggregates. Elasticsearch
// has no CREATE FUNCTION in SQL, so a caller must set with_system to list any.
func functionParams(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern. Elasticsearch has no schema, so only the empty pattern matches", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
		{
			Name:    "with_system",
			Desc:    "include the functions built into Elasticsearch, which are every one there is",
			Default: false,
		},
	}
}

// walkFunctions reads SHOW FUNCTIONS, which lists 161 names on 9.5.3 in one
// answer. It takes no pattern the walk can pass on, because the pattern of
// the statement is its own LIKE and the walk matches in Go. One statement.
func walkFunctions(aggregate bool) func(ctx context.Context, db dbmeta.Queryer, args map[string]any) iter.Seq2[dbmeta.Function, error] {
	return func(ctx context.Context, db dbmeta.Queryer, args map[string]any) iter.Seq2[dbmeta.Function, error] {
		if !withSystem(args) {
			return func(func(dbmeta.Function, error) bool) {}
		}
		return readAll(ctx, db, `SHOW FUNCTIONS`, func(rows *sql.Rows) (dbmeta.Function, bool, error) {
			var name, class string
			if err := rows.Scan(&name, &class); err != nil {
				return dbmeta.Function{}, false, err
			}
			isAgg := class == "AGGREGATE"
			keep := dbmeta.Like(arg(args, "schema"), "") && dbmeta.Like(arg(args, "name"), name) &&
				(!aggregate || isAgg)
			kind := "func"
			if isAgg {
				kind = "agg"
			}
			return dbmeta.Function{
				Name: name, Kind: kind, Language: "internal",
			}, keep, nil
		})
	}
}

func registerRoutines() {
	// \df.
	dbmeta.Functions.Register(dbmeta.Elasticsearch, &dbmeta.Binding[dbmeta.Function]{
		Fields: functionFields("function"),
		Params: functionParams("function"),
		Walk:   walkFunctions(false),
	})
	// \da.
	dbmeta.Aggregates.Register(dbmeta.Elasticsearch, &dbmeta.Binding[dbmeta.Function]{
		Fields: functionFields("aggregate"),
		Params: functionParams("aggregate"),
		Walk:   walkFunctions(true),
	})

	// \dT. SYS TYPES has one row for each type of SQL. They are built in.
	dbmeta.Types.Register(dbmeta.Elasticsearch, &dbmeta.Binding[dbmeta.Type]{
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: a type belongs to the server and not a cluster"},
			{Name: "schema", Desc: "always empty: Elasticsearch has no schema"},
			{Name: "name", Desc: "the type as SQL names it, such as LONG, KEYWORD and DATETIME. It is also the name that Columns.data_type holds. SYS TYPES lists NESTED, OBJECT and UNSUPPORTED too, and no column can have them"},
			{Name: "internal", Desc: "the same as the name: SYS TYPES has one name for a type"},
			{Name: "kind", Desc: "always base: every type is built in, and there is no CREATE TYPE"},
			{Name: "elements", Desc: "always empty: a type of SQL has no member"},
			{Name: "owner", Desc: "always absent: nobody owns a built in type"},
			{Name: "access", Desc: "always absent: a type is not grantable"},
			{Name: "comment", Desc: "always absent: SYS TYPES records no description"},
			{Name: "size", Desc: "always absent: SYS TYPES gives a precision in digits or characters, which is not a length in bytes"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern. Elasticsearch has no schema, so only the empty pattern matches", Default: ""},
			{Name: "name", Desc: "type name pattern, empty for every type", Default: ""},
		},
		Walk: func(ctx context.Context, db dbmeta.Queryer, args map[string]any) iter.Seq2[dbmeta.Type, error] {
			return readAll(ctx, db, `SYS TYPES`, func(rows *sql.Rows) (dbmeta.Type, bool, error) {
				// The columns are 19: the name first, and the code, the
				// precision and the flags of JDBC's getTypeInfo.
				dest := make([]any, sysTypes)
				for i := range dest {
					dest[i] = new(any)
				}
				var v dbmeta.Type
				dest[0] = &v.Name
				if err := rows.Scan(dest...); err != nil {
					return dbmeta.Type{}, false, err
				}
				v.Internal, v.Kind = v.Name, "base"
				return v, dbmeta.Like(arg(args, "schema"), "") && dbmeta.Like(arg(args, "name"), v.Name), nil
			})
		},
	})

	// The user of the connection. USER() is the one statement that names it,
	// and it is a SELECT of a function, so this is a statement and not a walk.
	dbmeta.CurrentUser.Register(dbmeta.Elasticsearch, &dbmeta.Binding[dbmeta.User]{
		Stmt: dbmeta.Stmt{
			always(`SELECT USER() AS "name"`),
			always(`, NULL AS "session"`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "session", Desc: "always absent: Elasticsearch has no statement that changes who the statement runs as"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.User, error) {
			var v dbmeta.User
			err := rows.Scan(&v.Name, &v.Session)
			return v, err
		},
	})
}

// sysTypes is how many columns SYS TYPES returns.
const sysTypes = 19

// always is a fragment that every release takes.
func always(query string) dbmeta.Choice { return dbmeta.Choice{{Query: query}} }
