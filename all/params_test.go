package all_test

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
)

// childKinds are the kinds whose objects belong to another object: a column,
// an index, a constraint and a trigger to a table, and a parameter to a
// routine.
var childKinds = []dbmeta.AnyQuery{
	dbmeta.Columns, dbmeta.ColumnStats, dbmeta.Constraints, dbmeta.ConstraintColumns,
	dbmeta.Indexes, dbmeta.IndexColumns, dbmeta.Triggers, dbmeta.RoutineParameters,
}

// TestEveryChildKindTakesParent checks that every model filters a child kind
// the same way: parent for the object it belongs to, and name for the object
// itself.
//
// usql found Oracle taking the table as name, so a caller asking for the
// columns of one table had to know which model it was talking to. ClickHouse,
// Cassandra, Couchbase and Vertica did the same, and nothing had said so. A
// query that answers must take both, because [dbmeta.Args] promises the
// meaning of each.
func TestEveryChildKindTakesParent(t *testing.T) {
	t.Parallel()
	for _, d := range dbmeta.Dialects() {
		m, err := dbmeta.New(d, dbmeta.VersionSet{})
		if err != nil {
			t.Fatalf("%s: building the meta: %v", d, err)
		}
		for _, q := range childKinds {
			if q.Support(m) != dbmeta.Supported {
				continue
			}
			params, err := q.Params(m)
			if err != nil {
				t.Errorf("%s %s: reading the parameters: %v", d, q.Name(), err)
				continue
			}
			byName := map[string]string{}
			for _, p := range params {
				byName[p.Name] = p.Desc
			}
			for _, want := range []string{"parent", "name"} {
				if _, ok := byName[want]; !ok {
					t.Errorf("%s %s: takes no %s, and takes %v", d, q.Name(), want, slices.Sorted(maps.Keys(byName)))
				}
			}
			// A routine parameter belongs to a routine, and the description
			// says so.
			if q.Name() == dbmeta.RoutineParameters.Name() && strings.HasPrefix(byName["parent"], "table") {
				t.Errorf("%s %s: parent is described as a table: %q", d, q.Name(), byName["parent"])
			}
		}
	}
}

// TestEveryTablesTakesTypes checks that every model's Tables takes types,
// which [dbmeta.Args] promises. No model took it, and a caller that passed it
// got ErrUnknownParam from every one. See D138.
func TestEveryTablesTakesTypes(t *testing.T) {
	t.Parallel()
	for _, d := range dbmeta.Dialects() {
		m, err := dbmeta.New(d, dbmeta.VersionSet{})
		if err != nil {
			t.Fatalf("%s: building the meta: %v", d, err)
		}
		if dbmeta.Tables.Support(m) != dbmeta.Supported {
			continue
		}
		params, err := dbmeta.Tables.Params(m)
		if err != nil {
			t.Errorf("%s: reading the parameters of Tables: %v", d, err)
			continue
		}
		if !slices.ContainsFunc(params, func(p dbmeta.Param) bool { return p.Name == "types" }) {
			t.Errorf("%s: Tables takes no types", d)
		}
	}
}
