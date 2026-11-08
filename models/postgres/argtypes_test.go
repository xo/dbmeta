package postgres

import "testing"

// TestArgTypeItem checks how a pattern becomes the three parts that the
// statement compares, against what psql 18 sent for the same pattern.
func TestArgTypeItem(t *testing.T) {
	t.Parallel()
	const f = fieldSeparator
	for _, tt := range []struct{ pattern, want string }{
		{"-", "-"},
		{"int", f + "int" + f + "integer"},
		{"float", f + "float" + f + "double precision"},
		{"decimal", f + "decimal" + f + "numeric"},
		{"bool", f + "bool" + f + "boolean"},
		{"integer", f + "integer" + f + "integer"},
		{"text", f + "text" + f + "text"},
		{"int[]", f + "int[]" + f + "integer[]"},
		{"int2[]", f + "int2[]" + f + "smallint[]"},
		{"text[]", f + "text[]" + f + "text[]"},
		{"int[][]", f + "int[][]" + f + "int[][]"},
		{"int%", f + "int%" + f + "int%"},
		{"pg_catalog.int", "pg_catalog" + f + "int" + f + "int"},
		{"Int", f + "Int" + f + "Int"},
	} {
		if got := argTypeItem(tt.pattern); got != tt.want {
			t.Errorf("argTypeItem(%q) = %q, want %q", tt.pattern, got, tt.want)
		}
	}
}

// TestArgTypeItemsJoinsTheArguments checks that the derived parameter is empty
// when no type is named and holds one item for each argument otherwise.
func TestArgTypeItemsJoinsTheArguments(t *testing.T) {
	t.Parallel()
	if got := argTypeItems(map[string]any{}); got != "" {
		t.Errorf("no types gave %q, want an empty string", got)
	}
	got := argTypeItems(map[string]any{"arg_types": []string{"int", "-"}})
	want := fieldSeparator + "int" + fieldSeparator + "integer" + itemSeparator + "-"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
