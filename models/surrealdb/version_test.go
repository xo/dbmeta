package surrealdb

import (
	"errors"
	"testing"

	"github.com/xo/dbmeta"
)

// TestParseVersion checks the answer parseVersion reads, which is the answer
// of the RPC method version.
func TestParseVersion(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		in      string
		main    string
		display string
	}{
		{"surrealdb-3.3.0", "3.3.0", "SurrealDB 3.3.0"},
		{"surrealdb-2.7.0", "2.7.0", "SurrealDB 2.7.0"},
		{"surrealdb-3.1.6+20260813.cfbaec4", "3.1.6-+20260813.cfbaec4", "SurrealDB 3.1.6+20260813.cfbaec4"},
	} {
		s, err := parseVersion([]string{c.in})
		if err != nil {
			t.Errorf("%q: %v", c.in, err)
			continue
		}
		if got := s.Main().String(); got != c.main {
			t.Errorf("%q: the release is %q, want %q", c.in, got, c.main)
		}
		if got := s.String(); got != c.display {
			t.Errorf("%q: the display line is %q, want %q", c.in, got, c.display)
		}
	}
	for _, in := range [][]string{{""}, {"surrealdb-"}, {"unknown"}, {}, {"surrealdb-3.3.0", "surrealdb-3.3.0"}} {
		if _, err := parseVersion(in); !errors.Is(err, dbmeta.ErrInvalidVersion) {
			t.Errorf("%q: expected ErrInvalidVersion, got %v", in, err)
		}
	}
}

// TestEveryKindButTheCurrentSchemaNeeds3 checks that 2.x is too old for every
// kind but the current schema, because a 2.x statement cannot read INFO as a
// value, and that 3.0 answers 18.
func TestEveryKindButTheCurrentSchemaNeeds3(t *testing.T) {
	t.Parallel()
	count := func(ver dbmeta.Version) (supported, tooOld int) {
		var s dbmeta.VersionSet
		s.Set("", ver)
		m, err := dbmeta.New(dbmeta.SurrealDB, s)
		if err != nil {
			t.Fatal(err)
		}
		for _, q := range dbmeta.Queries() {
			switch q.Support(m) {
			case dbmeta.Supported:
				supported++
			case dbmeta.TooOld:
				tooOld++
			}
		}
		return supported, tooOld
	}
	if s, o := count(dbmeta.V(2, 7, 0)); s != 1 || o != 17 {
		t.Errorf("2.7.0: %d supported and %d too old, want 1 and 17", s, o)
	}
	if s, o := count(dbmeta.V(3)); s != 18 || o != 0 {
		t.Errorf("3: %d supported and %d too old, want 18 and 0", s, o)
	}
}
