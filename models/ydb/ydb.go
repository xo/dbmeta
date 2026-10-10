// Package ydb holds the metadata queries for YDB.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/ydb"
//
// # A path, not a catalog
//
// YDB keeps its objects in a tree of directories, the way a file system
// does. A database is a path such as /local, and a table is a path inside
// it, such as /local/dbmeta/dbmeta_fixture/author. A statement names a
// table by its path relative to the database, between backticks.
//
// So this model maps the tree onto three levels. The catalog is the path of
// the database. A schema is a directory, named by its path relative to the
// database, such as dbmeta/dbmeta_fixture. An object is the last part of its
// path. A table at the root of the database has the empty schema. D161 holds
// the mapping and the reasons for it.
//
// # The .sys views, and nothing else
//
// YQL has no information_schema and no catalog of columns. The only source
// a SELECT reads is the directory .sys in each database, which holds views
// of the access control lists, the partitions of each table, the tablets and
// the storage pools. Two of them name every object:
//
//   - auth_owners holds a row for every path, with its owner. It has no
//     column that says what kind of object a path is.
//   - partition_stats holds a row for every partition of every table. It
//     also holds the tables that implement a secondary index, which
//     auth_owners does not hold.
//
// hive_tablets says whether the tablet behind a partition is a row shard or
// a column shard, which is the difference between a row table and a column
// table. A new column table reports no tablet in partition_stats for about
// a minute, so that join gives a wrong answer for that minute, and the
// model reports every table as a table. See D161.
//
// A table is a path in both auth_owners and partition_stats. A directory is
// a path that holds another path. A view, a topic and every other kind of
// object is a path that is neither, and no view tells them apart.
//
// # What it answers
//
// 7 of the 65. Databases, schemas, tables, tablespaces, roles, role grants
// and privileges.
//
// Columns, indexes, views, sequences and the rest are in the schema of each
// object, which YDB gives only to a gRPC call per object, DescribeTable and
// its relatives. A SELECT cannot reach them and D146 does not allow a walk
// here, so they are unanswered. docs/COVERAGE.md holds each one.
//
// Every .sys view refuses an ordinary user, so every query here answers
// only for an administrator. The parity test records it.
package ydb

import (
	"strconv"

	"github.com/xo/dbmeta"
)

// versionQuery reads the server version, which YDB reports as four numbers
// such as 26.3.1.17.
const versionQuery = `SELECT version()`

// parseVersion reads the one column versionQuery returns.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	var s dbmeta.VersionSet
	if len(cols) != 1 {
		return s, dbmeta.ErrInvalidVersion
	}
	s.Set("", dbmeta.ParseVersion(cols[0]))
	s.Display = "YDB " + cols[0]
	return s, nil
}

func init() {
	dbmeta.RegisterDialect(dbmeta.YDB, &dbmeta.Info{
		// YQL quotes a name with backticks and has block comments. A string
		// is between single or double quotes, so a double quote does not
		// quote a name here. A name keeps its case, measured by
		// scanEveryQuery (D143).
		Syntax: dbmeta.Syntax{BlockComments: true, Backticks: true},
		// ydb-go-sdk names a parameter that has no name $p0, $p1 and so
		// on, counting from zero, and YDB takes a parameter that no DECLARE
		// names. So a positional value needs no option in the connection
		// string. See D161.
		Placeholder:    func(n int) string { return "$p" + strconv.Itoa(n-1) },
		VersionQuery:   versionQuery,
		VersionColumns: 1,
		ParseVersion:   parseVersion,
	})
	registerRelations()
	registerRoles()
}

// always is a fragment every release takes.
func always(query string) dbmeta.Choice { return dbmeta.Choice{{Query: query}} }

// root is a relation of one row, which holds the path of the database as
// root and the number of parts in that path as depth.
//
// No function returns the database, so it is read from auth_owners. The
// database is a prefix of every path there, so it is the shortest one. It
// cannot be found by its .sys child, because a user can name a table .sys
// in any directory. split on /local is "" and "local", so depth is 2, and
// the parts of a path after the first depth are its path relative to the
// database.
const root = "(SELECT Path AS root, ListLength(String::SplitToList(Path, '/')) AS depth" +
	" FROM `.sys/auth_owners` ORDER BY Length(Path) LIMIT 1) AS d"

// schemaOf is the directory of a path, relative to the database, from the
// list of its parts. It is empty for an object at the root.
func schemaOf(parts string) string {
	return `String::JoinFromList(ListSkip(ListTake(` + parts + `, ListLength(` + parts +
		`) - 1ul), d.depth), '/')`
}

// topOf is the first part of a path relative to the database, which says
// whether the object is one YDB keeps for itself.
func topOf(parts string) string {
	return `ListHead(ListSkip(` + parts + `, d.depth))`
}

// systemTops are the directories at the root of every database that YDB
// keeps for itself. A user can make a directory whose name begins with a
// dot, so the test names these three rather than the dot.
const systemTops = `'.sys', '.metadata', '.sys_health'`

// notSystem filters those out unless the caller asks for them. top is the
// first part of the path relative to the database.
func notSystem(top string) string {
	return `(@with_system = true OR ` + top + ` NOT IN (` + systemTops + `))`
}

// tablePaths is a relation of one row per table, which holds its path as
// path.
//
// partition_stats holds a row per partition, so the paths are made
// distinct. The tables that implement an index are in partition_stats and
// not in auth_owners, so the join leaves them out.
const tablePaths = "(SELECT DISTINCT s.Path AS path FROM `.sys/partition_stats` AS s" +
	" JOIN `.sys/auth_owners` AS a ON a.Path = s.Path)"

// tableStats is tablePaths with the owner, and the size and the rows that
// the partitions add up to. It is the one read of partition_stats that Tables
// makes, and the partitions of a table are summed in it. See D210.
const tableStats = "(SELECT s.Path AS path, a.Sid AS owner" +
	", CAST(SUM(s.DataSize) AS Int64) AS size, CAST(SUM(s.RowCount) AS Int64) AS rows" +
	" FROM `.sys/partition_stats` AS s" +
	" JOIN `.sys/auth_owners` AS a ON a.Path = s.Path GROUP BY s.Path, a.Sid)"

// directories is a relation of one row per directory that holds another
// path, which holds its path as path.
//
// The parent of every path in auth_owners is a directory, because a table
// holds no path there: its index tables are not in auth_owners. The root is
// the parent of the paths at the top, so it is left out by its depth.
const directories = "(SELECT DISTINCT CAST(String::JoinFromList(" +
	"ListTake(c.parts, ListLength(c.parts) - 1ul), '/') AS Utf8) AS path" +
	" FROM (SELECT String::SplitToList(Path, '/') AS parts FROM `.sys/auth_owners`) AS c" +
	" CROSS JOIN " + root +
	" WHERE ListLength(c.parts) > d.depth + 1ul)"

// acl is a relation of one row per path that has an explicit grant, which
// holds its path as path and its grants as access.
//
// A grant is sid=permission, and the grants are sorted, because
// AGGREGATE_LIST keeps no order.
const acl = "(SELECT Path AS path" +
	", String::JoinFromList(ListSort(AGGREGATE_LIST(Sid || '=' || Permission)), ', ') AS access" +
	" FROM `.sys/auth_permissions` GROUP BY Path)"
