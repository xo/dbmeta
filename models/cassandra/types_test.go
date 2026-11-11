package cassandra

import (
	"errors"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

func TestTheTypesOfCassandraAreRead(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]dump.Kind{
		"int":               dump.Integer,
		"varint":            dump.BigInteger,
		"text":              dump.String,
		"list<int>":         dump.Array,
		"frozen<set<text>>": dump.Set,
		"timestamp":         dump.Timestamp,
		"map<text, int>":    dump.Other,
	} {
		got, err := dump.ParseType(dbmeta.Cassandra, text)
		if err != nil || got.Kind != want {
			t.Errorf("ParseType(%q) is %v, %v, and want %s", text, got.Kind, err, want)
		}
	}
}

func TestTheTypesOfCassandraAreWritten(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		in    dump.Type
		sql   string
		level dump.Level
	}{
		{dump.Type{Kind: dump.Integer, Bits: 8}, "tinyint", dump.Exact},
		{dump.Type{Kind: dump.Decimal, Precision: 10, Scale: 2}, "decimal", dump.Exact},
		{dump.Type{Kind: dump.LocalTimestamp}, "timestamp", dump.Lossy},
		{dump.Type{Kind: dump.Integer, Bits: 64, Unsigned: true}, "varint", dump.Exact},
	} {
		got, err := dump.RenderType(dbmeta.Cassandra, tt.in)
		if err != nil || got.SQL != tt.sql || got.Level != tt.level {
			t.Errorf("RenderType(%+v) is %q %v, %v, and want %q %v", tt.in, got.SQL, got.Level, err, tt.sql, tt.level)
		}
	}
	for _, in := range []dump.Type{{Kind: dump.Interval}, {Kind: dump.JSON}} {
		if _, err := dump.RenderType(dbmeta.Cassandra, in); !errors.Is(err, dump.ErrNoEquivalent) {
			t.Errorf("RenderType(%s) is %v, and want no equivalent", in.Kind, err)
		}
	}
}
