// Package redshift holds the metadata queries for Amazon Redshift.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/redshift"
//
// Redshift speaks PostgreSQL's protocol, and pgx reaches it, as dburl's
// redshift scheme opens it. Its catalog is PostgreSQL 8.0's, with system
// views of its own beside it, so the postgres model's statements, which need
// 9.6, do not run, and every statement here reads the pg_catalog tables 8.0
// already had.
//
// # Not run yet
//
// Ken chose on 2026-09-30 to write this model from Redshift's
// documentation, before anyone has a cluster to run it against. Hard rule 9
// says a query that has never run is not finished, and none here has. The
// tests in the test module run when dbrun resolves a connection string for
// Redshift (D117). See D144.
//
// It answers 11 of the 55 questions.
package redshift

import (
	"database/sql"
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
)

// versionQuery reads the banner, which names the PostgreSQL release Redshift
// was built from and then its own, as "PostgreSQL 8.0.2 on i686-pc-linux-gnu,
// compiled by GCC gcc (GCC) 3.4.2 20041017 (Red Hat 3.4.2-6.fc3), Redshift
// 1.0.77467".
const versionQuery = `SELECT version()`

func init() {
	dbmeta.RegisterDialect(dbmeta.Redshift, &dbmeta.Info{
		// The syntax is usql's lexer flags for this product, which uses the
		// pgx driver's, and the fold is Redshift's documented rule: a name
		// that is not quoted is stored in lower case (D143).
		Syntax:         dbmeta.Syntax{DollarQuotes: true, BlockComments: true},
		Fold:           dbmeta.FoldLower,
		Placeholder:    func(n int) string { return "$" + strconv.Itoa(n) },
		VersionQuery:   versionQuery,
		VersionColumns: 1,
		ParseVersion:   parseVersion,
		ChangePassword: changePassword,
	})
	register()
}

// parseVersion reads the Redshift release after the word Redshift.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	if len(cols) < 1 {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	_, rest, ok := strings.Cut(cols[0], "Redshift ")
	if !ok {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	release := strings.TrimSpace(rest)
	v := dbmeta.ParseVersion(release)
	if v.Unknown {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	v.Raw = release
	var set dbmeta.VersionSet
	set.Set("", v)
	set.Display = "Amazon Redshift " + release
	return set, nil
}

// changePassword builds ALTER USER. A backslash escapes the next character
// in a Redshift string.
func changePassword(c dbmeta.PasswordChange, _ dbmeta.Quoting) (string, error) {
	backslashes := dbmeta.Quoting{BackslashEscapes: sql.Null[bool]{V: true, Valid: true}}
	return "ALTER USER " + dbmeta.QuoteIdentifier(c.User, `"`, `"`) +
		" PASSWORD " + dbmeta.QuoteLiteral(c.Password, backslashes), nil
}
