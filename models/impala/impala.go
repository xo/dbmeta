// Package impala holds the metadata queries for Apache Impala.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/impala"
//
// # SHOW, not SELECT
//
// Impala has no catalog that a SELECT reads. It lists the databases with
// SHOW DATABASES, the tables and views of one database with SHOW TABLES IN
// and SHOW VIEWS IN, the columns of one table with DESCRIBE, and the
// functions of one database with SHOW FUNCTIONS IN. So most kinds here are
// answered by a walk, [dbmeta.Binding.Walk], which lists the parents and then
// asks inside each one. Ken allowed that for Impala on 2026-09-30, and it
// costs a statement for each database, or for each table, as a walk goes
// deeper. The caller's patterns are matched in Go, with [dbmeta.Like],
// because a SHOW statement takes none. See D146.
//
// HiveServer2, which Impala serves, carries no parameter, so a statement
// here binds nothing.
//
// It answers 11 of the 56 questions, on 4.4.1 and 4.5.2.
package impala

import (
	"strings"

	"github.com/xo/dbmeta"
)

// versionQuery reads the banner, which is "impalad version 4.5.2-RELEASE
// RELEASE (build ...)".
const versionQuery = `SELECT version()`

func init() {
	dbmeta.RegisterDialect(dbmeta.Impala, &dbmeta.Info{
		// Impala folds a name to lower case, and takes block comments and
		// names between backticks. usql had no lexer flags for Impala, and
		// these are Impala's documented syntax (D143).
		Syntax:         dbmeta.Syntax{BlockComments: true, Backticks: true},
		Fold:           dbmeta.FoldLower,
		Placeholder:    func(int) string { return "?" },
		VersionQuery:   versionQuery,
		VersionColumns: 1,
		ParseVersion:   parseVersion,
	})
	register()
}

// parseVersion reads the release after "impalad version".
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	if len(cols) < 1 {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	_, rest, ok := strings.Cut(cols[0], "impalad version ")
	if !ok {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	release, _, _ := strings.Cut(rest, "-")
	release, _, _ = strings.Cut(release, " ")
	v := dbmeta.ParseVersion(release)
	if v.Unknown {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	v.Raw = release
	var set dbmeta.VersionSet
	set.Set("", v)
	set.Display = "Apache Impala " + release
	return set, nil
}

// quote returns name between backticks, as a SHOW statement names a
// database or a table. A backtick in the name is doubled.
func quote(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}
