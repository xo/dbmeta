package typeparse_test

import (
	"errors"
	"testing"

	"github.com/xo/dbmeta/dump"
	"github.com/xo/dbmeta/models/internal/typeparse"
)

func TestTheFamiliesOfTypeNamesAreRead(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		parse func(string) (dump.Type, error)
		text  string
		kind  dump.Kind
	}{
		{"presto", typeparse.Presto, "varchar(255)", dump.String},
		{"presto", typeparse.Presto, "decimal(10, 2)", dump.Decimal},
		{"presto", typeparse.Presto, "timestamp(3) with time zone", dump.Timestamp},
		{"presto", typeparse.Presto, "timestamp(6)", dump.LocalTimestamp},
		{"presto", typeparse.Presto, "array(varchar)", dump.Array},
		{"presto", typeparse.Presto, "map(varchar, integer)", dump.Other},
		{"presto", typeparse.Presto, "row(a integer, b varchar)", dump.Other},
		{"hive", typeparse.Hive, "decimal(10,2)", dump.Decimal},
		{"hive", typeparse.Hive, "string", dump.String},
		{"hive", typeparse.Hive, "array<string>", dump.Array},
		{"hive", typeparse.Hive, "struct<a:int>", dump.Other},
		{"jdbc", typeparse.JDBC, "CHARACTER VARYING", dump.String},
		{"jdbc", typeparse.JDBC, "BIGINT", dump.Integer},
		{"jdbc", typeparse.JDBC, "ANY", dump.Other},
		{"elastic", typeparse.Elastic, "long", dump.Integer},
		{"elastic", typeparse.Elastic, "KEYWORD", dump.String},
		{"elastic", typeparse.Elastic, "DATETIME", dump.Timestamp},
		{"elastic", typeparse.Elastic, "GEO_POINT", dump.Other},
		{"arrow", typeparse.Arrow, "Int64", dump.Integer},
		{"arrow", typeparse.Arrow, "Timestamp(ns)", dump.LocalTimestamp},
		{"arrow", typeparse.Arrow, "Dictionary(Int32, Utf8)", dump.String},
	} {
		got, err := tt.parse(tt.text)
		if err != nil || got.Kind != tt.kind {
			t.Errorf("%s: parsing %q is %v, %v, and want %s", tt.name, tt.text, got.Kind, err, tt.kind)
		}
	}
	if got, _ := typeparse.Presto("array(varchar)"); got.Elem == nil || got.Elem.Kind != dump.String {
		t.Errorf("the element of array(varchar) is %+v", got.Elem)
	}
	if got, _ := typeparse.Arrow("Timestamp(ns)"); got.Precision != 9 {
		t.Errorf("Timestamp(ns) has %d digits, and want 9", got.Precision)
	}
}

func TestAProductThatIsOnlyASourceWritesNoType(t *testing.T) {
	t.Parallel()
	if _, err := typeparse.NoRender(dump.Type{Kind: dump.Integer}); !errors.Is(err, dump.ErrNoEquivalent) {
		t.Errorf("NoRender gave %v, and want no equivalent", err)
	}
}
