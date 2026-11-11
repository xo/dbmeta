package vertica

import (
	"errors"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

func TestTheTypesOfVerticaAreRead(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]dump.Kind{
		"int":                   dump.Integer,
		"varchar(20)":           dump.String,
		"numeric(10,2)":         dump.Decimal,
		"long varchar(1048576)": dump.String,
		"timestamptz":           dump.Timestamp,
		"varbinary(10)":         dump.Binary,
		"array[int8](65000)":    dump.Array,
		"geometry":              dump.Other,
	} {
		got, err := dump.ParseType(dbmeta.Vertica, text)
		if err != nil || got.Kind != want {
			t.Errorf("ParseType(%q) is %v, %v, and want %s", text, got.Kind, err, want)
		}
	}
}

func TestTheTypesOfVerticaAreWritten(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		in    dump.Type
		sql   string
		level dump.Level
	}{
		{dump.Type{Kind: dump.Integer, Bits: 16}, "INT", dump.Exact},
		{dump.Type{Kind: dump.String, Length: 20}, "VARCHAR(80)", dump.Exact},
		{dump.Type{Kind: dump.String}, "LONG VARCHAR", dump.Exact},
		{dump.Type{Kind: dump.Timestamp}, "TIMESTAMPTZ", dump.Exact},
		{dump.Type{Kind: dump.Integer, Bits: 64, Unsigned: true}, "NUMERIC(20, 0)", dump.Exact},
	} {
		got, err := dump.RenderType(dbmeta.Vertica, tt.in)
		if err != nil || got.SQL != tt.sql || got.Level != tt.level {
			t.Errorf("RenderType(%+v) is %q %v, %v, and want %q %v", tt.in, got.SQL, got.Level, err, tt.sql, tt.level)
		}
	}
	for _, in := range []dump.Type{{Kind: dump.Interval}, {Kind: dump.JSON}, {Kind: dump.Decimal}} {
		if _, err := dump.RenderType(dbmeta.Vertica, in); !errors.Is(err, dump.ErrNoEquivalent) {
			t.Errorf("RenderType(%s) is %v, and want no equivalent", in.Kind, err)
		}
	}
}
