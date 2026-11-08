package mysql

import (
	"errors"
	"reflect"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// TestParseTypeReadsColumnType checks the kind of each COLUMN_TYPE text.
func TestParseTypeReadsColumnType(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		text string
		want dump.Type
	}{
		{"tinyint(1)", dump.Type{Kind: dump.Boolean}},
		{"tinyint(4)", dump.Type{Kind: dump.Integer, Bits: 8}},
		{"tinyint(1) unsigned", dump.Type{Kind: dump.Integer, Bits: 8, Unsigned: true}},
		{"int unsigned", dump.Type{Kind: dump.Integer, Bits: 32, Unsigned: true}},
		{"bigint unsigned zerofill", dump.Type{Kind: dump.Integer, Bits: 64, Unsigned: true}},
		{"double precision", dump.Type{Kind: dump.Float, Bits: 64}},
		{"smallint", dump.Type{Kind: dump.Integer, Bits: 16}},
		{"int", dump.Type{Kind: dump.Integer, Bits: 32}},
		{"int(11) unsigned", dump.Type{Kind: dump.Integer, Bits: 32, Unsigned: true}},
		{"bigint(20) unsigned", dump.Type{Kind: dump.Integer, Bits: 64, Unsigned: true}},
		{"float", dump.Type{Kind: dump.Float, Bits: 32}},
		{"double", dump.Type{Kind: dump.Float, Bits: 64}},
		{"decimal(10,2)", dump.Type{Kind: dump.Decimal, Precision: 10, Scale: 2}},
		{"decimal(10,2) unsigned", dump.Type{Kind: dump.Decimal, Precision: 10, Scale: 2, Unsigned: true}},
		{"char(10)", dump.Type{Kind: dump.String, Fixed: true, Length: 10}},
		{"varchar(255)", dump.Type{Kind: dump.String, Length: 255}},
		{"longtext", dump.Type{Kind: dump.String}},
		{"binary(16)", dump.Type{Kind: dump.Binary, Fixed: true, Length: 16}},
		{"varbinary(255)", dump.Type{Kind: dump.Binary, Length: 255}},
		{"blob", dump.Type{Kind: dump.Binary}},
		{"bit(1)", dump.Type{Kind: dump.Boolean}},
		{"bit(8)", dump.Type{Kind: dump.Other}},
		{"date", dump.Type{Kind: dump.Date}},
		{"time(3)", dump.Type{Kind: dump.TimeOfDay, Precision: 3}},
		{"datetime(6)", dump.Type{Kind: dump.LocalTimestamp, Precision: 6}},
		{"timestamp", dump.Type{Kind: dump.Timestamp}},
		{"year", dump.Type{Kind: dump.Integer, Bits: 16, Unsigned: true}},
		{"json", dump.Type{Kind: dump.JSON}},
		{"point", dump.Type{Kind: dump.Geometry}},
		{"enum('a','b')", dump.Type{Kind: dump.String, Values: []string{"a", "b"}}},
		{`enum('it''s','a\\b')`, dump.Type{Kind: dump.String, Values: []string{"it's", `a\b`}}},
		{"set('x','y')", dump.Type{Kind: dump.Set, Elem: &dump.Type{Kind: dump.String}, Values: []string{"x", "y"}}},
	} {
		got, err := dump.ParseType(dbmeta.MySQL, tt.text)
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
	for _, text := range []string{"enum('a", "enum(a)"} {
		if _, err := dump.ParseType(dbmeta.MySQL, text); !errors.Is(err, dump.ErrUnknownType) {
			t.Errorf("ParseType(%q) gave %v, want ErrUnknownType", text, err)
		}
	}
}

// TestRenderTypeWritesMySQL checks the text and the level of each kind.
func TestRenderTypeWritesMySQL(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		in    dump.Type
		sql   string
		level dump.Level
	}{
		{"boolean", dump.Type{Kind: dump.Boolean}, "tinyint(1)", dump.Exact},
		{"smallint", dump.Type{Kind: dump.Integer, Bits: 16}, "smallint", dump.Exact},
		{"unsigned bigint", dump.Type{Kind: dump.Integer, Bits: 64, Unsigned: true}, "bigint unsigned", dump.Exact},
		{"decimal", dump.Type{Kind: dump.Decimal, Precision: 10, Scale: 2}, "decimal(10,2)", dump.Exact},
		{"huge decimal", dump.Type{Kind: dump.Decimal, Precision: 100, Scale: 40}, "decimal(65,30)", dump.Lossy},
		{"varchar", dump.Type{Kind: dump.String, Length: 255}, "varchar(255)", dump.Exact},
		{"long varchar", dump.Type{Kind: dump.String, Length: 100000}, "longtext", dump.Exact},
		{"text", dump.Type{Kind: dump.String}, "longtext", dump.Exact},
		{"char", dump.Type{Kind: dump.String, Fixed: true, Length: 5}, "char(5)", dump.Exact},
		{"enum", dump.Type{Kind: dump.String, Values: []string{"it's", `a\b`}}, `enum('it''s','a\\b')`, dump.Exact},
		{"binary", dump.Type{Kind: dump.Binary, Fixed: true, Length: 16}, "binary(16)", dump.Exact},
		{"bytea", dump.Type{Kind: dump.Binary}, "longblob", dump.Exact},
		{"timestamp", dump.Type{Kind: dump.LocalTimestamp, Precision: 3}, "datetime(3)", dump.Exact},
		{"nanoseconds", dump.Type{Kind: dump.LocalTimestamp, Precision: 9}, "datetime(6)", dump.Lossy},
		{"instant", dump.Type{Kind: dump.Timestamp}, "datetime", dump.Lossy},
		{"json", dump.Type{Kind: dump.JSON}, "json", dump.Exact},
	} {
		got, err := dump.RenderType(dbmeta.MySQL, tt.in)
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		if got.SQL != tt.sql || got.Level != tt.level {
			t.Errorf("%s: got %q at %s, want %q at %s", tt.name, got.SQL, got.Level, tt.sql, tt.level)
		}
	}
	for _, in := range []dump.Type{
		{Kind: dump.UUID}, {Kind: dump.Interval}, {Kind: dump.Array}, {Kind: dump.Decimal},
		{Kind: dump.TimeOfDayOffset}, {Kind: dump.IPAddress},
	} {
		if _, err := dump.RenderType(dbmeta.MySQL, in); !errors.Is(err, dump.ErrNoEquivalent) {
			t.Errorf("%s gave %v, want ErrNoEquivalent", in.Kind, err)
		}
	}
}

// TestTextForWritesALongText checks the Text level, which a client asks for.
func TestTextForWritesALongText(t *testing.T) {
	t.Parallel()
	got, err := dump.TextFor(dbmeta.MySQL, dump.Type{Kind: dump.UUID})
	if err != nil || got.SQL != "longtext" || got.Level != dump.Text || got.Note == "" {
		t.Errorf("got %+v, %v, want longtext at text with a note", got, err)
	}
}
