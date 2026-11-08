package postgres

import (
	"errors"
	"reflect"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// TestParseTypeReadsWhatFormatTypeGives checks the kind of each text that
// format_type gives, for the types of a table that has one of each.
func TestParseTypeReadsWhatFormatTypeGives(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		text string
		want dump.Type
	}{
		{"smallint", dump.Type{Kind: dump.Integer, Bits: 16}},
		{"integer", dump.Type{Kind: dump.Integer, Bits: 32}},
		{"bigint", dump.Type{Kind: dump.Integer, Bits: 64}},
		{"real", dump.Type{Kind: dump.Float, Bits: 32}},
		{"double precision", dump.Type{Kind: dump.Float, Bits: 64}},
		{"numeric", dump.Type{Kind: dump.Decimal}},
		{"numeric(10,2)", dump.Type{Kind: dump.Decimal, Precision: 10, Scale: 2}},
		{"numeric(10)", dump.Type{Kind: dump.Decimal, Precision: 10}},
		{"character(5)", dump.Type{Kind: dump.String, Fixed: true, Length: 5}},
		{"character varying(10)", dump.Type{Kind: dump.String, Length: 10}},
		{"character varying", dump.Type{Kind: dump.String}},
		{"text", dump.Type{Kind: dump.String}},
		{"bytea", dump.Type{Kind: dump.Binary}},
		{"boolean", dump.Type{Kind: dump.Boolean}},
		{"date", dump.Type{Kind: dump.Date}},
		{"time without time zone", dump.Type{Kind: dump.TimeOfDay}},
		{"time(3) without time zone", dump.Type{Kind: dump.TimeOfDay, Precision: 3}},
		{"time with time zone", dump.Type{Kind: dump.TimeOfDayOffset}},
		{"timestamp(6) without time zone", dump.Type{Kind: dump.LocalTimestamp, Precision: 6}},
		{"timestamp with time zone", dump.Type{Kind: dump.Timestamp}},
		{"interval", dump.Type{Kind: dump.Interval}},
		{"interval year to month", dump.Type{Kind: dump.Interval}},
		{"interval(3)", dump.Type{Kind: dump.Interval, Precision: 3}},
		{"uuid", dump.Type{Kind: dump.UUID}},
		{"json", dump.Type{Kind: dump.JSON}},
		{"jsonb", dump.Type{Kind: dump.JSON}},
		{"inet", dump.Type{Kind: dump.IPAddress}},
		{"int4range", dump.Type{Kind: dump.Range}},
		{"public.mood", dump.Type{Kind: dump.Other}},
		{"money", dump.Type{Kind: dump.Other}},
		{"bit varying(5)", dump.Type{Kind: dump.Other}},
	} {
		got, err := dump.ParseType(dbmeta.PostgreSQL, tt.text)
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
}

// TestParseTypeReadsAnArray checks that an array has the element type inside.
func TestParseTypeReadsAnArray(t *testing.T) {
	t.Parallel()
	got, err := dump.ParseType(dbmeta.PostgreSQL, "character varying(10)[]")
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != dump.Array || got.Elem == nil || got.Elem.Kind != dump.String || got.Elem.Length != 10 {
		t.Errorf("got %+v, want an array of character varying(10)", got)
	}
}

// TestRenderTypeWritesPostgreSQL checks the text and the level of each kind.
func TestRenderTypeWritesPostgreSQL(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		in    dump.Type
		sql   string
		level dump.Level
	}{
		{"tinyint", dump.Type{Kind: dump.Integer, Bits: 8}, "smallint", dump.Exact},
		{"unsigned int", dump.Type{Kind: dump.Integer, Bits: 32, Unsigned: true}, "bigint", dump.Exact},
		{"unsigned bigint", dump.Type{Kind: dump.Integer, Bits: 64, Unsigned: true}, "numeric(20,0)", dump.Exact},
		{"float32", dump.Type{Kind: dump.Float, Bits: 32}, "real", dump.Exact},
		{"decimal", dump.Type{Kind: dump.Decimal, Precision: 10, Scale: 2}, "numeric(10,2)", dump.Exact},
		{"huge decimal", dump.Type{Kind: dump.Decimal, Precision: 2000, Scale: 2}, "numeric", dump.Lossy},
		{"char", dump.Type{Kind: dump.String, Fixed: true, Length: 5}, "character(5)", dump.Exact},
		{"varchar", dump.Type{Kind: dump.String, Length: 255}, "character varying(255)", dump.Exact},
		{"longtext", dump.Type{Kind: dump.String}, "text", dump.Exact},
		{"datetime", dump.Type{Kind: dump.LocalTimestamp}, "timestamp without time zone", dump.Exact},
		{"datetime(3)", dump.Type{Kind: dump.LocalTimestamp, Precision: 3}, "timestamp(3) without time zone", dump.Exact},
		{"nanoseconds", dump.Type{Kind: dump.Timestamp, Precision: 9}, "timestamp(6) with time zone", dump.Lossy},
		{"timetz", dump.Type{Kind: dump.TimeOfDayOffset}, "time with time zone", dump.Exact},
		{"interval", dump.Type{Kind: dump.Interval}, "interval", dump.Exact},
		{"uuid", dump.Type{Kind: dump.UUID}, "uuid", dump.Exact},
		{"array", dump.Type{Kind: dump.Array, Elem: &dump.Type{Kind: dump.Integer, Bits: 32}}, "integer[]", dump.Exact},
		{"array of lossy", dump.Type{Kind: dump.Array, Elem: &dump.Type{Kind: dump.Timestamp, Precision: 9}}, "timestamp(6) with time zone[]", dump.Lossy},
	} {
		got, err := dump.RenderType(dbmeta.PostgreSQL, tt.in)
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		if got.SQL != tt.sql || got.Level != tt.level {
			t.Errorf("%s: got %q at %s, want %q at %s", tt.name, got.SQL, got.Level, tt.sql, tt.level)
		}
		if (got.Level != dump.Exact) != (got.Note != "") {
			t.Errorf("%s: the note %q does not match the level %s", tt.name, got.Note, got.Level)
		}
	}
}

// TestRenderTypeRefusesWhatPostgreSQLLacks checks that a kind with no type is an
// error and not a guess.
func TestRenderTypeRefusesWhatPostgreSQLLacks(t *testing.T) {
	t.Parallel()
	for _, k := range []dump.Kind{dump.Geometry, dump.Vector, dump.Map, dump.Other} {
		if _, err := dump.RenderType(dbmeta.PostgreSQL, dump.Type{Kind: k}); !errors.Is(err, dump.ErrNoEquivalent) {
			t.Errorf("%s gave %v, want ErrNoEquivalent", k, err)
		}
	}
}

// TestTypeForWritesTheSameDialectBack checks that a column from the same
// dialect is written as it was read, whatever the neutral kind says.
func TestTypeForWritesTheSameDialectBack(t *testing.T) {
	t.Parallel()
	typ, err := dump.ParseType(dbmeta.PostgreSQL, "money")
	if err != nil {
		t.Fatal(err)
	}
	got, err := dump.TypeFor(dbmeta.PostgreSQL, dbmeta.PostgreSQL, typ)
	if err != nil || got.SQL != "money" || got.Level != dump.Exact {
		t.Errorf("got %+v, %v, want money at exact", got, err)
	}
}
