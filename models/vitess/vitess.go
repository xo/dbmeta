// Package vitess holds the metadata queries for Vitess.
//
// Import it for its effect. It registers what Vitess provides, and the root
// package answers for it afterwards:
//
//	import _ "github.com/xo/dbmeta/models/vitess"
//
// Vitess speaks MySQL's protocol, and vtgate passes a query of
// information_schema to the MySQL of a tablet. So its answers are the mysql
// model's statements, shared with [dbmeta.Query.Share], and read what that
// MySQL holds. It imports the mysql model, which registers first. See D135.
//
// It answers 20 of the 61 questions, on 23.0.7 and 24.0.4. Every statement
// but one is the mysql model's. Sequences is its own, because a Vitess
// sequence is a table with the comment vitess_sequence. docs/COVERAGE.md says
// why each of the rest is not answered.
//
// # A schema is a keyspace
//
// A keyspace is stored in one MySQL database for each shard, named vt_, the
// keyspace, an underscore and the shard, so the keyspace dbmeta with one
// shard is vt_dbmeta_0. information_schema names that database and never the
// keyspace, and vtgate refuses that name in a query: SELECT from
// vt_dbmeta_0.author fails with VT05003, unknown database, and SELECT from
// dbmeta.author succeeds. So every statement reports the keyspace, which
// [mysql.Keyspace] reads from the name of the database, and a filter matches
// the keyspace. See D135.
package vitess

import (
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/models/mysql"
)

// Release is the version key of Vitess's own release, such as 24.0.3.
//
// The main version is the MySQL release Vitess claims, which is what VERSION()
// answers before -Vitess, and it is set under the mysql model's MySQL key too,
// so that a shared statement takes the fragments it takes on that MySQL.
// The mysql model's [mysql.Vitess] key is set too, so that a shared statement
// takes the one alternative Vitess needs, its list of system schemas.
const Release = mysql.Vitess

// versionQuery reads the MySQL release Vitess claims, and its own release,
// which only @@version_comment carries, as "Version: 24.0.3 (Git revision
// ...".
const versionQuery = `SELECT VERSION(), @@version_comment`

func init() {
	my, ok := dbmeta.MySQL.Info()
	if !ok {
		panic("dbmeta: the vitess model needs the mysql model registered first")
	}
	info := *my
	info.VersionQuery = versionQuery
	info.VersionColumns = 2
	info.ParseVersion = parseVersion
	// vtgate refuses ALTER USER, so there is no statement to build.
	info.ChangePassword = nil
	dbmeta.RegisterDialect(dbmeta.Vitess, &info)
	register()
}

// parseVersion reads "8.4.6-Vitess" and "Version: 24.0.3 (Git revision ...".
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	if len(cols) < 2 {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	claimed, _, ok := strings.Cut(strings.TrimSpace(cols[0]), "-Vitess")
	if !ok {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	own, ok := strings.CutPrefix(strings.TrimSpace(cols[1]), "Version: ")
	if !ok {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	own, _, _ = strings.Cut(own, " ")
	my := dbmeta.ParseVersion(claimed)
	my.Raw = claimed
	release := dbmeta.ParseVersion(own)
	release.Raw = own
	if my.Unknown || release.Unknown {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	var set dbmeta.VersionSet
	set.Set("", my)
	set.Set(mysql.MySQL, my)
	set.Set(Release, release)
	set.Display = "Vitess " + own + ", which claims MySQL " + claimed
	return set, nil
}

// share registers the mysql model's binding of q for Vitess.
func share[T any](q *dbmeta.Query[T]) {
	if !q.Share(dbmeta.MySQL, dbmeta.Vitess) {
		panic("dbmeta: the mysql model registers no " + q.Name())
	}
}

// register shares each statement of the mysql model that answers on Vitess.
//
// Settings, access methods and extensions describe the MySQL of the tablet
// that answers, which is the MySQL that stores the rows. Roles, role grants
// and privileges are not shared. They list the accounts of that MySQL, such
// as vt_dba and vt_app, which Vitess uses itself and no client of vtgate logs
// in as. vtgate refuses CREATE TRIGGER, CREATE FUNCTION and CREATE SERVER, so
// triggers, aggregates, foreign servers, user mappings and foreign tables are
// not shared either.
func register() {
	share(dbmeta.Tables)
	share(dbmeta.Schemas)
	share(dbmeta.Columns)
	share(dbmeta.Indexes)
	share(dbmeta.IndexColumns)
	share(dbmeta.Constraints)
	share(dbmeta.ConstraintColumns)
	share(dbmeta.PartitionedTables)
	share(dbmeta.Views)
	share(dbmeta.Comments)
	share(dbmeta.Databases)
	share(dbmeta.Collations)
	share(dbmeta.Settings)
	share(dbmeta.AccessMethods)
	share(dbmeta.Extensions)
	share(dbmeta.Functions)
	share(dbmeta.RoutineParameters)
	share(dbmeta.CurrentSchema)
	share(dbmeta.CurrentUser)
	registerOwn()
}
