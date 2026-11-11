package exasol

import (
	"errors"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

func TestTheTypesOfExasolAreRead(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]dump.Kind{
		"DECIMAL(18,0)":                  dump.Decimal,
		"VARCHAR(128) UTF8":              dump.String,
		"CHAR(5) UTF8":                   dump.String,
		"DOUBLE":                         dump.Float,
		"TIMESTAMP WITH LOCAL TIME ZONE": dump.Timestamp,
		"INTERVAL DAY(2) TO SECOND(3)":   dump.Other,
	} {
		got, err := dump.ParseType(dbmeta.Exasol, text)
		if err != nil || got.Kind != want {
			t.Errorf("ParseType(%q) is %v, %v, and want %s", text, got.Kind, err, want)
		}
	}
}

func TestTheTypesOfExasolAreWritten(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		in    dump.Type
		sql   string
		level dump.Level
	}{
		{dump.Type{Kind: dump.Integer, Bits: 32}, "DECIMAL(10, 0)", dump.Exact},
		{dump.Type{Kind: dump.String, Length: 20}, "VARCHAR(20) UTF8", dump.Exact},
		{dump.Type{Kind: dump.Timestamp}, "TIMESTAMP(6)", dump.Lossy},
		{dump.Type{Kind: dump.UUID}, "CHAR(36) UTF8", dump.Lossy},
	} {
		got, err := dump.RenderType(dbmeta.Exasol, tt.in)
		if err != nil || got.SQL != tt.sql || got.Level != tt.level {
			t.Errorf("RenderType(%+v) is %q %v, %v, and want %q %v", tt.in, got.SQL, got.Level, err, tt.sql, tt.level)
		}
	}
	for _, in := range []dump.Type{{Kind: dump.Binary}, {Kind: dump.JSON}, {Kind: dump.Decimal, Precision: 40}} {
		if _, err := dump.RenderType(dbmeta.Exasol, in); !errors.Is(err, dump.ErrNoEquivalent) {
			t.Errorf("RenderType(%s) is %v, and want no equivalent", in.Kind, err)
		}
	}
}
