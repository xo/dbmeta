package databend

import (
	"errors"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

func TestTheTypesOfDatabendAreRead(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]dump.Kind{
		"INT":              dump.Integer,
		"TINYINT UNSIGNED": dump.Integer,
		"DECIMAL(10, 2)":   dump.Decimal,
		"VARCHAR":          dump.String,
		"TIMESTAMP":        dump.Timestamp,
		"ARRAY(INT32)":     dump.Array,
		"VARIANT":          dump.JSON,
		"BITMAP":           dump.Other,
	} {
		got, err := dump.ParseType(dbmeta.Databend, text)
		if err != nil || got.Kind != want {
			t.Errorf("ParseType(%q) is %v, %v, and want %s", text, got.Kind, err, want)
		}
	}
}

func TestTheTypesOfDatabendAreWritten(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		in    dump.Type
		sql   string
		level dump.Level
	}{
		{dump.Type{Kind: dump.Integer, Bits: 8, Unsigned: true}, "TINYINT UNSIGNED", dump.Exact},
		{dump.Type{Kind: dump.Decimal, Precision: 10, Scale: 2}, "DECIMAL(10, 2)", dump.Exact},
		{dump.Type{Kind: dump.LocalTimestamp}, "TIMESTAMP", dump.Lossy},
		{dump.Type{Kind: dump.JSON}, "VARIANT", dump.Exact},
	} {
		got, err := dump.RenderType(dbmeta.Databend, tt.in)
		if err != nil || got.SQL != tt.sql || got.Level != tt.level {
			t.Errorf("RenderType(%+v) is %q %v, %v, and want %q %v", tt.in, got.SQL, got.Level, err, tt.sql, tt.level)
		}
	}
	for _, in := range []dump.Type{{Kind: dump.Interval}, {Kind: dump.UUID}, {Kind: dump.Decimal}} {
		if _, err := dump.RenderType(dbmeta.Databend, in); !errors.Is(err, dump.ErrNoEquivalent) {
			t.Errorf("RenderType(%s) is %v, and want no equivalent", in.Kind, err)
		}
	}
}
