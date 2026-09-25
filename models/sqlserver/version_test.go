package sqlserver_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/sqlserver"
)

// Every version gate in this model sits below 2017, which is the oldest
// release that can be tested at all: Microsoft shipped SQL Server on Linux
// from 2017 and there is no container for anything earlier. So CI reaches the
// new branch of every gate and never the old one.
//
// That is the objection this file answers. A gate whose old branch no test
// reaches is a claim nobody checks, and D54 keeps the gates only because the
// resolution below the floor is checked here. This needs no server: a
// statement resolves against a version set, and the version set is a value.
//
// It does not claim that the query runs on 2014. Nothing can claim that. It
// claims that asking an old server produces the right answer or the right
// refusal, rather than a statement naming a catalog view that release has not
// got.
//
// It checks the statement rather than [dbmeta.Query.Support]. Support answers
// a question about the product and not about the release: a query the product
// has, on a server too old to answer it, is supported and fails with
// ErrVersionTooOld when it is asked. See TestWrongProductIsNotSupported.

// at builds the metadata for a SQL Server major release, with no connection.
func at(t *testing.T, major uint32) *dbmeta.Meta {
	t.Helper()
	var set dbmeta.VersionSet
	set.Set("", dbmeta.V(major))
	m, err := dbmeta.New(dbmeta.SQLServer, set)
	if err != nil {
		t.Fatalf("building the metadata for %d: %v", major, err)
	}
	return m
}

// The releases these numbers name, oldest first. SQL Server counts its majors
// two apart from 2012 on, except that there is no 14 between 2016 and 2017.
var releases = []struct {
	major uint32
	name  string
}{
	{10, "2008 R2"},
	{11, "2012"},
	{12, "2014"},
	{13, "2016"},
	{14, "2017"},
	{15, "2019"},
	{16, "2022"},
	{17, "2025"},
}

// TestGatedQueriesResolveBelowTheFloor checks the three queries that a whole
// statement gates. Each builds from the release that added the catalog view it
// reads, and refuses below it rather than naming a view that is not there.
func TestGatedQueriesResolveBelowTheFloor(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		// from is the first major that answers.
		from uint32
		// view is the catalog view the statement reads, which no older
		// release has.
		view string
		sql  func(*dbmeta.Meta, map[string]any) (string, []any, error)
	}{
		// sys.sequences arrived in 2012
		{"Sequences", 11, "sys.sequences", dbmeta.Sequences.SQL},
		// sys.dm_db_stats_properties arrived in 2012
		{"ColumnStats", 11, "sys.dm_db_stats_properties", dbmeta.ColumnStats.SQL},
		// sys.external_tables arrived in 2016
		{"ForeignTables", 13, "sys.external_tables", dbmeta.ForeignTables.SQL},
	} {
		for _, r := range releases {
			sql, _, err := c.sql(at(t, r.major), dbmeta.Args{}.Map())
			if r.major < c.from {
				if !errors.Is(err, dbmeta.ErrVersionTooOld) {
					t.Errorf("%s on SQL Server %s: expected ErrVersionTooOld, got %v and:\n%s",
						c.name, r.name, err, sql)
				}
				continue
			}
			if err != nil {
				t.Errorf("%s on SQL Server %s: %v", c.name, r.name, err)
				continue
			}
			if !strings.Contains(sql, c.view) {
				t.Errorf("%s on SQL Server %s: expected the query to read %s, got:\n%s",
					c.name, r.name, c.view, sql)
			}
		}
	}
}

// TestTablesPadsBelowTheFloor checks the one gate that pads rather than
// refuses. An older server has neither a system versioned table nor an
// external one, so it reports every table as a table, and the column set does
// not change. The padding rule in docs/NULLS.md is what this holds.
func TestTablesPadsBelowTheFloor(t *testing.T) {
	t.Parallel()
	var cols int
	for _, r := range releases {
		m := at(t, r.major)
		sql, _, err := dbmeta.Tables.SQL(m, dbmeta.Args{}.Map())
		if err != nil {
			t.Fatalf("resolving Tables on SQL Server %s: %v", r.name, err)
		}
		// temporal_type and is_external are columns of sys.tables that 2014
		// has not got. Naming either one on an older server is the fault this
		// catches, and it would be a syntax error rather than a wrong answer.
		if r.major < 13 {
			for _, bad := range []string{"temporal_type", "is_external"} {
				if strings.Contains(sql, bad) {
					t.Errorf("SQL Server %s has no sys.tables.%s, and the query names it:\n%s",
						r.name, bad, sql)
				}
			}
			if !strings.Contains(sql, `'table' AS "type"`) {
				t.Errorf("SQL Server %s: expected the padded type, got:\n%s", r.name, sql)
			}
		} else if !strings.Contains(sql, "temporal_type") {
			t.Errorf("SQL Server %s: expected the temporal branch, got:\n%s", r.name, sql)
		}

		// Whichever branch resolved, the column set is the same. This is the
		// rule a fragment must never break.
		fields, err := dbmeta.Tables.Fields(m)
		if err != nil {
			t.Fatalf("reading the fields on SQL Server %s: %v", r.name, err)
		}
		if cols == 0 {
			cols = len(fields)
		} else if len(fields) != cols {
			t.Errorf("SQL Server %s returns %d columns where the others return %d",
				r.name, len(fields), cols)
		}
	}
}

// TestOnlyTheGatedQueriesDependOnTheRelease checks that nothing else quietly
// depends on a release. The queries that build on 2008 R2 must be the ones
// that build on 2025, less exactly the three a gate names.
//
// It compares two sets rather than counting, so a query that silently needs a
// modern catalog view fails here and is named.
func TestOnlyTheGatedQueriesDependOnTheRelease(t *testing.T) {
	t.Parallel()
	builds := func(major uint32) map[string]bool {
		m := at(t, major)
		out := map[string]bool{}
		for _, q := range dbmeta.Queries() {
			if q.Support(m) != dbmeta.Supported {
				continue
			}
			if _, _, err := q.SQL(m, dbmeta.Args{}.Map()); err == nil {
				out[q.Name()] = true
			}
		}
		return out
	}
	newest, oldest := builds(17), builds(10)
	gated := map[string]bool{"sequences": true, "column_stats": true, "foreign_tables": true}

	for name := range newest {
		switch {
		case oldest[name] && gated[name]:
			t.Errorf("%s carries a gate and still builds on SQL Server 2008 R2", name)
		case !oldest[name] && !gated[name]:
			t.Errorf("%s does not build on SQL Server 2008 R2 and carries no gate that says why",
				name)
		}
	}
	for name := range oldest {
		if !newest[name] {
			t.Errorf("%s builds on SQL Server 2008 R2 and not on 2025", name)
		}
	}
	// 31 of the 54, which is the number docs/COVERAGE.md and the package
	// comment both quote. It is here so that changing it is deliberate.
	if want := 31; len(newest) != want {
		t.Errorf("expected %d queries on SQL Server 2025, got %d", want, len(newest))
	}
}
