package surrealdb

import (
	"testing"

	"github.com/xo/dbmeta/dump"
)

func TestTheTypesOfSurrealDBAreRead(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]dump.Kind{
		"int":            dump.Integer,
		"option<string>": dump.String,
		"array<int>":     dump.Array,
		"set<string, 5>": dump.Set,
		"datetime":       dump.Timestamp,
		"record<person>": dump.Other,
		"any":            dump.Other,
		"object":         dump.JSON,
	} {
		got, err := parseType(text)
		if err != nil || got.Kind != want {
			t.Errorf("parseType(%q) is %v, %v, and want %s", text, got.Kind, err, want)
		}
	}
	if got, _ := parseType("array<option<int>>"); got.Elem == nil || got.Elem.Kind != dump.Integer {
		t.Errorf("the element of array<option<int>> is %+v", got.Elem)
	}
}
