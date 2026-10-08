package dbmeta

import (
	"maps"
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
