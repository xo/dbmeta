// Package singlestore holds the metadata queries for SingleStore.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/singlestore"
//
// SingleStore speaks MySQL's protocol and imitates MySQL's
// information_schema, so most of its answers are the mysql model's
// statements, shared with [dbmeta.Query.Share], as TiDB's are (D133). It has
// no mysql database and no performance_schema, and keeps its users, roles,
// groups and variables in views of its own, so the statements that read
// those are its own. It imports the mysql model, which registers first. The
// dialect is memsql, the name SingleStore had until 2020, which dburl keeps.
//
// It answers 23 of the 65 questions, on 9.0 and 9.1. docs/COVERAGE.md says
// why each of the rest is not answered. See D141.
package singlestore

import (
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/models/mysql"
)

// Release is the version key of SingleStore's own release, such as 9.1.1.
//
// The main version is the MySQL release SingleStore claims, which is what
// VERSION() answers, 5.7.32 on both releases, and it is set under the mysql
// model's MySQL key too, so that a shared statement takes the fragments it
// takes on that MySQL. The mysql model's [mysql.MemSQL] key is set too,
// so that a shared statement takes SingleStore's list of system schemas.
const Release = mysql.MemSQL

// versionQuery reads the MySQL release SingleStore claims and its own, which
// only @@memsql_version carries.
const versionQuery = `SELECT VERSION(), @@memsql_version`

func init() {
	my, ok := dbmeta.MySQL.Info()
	if !ok {
		panic("dbmeta: the singlestore model needs the mysql model registered first")
	}
	info := *my
	info.VersionQuery = versionQuery
	info.VersionColumns = 2
	info.ParseVersion = parseVersion
	dbmeta.RegisterDialect(dbmeta.MemSQL, &info)
	register()
}

// parseVersion reads "5.7.32" and "9.1.1".
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	if len(cols) < 2 {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	claimed := strings.TrimSpace(cols[0])
	own := strings.TrimSpace(cols[1])
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
	set.Display = "SingleStore " + own + ", which claims MySQL " + claimed
	return set, nil
}

// share registers the mysql model's binding of q for SingleStore.
func share[T any](q *dbmeta.Query[T]) {
	if !q.Share(dbmeta.MySQL, dbmeta.MemSQL) {
		panic("dbmeta: the mysql model registers no " + q.Name())
	}
}

// register shares each statement of the mysql model that answers on
// SingleStore, and registers SingleStore's own statement for the rest.
func register() {
	registerOwn()
	share(dbmeta.Tables)
	share(dbmeta.Schemas)
	share(dbmeta.Columns)
	share(dbmeta.Indexes)
	share(dbmeta.IndexColumns)
	share(dbmeta.Constraints)
	share(dbmeta.ConstraintColumns)
	share(dbmeta.Views)
	share(dbmeta.Comments)
	share(dbmeta.Databases)
	share(dbmeta.Collations)
	share(dbmeta.Functions)
	share(dbmeta.RoutineParameters)
	share(dbmeta.AccessMethods)
	share(dbmeta.CurrentSchema)
	share(dbmeta.CurrentUser)
}
