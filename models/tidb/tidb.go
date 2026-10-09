// Package tidb holds the metadata queries for TiDB.
//
// Import it for its effect. It registers what TiDB provides, and the root
// package answers for it afterwards:
//
//	import _ "github.com/xo/dbmeta/models/tidb"
//
// TiDB speaks MySQL's protocol and imitates MySQL's information_schema, so
// most of its answers are the mysql model's statements, shared with
// [dbmeta.Query.Share], and the model writes a statement of its own only
// where TiDB's catalog differs. It imports the mysql model, which registers
// first. See D123 and D133.
//
// It answers 20 of the 65 questions on 8.5.8, and 19 on 7.5.8 and 8.1.2,
// where privileges is too old. 17 are the mysql model's statements and 3 are
// its own: settings, sequences and privileges. TiDB has no stored function,
// procedure or trigger and no foreign server, and docs/COVERAGE.md says what
// each answer lacks and why the rest are not answered.
package tidb

import (
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/models/mysql"
)

// Release is the version key of TiDB's own release, such as 8.5.8. A fragment
// of this model's own gates on it.
//
// The main version is the MySQL release TiDB claims to be, 8.0.11, and it is
// set under the mysql model's MySQL key too, so that a shared statement takes
// the fragments it takes on MySQL 8.0.11. The mysql model's [mysql.TiDB]
// key is set too, so that a shared statement takes the one alternative TiDB
// needs, its list of system schemas.
const Release = mysql.TiDB

func init() {
	my, ok := dbmeta.MySQL.Info()
	if !ok {
		panic("dbmeta: the tidb model needs the mysql model registered first")
	}
	info := *my
	info.ParseVersion = parseVersion
	dbmeta.RegisterDialect(dbmeta.TiDB, &info)
	register()
}

// parseVersion reads what SELECT VERSION() returns, such as
// "8.0.11-TiDB-v8.5.8": the MySQL release TiDB claims, and its own after it.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	if len(cols) == 0 {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	raw := strings.TrimSpace(cols[0])
	claimed, own, ok := strings.Cut(raw, "-TiDB-v")
	if !ok {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	own, _, _ = strings.Cut(own, "-")
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
	set.Display = "TiDB " + own + ", which claims MySQL " + claimed
	return set, nil
}

// share registers the mysql model's binding of q for TiDB.
func share[T any](q *dbmeta.Query[T]) {
	if !q.Share(dbmeta.MySQL, dbmeta.TiDB) {
		panic("dbmeta: the mysql model registers no " + q.Name())
	}
}

// register shares each statement of the mysql model that answers on TiDB,
// and registers TiDB's own statement for the rest.
func register() {
	registerOwn()
	share(dbmeta.Tables)
	share(dbmeta.Schemas)
	share(dbmeta.Columns)
	share(dbmeta.Indexes)
	share(dbmeta.IndexColumns)
	share(dbmeta.Constraints)
	share(dbmeta.ConstraintColumns)
	share(dbmeta.PartitionedTables)
	share(dbmeta.Partitions)
	share(dbmeta.Views)
	share(dbmeta.Comments)
	share(dbmeta.Databases)
	share(dbmeta.Collations)
	share(dbmeta.Roles)
	share(dbmeta.RoleGrants)
	share(dbmeta.CurrentSchema)
	share(dbmeta.CurrentUser)
}
