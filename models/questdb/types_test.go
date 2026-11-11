package questdb

import (
	"errors"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

func TestTheTypesOfQuestDBAreRead(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]dump.Kind{
		"integer":                     dump.Integer,
		"smallint":                    dump.Integer,
		"double precision":            dump.Float,
		"character varying":           dump.String,
		"timestamp without time zone": dump.LocalTimestamp,
		"bytea":                       dump.Binary,
		"unknown":                     dump.Other,
	} {
		got, err := dump.ParseType(dbmeta.QuestDB, text)
		if err != nil || got.Kind != want {
			t.Errorf("ParseType(%q) is %v, %v, and want %s", text, got.Kind, err, want)
		}
	}
}

func TestTheTypesOfQuestDBAreWritten(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		in    dump.Type
		sql   string
		level dump.Level
	}{
		{dump.Type{Kind: dump.Integer, Bits: 32}, "int", dump.Exact},
		{dump.Type{Kind: dump.Integer, Bits: 64}, "long", dump.Exact},
		{dump.Type{Kind: dump.Integer, Bits: 16}, "short", dump.Lossy},
		{dump.Type{Kind: dump.Boolean}, "boolean", dump.Lossy},
		{dump.Type{Kind: dump.Decimal, Precision: 10, Scale: 2}, "decimal(10,2)", dump.Exact},
		{dump.Type{Kind: dump.String, Length: 20}, "varchar", dump.Exact},
		{dump.Type{Kind: dump.Timestamp, Precision: 9}, "timestamp_ns", dump.Exact},
		{dump.Type{Kind: dump.Timestamp}, "timestamp", dump.Exact},
	} {
		got, err := dump.RenderType(dbmeta.QuestDB, tt.in)
		if err != nil || got.SQL != tt.sql || got.Level != tt.level {
			t.Errorf("RenderType(%+v) is %q %v, %v, and want %q %v", tt.in, got.SQL, got.Level, err, tt.sql, tt.level)
		}
	}
	for _, in := range []dump.Type{{Kind: dump.Binary}, {Kind: dump.Interval}, {Kind: dump.JSON}, {Kind: dump.Decimal}} {
		if _, err := dump.RenderType(dbmeta.QuestDB, in); !errors.Is(err, dump.ErrNoEquivalent) {
			t.Errorf("RenderType(%s) is %v, and want no equivalent", in.Kind, err)
		}
	}
}
