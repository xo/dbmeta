// Package databricks holds the metadata queries for Databricks SQL on Unity
// Catalog.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/databricks"
//
// # What it reads
//
// It reads the hosted service. Unity Catalog has three levels: a catalog holds
// schemas, and a schema holds tables, views, volumes and functions. The model
// calls a catalog a database, as the other hosted models do. Every statement reads
// the INFORMATION_SCHEMA of the catalog that the connection is in, and writes
// the view name with no catalog in front of it. Databricks resolves that name
// to the current catalog, so a statement cannot name another catalog, because a
// catalog is part of a name and a statement cannot bind a name. Databases is the
// exception. It reads system.information_schema.catalogs, which lists every
// catalog that the principal can see.
//
// INFORMATION_SCHEMA shows a principal only the objects that it can use, so a
// principal that has no EXECUTE on a function does not see the function and a
// principal that has no READ VOLUME on a volume does not see the volume.
// docs/COVERAGE.md says what that changes.
//
// # What it answers
//
// 17 of the 65. Databases, schemas, tables, columns, views, constraints,
// constraint columns, functions, routine parameters, comments, partitioned
// tables, privileges, policies, collations, foreign servers, the current schema
// and the current user.
//
// Four of those are an analogue, and docs/COVERAGE.md says why each is one. A
// connection of Lakehouse Federation is a foreign server, and CREATE SERVER is
// a synonym of CREATE CONNECTION. A row filter and a column mask are policies. A
// partition column of a Delta table is a partition by the value of the column,
// so the table is partitioned by list. A catalog is a database.
//
// Databricks has primary keys and foreign keys, and it never enforces either.
// Constraints reports Enforced as false for each of them. A CHECK constraint of
// Delta is not in INFORMATION_SCHEMA, and neither is a column default, an
// identity column or a generated column. Those live in the table properties
// that only SHOW CREATE TABLE and DESCRIBE read, which are statements and not
// relations. D146 does not allow a walk here, so the model does not answer
// them. The same holds for the size, the row count and the clustering columns
// of a table, which only DESCRIBE DETAIL reports.
//
// The rest is absent. Databricks has no index, no sequence, no trigger, no user
// defined type, no extension and no role that SQL reads, because a user and a
// group belong to the account.
//
// # The version
//
// The version is the release of the SQL channel, such as 2026.38, which
// current_version() reports as dbsql_version. A cluster reports dbr_version
// instead, and the model reads whichever one is set. See D224.
//
// # The driver
//
// dburl v0.49.0 names github.com/databricks/databricks-sql-go, and the test
// module uses it. A later dburl names github.com/xo/dbimp/databricks, which is
// not tagged yet, and the tests follow dburl when a tag names it. See D224. The
// driver binds a parameter by position with a question mark.
package databricks

import (
	"fmt"
	"strings"

	"github.com/xo/dbmeta"
)

// versionQuery reads the release of the SQL channel, or the release of the
// runtime for a cluster. current_version() answers a struct.
const versionQuery = "SELECT COALESCE(current_version().dbsql_version, current_version().dbr_version)"

func init() {
	dbmeta.RegisterDialect(dbmeta.Databricks, &dbmeta.Info{
		// Databricks has block comments and quotes a name with backticks.
		Syntax: dbmeta.Syntax{BlockComments: true, Backticks: true},
		// An unquoted name is stored in lower case.
		Fold: dbmeta.FoldLower,
		// The driver binds by position, and a position is a question mark.
		Placeholder:    func(int) string { return "?" },
		VersionQuery:   versionQuery,
		VersionColumns: 1,
		ParseVersion:   parseVersion,
	})
	registerRelations()
	registerDetail()
	registerExtra()
}

// parseVersion reads the release, such as 2026.38.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	if len(cols) < 1 {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	release := strings.TrimSpace(cols[0])
	v := dbmeta.ParseVersion(release)
	if v.Unknown {
		return dbmeta.VersionSet{}, fmt.Errorf("databricks: %w: %q", dbmeta.ErrInvalidVersion, release)
	}
	v.Raw = release
	var set dbmeta.VersionSet
	set.Set("", v)
	set.Display = "Databricks " + release
	return set, nil
}

// always is a fragment every release takes.
func always(query string) dbmeta.Choice { return dbmeta.Choice{{Query: query}} }

// like is the filter that matches col to the pattern in param, and passes when
// the pattern is empty.
func like(col, param string) string {
	return `(` + param + ` = '' OR ` + col + ` LIKE ` + param + `)`
}

// notSystem hides the schema information_schema, which every catalog has,
// unless the caller asks for it.
func notSystem(schema string) string {
	return `(@with_system = true OR ` + schema + ` <> 'information_schema')`
}

const systemDesc = "include information_schema, which every catalog has"

func schemaName(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
		{Name: "with_system", Desc: systemDesc, Default: false},
	}
}

func schemaParentName(parent, kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
		{Name: "parent", Desc: parent + " name pattern, empty for every " + parent, Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
		{Name: "with_system", Desc: systemDesc, Default: false},
	}
}

func nameOnly(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
	}
}

// grants aggregates the grants of one view of privileges into one text for each
// object, a line for each grant, as grantee=PRIVILEGE/grantor. A grant that
// the object inherits from a schema or a catalog says so. The key columns are
// the ones that name the object in the view, and the result names them the same.
// The view is read once, and a statement joins it, so the cost does not grow
// with the number of objects.
func grants(view string, key ...string) string {
	list := strings.Join(key, ", ")
	return `(SELECT ` + list + `, concat_ws(chr(10), sort_array(collect_list(` +
		`grantee || '=' || privilege_type || '/' || grantor` +
		` || IF(inherited_from = 'NONE', '', ' (inherited from ' || LOWER(inherited_from) || ')')))) AS acl` +
		` FROM information_schema.` + view + ` GROUP BY ` + list + `)`
}
