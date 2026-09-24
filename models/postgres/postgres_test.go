package postgres_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/postgres"
)

// releases that dbmeta supports, per D20. 9.6 comes from a release 15 or older
// checkout, the rest from the tree named in postgres.Tree.
var releases = []string{"9.6.24", "10.23", "11.22", "12.18", "13.15", "14.12", "15.7", "16.2", "17.4", "18.6"}

func meta(t *testing.T, ver string) *dbmeta.Meta {
	t.Helper()
	var s dbmeta.VersionSet
	s.Set("", dbmeta.ParseVersion(ver))
	s.Display = "PostgreSQL " + ver
	m, err := dbmeta.New(dbmeta.PostgreSQL, s)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	return m
}

// TestEveryReleaseResolves checks that every supported release produces a
// statement. A release with no applicable fragment would fail here.
// TestEveryReleaseResolves checks that each query either produces a statement
// or says the server is too old. An object that PostgreSQL did not have yet is
// the second case: publications and subscriptions arrived in release 10, so a
// 9.6 server is told the version is too old rather than handed an empty
// result. D34 requires that distinction.
func TestEveryReleaseResolves(t *testing.T) {
	t.Parallel()
	for _, ver := range releases {
		m := meta(t, ver)
		for _, q := range supported(t, m) {
			s, _, err := q.SQL(m, nil)
			switch {
			case errors.Is(err, dbmeta.ErrVersionTooOld):
				continue
			case err != nil:
				t.Errorf("%s at %s: expected no error, got: %v", q.Name(), ver, err)
			case !strings.HasPrefix(s, "SELECT "):
				t.Errorf("%s at %s: expected a select, got:\n%s", q.Name(), ver, s)
			}
		}
	}
}

// TestObjectsAddedInTen records which queries need a server newer than the
// floor, and asserts the boundary rather than leaving it implicit. Each of
// these objects arrived in release 10.
func TestObjectsAddedInTen(t *testing.T) {
	t.Parallel()
	for _, q := range []dbmeta.AnyQuery{
		dbmeta.Publications, dbmeta.PublicationTables,
		dbmeta.Subscriptions, dbmeta.ExtendedStats,
	} {
		if _, _, err := q.SQL(meta(t, "9.6.24"), nil); !errors.Is(err, dbmeta.ErrVersionTooOld) {
			t.Errorf("%s at 9.6: expected ErrVersionTooOld, got: %v", q.Name(), err)
		}
		if _, _, err := q.SQL(meta(t, "10.23"), nil); err != nil {
			t.Errorf("%s at 10.23: expected no error, got: %v", q.Name(), err)
		}
	}
}

// TestColumnSetNeverChanges is the rule the whole design rests on. Every
// release must return the same columns in the same order, or one scan function
// cannot read them all.
func TestColumnSetNeverChanges(t *testing.T) {
	t.Parallel()
	for _, q := range supported(t, meta(t, "18.6")) {
		var want []string
		for _, ver := range releases {
			m := meta(t, ver)
			s, _, err := q.SQL(m, nil)
			if errors.Is(err, dbmeta.ErrVersionTooOld) {
				// the object did not exist at this release
				continue
			}
			if err != nil {
				t.Fatalf("%s at %s: expected no error, got: %v", q.Name(), ver, err)
			}
			got := namedColumns(s)
			if want == nil {
				want = got
				if len(want) == 0 {
					t.Fatalf("%s: expected named columns", q.Name())
				}
				continue
			}
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("%s at %s: column set changed\n want %v\n  got %v", q.Name(), ver, want, got)
			}
		}
	}
}

