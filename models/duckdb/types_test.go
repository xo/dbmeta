package duckdb

import (
	"errors"
	"reflect"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// TestParseTypeReadsDuckDB checks the kind of each type text.
func TestParseTypeReadsDuckDB(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		text string
		want dump.Type
	}{
		{"BOOLEAN", dump.Type{Kind: dump.Boolean}},
		{"TINYINT", dump.Type{Kind: dump.Integer, Bits: 8}},
		{"UBIGINT", dump.Type{Kind: dump.Integer, Bits: 64, Unsigned: true}},
		{"HUGEINT", dump.Type{Kind: dump.BigInteger}},
		{"FLOAT", dump.Type{Kind: dump.Float, Bits: 32}},
		{"DOUBLE", dump.Type{Kind: dump.Float, Bits: 64}},
		{"DECIMAL(10,2)", dump.Type{Kind: dump.Decimal, Precision: 10, Scale: 2}},
		{"VARCHAR", dump.Type{Kind: dump.String}},
		{"BLOB", dump.Type{Kind: dump.Binary}},
		{"TIME WITH TIME ZONE", dump.Type{Kind: dump.TimeOfDayOffset}},
		{"TIMESTAMP", dump.Type{Kind: dump.LocalTimestamp}},
		{"TIMESTAMP_MS", dump.Type{Kind: dump.LocalTimestamp, Precision: 3}},
		{"TIMESTAMP_NS", dump.Type{Kind: dump.LocalTimestamp, Precision: 9}},
		{"TIMESTAMP WITH TIME ZONE", dump.Type{Kind: dump.Timestamp}},
		{"INTERVAL", dump.Type{Kind: dump.Interval}},
		{"UUID", dump.Type{Kind: dump.UUID}},
		{"JSON", dump.Type{Kind: dump.JSON}},
		{"STRUCT(a INTEGER, b VARCHAR)", dump.Type{Kind: dump.Tuple}},
		{"MAP(VARCHAR, INTEGER)", dump.Type{Kind: dump.Map}},
		{"ENUM('a', 'b')", dump.Type{Kind: dump.String}},
		{"INTEGER[3]", dump.Type{Kind: dump.Other}},
	} {
		got, err := dump.ParseType(dbmeta.DuckDB, tt.text)
		if err != nil {
			t.Errorf("ParseType(%q): %v", tt.text, err)
			continue
		}
		want := tt.want
		want.Raw = tt.text
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ParseType(%q) = %+v, want %+v", tt.text, got, want)
		}
	}
	got, err := dump.ParseType(dbmeta.DuckDB, "INTEGER[]")
	if err != nil || got.Kind != dump.Array || got.Elem == nil || got.Elem.Bits != 32 {
		t.Errorf("got %+v, %v, want an array of integer", got, err)
	}
}

// TestRenderTypeWritesDuckDB checks the text and the level of each kind.
func TestRenderTypeWritesDuckDB(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		in    dump.Type
		sql   string
		level dump.Level
	}{
		{"boolean", dump.Type{Kind: dump.Boolean}, "BOOLEAN", dump.Exact},
		{"unsigned smallint", dump.Type{Kind: dump.Integer, Bits: 16, Unsigned: true}, "USMALLINT", dump.Exact},
		{"decimal", dump.Type{Kind: dump.Decimal, Precision: 10, Scale: 2}, "DECIMAL(10,2)", dump.Exact},
		{"huge decimal", dump.Type{Kind: dump.Decimal, Precision: 50, Scale: 2}, "DECIMAL(38,2)", dump.Lossy},
		{"varchar", dump.Type{Kind: dump.String, Length: 20}, "VARCHAR", dump.Exact},
		{"nanoseconds", dump.Type{Kind: dump.LocalTimestamp, Precision: 9}, "TIMESTAMP_NS", dump.Exact},
		{"instant nanoseconds", dump.Type{Kind: dump.Timestamp, Precision: 9}, "TIMESTAMP WITH TIME ZONE", dump.Lossy},
		{"uuid", dump.Type{Kind: dump.UUID}, "UUID", dump.Exact},
		{"array", dump.Type{Kind: dump.Array, Elem: &dump.Type{Kind: dump.Integer, Bits: 32}}, "INTEGER[]", dump.Exact},
	} {
		got, err := dump.RenderType(dbmeta.DuckDB, tt.in)
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		if got.SQL != tt.sql || got.Level != tt.level {
			t.Errorf("%s: got %q at %s, want %q at %s", tt.name, got.SQL, got.Level, tt.sql, tt.level)
		}
	}
	for _, in := range []dump.Type{{Kind: dump.Decimal}, {Kind: dump.IPAddress}, {Kind: dump.Geometry}} {
		if _, err := dump.RenderType(dbmeta.DuckDB, in); !errors.Is(err, dump.ErrNoEquivalent) {
			t.Errorf("%s gave %v, want ErrNoEquivalent", in.Kind, err)
		}
	}
}
