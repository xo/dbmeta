package clickhouse

import (
	"errors"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

func TestTheTypesOfClickHouseAreRead(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]dump.Kind{
		"Int32":                    dump.Integer,
		"UInt64":                   dump.Integer,
		"Nullable(Decimal(10, 2))": dump.Decimal,
		"Decimal64(4)":             dump.Decimal,
		"LowCardinality(String)":   dump.String,
		"FixedString(8)":           dump.String,
		"DateTime64(3, 'UTC')":     dump.Timestamp,
		"DateTime64(6)":            dump.LocalTimestamp,
		"Date32":                   dump.Date,
		"Array(Nullable(String))":  dump.Array,
		"Map(String, Int32)":       dump.Other,
		"Enum8('a' = 1, 'b' = 2)":  dump.Other,
	} {
		got, err := dump.ParseType(dbmeta.ClickHouse, text)
		if err != nil || got.Kind != want {
			t.Errorf("ParseType(%q) is %v, %v, and want %s", text, got.Kind, err, want)
		}
	}
	got, err := dump.ParseType(dbmeta.ClickHouse, "Nullable(Decimal(10, 2))")
	if err != nil || got.Precision != 10 || got.Scale != 2 {
		t.Errorf("Nullable(Decimal(10, 2)) is %+v, %v", got, err)
	}
}

func TestTheTypesOfClickHouseAreWritten(t *testing.T) {
	t.Parallel()
	text := dump.Type{Kind: dump.String}
	for _, tt := range []struct {
		in    dump.Type
		sql   string
		level dump.Level
	}{
		{dump.Type{Kind: dump.Integer, Bits: 16, Unsigned: true}, "UInt16", dump.Exact},
		{dump.Type{Kind: dump.Decimal, Precision: 10, Scale: 2}, "Decimal(10, 2)", dump.Exact},
		{dump.Type{Kind: dump.Binary}, "String", dump.Lossy},
		{dump.Type{Kind: dump.Date}, "Date32", dump.Exact},
		{dump.Type{Kind: dump.LocalTimestamp}, "DateTime64(6)", dump.Exact},
		{dump.Type{Kind: dump.Timestamp, Precision: 12}, "DateTime64(9, 'UTC')", dump.Exact},
		{dump.Type{Kind: dump.Array, Elem: &text}, "Array(String)", dump.Exact},
	} {
		got, err := dump.RenderType(dbmeta.ClickHouse, tt.in)
		if err != nil || got.SQL != tt.sql || got.Level != tt.level {
			t.Errorf("RenderType(%+v) is %q %v, %v, and want %q %v", tt.in, got.SQL, got.Level, err, tt.sql, tt.level)
		}
	}
	for _, in := range []dump.Type{{Kind: dump.Interval}, {Kind: dump.TimeOfDay}, {Kind: dump.JSON}, {Kind: dump.Decimal}} {
		if _, err := dump.RenderType(dbmeta.ClickHouse, in); !errors.Is(err, dump.ErrNoEquivalent) {
			t.Errorf("RenderType(%s) is %v, and want no equivalent", in.Kind, err)
		}
	}
}