// TestFieldsMatchTheStatement checks that the declared fields are the columns
// the statement actually selects, in order. A generator will emit both, and
// this is what catches them drifting apart.
func TestFieldsMatchTheStatement(t *testing.T) {
	t.Parallel()
	m := meta(t, "18.6")
	for _, q := range supported(t, m) {
		fields, err := q.Fields(m)
		if err != nil {
			t.Fatalf("%s: expected no error, got: %v", q.Name(), err)
		}
		names := make([]string, len(fields))
		for i, f := range fields {
			names[i] = f.Name
		}
		s, _, err := q.SQL(m, nil)
		if err != nil {
			t.Fatalf("%s: expected no error, got: %v", q.Name(), err)
		}
		if got := namedColumns(s); strings.Join(names, ",") != strings.Join(got, ",") {
			t.Errorf("%s: declared fields and selected columns differ\n fields  %v\n columns %v",
				q.Name(), names, got)
		}
	}
}

// TestPaddingAtOldReleases covers the two gated columns of the column query.
// attidentity arrived in release 11 and attgenerated in release 12.
func TestPaddingAtOldReleases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		ver      string
		contains string
	}{
		{"10.23", `, NULL AS "identity"`},
		{"10.23", `, NULL AS "generated"`},
		{"11.22", `a.attidentity`},
		{"11.22", `, NULL AS "generated"`},
		{"12.18", `a.attidentity`},
		{"12.18", `a.attgenerated`},
		{"18.6", `a.attgenerated`},
	}
	for _, test := range tests {
		s, _, err := dbmeta.Columns.SQL(meta(t, test.ver), nil)
		if err != nil {
			t.Fatalf("%s: expected no error, got: %v", test.ver, err)
		}
		if !strings.Contains(s, test.contains) {
			t.Errorf("%s: expected %q in:\n%s", test.ver, test.contains, s)
		}
	}
}

// TestFieldMinMatchesTheGate checks that a gated column declares the same
// minimum its fragment does. If they disagree, a caller is told a field is
// present when the statement padded it.
func TestFieldMinMatchesTheGate(t *testing.T) {
	t.Parallel()
	fields, err := dbmeta.Columns.Fields(meta(t, "18.6"))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	mins := map[string]string{"identity": "11", "generated": "12"}
	for _, f := range fields {
		want, gated := mins[f.Name]
		switch {
		case gated && f.Min.String() != want:
			t.Errorf("%s: expected a minimum of %s, got %s", f.Name, want, f.Min)
		case !gated && !f.Min.IsZero():
			t.Errorf("%s: expected no minimum, got %s", f.Name, f.Min)
		}
	}
	// and the statement pads exactly when the field says it will
	for name, minVer := range mins {
		for _, ver := range releases {
			s, _, err := dbmeta.Columns.SQL(meta(t, ver), nil)
			if err != nil {
				t.Fatal(err)
			}
			padded := strings.Contains(s, `, NULL AS "`+name+`"`)
			old := !dbmeta.ParseVersion(ver).AtLeast(dbmeta.ParseVersion(minVer))
			if padded != old {
				t.Errorf("%s at %s: padded=%v but the field minimum says old=%v", name, ver, padded, old)
			}
		}
	}
}

func TestPlaceholders(t *testing.T) {
	t.Parallel()
	s, args, err := dbmeta.Tables.SQL(meta(t, "16.2"), dbmeta.Args{Schema: "public", Name: "book"}.Map())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	for _, want := range []string{"$1", "$2", "$3"} {
		if !strings.Contains(s, want) {
			t.Errorf("expected %s in:\n%s", want, s)
		}
	}
	if strings.Contains(s, "@") {
		t.Errorf("expected every named parameter to be rewritten:\n%s", s)
	}
	// a parameter named twice in the statement binds twice, because a
	// placeholder style like MySQL's consumes one argument per placeholder
	if len(args) != 5 {
		t.Errorf("expected five arguments, got %v", args)
	}
}

// TestDefaultsMeanEveryOne checks that a caller can ask for everything without
// naming a parameter, while a misspelled parameter is still an error.
func TestDefaultsMeanEveryOne(t *testing.T) {
	t.Parallel()
	m := meta(t, "16.2")
	if _, _, err := dbmeta.Tables.SQL(m, nil); err != nil {
		t.Errorf("expected no error with no arguments, got: %v", err)
	}
	_, _, err := dbmeta.Tables.SQL(m, map[string]any{"schma": "public"})
	if err == nil {
		t.Error("expected a misspelled parameter to be an error")
	}
}

