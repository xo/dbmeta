package avatica

import (
	"errors"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

func TestTheTypesOfAvaticaAreRead(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]dump.Kind{
		"INTEGER":                            dump.Integer,
		"CHARACTER VARYING(20)":              dump.String,
		"DECIMAL(10,2)":                      dump.Decimal,
		"CHARACTER LARGE OBJECT(1073741824)": dump.String,
		"TIMESTAMP WITH TIME ZONE":           dump.Timestamp,
		"VARBINARY(10)":                      dump.Binary,
		"ARRAY":                              dump.Other,
	} {
		got, err := dump.ParseType(dbmeta.Avatica, text)
		if err != nil || got.Kind != want {
			t.Errorf("ParseType(%q) is %v, %v, and want %s", text, got.Kind, err, want)
		}
	}
}

func TestTheTypesOfAvaticaAreWritten(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		in    dump.Type
		sql   string
		level dump.Level
	}{
		{dump.Type{Kind: dump.String, Length: 20}, "VARCHAR(20)", dump.Exact},
		{dump.Type{Kind: dump.String}, "LONGVARCHAR", dump.Exact},
		{dump.Type{Kind: dump.Timestamp}, "TIMESTAMP(6)", dump.Lossy},
		{dump.Type{Kind: dump.Integer, Bits: 32, Unsigned: true}, "BIGINT", dump.Exact},
	} {
		got, err := dump.RenderType(dbmeta.Avatica, tt.in)
		if err != nil || got.SQL != tt.sql || got.Level != tt.level {
			t.Errorf("RenderType(%+v) is %q %v, %v, and want %q %v", tt.in, got.SQL, got.Level, err, tt.sql, tt.level)
		}
	}
	for _, in := range []dump.Type{{Kind: dump.Interval}, {Kind: dump.JSON}, {Kind: dump.Decimal}} {
		if _, err := dump.RenderType(dbmeta.Avatica, in); !errors.Is(err, dump.ErrNoEquivalent) {
			t.Errorf("RenderType(%s) is %v, and want no equivalent", in.Kind, err)
		}
	}
}
