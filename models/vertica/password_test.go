package vertica

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/xo/dbmeta"
)

// TestChangePassword checks the statement text under both settings. The
// escaping is proved against a real server in the test module, which logs in
// with each password it sets.
func TestChangePassword(t *testing.T) {
	t.Parallel()
	escaping := dbmeta.Quoting{BackslashEscapes: sql.Null[bool]{V: true, Valid: true}}
	plain := dbmeta.Quoting{BackslashEscapes: sql.Null[bool]{V: false, Valid: true}}
	for _, c := range []struct {
		name string
		in   dbmeta.PasswordChange
		q    dbmeta.Quoting
		want string
	}{
		{"plain", dbmeta.PasswordChange{User: "bob", Password: "hunter2"}, plain,
			`ALTER USER "bob" IDENTIFIED BY 'hunter2'`},
		{"a quote", dbmeta.PasswordChange{User: "bob", Password: "a'b"}, plain,
			`ALTER USER "bob" IDENTIFIED BY 'a''b'`},
		{"a trailing backslash, escaping", dbmeta.PasswordChange{User: "bob", Password: `x\`}, escaping,
			`ALTER USER "bob" IDENTIFIED BY 'x\\'`},
		{"a trailing backslash, conforming", dbmeta.PasswordChange{User: "bob", Password: `x\`}, plain,
			`ALTER USER "bob" IDENTIFIED BY 'x\'`},
		{"the user is quoted", dbmeta.PasswordChange{User: `b"ob`, Password: "p"}, plain,
			`ALTER USER "b""ob" IDENTIFIED BY 'p'`},
		{"the current password", dbmeta.PasswordChange{User: "bob", Password: "new", Old: "o'ld"}, plain,
			`ALTER USER "bob" IDENTIFIED BY 'new' REPLACE 'o''ld'`},
	} {
		got, err := dbmeta.Vertica.ChangePassword(c.in, c.q)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
	if _, err := dbmeta.Vertica.ChangePassword(
		dbmeta.PasswordChange{User: "bob", Password: "p"}, dbmeta.Quoting{}); !errors.Is(err, dbmeta.ErrQuotingUnknown) {
		t.Errorf("expected an unread quoting state to be refused, got %v", err)
	}
}

// TestParseQuoting reads the two columns SHOW returns.
func TestParseQuoting(t *testing.T) {
	t.Parallel()
	for setting, want := range map[string]bool{"on": false, "off": true} {
		q, err := parseQuoting([]string{"standard_conforming_strings", setting})
		if err != nil || !q.BackslashEscapes.Valid || q.BackslashEscapes.V != want {
			t.Errorf("%s: got %v, %v", setting, q, err)
		}
	}
	if _, err := parseQuoting([]string{"on"}); !errors.Is(err, dbmeta.ErrQuotingUnknown) {
		t.Errorf("expected one column to be refused, got %v", err)
	}
}
