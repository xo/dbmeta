package dbmeta

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
)

const testDialect Dialect = "testdb"

func init() {
	RegisterDialect(quietDialect, &Info{Placeholder: func(int) string { return "?" }})
	RegisterDialect(seqDialect, &Info{Placeholder: func(int) string { return "?" }})
	repeatedQuery.Register(seqDialect, &Binding[Table]{
		Stmt:   Always(`SELECT 1 WHERE (@name = '' OR a LIKE @name)`),
		Params: []Param{{Name: "name", Default: ""}},
	})
	RegisterDialect(testDialect, &Info{
		Placeholder:    func(n int) string { return "$" + string(rune('0'+n)) },
		VersionSQL:     `SHOW server_version`,
		VersionColumns: 1,
		ParseVersion: func(cols []string) (VersionSet, error) {
			var s VersionSet
			s.Set("", ParseVersion(cols[0]))
			s.Display = "TestDB " + cols[0]
			return s, nil
		},
	})
	Tables.Register(testDialect, &Binding[Table]{
		Stmt: Stmt{
			{{SQL: `SELECT n.nspname AS "schema"`}},
			{{SQL: `, c.relname AS "name"`}},
			// added in 15, so older releases pad
			{
				{SQL: `, NULL AS "access_privileges"`},
				{Min: V(15), SQL: `, c.relacl AS "access_privileges"`},
			},
			// removed in 12, so newer releases pad
			{
				{SQL: `, d.adsrc AS "default_source"`},
				{Min: V(12), SQL: `, NULL AS "default_source"`},
			},
			{{SQL: `FROM pg_class c WHERE n.nspname = @schema`}},
		},
		Fields: []Field{
			{Name: "schema"},
			{Name: "name"},
			{Name: "access_privileges", Min: V(15)},
			{Name: "default_source"},
		},
		Params: []Param{{Name: "schema"}},
		Scan: func(rows *sql.Rows) (Table, error) {
			var t Table
			var priv, def *string
			err := rows.Scan(&t.Schema, &t.Name, &priv, &def)
			return t, err
		},
	})
}

func meta(t *testing.T, ver string) *Meta {
	t.Helper()
	var s VersionSet
	s.Set("", ParseVersion(ver))
	m, err := New(testDialect, s)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	return m
}

// TestPaddingBothDirections is the rule the whole design rests on. A query
// returns the same columns on every version. Padding happens on the old side
// and on the new side, and the new side is the case an earlier draft missed.
func TestPaddingBothDirections(t *testing.T) {
	t.Parallel()
	want := map[string][]string{
		"11": {`NULL AS "access_privileges"`, `d.adsrc AS "default_source"`},
		"12": {`NULL AS "access_privileges"`, `NULL AS "default_source"`},
		"18": {`c.relacl AS "access_privileges"`, `NULL AS "default_source"`},
	}
	for ver, parts := range want {
		s, _, err := Tables.SQL(meta(t, ver), map[string]any{"schema": "public"})
		if err != nil {
			t.Fatalf("%s: expected no error, got: %v", ver, err)
		}
		for _, p := range parts {
			if !strings.Contains(s, p) {
				t.Errorf("%s: expected %q in:\n%s", ver, p, s)
			}
		}
		if n := strings.Count(s, ` AS "`); n != 4 {
			t.Errorf("%s: expected 4 named columns, got %d:\n%s", ver, n, s)
		}
	}
}

// TestFieldMinDistinguishesAbsentFromNull covers the ambiguity the padding
// rule creates. A NULL means either that the value is null or that the server
// is too old to have the field, and only the declared minimum tells them
// apart.
func TestFieldMinDistinguishesAbsentFromNull(t *testing.T) {
	t.Parallel()
	fields, err := Tables.Fields(meta(t, "11"))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	var found bool
	for _, f := range fields {
		if f.Name != "access_privileges" {
			continue
		}
		found = true
		if ParseVersion("11").AtLeast(f.Min) {
			t.Error("expected access_privileges to be absent at release 11")
		}
		if !ParseVersion("18").AtLeast(f.Min) {
			t.Error("expected access_privileges to be present at release 18")
		}
	}
	if !found {
		t.Fatal("expected an access_privileges field")
	}
}

func TestSupportStates(t *testing.T) {
	t.Parallel()
	m := meta(t, "18")
	if got := Tables.Support(m); got != Supported {
		t.Errorf("expected supported, got %v", got)
	}
	// registered dialect, no binding for this query
	if got := Schemas.Support(m); got != NotSupported {
		t.Errorf("expected not supported, got %v", got)
	}
	if _, err := Schemas.Fields(m); !errors.Is(err, ErrNotSupported) {
		t.Errorf("expected ErrNotSupported, got: %v", err)
	}
	// a dialect no model was built for is a different state entirely
	if _, err := New("nosuchdb", VersionSet{}); !errors.Is(err, ErrModelNotBuilt) {
		t.Errorf("expected ErrModelNotBuilt, got: %v", err)
	}
	if got := Tables.Support(nil); got != NotBuilt {
		t.Errorf("expected not built, got %v", got)
	}
}

// TestParamsAreCheckedNotIgnored is the criticism both reviews made of the
// earlier sketch. A typo must not silently drop a filter.
func TestParamsAreCheckedNotIgnored(t *testing.T) {
	t.Parallel()
	m := meta(t, "18")
	if _, _, err := Tables.SQL(m, map[string]any{"schma": "public"}); !errors.Is(err, ErrUnknownParam) {
		t.Errorf("expected ErrUnknownParam for a typo, got: %v", err)
	}
	if _, _, err := Tables.SQL(m, nil); !errors.Is(err, ErrMissingParam) {
		t.Errorf("expected ErrMissingParam, got: %v", err)
	}
}

