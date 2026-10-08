// Package opensearch holds the metadata queries for OpenSearch, in its SQL.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/opensearch"
//
// # SHOW and DESCRIBE, not SELECT
//
// The SQL plugin of OpenSearch has no table that a SELECT reads. It has the
// statements SHOW TABLES LIKE and DESCRIBE TABLES LIKE. A statement of that
// kind takes no WHERE and no ORDER BY, and SHOW TABLES must have a LIKE. So
// each kind here is a walk, [dbmeta.Binding.Walk], that matches the patterns
// of the caller in Go, with [dbmeta.Like]. Ken allowed a walk for OpenSearch on
// 2026-10-07, and the cost of each one is written beside it. See D175 and D181.
//
// # The mapping
//
// An index is a table. The cluster is the catalog, and the one database.
// OpenSearch has no schema, so every schema is empty. A field of the mapping
// is a column, and so is an object or a nested field. A subfield, such as
// dims.h, is a column of its own. An alias is not a view here, because SQL lists an alias as BASE
// TABLE on 2.19.6 and does not list it on 3.9.0, so no row says which name is
// an alias. D181 holds the mapping and the reasons for it.
//
// # The walk
//
// SHOW TABLES LIKE % lists every index once. DESCRIBE TABLES LIKE lists the
// columns of the indices that match its pattern, and a pattern merges them
// into one table that is named for the pattern. So the walk runs DESCRIBE once
// for each index, with the exact name of the index. An underscore in SHOW
// TABLES LIKE is a wildcard that has no escape, so the walk never gives a name
// to it. It matches the name in Go. A request reads the mapping in the
// cluster state and never a document.
//
// An index that the server refuses to describe has no column here. The
// security plugin refuses the ordinary user an index that its role does not
// read, and the walk goes on to the next index.
//
// # What a user sees
//
// SHOW TABLES and DESCRIBE need the permission indices:admin/get, and the
// ordinary user of the tests holds it on every index. So the user sees the name
// of every index, which includes the ones that it cannot read. The security
// plugin refuses DESCRIBE of an index that the user holds no permission on.
//
// # What it answers
//
// 3 of the 56. The cluster as the database, indices as tables, and the fields
// of a mapping as columns. Columns needs DESCRIBE, and dbimp's driver cannot
// read a DESCRIBE row on 2.19.6, because that release declares every column
// keyword and sends numbers in some of them. So Columns answers on 3.9.0 only,
// until the driver reads the row.
//
// # What is missing
//
// 53 kinds. OpenSearch has no DDL in SQL, so it has no schema, view that SQL
// can tell from an index, index in the sense of SQL, constraint, trigger,
// sequence, function list, type list, comment or setting that SQL lists. Its
// users and roles are in the security plugin, which is an HTTP API and not a
// catalog. docs/COVERAGE.md holds the rest.
//
// # The version
//
// The version query is SELECT version(). The SQL of the server fails that
// statement, and the driver of dbimp answers it from GET /, which holds
// version.number. On 2.19.6 a user needs the cluster permission
// cluster:monitor/main, which the role of the ordinary user in the entry of
// dbrun holds. A user without it gets HTTP 403 as the error. On 3.9.0 every
// user can read it, from the header X-OpenSearch-Version. No query here
// depends on the release. See D181, D183, D191 and D192.
package opensearch

import (
	"strings"

	"github.com/xo/dbmeta"
)

func init() {
	dbmeta.RegisterDialect(dbmeta.OpenSearch, &dbmeta.Info{
		// Its SQL takes `--`, `#` and `/* */` comments, and a name between
		// backticks. It refuses a `//` comment, and it reads a double quote as
		// a string and not as a name. It compares a name with its case, and an
		// index name is lower case. It takes a semicolon at the end of a
		// statement on both releases, so a client keeps it. Measured on 2.19.6
		// and 3.9.0.
		Syntax:      dbmeta.Syntax{BlockComments: true, HashComments: true, Backticks: true},
		Terminator:  dbmeta.TerminatorKept,
		Fold:        dbmeta.FoldNone,
		Placeholder: func(int) string { return "?" },
		// The driver of dbimp answers this statement. See the package comment.
		VersionQuery:   versionQuery,
		VersionColumns: 1,
		ParseVersion:   parseVersion,
	})
	registerRelations()
}

// versionQuery reads the release, such as 3.9.0. The SQL of the server fails
// it, and the driver of dbimp answers it from GET /.
const versionQuery = `SELECT version()`

// parseVersion reads the one column that versionQuery returns, such as 3.9.0.
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
	set.Display = "OpenSearch " + release
	return set, nil
}
