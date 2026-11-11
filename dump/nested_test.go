package dump_test

import (
	"testing"

	"github.com/xo/dbmeta/dump"
)

func TestAMapOrAListBecomesJSONText(t *testing.T) {
	t.Parallel()
	for _, kind := range []dump.Kind{dump.JSON, dump.String, dump.Other} {
		got, err := dump.Convert(map[string]any{"h": 1, "w": 2}, dump.Type{Kind: dump.Other}, dump.Type{Kind: kind})
		if err != nil || got != `{"h":1,"w":2}` {
			t.Errorf("a map for %s is %v, %v", kind, got, err)
		}
	}
	got, err := dump.Convert([]any{"a", 1}, dump.Type{Kind: dump.Other}, dump.Type{Kind: dump.Array})
	if err != nil || got != `["a",1]` {
		t.Errorf("a list is %v, %v", got, err)
	}
}

func TestABooleanOrANumberInATextColumnBecomesText(t *testing.T) {
	t.Parallel()
	for v, want := range map[any]string{true: "true", int64(7): "7", 1.5: "1.5"} {
		got, err := dump.Convert(v, dump.Type{Kind: dump.String}, dump.Type{Kind: dump.String})
		if err != nil || got != want {
			t.Errorf("%v in a text column is %v, %v, and want %q", v, got, err, want)
		}
	}
}
