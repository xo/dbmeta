package tablestore

import (
	"database/sql"
	"strings"

	"github.com/xo/dbmeta"
)

// always is a fragment every release takes.
func always(query string) dbmeta.Choice { return dbmeta.Choice{{Query: query}} }

// arg returns a string argument, and the empty string when it is absent.
func arg(args map[string]any, name string) string {
	s, _ := args[name].(string)
	return s
}

// tableType is the word for the kind of a relation. SHOW FULL TABLES says BASE
// TABLE for a mapped table, and the model says table, as the other models do.
// A word that the service adds later is passed on in lower case.
func tableType(s string) string {
	if s == "BASE TABLE" {
		return "table"
	}
	return strings.ToLower(s)
}

func registerRelations() {
	// \dt. SHOW FULL TABLES lists the mapped tables of the instance, with the type
	// of each. It is a statement and not a relation, so it takes no filter that
	// binds, and Keep narrows the rows in Go.
	dbmeta.Tables.Register(dbmeta.Tablestore, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			always(`SHOW FULL TABLES`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the mapped table. Catalog and Schema of the Table are always empty, because the instance is the only namespace and no statement names it"},
			{Name: "type", Desc: "table, for the BASE TABLE that SHOW FULL TABLES reports for each mapped table. Tablestore SQL has no view, so no other value has been seen"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern. A table has no schema, so only an empty pattern matches", Default: ""},
			{Name: "name", Desc: "table name pattern, empty for every table", Default: ""},
			dbmeta.TypesParam(),
		},
		Keep: func(v dbmeta.Table, args map[string]any) bool {
			return dbmeta.Like(arg(args, "schema"), v.Schema) &&
				dbmeta.Like(arg(args, "name"), v.Name) &&
				dbmeta.ListHas(arg(args, "types"), v.Type)
		},
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var (
				v     dbmeta.Table
				ttype string
			)
			if err := rows.Scan(&v.Name, &ttype); err != nil {
				return v, err
			}
			v.Type = tableType(ttype)
			return v, nil
		},
	})
}
