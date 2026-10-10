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
// statement needed a change, and D190 holds what the run found. D193 measured
// parity, conformance and the password statement, and D203 reads the columns
// of a key.
//
// It answers 15 of the 65 questions.
//
// # Keys
//
// INFORMATION_SCHEMA has no KEY_COLUMN_USAGE, and the columns of a key are only
// in SHOW PRIMARY KEYS, SHOW UNIQUE KEYS and SHOW IMPORTED KEYS. The pipe
// operator chains a select onto a SHOW, so one statement reads them. A bind
// parameter is refused after the pipe, so this dialect writes every value into
// its statements with Info.Literal, and the SHOW of the two key statements is
// scoped to one table or one schema when the patterns name one. See D203.
package snowflake

import (
	"database/sql"
	"fmt"
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
		Syntax: dbmeta.Syntax{DollarQuotes: true, BlockComments: true},
		Fold:   dbmeta.FoldUpper,
		// A bind parameter is refused after the pipe operator, so the
		// dialect writes every value into the statement and a query never
		// calls Placeholder. A client writing its own INSERT does, and
		// Snowflake takes a question mark there. See literal, D203 and D230.
		Placeholder:    func(int) string { return "?" },
		Literal:        literal,
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

// literal renders one parameter value for a statement. It is dialect wide, so
// every statement of this model has its values written into it.
//
// A string is a single quoted literal. A backslash escapes the next character
// in a Snowflake string and a quote is doubled, so both are escaped, which is
// the rule that [changePassword] uses. A [scope] is SQL that keyScope quoted,
// and it is written as it is. Anything else is refused, and so is a string with
// a NUL, as Hive's is.
func literal(v any) (string, error) {
	switch t := v.(type) {
	case scope:
		return string(t), nil
	case string:
		if strings.ContainsRune(t, 0) {
			return "", fmt.Errorf("snowflake: %w: a filter cannot hold a NUL", dbmeta.ErrInvalidParam)
		}
		return dbmeta.QuoteLiteral(t, dbmeta.Quoting{BackslashEscapes: sql.Null[bool]{V: true, Valid: true}}), nil
	case bool:
		if t {
			return "TRUE", nil
		}
		return "FALSE", nil
	}
	return "", fmt.Errorf("snowflake: %w: cannot render %T as a literal", dbmeta.ErrInvalidParam, v)
}

// changePassword builds ALTER USER. A backslash escapes the next character
// in a Snowflake string, and a name between double quotes keeps its case.
func changePassword(c dbmeta.PasswordChange, _ dbmeta.Quoting) (string, error) {
	backslashes := dbmeta.Quoting{BackslashEscapes: sql.Null[bool]{V: true, Valid: true}}
	return "ALTER USER " + dbmeta.QuoteIdentifier(c.User, `"`, `"`) +
		" SET PASSWORD = " + dbmeta.QuoteLiteral(c.Password, backslashes), nil
}
