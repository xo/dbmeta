// Package solr holds the metadata queries for Apache Solr, in the SQL of its
// Parallel SQL handler.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/solr"
//
// # The catalog
//
// Solr answers SQL with Apache Calcite, at POST /solr/{collection}/sql. The
// SQL has no INFORMATION_SCHEMA and no SHOW. Its only catalog is the schema
// metadata, which holds two tables, TABLES and COLUMNS, in the form of JDBC's
// DatabaseMetaData. Every query here is one statement over one of them, and
// the model reads them through dbimp's solr driver. D179 holds the mapping
// and the reasons for it.
//
// # The mapping
//
// A collection is a table, and an alias of a collection is a table too, because
// SQL cannot tell the two apart. Every collection is in the schema solr and
// every one is in the catalog solr. Solr gives the schema the address of
// ZooKeeper, which changes with the machine, so the model reports the fixed
// name instead. The two tables of the schema metadata are system tables, and
// they are listed only with with_system. A column is a field of the collection.
//
// # What a user sees
//
// The administrator and the ordinary user of the tests read the same rows from
// both tables, on every release. A user sees every collection that Solr lets it
// read, and the statement of the SQL handler is refused for a collection the
// user cannot read.
//
// # The cost
//
// metadata.TABLES and metadata.COLUMNS read every collection of the cluster.
// A filter on a collection does not prune the read, so each statement costs
// the same however narrow it is. D179 holds the measurement.
//
// # What it answers
//
// 4 of the 56. Schemas, the current schema, tables and columns.
//
// # What is missing
//
// 52 kinds. Solr has no DDL, so it has no index, constraint, trigger, sequence,
// type, comment or role that SQL lists. Its users and roles are in
// security.json, and its aliases, its settings and its cluster state are in the
// Collections API and the Schema API. Those are HTTP calls that only an
// administrator can make, so they are not a catalog a Queryer reads.
// docs/COVERAGE.md holds the rest.
//
// # The version
//
// The version query is SELECT version(). The SQL of the server has no such
// function. The driver of dbimp answers the statement from
// GET /solr/admin/info/system, which holds the release as
// lucene.solr-spec-version. Every user can read it when the security.json
// of the server lets the role of the user read that path, as the entry of
// dbrun does for the ordinary user. A user without that right gets HTTP 403,
// and [dbmeta.Dialect.Version] returns the error and not an unknown version.
// See D179, D183, D191 and D192.
package solr

import (
	"strings"

	"github.com/xo/dbmeta"
)

// versionQuery reads the release, such as 10.0.0. The SQL of the server has no
// function for it, and the driver of dbimp answers this statement from
// GET /solr/admin/info/system.
const versionQuery = `SELECT version()`

// parseVersion reads the one column that versionQuery returns, such as 10.0.0.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	var s dbmeta.VersionSet
	if len(cols) != 1 || strings.TrimSpace(cols[0]) == "" {
		return s, dbmeta.ErrInvalidVersion
	}
	release := strings.TrimSpace(cols[0])
	ver := dbmeta.ParseVersion(release)
	if ver.Unknown {
		return s, dbmeta.ErrInvalidVersion
	}
	ver.Raw = release
	s.Set("", ver)
	s.Display = "Apache Solr " + release
	return s, nil
}

func init() {
	dbmeta.RegisterDialect(dbmeta.Solr, &dbmeta.Info{
		// The handler sets the lexical rules of MySQL, so a name can be in
		// backticks and a comment can be a block. It refuses a semicolon at
		// the end of a statement. It keeps the case of a name that is not
		// quoted and matches names without regard to case.
		Syntax:      dbmeta.Syntax{BlockComments: true, Backticks: true},
		Terminator:  dbmeta.TerminatorStripped,
		Fold:        dbmeta.FoldNone,
		Placeholder: func(int) string { return "?" },
		// The driver of dbimp answers this statement. See the package comment.
		VersionQuery:   versionQuery,
		VersionColumns: 1,
		ParseVersion:   parseVersion,
	})
	registerRelations()
}

// always is a fragment that every release takes.
func always(query string) dbmeta.Choice { return dbmeta.Choice{{Query: query}} }

// text is a string literal of the type VARCHAR. Calcite gives a bare literal
// the type CHAR of its own length, and pads the shorter literal of a CASE with
// spaces, so every literal here is cast.
func text(s string) string { return `CAST('` + s + `' AS VARCHAR)` }

// al quotes the name of a column. The handler sets the lexical rules of MySQL,
// so a name between double quotes is a string and a name needs backticks.
func al(name string) string { return "`" + name + "`" }

// null is a NULL of the type VARCHAR. A bare NULL in a projection fails.
const null = `CAST(NULL AS VARCHAR)`

// like builds a filter that matches everything when the parameter is empty.
func like(expr, param string) string {
	return `(` + param + ` = '' OR ` + expr + ` LIKE ` + param + `)`
}

// schemaOf is the name the model reports for the schema of a row of alias. Solr
// names the schema of a collection the address of ZooKeeper, and the two system
// tables are in the schema metadata.
func schemaOf(alias string) string {
	return `CASE WHEN ` + alias + `.tableSchem = 'metadata' THEN ` + text("metadata") + ` ELSE ` + text("solr") + ` END`
}

// system is the parameter that adds the schema metadata.
var system = dbmeta.Param{
	Name:    "with_system",
	Desc:    "include the schema metadata, which Solr keeps for itself",
	Default: false,
}
