package influxql

import (
	"testing"

	"github.com/xo/dbmeta/dump"
)

func TestTheTypesOfInfluxQLAreRead(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]dump.Kind{
		"timestamp": dump.LocalTimestamp,
		"tag":       dump.String,
		"float":     dump.Float,
		"integer":   dump.Integer,
		"unsigned":  dump.Integer,
		"boolean":   dump.Boolean,
		"other":     dump.Other,
	} {
		got, err := parseType(text)
		if err != nil || got.Kind != want {
			t.Errorf("parseType(%q) is %v, %v, and want %s", text, got.Kind, err, want)
		}
	}
}
