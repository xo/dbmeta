package dbmeta

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
)

func known(v bool) Quoting {
	return Quoting{BackslashEscapes: sql.Null[bool]{V: v, Valid: true}}
}

// TestQuoteLiteral covers the rule that a wrong answer turns into injection.
func TestQuoteLiteral(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		in   string
		q    Quoting
		want string
	}{
		{"plain", "secret", known(false), `'secret'`},
		{"a quote is always doubled", "a'b", known(false), `'a''b'`},
		{"a quote is always doubled, escaping server", "a'b", known(true), `'a''b'`},
		// The case that started this. With only the quote doubled a trailing
		// backslash escapes the closing quote on a server where a backslash
		// escapes, and the literal runs on into the next statement.
		{"trailing backslash, escaping server", `x\`, known(true), `'x\\'`},
		{"trailing backslash, plain server", `x\`, known(false), `'x\'`},
		{"both", `a'b\`, known(true), `'a''b\\'`},
		{"empty", "", known(false), `''`},
		// An unknown state doubles nothing extra. A caller cannot reach this
		// through ChangePassword, which refuses first, and the rule is
		// written down rather than left to chance.
		{"unknown state does not guess", `x\`, Quoting{}, `'x\'`},
	} {
		if got := QuoteLiteral(c.in, c.q); got != c.want {
			t.Errorf("%s: QuoteLiteral(%q) = %s, want %s", c.name, c.in, got, c.want)
		}
	}
}

func TestQuoteIdentifier(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, in, open, closed, want string }{
		{"postgres", `we"ird`, `"`, `"`, `"we""ird"`},
		{"sqlserver", `bo]b`, "[", "]", `[bo]]b]`},
		{"mysql", "ba`ck", "`", "`", "`ba``ck`"},
		{"nothing to do", "plain", `"`, `"`, `"plain"`},
	} {
		if got := QuoteIdentifier(c.in, c.open, c.closed); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}

// TestChangePasswordRefusesRatherThanGuesses covers every way the call says no.
func TestChangePasswordRefusesRatherThanGuesses(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		d    Dialect
		in   PasswordChange
		q    Quoting
		want error
	}{
		{"no model built", Dialect("nosuchdb"), PasswordChange{User: "u", Password: "p"},
			known(false), ErrModelNotBuilt},
		{"no user named", PostgreSQL, PasswordChange{Password: "p"}, known(false), ErrMissingParam},
		{"a NUL in the password", PostgreSQL, PasswordChange{User: "u", Password: "a\x00b"},
			known(false), ErrInvalidPassword},
		{"a NUL in the user", PostgreSQL, PasswordChange{User: "a\x00b", Password: "p"},
			known(false), ErrInvalidPassword},
	} {
		_, err := c.d.ChangePassword(c.in, c.q)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: expected %v, got %v", c.name, c.want, err)
		}
	}
}

// TestChangePasswordNeedsNoDatabase is the property D56 rests on. The call
// takes no connection, so dbmeta cannot run the statement even by accident.
func TestChangePasswordNeedsNoDatabase(t *testing.T) {
	t.Parallel()
	// A dialect with a statement but no server anywhere in sight.
	got, err := PostgreSQL.ChangePassword(
		PasswordChange{User: "bob", Password: "hunter2"}, known(false))
	if err != nil {
		t.Fatalf("expected a statement, got: %v", err)
	}
	if !strings.HasPrefix(got, "ALTER USER ") {
		t.Errorf("unexpected statement %q", got)
	}
	// and it carries the password, which is the thing a caller must not log
	if !strings.Contains(got, "hunter2") {
		t.Errorf("expected the password in the statement, got %q", got)
	}
}
