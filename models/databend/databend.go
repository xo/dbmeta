// Package databend holds the metadata queries for Databend.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/databend"
//
// # system, not information_schema
//
// Databend ships both, and information_schema answers less and answers some
// of it wrong: it reports an ordinal position of 1 for every column, measured
// on 1.2.881 and 1.2.948 on 2026-09-30. Every query here reads system, and
// sequences, which only the table function show_sequences() lists.
//
// # A database is a schema
//
// Databend's names are a catalog, a database and a table, and a connection
// reaches the catalog default. Schemas and Databases both read
// system.databases, which is what models/mysql does for the same reason.
//
// # Functions and sequences belong to no database
//
// A user defined function, a procedure and a sequence belong to the tenant
// and not to a database, so each is reported with an empty schema.
//
// # What it answers
//
// 20 of the 65, on 1.2.881 and 1.2.951. Index columns and constraint
// columns are read out of a list that system records as text. docs/COVERAGE.md says why each of the
// rest is not answered. See D140.
package databend

import (
	"database/sql"
	"strings"

	"github.com/xo/dbmeta"
)

// versionQuery reads the banner, which is "Databend Query v1.2.948-nightly-
// 1df0991d1f(rust-1.100.0-nightly-...)".
const versionQuery = `SELECT version()`

func init() {
	dbmeta.RegisterDialect(dbmeta.Databend, &dbmeta.Info{
		// The syntax is usql's lexer flags for this product, and the fold
		// is measured by scanEveryQuery (D143).
		Fold: dbmeta.FoldLower,
		// dbimp's driver sends a positional argument for each question mark,
		// and the server binds it (dbimp D120).
		Placeholder:    func(int) string { return "?" },
		VersionQuery:   versionQuery,
		VersionColumns: 1,
		ParseVersion:   parseVersion,
		ChangePassword: changePassword,
	})
	register()
}

// parseVersion reads the release from the banner.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	if len(cols) < 1 {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	_, rest, ok := strings.Cut(cols[0], "Databend Query v")
	if !ok {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	release, _, _ := strings.Cut(rest, "-")
	release, _, _ = strings.Cut(release, "(")
	v := dbmeta.ParseVersion(release)
	if v.Unknown {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	v.Raw = release
	var set dbmeta.VersionSet
	set.Set("", v)
	set.Display = "Databend " + release
	return set, nil
}

// changePassword builds ALTER USER. Databend names a user with a string
// literal, and a backslash escapes the next character in one.
func changePassword(c dbmeta.PasswordChange, _ dbmeta.Quoting) (string, error) {
	backslashes := dbmeta.Quoting{BackslashEscapes: sql.Null[bool]{V: true, Valid: true}}
	return "ALTER USER " + dbmeta.QuoteLiteral(c.User, backslashes) +
		" IDENTIFIED BY " + dbmeta.QuoteLiteral(c.Password, backslashes), nil
}
