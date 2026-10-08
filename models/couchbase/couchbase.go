// Package couchbase holds the metadata queries for Couchbase Server, in
// SQL++.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/couchbase"
//
// # The system keyspaces
//
// Couchbase has no information_schema. Its catalog is the system namespace,
// which SQL++ reads like any keyspace: system:buckets, system:all_scopes,
// system:all_keyspaces, system:indexes, system:functions, system:sequences,
// system:user_info and system:applicable_roles. A statement filters, joins,
// groups and unnests them, so every query here is one statement.
//
// The hierarchy is a bucket, then a scope, then a collection. They are the
// database, the schema and the table here, so a table's catalog is its bucket
// and its schema is its scope. A bucket's default collection is the scope
// _default and the collection _default, and it is reported under those names,
// which is how Couchbase addresses it.
//
// # MISSING is not NULL
//
// A document has no fixed shape, so a field can be absent from one row and
// present in the next, and SQL++ leaves an absent value out of the row
// entirely rather than writing it as null. A driver takes its columns from the
// row, so a row that drops a key drops a column. Every expression that can be
// MISSING is written IFMISSING(x, NULL), which keeps the key and makes the
// value NULL. See docs/NULLS.md.
//
// # The floor is 7.6
//
// Couchbase 7.2 returns the fields of every result in the order of their
// names, whatever order the statement selects them in, and 7.6 keeps the
// statement's order. Every query here is read by position, so every fragment
// gates on 7.6, and a 7.2 server reports that it is too old. Ken chose that
// over reading by name. See D104.
//
// # What it answers
//
// 12 of the 61. Buckets as databases, scopes as schemas, collections as
// tables, indexes, index columns, functions, routine parameters, sequences,
// users as roles, a user's groups as role grants, privileges and the current
// user. Sequences arrived in 7.6,
// which is the floor, so no query needs a second fragment.
//
// # What is missing
//
// 44 kinds. A document has no fixed shape, so there is no column catalog,
// and INFER is a statement rather than a relation. There is no view, no
// constraint of any kind, no trigger that SQL++ reads, no user defined type,
// no comment and no setting a statement can read. docs/COVERAGE.md holds the
// rest.
package couchbase

import (
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
)

// versionQuery reads the release, such as 7.6.12-8946-enterprise.
//
// It is the statement usql runs for Couchbase. usql unquotes the result,
// because go_n1ql handed it over as JSON text, and the driver dbmeta tests
// with decodes it into a plain string. See docs/USQL.md.
const versionQuery = `SELECT RAW ds_version()`

// parseVersion reads the one column versionQuery returns.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	var s dbmeta.VersionSet
	if len(cols) != 1 || cols[0] == "" {
		return s, dbmeta.ErrInvalidVersion
	}
	ver := dbmeta.ParseVersion(strings.TrimSpace(cols[0]))
	if ver.Unknown {
		return s, dbmeta.ErrInvalidVersion
	}
	s.Set("", ver)
	s.Display = "Couchbase " + cols[0]
	return s, nil
}

func init() {
	dbmeta.RegisterDialect(dbmeta.Couchbase, &dbmeta.Info{
		// The syntax is usql's lexer flags for this product, and the fold
		// is measured by scanEveryQuery (D143).
		Syntax: dbmeta.Syntax{BlockComments: true},
		// The driver binds a positional value at $1, $2 and so on.
		Placeholder:    func(n int) string { return "$" + strconv.Itoa(n) },
		VersionQuery:   versionQuery,
		VersionColumns: 1,
		ParseVersion:   parseVersion,
	})
	registerRelations()
	registerRoutines()
	registerRoles()
}

// v76 is the floor of the model. See the package doc.
var v76 = dbmeta.V(7, 6)

// from76 is a fragment that every release from 7.6 takes.
func from76(query string) dbmeta.Choice { return dbmeta.Choice{{Min: v76, Query: query}} }

// like builds a filter that matches everything when the parameter is empty.
func like(col, param string) string {
	return `(` + param + ` = '' OR ` + col + ` LIKE ` + param + `)`
}
