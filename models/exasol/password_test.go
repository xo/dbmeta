package exasol

import (
	"testing"

	"github.com/xo/dbmeta"
)

// TestChangePassword checks the statement text. The escaping is proved
// against a real server in the test module, which logs in with each
// password it sets.
func TestChangePassword(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		in   dbmeta.PasswordChange
		want string
	}{
		{"plain", dbmeta.PasswordChange{User: "BOB", Password: "hunter2"},
			`ALTER USER "BOB" IDENTIFIED BY "hunter2"`},
		// The password is an identifier, so a double quote is the one
		// character that is doubled.
		{"a double quote", dbmeta.PasswordChange{User: "BOB", Password: `a"b`},
			`ALTER USER "BOB" IDENTIFIED BY "a""b"`},
		// A single quote, a backslash and a semicolon are ordinary inside an
		// identifier and are left alone.
		{"a literal's specials", dbmeta.PasswordChange{User: "BOB", Password: `a'b\;`},
			`ALTER USER "BOB" IDENTIFIED BY "a'b\;"`},
		{"the user is quoted too", dbmeta.PasswordChange{User: `B"OB`, Password: "p"},
			`ALTER USER "B""OB" IDENTIFIED BY "p"`},
		{"the current password", dbmeta.PasswordChange{User: "BOB", Password: "new", Old: `o"ld`},
			`ALTER USER "BOB" IDENTIFIED BY "new" REPLACE "o""ld"`},
	} {
		got, err := dbmeta.Exasol.ChangePassword(c.in, dbmeta.Quoting{})
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}
