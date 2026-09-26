package exasol

import "github.com/xo/dbmeta"

// changePassword builds the statement that sets a user's password.
//
// Exasol takes a password as a quoted identifier rather than as a string
// literal:
//
//	ALTER USER "DBMETA_PW" IDENTIFIED BY "new" REPLACE "old"
//
// So the only escaping is doubling a double quote. A single quote, a
// backslash and a semicolon are ordinary characters inside the identifier,
// and there is no session state that changes that, which is why this reads
// no Quoting.
//
// The user is quoted as well, so it is matched exactly. Exasol folds an
// unquoted name to upper case, and the catalog records it that way, so the
// name to pass is the one EXA_ALL_USERS reports, such as DBMETA_PW for a
// user created as dbmeta_pw.
//
// REPLACE names the current password, which a user changing its own
// password needs and a user with ALTER USER does not. It is written only
// when the caller gives one.
func changePassword(c dbmeta.PasswordChange, _ dbmeta.Quoting) (string, error) {
	stmt := "ALTER USER " + dbmeta.QuoteIdentifier(c.User, `"`, `"`) +
		" IDENTIFIED BY " + dbmeta.QuoteIdentifier(c.Password, `"`, `"`)
	if c.Old != "" {
		stmt += " REPLACE " + dbmeta.QuoteIdentifier(c.Old, `"`, `"`)
	}
	return stmt, nil
}