func TestParseVersion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		raw     string
		version string
		display string
	}{
		{"16.2", "16.2", "PostgreSQL 16.2"},
		{"9.6.24", "9.6.24", "PostgreSQL 9.6.24"},
		{"16.2 (Debian 16.2-1.pgdg120+2)", "16.2", "PostgreSQL 16.2 (Debian 16.2-1.pgdg120+2)"},
	}
	for _, test := range tests {
		set, err := dbmeta.PostgreSQL.ParseVersion([]string{test.raw})
		if err != nil {
			t.Fatalf("%s: expected no error, got: %v", test.raw, err)
		}
		if got := set.Main(); got.String() != test.version && !strings.HasPrefix(got.String(), test.version) {
			t.Errorf("%s: expected %s, got %s", test.raw, test.version, got)
		}
		if set.Display != test.display {
			t.Errorf("%s: expected display %q, got %q", test.raw, test.display, set.Display)
		}
	}
}

func TestVersionQuery(t *testing.T) {
	t.Parallel()
	s, n, ok := dbmeta.PostgreSQL.VersionQuery()
	if !ok || s != "SHOW server_version" || n != 1 {
		t.Errorf("unexpected version query %q, %d, %v", s, n, ok)
	}
}

// supported returns every query PostgreSQL answers, so a test covers each new
// one the moment it is registered and nothing has to be listed by hand.
func supported(t *testing.T, m *dbmeta.Meta) []dbmeta.AnyQuery {
	t.Helper()
	var out []dbmeta.AnyQuery
	for _, q := range dbmeta.Queries() {
		if q.Support(m) == dbmeta.Supported {
			out = append(out, q)
		}
	}
	if len(out) == 0 {
		t.Fatal("expected PostgreSQL to answer at least one query")
	}
	return out
}

// namedColumns returns the names a statement gives its result columns, in
// order, by reading every AS "name" it contains.
func namedColumns(s string) []string {
	var out []string
	for {
		i := strings.Index(s, ` AS "`)
		if i < 0 {
			return out
		}
		s = s[i+5:]
		j := strings.IndexByte(s, '"')
		if j < 0 {
			return out
		}
		out = append(out, s[:j])
		s = s[j+1:]
	}
}

// TestCoverage records which queries PostgreSQL answers, so the count is
// visible and a regression that silently drops one is obvious.
func TestCoverage(t *testing.T) {
	t.Parallel()
	m := meta(t, "18.6")
	var named []string
	for _, q := range dbmeta.Queries() {
		if q.Support(m) == dbmeta.Supported {
			named = append(named, q.Name())
		}
	}
	t.Logf("PostgreSQL answers %d of %d declared queries: %s",
		len(named), len(dbmeta.Queries()), strings.Join(named, " "))
	if len(named) < 40 {
		t.Errorf("expected at least 40 queries, got %d", len(named))
	}
}

// TestNoCoalesceOnCatalogColumns guards the fix for a real bug. Wrapping a
// nullable catalog column in COALESCE collapses two different answers into
// one. PostgreSQL reports a NULL access control list when the default
// privileges apply and an empty list when every privilege was revoked, and
// psql prints the second as "(none)". Coalescing both to an empty string makes
// "the owner has full access" read the same as "nobody has any access".
//
// COALESCE is still right over an aggregate that matched no rows, where NULL
// and empty mean the same thing, so this checks the columns rather than the
// count.
func TestNoCoalesceOnCatalogColumns(t *testing.T) {
	t.Parallel()
	m := meta(t, "18.6")
	for _, q := range supported(t, m) {
		s, _, err := q.SQL(m, nil)
		if err != nil {
			t.Fatalf("%s: expected no error, got: %v", q.Name(), err)
		}
		for _, name := range []string{"access", "comment", "default", "options"} {
			if strings.Contains(s, `, '') AS "`+name+`"`) {
				t.Errorf("%s: %q is a nullable catalog column and must not be coalesced", q.Name(), name)
			}
		}
	}
}
