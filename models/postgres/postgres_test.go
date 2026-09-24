package postgres_test

import (
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
func TestEveryReleaseResolves(t *testing.T) {
	t.Parallel()
	for _, ver := range releases {
		m := meta(t, ver)
		for name, sqlOf := range statements() {
			s, err := sqlOf(m)
			if err != nil {
				t.Errorf("%s at %s: expected no error, got: %v", name, ver, err)
				continue
			}
			if !strings.HasPrefix(s, "SELECT ") {
				t.Errorf("%s at %s: expected a select, got:\n%s", name, ver, s)
			}
		}
	}
}

// TestColumnSetNeverChanges is the rule the whole design rests on. Every
// release must return the same columns in the same order, or one scan function
// cannot read them all.
func TestColumnSetNeverChanges(t *testing.T) {
	t.Parallel()
	for name, sqlOf := range statements() {
		var want []string
		for _, ver := range releases {
			s, err := sqlOf(meta(t, ver))
			if err != nil {
				t.Fatalf("%s at %s: expected no error, got: %v", name, ver, err)
			}
			got := namedColumns(s)
			if want == nil {
				want = got
				if len(want) == 0 {
					t.Fatalf("%s: expected named columns", name)
				}
				continue
			}
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("%s at %s: column set changed\n want %v\n  got %v", name, ver, want, got)
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
	for name, pair := range fieldsAndSQL(t, m) {
		if strings.Join(pair.fields, ",") != strings.Join(pair.columns, ",") {
			t.Errorf("%s: declared fields and selected columns differ\n fields  %v\n columns %v",
				name, pair.fields, pair.columns)
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
		{"10.23", `, '' AS "identity"`},
		{"10.23", `, '' AS "generated"`},
		{"11.22", `a.attidentity`},
		{"11.22", `, '' AS "generated"`},
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
			padded := strings.Contains(s, `, '' AS "`+name+`"`)
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
	if len(args) != 3 {
		t.Errorf("expected three arguments, got %v", args)
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

// statements returns a way to render each registered query, so a test can
// treat them alike despite their different result types.
func statements() map[string]func(*dbmeta.Meta) (string, error) {
	return map[string]func(*dbmeta.Meta) (string, error){
		"schemas": func(m *dbmeta.Meta) (string, error) {
			s, _, err := dbmeta.Schemas.SQL(m, nil)
			return s, err
		},
		"tables": func(m *dbmeta.Meta) (string, error) {
			s, _, err := dbmeta.Tables.SQL(m, nil)
			return s, err
		},
		"columns": func(m *dbmeta.Meta) (string, error) {
			s, _, err := dbmeta.Columns.SQL(m, nil)
			return s, err
		},
	}
}

type pair struct {
	fields  []string
	columns []string
}

func fieldsAndSQL(t *testing.T, m *dbmeta.Meta) map[string]pair {
	t.Helper()
	out := map[string]pair{}
	add := func(name string, fields []dbmeta.Field, err error, sqlstr string) {
		if err != nil {
			t.Fatalf("%s: expected no error, got: %v", name, err)
		}
		names := make([]string, len(fields))
		for i, f := range fields {
			names[i] = f.Name
		}
		out[name] = pair{fields: names, columns: namedColumns(sqlstr)}
	}
	f, err := dbmeta.Schemas.Fields(m)
	s, _, _ := dbmeta.Schemas.SQL(m, nil)
	add("schemas", f, err, s)
	f, err = dbmeta.Tables.Fields(m)
	s, _, _ = dbmeta.Tables.SQL(m, nil)
	add("tables", f, err, s)
	f, err = dbmeta.Columns.Fields(m)
	s, _, _ = dbmeta.Columns.SQL(m, nil)
	add("columns", f, err, s)
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
