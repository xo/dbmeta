package hana

import (
	"errors"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

func TestTheTypesOfHANAAreRead(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]dump.Kind{
		"TINYINT":       dump.Integer,
		"DECIMAL(10,2)": dump.Decimal,
		"NVARCHAR(20)":  dump.String,
		"NCLOB":         dump.String,
		"VARBINARY(10)": dump.Binary,
		"TIMESTAMP":     dump.LocalTimestamp,
		"ST_GEOMETRY":   dump.Other,
	} {
		got, err := dump.ParseType(dbmeta.HANA, text)
		if err != nil || got.Kind != want {
			t.Errorf("ParseType(%q) is %v, %v, and want %s", text, got.Kind, err, want)
		}
	}
}

func TestTheTypesOfHANAAreWritten(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		in    dump.Type
		sql   string
		level dump.Level
	}{
		{dump.Type{Kind: dump.Integer, Bits: 8, Unsigned: true}, "TINYINT", dump.Exact},
		{dump.Type{Kind: dump.String, Length: 20}, "NVARCHAR(20)", dump.Exact},
		{dump.Type{Kind: dump.String}, "NCLOB", dump.Exact},
		{dump.Type{Kind: dump.Timestamp}, "TIMESTAMP", dump.Lossy},
		{dump.Type{Kind: dump.Integer, Bits: 64, Unsigned: true}, "DECIMAL(20, 0)", dump.Exact},
	} {
		got, err := dump.RenderType(dbmeta.HANA, tt.in)
		if err != nil || got.SQL != tt.sql || got.Level != tt.level {
			t.Errorf("RenderType(%+v) is %q %v, %v, and want %q %v", tt.in, got.SQL, got.Level, err, tt.sql, tt.level)
		}
	}
	for _, in := range []dump.Type{{Kind: dump.Interval}, {Kind: dump.JSON}, {Kind: dump.Decimal}} {
		if _, err := dump.RenderType(dbmeta.HANA, in); !errors.Is(err, dump.ErrNoEquivalent) {
			t.Errorf("RenderType(%s) is %v, and want no equivalent", in.Kind, err)
		}
	}
}
