// Package libsql holds the metadata queries for libSQL.
//
// Import it for its effect. It registers what libSQL provides, and the root
// package answers for it afterwards:
//
//	import _ "github.com/xo/dbmeta/models/libsql"
//
// libSQL is the fork of SQLite by Turso, and sqld serves it over HTTP. Its
// answers are the sqlite3 model's statements, shared with
// [dbmeta.Query.Share]. It imports the sqlite3 model, which registers first.
// See D123 and D160.
//
// It answers 14 of the 56 questions, on 0.24.33, which are every one the
// sqlite3 model answers, and the same way: the conformance report is line for
// line SQLite's. libSQL adds a vector index. Two pieces of the sqlite3
// model's statements have a fragment for libSQL, which gates on the version
// key [sqlite3.LibSQL]: a vector index reports the type diskann, and the
// tables libSQL keeps for one are system tables.
//
// The version is the release of SQLite that the server runs, because every
// statement depends on SQLite and no SQL statement names the sqld release.
// Only the HTTP API reports that.
package libsql

import (
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/models/sqlite3"
)

func init() {
	sq, ok := dbmeta.SQLite3.Info()
	if !ok {
		panic("dbmeta: the libsql model needs the sqlite3 model registered first")
	}
	info := *sq
	info.Embedded = false
	info.ParseVersion = parseVersion
	dbmeta.RegisterDialect(dbmeta.LibSQL, &info)
	register()
}

// parseVersion reads what sqlite_version() returns, such as "3.45.1": the
// release of the SQLite library that the server runs.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	if len(cols) == 0 {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	raw := strings.TrimSpace(cols[0])
	v := dbmeta.ParseVersion(raw)
	if v.IsZero() {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	v.Raw = raw
	var set dbmeta.VersionSet
	set.Set("", v)
	// The key is what the fragments for libSQL in the sqlite3 model gate
	// on. No statement names the sqld release, so its version is unknown.
	set.Set(sqlite3.LibSQL, dbmeta.Version{Unknown: true})
	set.Display = "libSQL, which runs SQLite " + raw
	return set, nil
}

// share registers the sqlite3 model's binding of q for libSQL.
func share[T any](q *dbmeta.Query[T]) {
	if !q.Share(dbmeta.SQLite3, dbmeta.LibSQL) {
		panic("dbmeta: the sqlite3 model registers no " + q.Name())
	}
}

// register shares every statement of the sqlite3 model that libSQL answers.
func register() {
	share(dbmeta.Tables)
	share(dbmeta.Schemas)
	share(dbmeta.Columns)
	share(dbmeta.Indexes)
	share(dbmeta.IndexColumns)
	share(dbmeta.Constraints)
	share(dbmeta.ConstraintColumns)
	share(dbmeta.Triggers)
	share(dbmeta.Views)
	share(dbmeta.Databases)
	share(dbmeta.Functions)
	share(dbmeta.Collations)
	share(dbmeta.Settings)
	share(dbmeta.CurrentSchema)
}
