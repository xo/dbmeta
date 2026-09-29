package oracle

import (
	"errors"
	"testing"

	"github.com/xo/dbmeta"
)

// TestChangePassword checks the statement text. The escaping is proved
// against a real server in the test module, which logs in with each password
// it sets.
func TestChangePassword(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		in   dbmeta.PasswordChange
		want string
	}{
		{"a plain name folds to upper case", dbmeta.PasswordChange{User: "scott", Password: "tiger"},
			`ALTER USER "SCOTT" IDENTIFIED BY "tiger"`},
		{"a quoted name is kept as written", dbmeta.PasswordChange{User: `"scott"`, Password: "tiger"},
			`ALTER USER "scott" IDENTIFIED BY "tiger"`},
		// The password is an identifier, so a single quote, a backslash and
		// a semicolon are ordinary inside it.
		{"a literal's specials", dbmeta.PasswordChange{User: "SCOTT", Password: `a'b\c;--d`},
			`ALTER USER "SCOTT" IDENTIFIED BY "a'b\c;--d"`},
		{"the current password", dbmeta.PasswordChange{User: "SCOTT", Password: "new", Old: "old"},
			`ALTER USER "SCOTT" IDENTIFIED BY "new" REPLACE "old"`},
	} {
		got, err := dbmeta.Oracle.ChangePassword(c.in, dbmeta.Quoting{})
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}

// TestChangePasswordRefusesADoubleQuote checks each place a double quote
// cannot go. A quoted identifier has no escape for one, so the statement is
// refused rather than built wrong.
func TestChangePasswordRefusesADoubleQuote(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		in   dbmeta.PasswordChange
	}{
		{"in the password", dbmeta.PasswordChange{User: "SCOTT", Password: `a"b`}},
		{"in the current password", dbmeta.PasswordChange{User: "SCOTT", Password: "p", Old: `a"b`}},
		{"inside a quoted name", dbmeta.PasswordChange{User: `"sc"ott"`, Password: "p"}},
		{"in a plain name", dbmeta.PasswordChange{User: `sc"ott`, Password: "p"}},
		{"a quote that is never closed", dbmeta.PasswordChange{User: `"scott`, Password: "p"}},
		{"an empty quoted name", dbmeta.PasswordChange{User: `""`, Password: "p"}},
	} {
		if _, err := dbmeta.Oracle.ChangePassword(c.in, dbmeta.Quoting{}); !errors.Is(err, dbmeta.ErrInvalidPassword) {
			t.Errorf("%s: expected %v, got %v", c.name, dbmeta.ErrInvalidPassword, err)
		}
	}
}
