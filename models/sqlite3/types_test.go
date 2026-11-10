package sqlite3

import (
	"errors"
	"reflect"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// TestParseTypeReadsADeclaredType checks the kind of each declared type.
func TestParseTypeReadsADeclaredType(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		text string
		want dump.Type
	}{
		{"INTEGER", dump.Type{Kind: dump.Integer, Bits: 64}},
		{"BOOLEAN", dump.Type{Kind: dump.Boolean}},
		{"TINYINT", dump.Type{Kind: dump.Integer, Bits: 8}},
		{"UNSIGNED BIG INT", dump.Type{Kind: dump.Integer, Bits: 64, Unsigned: true}},
		{"REAL", dump.Type{Kind: dump.Float, Bits: 64}},
		{"DECIMAL(10,5)", dump.Type{Kind: dump.Decimal, Precision: 10, Scale: 5}},
		{"VARCHAR(20)", dump.Type{Kind: dump.String, Length: 20}},
		{"NATIVE CHARACTER(70)", dump.Type{Kind: dump.String, Fixed: true, Length: 70}},
		{"TEXT", dump.Type{Kind: dump.String}},
		{"BLOB", dump.Type{Kind: dump.Binary}},
		{"DATE", dump.Type{Kind: dump.Date}},
		{"DATETIME", dump.Type{Kind: dump.LocalTimestamp}},
		{"UUID", dump.Type{Kind: dump.UUID}},
		{"", dump.Type{Kind: dump.Untyped}},
		{"NUM", dump.Type{Kind: dump.Decimal}},
		{"FOO", dump.Type{Kind: dump.Other}},
	} {
		got, err := dump.ParseType(dbmeta.SQLite3, tt.text)
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

// TestRenderTypeWritesSQLite checks the text and the level of each kind.
func TestRenderTypeWritesSQLite(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		in    dump.Type
		sql   string
		level dump.Level
	}{
		{"bigint", dump.Type{Kind: dump.Integer, Bits: 64}, "integer", dump.Exact},
		{"unsigned bigint", dump.Type{Kind: dump.Integer, Bits: 64, Unsigned: true}, "integer", dump.Lossy},
		{"decimal", dump.Type{Kind: dump.Decimal, Precision: 10, Scale: 2}, "numeric", dump.Lossy},
		{"varchar", dump.Type{Kind: dump.String, Length: 20}, "varchar(20)", dump.Exact},
		{"text", dump.Type{Kind: dump.String}, "text", dump.Exact},
		{"blob", dump.Type{Kind: dump.Binary}, "blob", dump.Exact},
		{"timestamp", dump.Type{Kind: dump.LocalTimestamp}, "datetime", dump.Exact},
	} {
		got, err := dump.RenderType(dbmeta.SQLite3, tt.in)
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		if got.SQL != tt.sql || got.Level != tt.level {
			t.Errorf("%s: got %q at %s, want %q at %s", tt.name, got.SQL, got.Level, tt.sql, tt.level)
		}
	}
	for _, k := range []dump.Kind{dump.UUID, dump.Interval, dump.Array, dump.IPAddress} {
		if _, err := dump.RenderType(dbmeta.SQLite3, dump.Type{Kind: k}); !errors.Is(err, dump.ErrNoEquivalent) {
			t.Errorf("%s gave %v, want ErrNoEquivalent", k, err)
		}
	}
}
