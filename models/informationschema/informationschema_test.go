package informationschema_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
	is "github.com/xo/dbmeta/models/informationschema"
)

// Two profiles, registered once. The first is a database close to the
// standard. The second is MySQL, which is the awkward one: no sequences, no
// check constraints, no deferred constraints, and a different column for the
// declared type.
const (
	plain  dbmeta.Dialect = "plaindb"
	mylike dbmeta.Dialect = "mylikedb"
)

func init() {
	is.Register(plain, is.Profile{
		Placeholder: func(int) string { return "?" },
	})
	is.Register(mylike, is.Profile{
		Placeholder: func(int) string { return "?" },
		Has:         is.Standard().Without(is.Sequences, is.CheckConstraints),
		Clauses: map[is.Clause]string{
			is.ColumnDataType:       "c.column_type",
			is.ConstraintDeferrable: "''",
			is.ConstraintDeferred:   "''",
			is.PrivilegeGrantor:     "''",
		},
		SystemSchemas: []string{"mysql", "information_schema", "performance_schema", "sys"},
	})
}

func meta(t *testing.T, d dbmeta.Dialect) *dbmeta.Meta {
	t.Helper()
	m, err := dbmeta.New(d, dbmeta.VersionSet{})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	return m
}

// TestFeaturesDecideRegistration is the point of the profile. A database that
// lacks a feature must report it as unsupported, not answer with an empty
// result. D34 requires that difference.
func TestFeaturesDecideRegistration(t *testing.T) {
	t.Parallel()
	std, my := meta(t, plain), meta(t, mylike)
	if got := dbmeta.Sequences.Support(std); got != dbmeta.Supported {
		t.Errorf("expected a standard database to answer for sequences, got %v", got)
	}
	if got := dbmeta.Sequences.Support(my); got != dbmeta.NotSupported {
		t.Errorf("expected a database without sequences to say so, got %v", got)
	}
	if _, _, err := dbmeta.Sequences.SQL(my, nil); !errors.Is(err, dbmeta.ErrNotSupported) {
		t.Errorf("expected ErrNotSupported, got: %v", err)
	}
	// an object information_schema knows nothing about is unsupported for both
	for _, m := range []*dbmeta.Meta{std, my} {
		if got := dbmeta.Extensions.Support(m); got != dbmeta.NotSupported {
			t.Errorf("expected extensions to be unsupported, got %v", got)
		}
	}
}

// TestClauseOverride checks that a database gets its own spelling and the
// standard one otherwise.
func TestClauseOverride(t *testing.T) {
	t.Parallel()
	stdSQL, _, err := dbmeta.Columns.SQL(meta(t, plain), nil)
	if err != nil {
		t.Fatal(err)
	}
	mySQL, _, err := dbmeta.Columns.SQL(meta(t, mylike), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdSQL, `c.data_type AS "data_type"`) {
		t.Errorf("expected the standard spelling:\n%s", stdSQL)
	}
	if !strings.Contains(mySQL, `c.column_type AS "data_type"`) {
		t.Errorf("expected the overridden spelling:\n%s", mySQL)
	}
}

// TestColumnSetIsTheSameAcrossProfiles is the rule that makes one Go type read
// every database. Two profiles must not produce different columns.
func TestColumnSetIsTheSameAcrossProfiles(t *testing.T) {
	t.Parallel()
	std, my := meta(t, plain), meta(t, mylike)
	for _, q := range dbmeta.Queries() {
		if q.Support(std) != dbmeta.Supported || q.Support(my) != dbmeta.Supported {
			continue
		}
		a, _, err := q.SQL(std, nil)
		if err != nil {
			t.Fatalf("%s: %v", q.Name(), err)
		}
		b, _, err := q.SQL(my, nil)
		if err != nil {
			t.Fatalf("%s: %v", q.Name(), err)
		}
		if x, y := named(a), named(b); strings.Join(x, ",") != strings.Join(y, ",") {
			t.Errorf("%s: profiles disagree on the column set\n std %v\n my  %v", q.Name(), x, y)
		}
	}
}

// TestFieldsMatchTheStatement catches a binding whose declared fields drift
// from the columns it selects.
func TestFieldsMatchTheStatement(t *testing.T) {
	t.Parallel()
	m := meta(t, plain)
	for _, q := range dbmeta.Queries() {
		if q.Support(m) != dbmeta.Supported {
			continue
		}
		fields, err := q.Fields(m)
		if err != nil {
			t.Fatal(err)
		}
		names := make([]string, len(fields))
		for i, f := range fields {
			names[i] = f.Name
		}
		s, _, err := q.SQL(m, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := named(s); strings.Join(names, ",") != strings.Join(got, ",") {
			t.Errorf("%s: declared fields and selected columns differ\n fields  %v\n columns %v",
				q.Name(), names, got)
		}
	}
}

// TestSystemSchemasAreQuoted checks that a schema name reaches the SQL as a
// quoted literal and cannot end the string early.
func TestSystemSchemasAreQuoted(t *testing.T) {
	t.Parallel()
	const odd dbmeta.Dialect = "odddb"
	is.Register(odd, is.Profile{
		Placeholder:   func(int) string { return "?" },
		SystemSchemas: []string{"it's"},
	})
	s, _, err := dbmeta.Tables.SQL(meta(t, odd), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, `'it''s'`) {
		t.Errorf("expected the quote to be doubled:\n%s", s)
	}
}

func TestCoverage(t *testing.T) {
	t.Parallel()
	m := meta(t, plain)
	var named []string
	for _, q := range dbmeta.Queries() {
		if q.Support(m) == dbmeta.Supported {
			named = append(named, q.Name())
		}
	}
	t.Logf("information_schema answers %d of %d: %s",
		len(named), len(dbmeta.Queries()), strings.Join(named, " "))
}

func named(s string) []string {
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
