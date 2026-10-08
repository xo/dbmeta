package dbmeta

import (
	"errors"
	"strings"
	"testing"
)

// TestDerivedValue checks that a statement reads a value that a function
// computes from the other arguments, through the literal of the dialect, and
// that a caller cannot pass it. See D203.
func TestDerivedValue(t *testing.T) {
	t.Parallel()
	info := &Info{Literal: func(v any) (string, error) {
		s, _ := v.(string)
		return "<" + s + ">", nil
	}}
	params := []Param{{Name: "schema", Default: ""}}
	derived := []Derived{{Name: "scope", From: func(args map[string]any) any {
		schema, _ := args["schema"].(string)
		if schema == "" {
			return "all"
		}
		return "one:" + schema
	}}}
	const stmt = `SHOW KEYS IN @scope WHERE s = @schema`
	for name, c := range map[string]struct {
		args map[string]any
		want string
	}{
		"given":   {map[string]any{"schema": "S"}, `SHOW KEYS IN <one:S> WHERE s = <S>`},
		"default": {nil, `SHOW KEYS IN <all> WHERE s = <>`},
	} {
		got, vals, err := bind(stmt, info, params, derived, c.args)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if got != c.want || len(vals) != 0 {
			t.Errorf("%s: got %s with %v, want %s", name, got, vals, c.want)
		}
	}
	if _, _, err := bind(stmt, info, params, derived, map[string]any{"scope": "x"}); !errors.Is(err, ErrUnknownParam) {
		t.Errorf("expected ErrUnknownParam for a derived name, got %v", err)
	}
	if strings.Contains(stmt, "<") {
		t.Error("the statement must not change")
	}
}
