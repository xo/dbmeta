package postgres

import "github.com/xo/dbmeta"

// fields declares result columns that no version gates, which is most of them.
func fields(names ...string) []dbmeta.Field {
	out := make([]dbmeta.Field, len(names))
	for i, name := range names {
		out[i] = dbmeta.Field{Name: name}
	}
	return out
}

// schemaNameSystem is the usual parameter set for an object that lives in a
// schema: narrow by schema, narrow by name, and choose whether to include the
// objects PostgreSQL keeps for itself.
func schemaNameSystem(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
		{Name: "with_system", Desc: "include the objects PostgreSQL keeps for itself", Default: false},
	}
}

// accessMethodName is the parameter set for an object that belongs to an
// access method.
func accessMethodName(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "access_method", Desc: "access method name pattern, empty for every one", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
	}
}

// schemaParentName is the parameter set for an object that belongs to a table:
// narrow by schema, by the table, and by its own name.
func schemaParentName(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
		{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
		{Name: "with_system", Desc: "include the objects PostgreSQL keeps for itself", Default: false},
	}
}
