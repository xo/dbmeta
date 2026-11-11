package spanner

import (
	"errors"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

func TestTheTypesOfSpannerAreRead(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]dump.Kind{
		"INT64":        dump.Integer,
		"STRING(MAX)":  dump.String,
		"BYTES(20)":    dump.Binary,
		"NUMERIC":      dump.Decimal,
		"TIMESTAMP":    dump.Timestamp,
		"ARRAY<INT64>": dump.Array,
		"PROTO<a.B>":   dump.Other,
	} {
		got, err := dump.ParseType(dbmeta.Spanner, text)
		if err != nil || got.Kind != want {
			t.Errorf("ParseType(%q) is %v, %v, and want %s", text, got.Kind, err, want)
		}
	}
}

func TestTheTypesOfSpannerAreWritten(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		in  dump.Type
		sql string
	}{
		{dump.Type{Kind: dump.Integer, Bits: 8}, "INT64"},
		{dump.Type{Kind: dump.Integer, Bits: 64, Unsigned: true}, "NUMERIC"},
		{dump.Type{Kind: dump.Decimal, Precision: 10, Scale: 2}, "NUMERIC"},
		{dump.Type{Kind: dump.String}, "STRING(MAX)"},
		{dump.Type{Kind: dump.String, Length: 20}, "STRING(20)"},
		{dump.Type{Kind: dump.Binary}, "BYTES(MAX)"},
		{dump.Type{Kind: dump.LocalTimestamp}, "TIMESTAMP"},
	} {
		got, err := dump.RenderType(dbmeta.Spanner, tt.in)
		if err != nil || got.SQL != tt.sql || got.Level != dump.Exact {
			t.Errorf("RenderType(%+v) is %q %v, %v, and want %q", tt.in, got.SQL, got.Level, err, tt.sql)
		}
	}
	for _, in := range []dump.Type{{Kind: dump.Interval}, {Kind: dump.TimeOfDay}, {Kind: dump.Decimal, Precision: 20, Scale: 12}} {
		if _, err := dump.RenderType(dbmeta.Spanner, in); !errors.Is(err, dump.ErrNoEquivalent) {
			t.Errorf("RenderType(%+v) is %v, and want no equivalent", in, err)
		}
	}
}
