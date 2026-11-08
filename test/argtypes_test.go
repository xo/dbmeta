package test

import (
	"database/sql"
	"testing"

	"github.com/xo/dbmeta"
)

// TestFunctionsMatchTheTypesOfTheirArguments checks Args.ArgTypes the way
// psql's \df takes its type patterns. Each case was run in psql 18 first. See
// D272.
func TestFunctionsMatchTheTypesOfTheirArguments(t *testing.T) {
	eachPostgres(t, func(t *testing.T, db *sql.DB, m *dbmeta.Meta) {
		count := func(name string, types ...string) int {
			t.Helper()
			args := dbmeta.Args{Name: name, WithSystem: true, ArgTypes: types}.Map()
			return len(readAll(t, dbmeta.Functions, m, db, args))
		}
		if count("abs") == 0 {
			t.Skipf("%s has no function abs to match against", m.Dialect())
		}
		// CockroachDB spells its 64 bit integer int, and psql reads int as
		// integer, so the same patterns name bigint there.
		i, wide := "int", "integer"
		if m.Dialect() == dbmeta.CockroachDB {
			i, wide = "bigint", "bigint"
		}
		for _, tt := range []struct {
			name  string
			types []string
			// min is the fewest rows and max the most that psql shows.
			min, max int
		}{
			{"abs", nil, 3, 100},
			{"abs", []string{i}, 1, 1},
			{"abs", []string{wide}, 1, 1},
			{"abs", []string{i, "-"}, 1, 1},
			{"abs", []string{i, i}, 0, 0},
			{"abs", []string{"-"}, 0, 0},
			{"abs", []string{"-", i}, 0, 0},
			{"abs", []string{"numeric", "-"}, 1, 1},
			{"abs", []string{"decimal"}, 1, 1},
			{"abs", []string{"pg_catalog.%int%"}, 1, 3},
			{"abs", []string{"nosuchtype"}, 0, 0},
			{"abs", []string{"text[]"}, 0, 0},
		} {
			got := count(tt.name, tt.types...)
			if got < tt.min || got > tt.max {
				t.Errorf("%s with %q gave %d rows, want %d to %d", tt.name, tt.types, got, tt.min, tt.max)
			}
		}
	})
}

// TestOperatorsMatchTheirOperands checks that the first pattern is the left
// operand and the second the right one, and that a prefix operator has no left
// operand. psql reads one pattern as the right operand, so usql passes - first.
func TestOperatorsMatchTheirOperands(t *testing.T) {
	eachPostgres(t, func(t *testing.T, db *sql.DB, m *dbmeta.Meta) {
		read := func(name string, types ...string) []dbmeta.Operator {
			t.Helper()
			args := dbmeta.Args{Name: name, WithSystem: true, ArgTypes: types}.Map()
			return readAll(t, dbmeta.Operators, m, db, args)
		}
		if len(read("+")) == 0 {
			t.Skipf("%s lists no operators to match against", m.Dialect())
		}
		i, wide := "int", "integer"
		if m.Dialect() == dbmeta.CockroachDB {
			i, wide = "bigint", "bigint"
		}
		for _, v := range read("+", i, i) {
			if v.LeftType != wide || v.RightType != wide {
				t.Errorf("+ with %s twice gave %q and %q", i, v.LeftType, v.RightType)
			}
		}
		if got := len(read("+", i, i)); got != 1 {
			t.Errorf("+ with %s twice gave %d rows, want 1", i, got)
		}
		prefix := read("-", "-", i)
		if len(prefix) != 1 || prefix[0].LeftType != "" || prefix[0].RightType != wide {
			t.Errorf("- with - and %s gave %+v, want the prefix operator on %s", i, prefix, wide)
		}
		if got := len(read("+", i, "-")); got != 0 {
			t.Errorf("+ with %s and - gave %d rows, want none", i, got)
		}
	})
}

// TestTypesAreRefusedWhereTheyCannotBeMatched checks that a model that does
// not take ArgTypes answers ErrNotSupported and not every row.
func TestTypesAreRefusedWhereTheyCannotBeMatched(t *testing.T) {
	m, err := dbmeta.New(dbmeta.Oracle, dbmeta.VersionSet{})
	if err != nil {
		t.Skipf("building the Oracle meta: %v", err)
	}
	args := dbmeta.Args{ArgTypes: []string{"number"}}
	if _, _, err := dbmeta.First(dbmeta.Operators.Each(t.Context(), m, nil, args)); err == nil {
		t.Error("Oracle took ArgTypes, and it must refuse them")
	}
}
