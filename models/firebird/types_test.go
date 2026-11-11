package firebird

import (
	"errors"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

func TestTheTypesOfFirebirdAreRead(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]dump.Kind{
		"VARCHAR(20)":      dump.String,
		"DECIMAL(10,2)":    dump.Decimal,
		"BLOB SUB_TYPE 1":  dump.String,
		"BLOB SUB_TYPE 0":  dump.Binary,
		"TIMESTAMP":        dump.LocalTimestamp,
		"DOUBLE PRECISION": dump.Float,
		"BOOLEAN":          dump.Boolean,
		"QUAD":             dump.Other,
	} {
		got, err := dump.ParseType(dbmeta.Firebird, text)
		if err != nil || got.Kind != want {
			t.Errorf("ParseType(%q) is %v, %v, and want %s", text, got.Kind, err, want)
		}
	}
}

func TestTheTypesOfFirebirdAreWritten(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		in    dump.Type
		sql   string
		level dump.Level
	}{
		{dump.Type{Kind: dump.Integer, Bits: 16, Unsigned: true}, "INTEGER", dump.Exact},
		{dump.Type{Kind: dump.String, Length: 20}, "VARCHAR(20) CHARACTER SET UTF8", dump.Exact},
		{dump.Type{Kind: dump.String}, "BLOB SUB_TYPE TEXT CHARACTER SET UTF8", dump.Exact},
		{dump.Type{Kind: dump.Timestamp}, "TIMESTAMP", dump.Lossy},
		{dump.Type{Kind: dump.Binary}, "BLOB SUB_TYPE BINARY", dump.Exact},
	} {
		got, err := dump.RenderType(dbmeta.Firebird, tt.in)
		if err != nil || got.SQL != tt.sql || got.Level != tt.level {
			t.Errorf("RenderType(%+v) is %q %v, %v, and want %q %v", tt.in, got.SQL, got.Level, err, tt.sql, tt.level)
		}
	}
	for _, in := range []dump.Type{{Kind: dump.Interval}, {Kind: dump.JSON}, {Kind: dump.Integer, Bits: 64, Unsigned: true}} {
		if _, err := dump.RenderType(dbmeta.Firebird, in); !errors.Is(err, dump.ErrNoEquivalent) {
			t.Errorf("RenderType(%s) is %v, and want no equivalent", in.Kind, err)
		}
	}
}
