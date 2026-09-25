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

// A dialect two products share. It is the MariaDB and MySQL case reduced to
// what the rules need: two keys, neither of them the main one.
const twinDialect Dialect = "twindb"

var (
	twinQuery      = NewQuery[Table]("twin")
	twinOnlyQuery  = NewQuery[Table]("twin_only")
	twinMixedQuery = NewQuery[Table]("twin_mixed")
)

func init() {
	RegisterDialect(twinDialect, &Info{Placeholder: func(int) string { return "?" }})
	twinQuery.Register(twinDialect, &Binding[Table]{
		Stmt: Stmt{
			{
				{SQL: `SELECT 'neither' AS "name"`},
				{Key: "alpha", Min: V(10, 2), SQL: `SELECT 'alpha' AS "name"`},
				{Key: "beta", Min: V(8, 0, 16), SQL: `SELECT 'beta' AS "name"`},
			},
		},
		Fields: []Field{{
			Name: "name",
			Min:  V(10, 2), Key: "alpha",
			Also: []Gate{{Key: "beta", Min: V(8, 0, 16)}},
		}},
	})
	// every alternative names a key, so a server reporting neither is the
	// wrong product rather than an old one
	twinOnlyQuery.Register(twinDialect, &Binding[Table]{
		Stmt: Stmt{{{Key: "alpha", Min: V(11, 5), SQL: `SELECT 1`}}},
	})
	// a model fault: two keys, both met, nothing to choose between them
	twinMixedQuery.Register(twinDialect, &Binding[Table]{
		Stmt: Stmt{{
			{Key: "alpha", SQL: `SELECT 'a'`},
			{Key: "beta", SQL: `SELECT 'b'`},
		}},
	})
}

func twin(t *testing.T, pairs ...string) *Meta {
	t.Helper()
	var s VersionSet
	for i := 0; i < len(pairs); i += 2 {
		s.Set(pairs[i], ParseVersion(pairs[i+1]))
	}
	m, err := New(twinDialect, s)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	return m
}

// TestKeyedFragmentNeedsTheKey is the rule D44 rests on. A named key is a fact
// about the server, so a server that does not report it does not meet the
// gate, however new its numbers are. Before this, MySQL 9 satisfied a gate
// written for MariaDB 10.2, because 9 is unknown under that key and unknown
// sorts above everything.
func TestKeyedFragmentNeedsTheKey(t *testing.T) {
	t.Parallel()
	want := map[*Meta]string{
		twin(t, "alpha", "11.8"): "alpha",
		twin(t, "beta", "9.7"):   "beta",
		// reports alpha, but too old for the alpha fragment
		twin(t, "alpha", "10.1"): "neither",
		// reports neither key
		twin(t, "", "1.0"): "neither",
	}
	for m, expect := range want {
		s, _, err := twinQuery.SQL(m, nil)
		if err != nil {
			t.Fatalf("%s: expected no error, got: %v", m, err)
		}
		if !strings.Contains(s, "'"+expect+"'") {
			t.Errorf("%s: expected the %s alternative, got:\n%s", m, expect, s)
		}
	}
}

// TestKeyedFieldKnowsBothProducts checks that a field arriving in a different
// release of each product reports its presence for both.
func TestKeyedFieldKnowsBothProducts(t *testing.T) {
	t.Parallel()
	want := map[*Meta]bool{
		twin(t, "alpha", "11.8"):   true,
		twin(t, "alpha", "10.1"):   false,
		twin(t, "beta", "9.7"):     true,
		twin(t, "beta", "8.0.1"):   false,
		twin(t, "beta", "8.0.16"):  true,
		twin(t, "gamma", "999.99"): false,
	}
	for m, expect := range want {
		fields, err := twinQuery.Fields(m)
		if err != nil {
			t.Fatalf("%s: expected no error, got: %v", m, err)
		}
		if got := fields[0].Present(m.Version()); got != expect {
			t.Errorf("%s: expected present=%v, got %v", m, expect, got)
		}
	}
}

// TestWrongProductIsNotSupported separates the two reasons a statement does
// not resolve. A server too old for a piece can be upgraded. A server of the
// other product cannot, so it gets ErrNotSupported and reports the query as
// unsupported rather than answering with a failure.
func TestWrongProductIsNotSupported(t *testing.T) {
	t.Parallel()
	other := twin(t, "beta", "9.7")
	if got := twinOnlyQuery.Support(other); got != NotSupported {
		t.Errorf("expected not supported, got %v", got)
	}
	if _, _, err := twinOnlyQuery.SQL(other, nil); !errors.Is(err, ErrNotSupported) {
		t.Errorf("expected ErrNotSupported, got: %v", err)
	}
	// the same query on the right product, at too old a release, is a
	// different answer again. It used to report Supported and leave the
	// release to the error, which D54 recorded as an open question and
	// answered: a caller that trusts Support walked into a query it could
	// not build.
	old := twin(t, "alpha", "10.6")
	if got := twinOnlyQuery.Support(old); got != TooOld {
		t.Errorf("expected version too old, got %v", got)
	}
	if _, _, err := twinOnlyQuery.SQL(old, nil); !errors.Is(err, ErrVersionTooOld) {
		t.Errorf("expected ErrVersionTooOld, got: %v", err)
	}
}

// TestTwoKeysBothMetIsAFault checks that nothing guesses. Two alternatives for
// different products cannot both apply, and picking by the number would
// compare releases that mean different things.
func TestTwoKeysBothMetIsAFault(t *testing.T) {
	t.Parallel()
	both := twin(t, "alpha", "11.8", "beta", "9.7")
	if _, _, err := twinMixedQuery.SQL(both, nil); !errors.Is(err, ErrAmbiguousFragment) {
		t.Errorf("expected ErrAmbiguousFragment, got: %v", err)
	}
	// one key alone resolves
	if _, _, err := twinMixedQuery.SQL(twin(t, "alpha", "11.8"), nil); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

// TestKeyedBeatsUnkeyed checks the specificity rule. A fragment written for
// one product wins over one written for the family, whatever the numbers say.
func TestKeyedBeatsUnkeyed(t *testing.T) {
	t.Parallel()
	c := Choice{
		{Min: V(99), SQL: `family`},
		{Key: "alpha", SQL: `product`},
	}
	var s VersionSet
	s.Set("", ParseVersion("100"))
	s.Set("alpha", ParseVersion("1"))
	got, err := c.Resolve(s)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if got != "product" {
		t.Errorf("expected the keyed alternative, got %q", got)
	}
}

func TestVersionSetHasAndKeys(t *testing.T) {
	t.Parallel()
	var s VersionSet
	s.Set("", ParseVersion("11.8"))
	s.Set("mariadb", ParseVersion("11.8"))
	if !s.Has("mariadb") || s.Has("mysql") {
		t.Error("expected only the key that was set")
	}
	if got := strings.Join(s.Keys(), ","); got != ",mariadb" {
		t.Errorf("expected the keys sorted, got %q", got)
	}
	// Get cannot tell an absent key from an unreadable version, which is why
	// Has exists
	if !s.Get("mysql").Unknown {
		t.Error("expected an absent key to read as unknown")
	}
}
