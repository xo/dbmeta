package vertica

import (
	"database/sql"
	"strings"

	"github.com/xo/dbmeta"
)

// quotingQuery reads whether a backslash escapes inside a string literal,
// the same setting PostgreSQL has. Vertica's SHOW returns the name and the
// setting, where PostgreSQL's returns the setting alone.
const quotingQuery = `SHOW STANDARD_CONFORMING_STRINGS`

// parseQuoting reads the setting from the second column. A backslash escapes
// when standard_conforming_strings is off. Measured on 7.2 and 25.1:
// SET STANDARD_CONFORMING_STRINGS TO OFF makes 'a\\b' three characters long.
func parseQuoting(cols []string) (dbmeta.Quoting, error) {
	if len(cols) != 2 {
		return dbmeta.Quoting{}, dbmeta.ErrQuotingUnknown
	}
	off := !strings.EqualFold(strings.TrimSpace(cols[1]), "on")
	return dbmeta.Quoting{BackslashEscapes: sql.Null[bool]{V: off, Valid: true}}, nil
}

// changePassword builds ALTER USER ... IDENTIFIED BY.
//
// Vertica takes the password as a string literal and the user as an
// identifier, so the two are quoted by different rules, the way PostgreSQL's
// are. REPLACE names the current password, which a user changing its own
// password needs and a superuser does not, and it is written only when the
// caller gives one.
//
// usql's Vertica driver concatenated the password into this statement with no
// escaping at all, so a password holding a quote broke it.
func changePassword(c dbmeta.PasswordChange, q dbmeta.Quoting) (string, error) {
	if !q.BackslashEscapes.Valid {
		return "", dbmeta.ErrQuotingUnknown
	}
	stmt := "ALTER USER " + dbmeta.QuoteIdentifier(c.User, `"`, `"`) +
		" IDENTIFIED BY " + dbmeta.QuoteLiteral(c.Password, q)
	if c.Old != "" {
		stmt += " REPLACE " + dbmeta.QuoteLiteral(c.Old, q)
	}
	return stmt, nil
}
