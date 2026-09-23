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
// # Connections
//
// dbmeta does not open a connection and does not import a database driver. The
// caller opens the connection with the driver of its choice and passes it in.
// Any type that satisfies [DB] works, which includes [database/sql.DB] and
// [database/sql.Tx].
//
// Every function that reads from a database takes a [context.Context] as its
// first argument.
//
// # Layout
//
// The models for one database live in a package under models, named after the
// driver. Those packages hold generated code. Callers use this package.
package dbmeta
