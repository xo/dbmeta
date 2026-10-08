// Package snowflake holds the metadata queries for Snowflake.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/snowflake"
//
// Snowflake is a hosted service, and every query here reads the
// INFORMATION_SCHEMA of the database the connection is in. ACCOUNT_USAGE
// answers more and lags behind by as much as three hours, and needs a grant
// an ordinary role does not have, so it is not read.
//
// # Measured
//
// The model ran on 2026-10-08 against a trial account, release 10.36.101. One
// statement needed a change, and D190 holds what the run found. Parity is not
// measured, because the role of the test account cannot make a second user.
//
// It answers 13 of the 65 questions.
package snowflake

import (
	"database/sql"
	"strings"

	"github.com/xo/dbmeta"
)

// versionQuery reads the release, such as 9.21.2.
const versionQuery = `SELECT CURRENT_VERSION()`

func init() {
	dbmeta.RegisterDialect(dbmeta.Snowflake, &dbmeta.Info{
		// The syntax is usql's lexer flags for this product, and the fold
		// is Snowflake's documented rule: a name that is not quoted is
		// stored in upper case (D143).
		Syntax:         dbmeta.Syntax{DollarQuotes: true, BlockComments: true},
		Fold:           dbmeta.FoldUpper,
		Placeholder:    func(int) string { return "?" },
		VersionQuery:   versionQuery,
		VersionColumns: 1,
		ParseVersion:   parseVersion,
		ChangePassword: changePassword,
	})
	register()
}

// parseVersion reads the release.
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
	set.Display = "Snowflake " + release
	return set, nil
}

// changePassword builds ALTER USER. A backslash escapes the next character
// in a Snowflake string, and a name between double quotes keeps its case.
func changePassword(c dbmeta.PasswordChange, _ dbmeta.Quoting) (string, error) {
	backslashes := dbmeta.Quoting{BackslashEscapes: sql.Null[bool]{V: true, Valid: true}}
	return "ALTER USER " + dbmeta.QuoteIdentifier(c.User, `"`, `"`) +
		" SET PASSWORD = " + dbmeta.QuoteLiteral(c.Password, backslashes), nil
}
