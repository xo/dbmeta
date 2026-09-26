package hive

import (
	"errors"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
)

// TestLiteral is the guard on the one thing this model does that no other
// model does: it writes a value into a statement.
//
// Hive has no parameter channel, so there is no binding to fall back on and
// the escaping is the whole of the protection. See D78.
func TestLiteral(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		in   any
		want string
	}{
		{"plain", "author", `'author'`},
		{"empty", "", `''`},
		{"pattern", "auth%", `'auth%'`},
		// The one Hive gets wrong if a doubled quote is used instead.
		{"quote", "o'brien", `'o\'brien'`},
		{"two quotes", "''", `'\'\''`},
		// The backslash is escaped first, so the escape this function
		// writes for the quote is not itself escaped away.
		{"backslash", `a\b`, `'a\\b'`},
		{"backslash then quote", `a\'b`, `'a\\\'b'`},
		// A closing quote followed by SQL is the shape that matters.
		{"break out", `x' OR '1'='1`, `'x\' OR \'1\'=\'1'`},
		{"comment", `x' --`, `'x\' --'`},
		{"true", true, `TRUE`},
		{"false", false, `FALSE`},
		{"int", 7, `7`},
	} {
		got, err := literal(c.in)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: literal(%#v) = %s, want %s", c.name, c.in, got, c.want)
		}
	}
}

// TestLiteralRefuses checks the two things it will not render.
func TestLiteralRefuses(t *testing.T) {
	t.Parallel()
	if _, err := literal("a\x00b"); !errors.Is(err, dbmeta.ErrInvalidParam) {
		t.Errorf("expected a NUL to be refused, got %v", err)
	}
	if _, err := literal(3.5); !errors.Is(err, dbmeta.ErrInvalidParam) {
		t.Errorf("expected an unknown type to be refused, got %v", err)
	}
}

// TestEveryQuoteIsAnAlias holds the invariant backtick relies on.
//
// The conversion turns every double quote in a fragment into a backtick, so
// a double quoted string literal in one of these statements would silently
// become an identifier. Every literal here is single quoted and this is what
// says so.
func TestEveryQuoteIsAnAlias(t *testing.T) {
	t.Parallel()
	var set dbmeta.VersionSet
	set.Set("", dbmeta.V(9999))
	m, err := dbmeta.New(dbmeta.Hive, set)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range dbmeta.Queries() {
		if q.Support(m) != dbmeta.Supported {
			continue
		}
		s, _, err := q.Build(m, nil)
		if err != nil {
			t.Errorf("%s: %v", q.Name(), err)
			continue
		}
		if strings.Contains(s, `"`) {
			t.Errorf("%s: a double quote survived the conversion, so a"+
				" fragment used one for something other than an alias:\n%s",
				q.Name(), s)
		}
	}
}

// TestNoValuesAreBound checks that a Hive statement carries its own values.
//
// A value returned here would be handed to a driver that discards it, which
// is the failure D78 exists to prevent.
func TestNoValuesAreBound(t *testing.T) {
	t.Parallel()
	var set dbmeta.VersionSet
	set.Set("", dbmeta.V(9999))
	m, err := dbmeta.New(dbmeta.Hive, set)
	if err != nil {
		t.Fatal(err)
	}
	args := dbmeta.Args{Schema: "s", Name: "n"}.Map()
	for _, q := range dbmeta.Queries() {
		if q.Support(m) != dbmeta.Supported {
			continue
		}
		params, err := q.Params(m)
		if err != nil {
			t.Fatal(err)
		}
		use := map[string]any{}
		for _, p := range params {
			if v, ok := args[p.Name]; ok {
				use[p.Name] = v
			}
		}
		s, vals, err := q.Build(m, use)
		if err != nil {
			t.Errorf("%s: %v", q.Name(), err)
			continue
		}
		if len(vals) != 0 {
			t.Errorf("%s: returned %d values and Hive cannot bind one", q.Name(), len(vals))
		}
		if strings.Contains(s, "?") {
			t.Errorf("%s: a placeholder reached the statement:\n%s", q.Name(), s)
		}
	}
}
