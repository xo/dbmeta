package test

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/redshift"
)

// redshiftPunctuation is every printable ASCII character that is not a letter
// or a digit, and the documentation of Redshift forbids five of them.
const redshiftPunctuation = "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~ "

// TestChangePasswordRedshift sets each password on a user that the test makes
// and logs in with it. The administrator is never changed. The documentation
// of Redshift forbids a quote, a double quote, a backslash, a slash, an at
// sign and a space, and a password of more than 64 characters. D204 measured
// that the server accepts every one of them, so they are in the list.
func TestChangePasswordRedshift(t *testing.T) {
	db := openRedshift(t)
	const user = "dbmeta_pw"
	exec(t, db, `DROP USER IF EXISTS `+user)
	exec(t, db, `CREATE USER `+user+` PASSWORD 'Start-P4ssw0rd'`)
	t.Cleanup(func() { cleanup(t, db, `DROP USER IF EXISTS `+user) })
	base := dsnOf(t, "DBMETA_REDSHIFT")

	// The server wants 8 characters with an upper case letter, a lower case
	// letter and a digit, so each password has all of them.
	passwords := append([]struct{ name, password string }{
		{"a slash", `Aa1/bcdefg`},
		{"an at sign", `Aa1@bcdefg`},
		{"a space", `Aa1 bcdefg`},
		{"a percent sign", "P4ss%s%d%%Aa"},
		{"dollar signs", "P4ss$$Aa$1$x"},
		{"every punctuation mark", "Aa1" + redshiftPunctuation},
		{"64 characters", "Aa1" + strings.Repeat("x", 61)},
		{"65 characters", "Aa1" + strings.Repeat("x", 62)},
	}, shortened()...)
	for _, c := range passwords {
		t.Run(c.name, func(t *testing.T) {
			stmt, err := dbmeta.Redshift.ChangePassword(
				dbmeta.PasswordChange{User: user, Password: c.password}, dbmeta.Quoting{})
			if err != nil {
				t.Fatalf("building the statement: %v", err)
			}
			exec(t, db, stmt)
			login(t, "pgx", replaceUser(t, base, user, c.password), `SELECT current_user`, user)
		})
	}

	// The three rules the server does enforce, with the words it answers.
	refused := []struct{ name, password, message string }{
		{"7 characters", "Aa1xxxx", "at least 8 characters"},
		{"no digit", "Aaxxxxxxx", "must contain a number"},
		{"no upper case letter", "aa1xxxxxx", "uppercase"},
		{"no lower case letter", "AA1XXXXXX", "lowercase"},
	}
	for _, c := range refused {
		t.Run("the server refuses "+c.name, func(t *testing.T) {
			stmt, err := dbmeta.Redshift.ChangePassword(
				dbmeta.PasswordChange{User: user, Password: c.password}, dbmeta.Quoting{})
			if err != nil {
				t.Fatalf("building the statement: %v", err)
			}
			_, err = db.ExecContext(t.Context(), stmt)
			if err == nil || !strings.Contains(err.Error(), c.message) {
				t.Errorf("expected the server to refuse with %q, got %v", c.message, err)
			}
		})
	}
}

// shortened is the hostile passwords of D127 with a prefix. One of them has 7
// characters, and the server wants 8 or more, so the prefix has the upper case
// letter, the lower case letter and the digit as well.
func shortened() []struct{ name, password string } {
	out := make([]struct{ name, password string }, len(hostilePasswords))
	for i, c := range hostilePasswords {
		out[i] = struct{ name, password string }{c.name, "Aa1-" + c.password}
	}
	return out
}

// TestChangePasswordRedshiftQuoting needs no server. The statement is the
// same one whatever the session says.
func TestChangePasswordRedshiftQuoting(t *testing.T) {
	got, err := dbmeta.Redshift.ChangePassword(
		dbmeta.PasswordChange{User: `a"b`, Password: `x'y\z`},
		dbmeta.Quoting{BackslashEscapes: sql.Null[bool]{V: false, Valid: true}})
	if err != nil {
		t.Fatalf("building the statement: %v", err)
	}
	want := `ALTER USER "a""b" PASSWORD 'x''y\\z'`
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}
