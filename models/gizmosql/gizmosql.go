// Package gizmosql holds the metadata queries for GizmoSQL.
//
// Import it for its effect. It registers what GizmoSQL provides, and the root
// package answers for it afterwards:
//
//	import _ "github.com/xo/dbmeta/models/gizmosql"
//
// GizmoSQL serves Arrow Flight SQL, and its engine is DuckDB, or SQLite when
// the server starts with the sqlite backend. This model is for the DuckDB
// backend, which is the default and the one dbrun starts. Its answers are the
// duckdb model's statements, shared with [dbmeta.Query.Share]. It imports the
// duckdb model, which registers first. See D123 and D187.
//
// It answers 21 of the 65 questions, on 1.40.0 and 1.41.0, which are every
// one the duckdb model answers, and the same way. GizmoSQL keeps a database of
// its own, _gizmosql_system, which holds two views for the Flight SQL metadata
// calls. DuckDB does not flag it internal, so a fragment of the duckdb model
// that gates on the version key [duckdb.GizmoSQL] leaves it out unless the
// caller asks for the system objects.
//
// The version is the release of DuckDB that the server runs, because every
// statement depends on DuckDB and no SQL statement names the GizmoSQL
// release. Only the Flight SQL call that reads the server information does.
//
// A backend of SQLite reports its own release for version() and has no
// duckdb_tables(), so nothing in this model runs on it.
package gizmosql

import (
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/models/duckdb"
)

func init() {
	dk, ok := dbmeta.DuckDB.Info()
	if !ok {
		panic("dbmeta: the gizmosql model needs the duckdb model registered first")
	}
	info := *dk
	info.Embedded = false
	info.ParseVersion = parseVersion
	dbmeta.RegisterDialect(dbmeta.GizmoSQL, &info)
	register()
}

// parseVersion reads what version() returns, such as "v1.5.6": the release of
// DuckDB that the server runs.
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
	// The key is what the fragments for GizmoSQL in the duckdb model gate
	// on. No statement names the GizmoSQL release, so its version is unknown.
	set.Set(duckdb.GizmoSQL, dbmeta.Version{Unknown: true})
	set.Display = "GizmoSQL, which runs DuckDB " + raw
	return set, nil
}

// share registers the duckdb model's binding of q for GizmoSQL.
func share[T any](q *dbmeta.Query[T]) {
	if !q.Share(dbmeta.DuckDB, dbmeta.GizmoSQL) {
		panic("dbmeta: the duckdb model registers no " + q.Name())
	}
}

// register shares every statement of the duckdb model.
func register() {
	share(dbmeta.Functions)
	share(dbmeta.Aggregates)
	share(dbmeta.RoutineParameters)
	share(dbmeta.Types)
	share(dbmeta.EnumValues)
	share(dbmeta.Collations)
	share(dbmeta.Settings)
	share(dbmeta.Extensions)
	share(dbmeta.CurrentUser)
	share(dbmeta.CurrentSchema)
	share(dbmeta.Schemas)
	share(dbmeta.Databases)
	share(dbmeta.Tables)
	share(dbmeta.Columns)
	share(dbmeta.Indexes)
	share(dbmeta.Sequences)
	share(dbmeta.Views)
	share(dbmeta.Comments)
	share(dbmeta.Constraints)
	share(dbmeta.ConstraintColumns)
	share(dbmeta.NotNulls)
}
