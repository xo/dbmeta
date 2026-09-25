// Package dbmeta provides database metadata for many databases.
//
// Metadata means the description of what a database contains: its catalogs,
// schemas, tables, columns, indexes, constraints, functions, sequences and the
// privileges on them. dbmeta reads that description. It does not change it and
// it does not render it.
//
// One API answers for every database. The caller asks for tables without
// knowing which database answers, and receives the same types whichever one
// does.
//
// PostgreSQL is the model. The shape of every answer follows the metadata
// commands of psql, the PostgreSQL command line client, because bringing that
// experience to other databases is the purpose of this package. A database
// answers for the objects it has, and reports the ones it does not.
//
// # Queries vary by version
//
// A database changes its metadata between releases, so a query does too. Each
// query is a set of SQL fragments, and each fragment carries the lowest server
// version it applies to. dbmeta reads the version once per connection and
// merges the fragments that apply.
//
// A fragment never changes the set of columns a query returns. When a column
// has no source on an older server, the fragment selects it as a literal NULL
// under the same name, so one result shape fits every version.
//
// # The caller drives
//
// dbmeta does not open a connection, does not import a database driver, and
// does not detect anything. The caller opens the connection, names the
// dialect, and supplies the version. Any type that satisfies [Querier] works, which
// includes [database/sql.DB] and [database/sql.Tx]. Pass a transaction to read
// more than one catalog in one snapshot.
//
// Every function that reads from a database takes a [context.Context] first.
//
// A caller reads the version by asking for the query and running it:
//
//	query, _, ok := dbmeta.Postgres.VersionQuery()
//	// caller runs query and scans the columns as strings
//	versions, err := dbmeta.Postgres.ParseVersion(cols)
//	m, err := dbmeta.New(dbmeta.Postgres, versions)
//
// The version can be anything the caller chooses. Overriding it matters for a
// proxy that hides the server, for a product that reports a version it does
// not behave like, and for a generator with no server at all.
//
// # Asking for an object
//
// There is one [Query] value per kind of object. Name the value, and the
// result type follows from it:
//
//	for t, err := range dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Schema: "public"}.Map()) {
//		...
//	}
//
// An iterator holds a database connection until it ends. Stopping early
// releases it, and so does cancelling the context. Do not open a second
// iterator inside the body of the first, because that needs a second
// connection and deadlocks on a pool of one.
//
// A caller that would rather run the statement itself asks for it instead:
//
//	query, args, err := dbmeta.Tables.Build(m, map[string]any{"schema": "public"})
//
// [Query.Fields] and [Query.Params] describe what a query returns and takes.
// [Query.Support] says whether it can be asked at all, and tells a database
// that has no such object apart from a model that was left out of the build.
//
// # Layout
//
// The models for one database live under internal, one file per model, and
// register what their dialect provides. Callers use this package.
package dbmeta
