// Package rqlite holds the metadata queries for rqlite.
//
// Import it for its effect. It registers what rqlite provides, and the root
// package answers for it afterwards:
//
//	import _ "github.com/xo/dbmeta/models/rqlite"
//
// rqlite runs SQLite behind an HTTP API, so its answers are the sqlite3
// model's statements, shared with [dbmeta.Query.Share]. It imports the sqlite3
// model, which registers first. See D123 and D148.
//
// It answers 14 of the 61 questions, on 9.4.5 and 10.5.2, which are every one
// the sqlite3 model answers, and the same way: the conformance report is line
// for line SQLite's. rqlite adds users, which it reads from a file, and no
// statement reaches them, so it answers nothing SQLite does not.
//
// The version is the release of SQLite the server runs, because every
// statement depends on SQLite and no SQL statement names the rqlite release.
// Only the HTTP API reports that.
package rqlite

import (
	"strings"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/sqlite3"
)

func init() {
	sq, ok := dbmeta.SQLite3.Info()
	if !ok {
		panic("dbmeta: the rqlite model needs the sqlite3 model registered first")
	}
	info := *sq
	info.Embedded = false
	info.ParseVersion = parseVersion
	dbmeta.RegisterDialect(dbmeta.Rqlite, &info)
	register()
}

// parseVersion reads what sqlite_version() returns, such as "3.51.0": the
// release of the SQLite library the server runs.
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
	set.Display = "rqlite, which runs SQLite " + raw
	return set, nil
}

// share registers the sqlite3 model's binding of q for rqlite.
func share[T any](q *dbmeta.Query[T]) {
	if !q.Share(dbmeta.SQLite3, dbmeta.Rqlite) {
		panic("dbmeta: the sqlite3 model registers no " + q.Name())
	}
}

// register shares every statement of the sqlite3 model that rqlite answers.
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
