// Package elasticsearch holds the metadata queries for Elasticsearch, in its
// SQL.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/elasticsearch"
//
// # SYS and SHOW, not SELECT
//
// The SQL of Elasticsearch has no table that a SELECT reads. It has the
// statements SYS TABLES, SYS COLUMNS, SYS TYPES, SHOW FUNCTIONS and SHOW
// CATALOGS. A statement of that kind takes no WHERE and no ORDER BY, and a
// SELECT cannot use one as a source. So each kind here is a walk,
// [dbmeta.Binding.Walk], that runs the one statement and matches the
// patterns of the caller in Go, with [dbmeta.Like]. Ken allowed a walk for
// Elasticsearch on 2026-10-07, and the cost of each one is written beside it.
// No walk runs more than one statement. See D175 and D177.
//
// # The mapping
//
// An index is a table, and an alias is a view. The cluster is the catalog, and
// the one database. Elasticsearch has no schema, so every schema is empty. A
// field of the mapping is a column, and a subfield, such as the multi-field
// name.raw or the object field dims.h, is a column of its own. The functions
// are the ones of its SQL, and the types are the ones SYS TYPES lists. D177
// holds the mapping and the reasons for it.
//
// # Pages
//
// SYS COLUMNS answers 1000 rows at a time and gives a cursor for the next
// page. dbimp's elasticsearch driver follows the cursor, so a walk reads the
// rows as one result, and closes the cursor on the server when the caller
// stops early.
//
// # What a user sees
//
// Elasticsearch shows a user the indices that the role of the user can read.
// An index with no permission is not listed. A hidden index, such as a
// system index or the backing index of a data stream, is not listed to
// anyone. The ordinary user of the tests has a role that can read the indices
// whose names start with dbmeta, so it sees the same rows as the
// administrator, except for the indices outside that name.
//
// # What it answers
//
// 8 of the 65. The cluster as the database, indices as tables, aliases as
// views, the fields of a mapping as columns, the functions, the aggregate
// functions, the types and the current user.
//
// # What is missing
//
// 48 kinds. Elasticsearch has no DDL in SQL, so it has no schema, index in
// the sense of SQL, constraint, trigger, sequence, type that a user makes,
// comment or setting that SQL lists. Its users and roles are in the security
// API, which is an HTTP API and not a catalog. docs/COVERAGE.md holds the
// rest.
//
// # The version
//
// The version query is SELECT version(). The server has no such SQL function.
// The driver of dbimp answers the statement from GET /, which holds
// version.number. A user needs the cluster privilege cluster:monitor/main,
// which the role of the ordinary user in the entry of dbrun holds. A user
// without it is refused with HTTP 403, and [dbmeta.Dialect.Version] returns
// that error and not an unknown version. No query here depends on the
// release. See D177, D183, D191 and D192.
package elasticsearch

import (
	"strings"

	"github.com/xo/dbmeta"
)

func init() {
	dbmeta.RegisterDialect(dbmeta.Elasticsearch, &dbmeta.Info{
		// Its SQL takes `--` and `/* */` comments and double quoted names,
		// and refuses a backtick, a `//` and a `#`. It compares a name with
		// its case, measured on 9.5.3. It refuses a semicolon at the end of
		// a statement, so a client removes it.
		Syntax:      dbmeta.Syntax{BlockComments: true},
		Terminator:  dbmeta.TerminatorStripped,
		Fold:        dbmeta.FoldNone,
		Placeholder: func(int) string { return "?" },
		// The driver of dbimp answers this statement. See the package comment.
		VersionQuery:   versionQuery,
		VersionColumns: 1,
		ParseVersion:   parseVersion,
	})
	registerRelations()
	registerRoutines()
}

// versionQuery reads the release, such as 9.5.3. The SQL of the server has no
// statement for it, and the driver of dbimp answers this one from GET /.
const versionQuery = `SELECT version()`

// parseVersion reads the one column that versionQuery returns, such as 9.5.3.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	if len(cols) < 1 {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	release := strings.TrimSpace(cols[0])
	v := dbmeta.ParseVersion(release)
	if v.Unknown {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	v.Raw = release
	var set dbmeta.VersionSet
	set.Set("", v)
	set.Display = "Elasticsearch " + release
	return set, nil
}
