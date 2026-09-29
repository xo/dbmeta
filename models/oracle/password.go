package oracle

import (
	"strings"

	"github.com/xo/dbmeta"
)

// changePassword builds ALTER USER ... IDENTIFIED BY.
//
// Oracle takes the password as a quoted identifier rather than a string
// literal, so it goes between double quotes. A quoted identifier has no
// escape for a double quote, so a password or a user name that holds one
// elsewhere is refused rather than guessed at.
//
// A plain user name folds to upper case, as Oracle stores it, so scott means
// SCOTT. A name given between double quotes is kept as it is written, so
// "scott" means the user created in lower case. Ken decided both for usql,
// whose rules these are.
//
// REPLACE names the current password. A user changing its own password needs
// it when the profile has a verify function, and it is written only when the
// caller gives one.
func changePassword(c dbmeta.PasswordChange, _ dbmeta.Quoting) (string, error) {
	user, err := oracleUser(c.User)
	if err != nil {
		return "", err
	}
	password, err := oracleQuoted(c.Password)
	if err != nil {
		return "", err
	}
	stmt := "ALTER USER " + user + " IDENTIFIED BY " + password
	if c.Old != "" {
		old, err := oracleQuoted(c.Old)
		if err != nil {
			return "", err
		}
		stmt += " REPLACE " + old
	}
	return stmt, nil
}

// oracleUser quotes a user name the way Oracle resolves it: upper case when it
// is plain, and as written when it is between double quotes.
func oracleUser(name string) (string, error) {
	if inner, ok := strings.CutPrefix(name, `"`); ok {
		if inner, ok = strings.CutSuffix(inner, `"`); ok && inner != "" {
			return oracleQuoted(inner)
		}
		return "", dbmeta.ErrInvalidPassword
	}
	return oracleQuoted(strings.ToUpper(name))
}

// oracleQuoted returns s between double quotes, and refuses a double quote
// inside it, which a quoted identifier cannot carry.
func oracleQuoted(s string) (string, error) {
	if strings.Contains(s, `"`) {
		return "", dbmeta.ErrInvalidPassword
	}
	return `"` + s + `"`, nil
}