func TestPlaceholdersAndArgs(t *testing.T) {
	t.Parallel()
	s, args, err := Tables.SQL(meta(t, "18"), map[string]any{"schema": "public"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(s, "= $1") {
		t.Errorf("expected the dialect placeholder, got:\n%s", s)
	}
	if strings.Contains(s, "@schema") {
		t.Error("expected the named parameter to be rewritten")
	}
	if len(args) != 1 || args[0] != "public" {
		t.Errorf("expected one argument, got %v", args)
	}
}

func TestVersionTooOld(t *testing.T) {
	t.Parallel()
	st := Stmt{{{Min: V(12), SQL: "SELECT 1"}}}
	if _, err := st.SQL(meta(t, "11").versions); !errors.Is(err, ErrVersionTooOld) {
		t.Errorf("expected ErrVersionTooOld, got: %v", err)
	}
}

func TestVersionQueryIsDataNotExecuted(t *testing.T) {
	t.Parallel()
	sql, cols, ok := testDialect.VersionQuery()
	if !ok {
		t.Fatal("expected a version query")
	}
	if sql != "SHOW server_version" || cols != 1 {
		t.Errorf("unexpected version query %q with %d columns", sql, cols)
	}
	// the caller runs it and hands the columns back
	s, err := testDialect.ParseVersion([]string{"18.6"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if got := s.Main().String(); got != "18.6" {
		t.Errorf("expected 18.6, got %s", got)
	}
	if got := s.String(); got != "TestDB 18.6" {
		t.Errorf("expected the display line, got %s", got)
	}
}

func TestFirstDistinguishesEmptyFromZero(t *testing.T) {
	t.Parallel()
	empty := func(yield func(Table, error) bool) {}
	if _, ok, err := First(empty); ok || err != nil {
		t.Errorf("expected not found and no error, got ok=%v err=%v", ok, err)
	}
	one := func(yield func(Table, error) bool) { yield(Table{Name: "t"}, nil) }
	v, ok, err := First(one)
	if !ok || err != nil || v.Name != "t" {
		t.Errorf("expected the first value, got %v ok=%v err=%v", v, ok, err)
	}
}

func TestArgsMapOmitsUnset(t *testing.T) {
	t.Parallel()
	m := Args{Schema: "public"}.Map()
	if len(m) != 1 || m["schema"] != "public" {
		t.Errorf("expected only schema, got %v", m)
	}
}

func TestAlwaysAndAccessors(t *testing.T) {
	t.Parallel()
	m := meta(t, "18")
	if s, err := Always("SELECT 1").SQL(m.versions); err != nil || s != "SELECT 1" {
		t.Errorf("expected SELECT 1, got %q err %v", s, err)
	}
	if Tables.Name() != "tables" {
		t.Errorf("expected tables, got %s", Tables.Name())
	}
	if m.Dialect() != testDialect {
		t.Errorf("expected %s, got %s", testDialect, m.Dialect())
	}
	if m.Version().Main().String() != "18" {
		t.Errorf("expected 18, got %s", m.Version().Main())
	}
	params, err := Tables.Params(m)
	if err != nil || len(params) != 1 || params[0].Name != "schema" {
		t.Errorf("expected one schema param, got %v err %v", params, err)
	}
	var found bool
	for _, d := range Dialects() {
		found = found || d == testDialect
	}
	if !found {
		t.Error("expected the test dialect to be listed")
	}
	if ErrNotSupported.Error() != "not supported" {
		t.Errorf("unexpected error text %q", ErrNotSupported.Error())
	}
	for s, exp := range map[Support]string{NotBuilt: "not built", NotSupported: "not supported", Supported: "supported"} {
		if s.String() != exp {
			t.Errorf("expected %q, got %q", exp, s)
		}
	}
	if !(Version{}).IsZero() || ParseVersion("1").IsZero() {
		t.Error("unexpected IsZero result")
	}
}

// quietDialect is a model whose database reports no version. Registration
// happens once, in init, because a registry rejects a second registration and
// a test body can run more than once.
const quietDialect Dialect = "quietdb"

func TestDialectVersion(t *testing.T) {
	t.Parallel()
	// a dialect no model was built for
	if _, err := Dialect("nosuchdb").Version(context.Background(), nil); !errors.Is(err, ErrModelNotBuilt) {
		t.Errorf("expected ErrModelNotBuilt, got: %v", err)
	}
	// a model that reports no version at all treats the server as newest
	versions, err := quietDialect.Version(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !versions.Main().Unknown {
		t.Error("expected an unknown version")
	}
}

// TestRepeatedParamBindsTwice guards a fault that only a placeholder style
// like MySQL's can show. A parameter named twice in one statement must get two
// placeholders and two values, because every ? consumes an argument.
// PostgreSQL hides this, since $2 may appear twice.
// declared at package level, not in the test body: NewQuery and Register both
// reject a duplicate, and a test body runs again under -count=2.
const seqDialect Dialect = "seqdb"

var repeatedQuery = NewQuery[Table]("repeated")

func TestRepeatedParamBindsTwice(t *testing.T) {
	t.Parallel()
	q := repeatedQuery
	m := &Meta{dialect: seqDialect}
	s, args, err := q.SQL(m, map[string]any{"name": "x"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if n := strings.Count(s, "?"); n != 2 {
		t.Errorf("expected two placeholders, got %d in:\n%s", n, s)
	}
	if len(args) != 2 || args[0] != "x" || args[1] != "x" {
		t.Errorf("expected the value twice, got %v", args)
	}
}
