package dbmeta

import (
	"errors"
	"strings"
	"testing"
)

var (
	v96 = Version{Major: 9, Minor: 6}
	v11 = Version{Major: 11}
	v12 = Version{Major: 12}
	v15 = Version{Major: 15}
	v18 = Version{Major: 18}
)

func TestChoiceResolve(t *testing.T) {
	t.Parallel()
	c := Choice{
		{SQL: "old"},
		{Min: v12, SQL: "twelve"},
		{Min: v15, SQL: "fifteen"},
	}
	tests := []struct {
		ver Version
		exp string
	}{
		{v96, "old"},
		{v11, "old"},
		{v12, "twelve"},
		{Version{Major: 13}, "twelve"},
		{v15, "fifteen"},
		{v18, "fifteen"},
	}
	for _, test := range tests {
		s, err := c.Resolve(test.ver)
		if err != nil {
			t.Fatalf("%v: expected no error, got: %v", test.ver, err)
		}
		if s != test.exp {
			t.Errorf("%v: expected %q, got %q", test.ver, test.exp, s)
		}
	}
}

// TestChoiceResolveUnordered checks that the order the alternatives are
// written in does not change the answer.
func TestChoiceResolveUnordered(t *testing.T) {
	t.Parallel()
	c := Choice{
		{Min: v15, SQL: "fifteen"},
		{SQL: "old"},
		{Min: v12, SQL: "twelve"},
	}
	for ver, exp := range map[Version]string{v96: "old", v12: "twelve", v18: "fifteen"} {
		s, err := c.Resolve(ver)
		if err != nil {
			t.Fatalf("%v: expected no error, got: %v", ver, err)
		}
		if s != exp {
			t.Errorf("%v: expected %q, got %q", ver, exp, s)
		}
	}
}

func TestChoiceResolveTooOld(t *testing.T) {
	t.Parallel()
	c := Choice{{Min: v12, SQL: "twelve"}}
	if _, err := c.Resolve(v11); !errors.Is(err, ErrVersionTooOld) {
		t.Errorf("expected ErrVersionTooOld, got: %v", err)
	}
	if _, err := (Choice{}).Resolve(v18); !errors.Is(err, ErrVersionTooOld) {
		t.Errorf("expected ErrVersionTooOld for an empty choice, got: %v", err)
	}
}

// TestPaddingBothDirections is the rule the whole design rests on. A query
// returns the same columns on every version. A column with no source on a
// server is selected as a literal NULL there, and that happens on the old side
// and on the new side.
//
// The new side is the case that an earlier draft of the plan missed.
// pg_attrdef.adsrc exists through release 11 and was removed in release 12.
func TestPaddingBothDirections(t *testing.T) {
	t.Parallel()
	q := Query{
		{{SQL: `SELECT`}},
		// added in 15, so releases below 15 pad
		{
			{SQL: `  NULL AS "access_privileges"`},
			{Min: v15, SQL: `  p.paracl AS "access_privileges"`},
		},
		// removed in 12, so releases from 12 pad
		{
			{SQL: `, d.adsrc AS "default_source"`},
			{Min: v12, SQL: `, NULL AS "default_source"`},
		},
		{{SQL: `FROM pg_catalog.pg_attrdef d`}},
	}
	exp := map[Version][]string{
		v11: {`NULL AS "access_privileges"`, `d.adsrc AS "default_source"`},
		v12: {`NULL AS "access_privileges"`, `NULL AS "default_source"`},
		v18: {`p.paracl AS "access_privileges"`, `NULL AS "default_source"`},
	}
	for ver, want := range exp {
		sql, err := q.SQL(ver)
		if err != nil {
			t.Fatalf("%v: expected no error, got: %v", ver, err)
		}
		for _, w := range want {
			if !strings.Contains(sql, w) {
				t.Errorf("%v: expected the query to contain %q, got:\n%s", ver, w, sql)
			}
		}
		// the column set must be identical on every version
		if n := strings.Count(sql, `AS "`); n != 2 {
			t.Errorf("%v: expected 2 named columns, got %d:\n%s", ver, n, sql)
		}
	}
}

func TestQuerySQL(t *testing.T) {
	t.Parallel()
	q := Query{
		{{SQL: "SELECT a"}},
		{{SQL: ", b"}, {Min: v15, SQL: ", c"}},
		{{SQL: "FROM t"}},
	}
	s, err := q.SQL(v96)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if exp := "SELECT a\n, b\nFROM t"; s != exp {
		t.Errorf("expected %q, got %q", exp, s)
	}
	if s, err = q.SQL(v18); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if exp := "SELECT a\n, c\nFROM t"; s != exp {
		t.Errorf("expected %q, got %q", exp, s)
	}
}

func TestQuerySQLEmpty(t *testing.T) {
	t.Parallel()
	if _, err := (Query{}).SQL(v18); !errors.Is(err, ErrEmptyQuery) {
		t.Errorf("expected ErrEmptyQuery, got: %v", err)
	}
	// every piece resolving to nothing is also empty
	q := Query{{{SQL: "  "}}, {{SQL: ""}}}
	if _, err := q.SQL(v18); !errors.Is(err, ErrEmptyQuery) {
		t.Errorf("expected ErrEmptyQuery, got: %v", err)
	}
}

func TestQuerySQLTooOld(t *testing.T) {
	t.Parallel()
	q := Query{{{SQL: "SELECT a"}}, {{Min: v12, SQL: ", b"}}}
	if _, err := q.SQL(v11); !errors.Is(err, ErrVersionTooOld) {
		t.Errorf("expected ErrVersionTooOld, got: %v", err)
	}
}

func TestAlways(t *testing.T) {
	t.Parallel()
	q := Always("SELECT 1")
	for _, ver := range []Version{v96, v12, v18, {}} {
		s, err := q.SQL(ver)
		if err != nil {
			t.Fatalf("%v: expected no error, got: %v", ver, err)
		}
		if s != "SELECT 1" {
			t.Errorf("%v: expected %q, got %q", ver, "SELECT 1", s)
		}
	}
}

func TestErrorsAreConstants(t *testing.T) {
	t.Parallel()
	// wrapping must keep errors.Is working
	err := errWrap(ErrNotSupported)
	if !errors.Is(err, ErrNotSupported) {
		t.Errorf("expected the wrapped error to match ErrNotSupported, got: %v", err)
	}
	if errors.Is(err, ErrNotFound) {
		t.Error("expected the wrapped error not to match ErrNotFound")
	}
	if s := ErrNotSupported.Error(); s != "not supported" {
		t.Errorf("expected %q, got %q", "not supported", s)
	}
}
