package cratedb

import (
	"errors"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

func TestTheTypesOfCrateDBAreRead(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]dump.Kind{
		"integer":                     dump.Integer,
		"byte":                        dump.Integer,
		"numeric(10,2)":               dump.Decimal,
		"character varying(5)":        dump.String,
		"character(5)":                dump.String,
		"timestamp with time zone":    dump.Timestamp,
		"timestamp without time zone": dump.LocalTimestamp,
		"ip":                          dump.IPAddress,
		"integer_array":               dump.Array,
		"text_array_array":            dump.Array,
		"object":                      dump.Other,
		"geo_point":                   dump.Other,
	} {
		got, err := dump.ParseType(dbmeta.CrateDB, text)
		if err != nil || got.Kind != want {
			t.Errorf("ParseType(%q) is %v, %v, and want %s", text, got.Kind, err, want)
		}
	}
	got, err := dump.ParseType(dbmeta.CrateDB, "text_array_array")
	if err != nil || got.Elem == nil || got.Elem.Elem == nil || got.Elem.Elem.Kind != dump.String {
		t.Errorf("text_array_array is %+v, %v, and want an array of an array of text", got, err)
	}
}

func TestTheTypesOfCrateDBAreWritten(t *testing.T) {
	t.Parallel()
	text := dump.Type{Kind: dump.String}
	for _, tt := range []struct {
		in    dump.Type
		sql   string
		level dump.Level
	}{
		{dump.Type{Kind: dump.Integer, Bits: 8}, "byte", dump.Exact},
		{dump.Type{Kind: dump.Integer, Bits: 32, Unsigned: true}, "bigint", dump.Exact},
		{dump.Type{Kind: dump.Decimal, Precision: 10, Scale: 2}, "numeric(10,2)", dump.Exact},
		{dump.Type{Kind: dump.String, Length: 20}, "varchar(20)", dump.Exact},
		{dump.Type{Kind: dump.Date}, "timestamp without time zone", dump.Lossy},
		{dump.Type{Kind: dump.Timestamp, Precision: 6}, "timestamp with time zone", dump.Lossy},
		{dump.Type{Kind: dump.Array, Elem: &text}, "array(text)", dump.Exact},
	} {
		got, err := dump.RenderType(dbmeta.CrateDB, tt.in)
		if err != nil || got.SQL != tt.sql || got.Level != tt.level {
			t.Errorf("RenderType(%+v) is %q %v, %v, and want %q %v", tt.in, got.SQL, got.Level, err, tt.sql, tt.level)
		}
	}
	for _, in := range []dump.Type{
		{Kind: dump.Binary}, {Kind: dump.Interval}, {Kind: dump.JSON}, {Kind: dump.TimeOfDay},
		{Kind: dump.Decimal},
	} {
		if _, err := dump.RenderType(dbmeta.CrateDB, in); !errors.Is(err, dump.ErrNoEquivalent) {
			t.Errorf("RenderType(%s) is %v, and want no equivalent", in.Kind, err)
		}
	}
}
