// Package fixture holds a known good libSQL schema.
//
// libSQL runs SQLite, and it builds the SQLite fixture as it is. [Everything]
// is that fixture and one table more, with a vector index, which is the one
// object libSQL adds that a query reads. The types are the sqlite3 fixture's.
package fixture

import (
	"slices"

	"github.com/xo/dbmeta"
	sqfixture "github.com/xo/dbmeta/models/sqlite3/fixture"
)

// Everything is a schema holding one of every object the libSQL queries
// read.
//
// The embedding table holds a vector of three float32 values, and
// embedding_vector is a vector index on it. libSQL keeps the index in a
// table of its own, embedding_vector_shadow, and records the index in
// libsql_vector_meta_shadow, which it makes for the first vector index and
// never drops. Both tables are objects libSQL keeps for itself, so a query
// reports them only when the caller asks for system objects. Dropping the
// index drops its shadow table.
var Everything = sqfixture.Fixture{
	Name:   sqfixture.Everything.Name,
	Schema: sqfixture.Everything.Schema,
	Setup: slices.Concat(sqfixture.Everything.Setup, []sqfixture.Step{
		at("embedding table", "CREATE TABLE embedding (\n"+
			"	embedding_id INTEGER PRIMARY KEY,\n"+
			"	book_id INTEGER NOT NULL REFERENCES book(book_id),\n"+
			"	vector F32_BLOB(3)\n"+
			")"),
		at("vector index", "CREATE INDEX embedding_vector ON embedding (libsql_vector_idx(vector, 'metric=cosine'))"),
	}),
	Teardown: slices.Concat([]sqfixture.Step{
		at("drop vector index", "DROP INDEX IF EXISTS embedding_vector"),
		at("drop embedding", "DROP TABLE IF EXISTS embedding"),
	}, sqfixture.Everything.Teardown),
}

func at(name, query string) sqfixture.Step {
	return sqfixture.Step{Name: name, Stmt: dbmeta.Always(query)}
}
