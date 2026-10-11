package dbmeta

import (
	"maps"
	"strings"
	"testing"
)

// TestArgsNamesTheParametersOfTheSections checks that the fields D201 added
// reach the map under the names the PostgreSQL statements declare. The
// postgres model tests that the statements take them.
func TestArgsNamesTheParametersOfTheSections(t *testing.T) {
	t.Parallel()
	args := Args{ParentSchema: "app", Parent: "base", PartitionSchema: "part", Name: "child", WithImplicit: true}
	want := map[string]any{
		"parent_schema":    "app",
		"parent":           "base",
		"partition_schema": "part",
		"name":             "child",
		"with_implicit":    true,
	}
	if got := args.Map(); !maps.Equal(got, want) {
		t.Errorf("Map: got %v, want %v", got, want)
	}
	if got := (Args{}).Map(); len(got) != 0 {
		t.Errorf("Map: got %v for no filter, want nothing", got)
	}
}

// TestArgsNamesTheTableInfoParameter checks that WithTableInfo reaches the map
// as the parameter that the Tables of Redshift declares (D232).
func TestArgsNamesTheTableInfoParameter(t *testing.T) {
	t.Parallel()
	want := map[string]any{"schema": "app", "with_table_info": true}
	if got := (Args{Schema: "app", WithTableInfo: true}).Map(); !maps.Equal(got, want) {
		t.Errorf("Map: got %v, want %v", got, want)
	}
	if got := (Args{Schema: "app"}).Map(); len(got) != 1 {
		t.Errorf("Map: got %v with WithTableInfo unset, want the schema alone", got)
	}
}

// TestAVariantReplacesTheStatementWhenItsParameterIsTrue checks that a
// binding builds its plain statement by default and its variant on request
// (D232).
func TestAVariantReplacesTheStatementWhenItsParameterIsTrue(t *testing.T) {
	t.Parallel()
	plain := Stmt{{{Query: "SELECT 1 FROM plain WHERE @flag = TRUE"}}}
	variant := Stmt{{{Query: "SELECT 1 FROM variant WHERE @flag = TRUE"}}}
	b := &Binding[int]{
		Stmt:     plain,
		Variants: []Variant{{Param: "flag", Stmt: variant}},
		Params:   []Param{{Name: "flag", Default: false}},
	}
	q := &Query[int]{name: "variant", bindings: map[Dialect]*Binding[int]{seqDialect: b}}
	m := &Meta{dialect: seqDialect}
	for args, want := range map[string]string{"off": "plain", "on": "variant"} {
		given := map[string]any{}
		if args == "on" {
			given["flag"] = true
		}
		s, _, err := q.Build(m, given)
		if err != nil {
			t.Fatalf("%s: building: %v", args, err)
		}
		if !strings.Contains(s, want) {
			t.Errorf("%s: got %q, want the %s statement", args, s, want)
		}
	}
}
