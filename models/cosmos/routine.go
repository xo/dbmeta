package cosmos

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// Functions and triggers. Both live in the container that the URL names.

// scriptFields declares the columns of the statements that read the scripts of
// a container, which the three share.
func scriptFields(kind string) []dbmeta.Field {
	return []dbmeta.Field{
		field("database", "the database that the URL names. It is the schema"),
		field("container", "the container that the URL names"),
		field("id", "the name of the "+kind),
		field("rid", "the resource id of the "+kind),
		field("link", "the self link of the "+kind+", which is rid based and names the container"),
		field("body", "the JavaScript of the "+kind),
	}
}

func registerRoutines() {
	// \df. One statement reads the stored procedures and the user defined
	// functions of a container. A user defined function is a JavaScript function
	// that a query calls as udf.name. A stored procedure is a script that a
	// request runs, so its kind is procedure, and its source is its body. The
	// resource id names a routine, because a stored procedure and a function of
	// one container can have the same id. The etag and the timestamp have no field.
	dbmeta.Functions.Register(dbmeta.Cosmos, &dbmeta.Binding[dbmeta.Function]{
		Stmt: catalog("$routines"),
		Fields: []dbmeta.Field{
			field("database", "the database that the URL names. It is the schema"),
			field("container", "the container that the URL names"),
			field("kind", "stored_procedure or function"),
			field("id", "the name of the routine"),
			field("body", "the JavaScript of the routine"),
			field("rid", "the resource id of the routine"),
			field("etag", "the etag of the routine"),
			field("timestamp", "the time of the last change, in UTC"),
		},
		Params: databaseFilters("function"),
		Keep:   keep(func(v dbmeta.Function) string { return v.Schema }, nil, func(v dbmeta.Function) string { return v.Name }),
		Scan: func(rows *sql.Rows) (dbmeta.Function, error) {
			r, err := scanRow(rows)
			kind := "function"
			if r.text("kind") == "stored_procedure" {
				kind = "procedure"
			}
			return dbmeta.Function{
				Schema:   r.text("database"),
				Name:     r.text("id"),
				ID:       r.nullText("rid"),
				Kind:     kind,
				Language: "javascript",
				Source:   r.nullText("body"),
			}, err
		},
	})

	// A trigger belongs to a container and runs before or after a request that
	// names it. It is not enabled or disabled: the request chooses it.
	dbmeta.Triggers.Register(dbmeta.Cosmos, &dbmeta.Binding[dbmeta.Trigger]{
		Stmt: catalog("$triggers"),
		Fields: append(scriptFields("trigger"),
			field("trigger_type", "Pre or Post"),
			field("trigger_operation", "the operation that the trigger can run for, All, Create, Update, Delete or Replace"),
		),
		Params: containerFilters("trigger"),
		Keep: keep(func(v dbmeta.Trigger) string { return v.Schema }, func(v dbmeta.Trigger) string { return v.Table },
			func(v dbmeta.Trigger) string { return v.Name }),
		Scan: func(rows *sql.Rows) (dbmeta.Trigger, error) {
			r, err := scanRow(rows)
			// The definition is the resource as the REST API holds it, because a
			// trigger has a type and an operation beside its body and no
			// statement writes all three.
			return dbmeta.Trigger{
				Schema: r.text("database"),
				Table:  r.text("container"),
				Name:   r.text("id"),
				Definition: jsonText(map[string]any{
					"id":               r.text("id"),
					"body":             r.text("body"),
					"triggerType":      r.text("trigger_type"),
					"triggerOperation": r.text("trigger_operation"),
				}),
			}, err
		},
	})
}
