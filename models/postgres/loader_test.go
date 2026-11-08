package postgres_test

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
	_ "github.com/xo/dbmeta/models/postgres"
)

// TestTheLoaderWritesWhatDriversGive covers two values that a driver gives and
// that the loader refused: NaN and the infinities of a float, and a time of day,
// which a driver gives with the year 0.
func TestTheLoaderWritesWhatDriversGive(t *testing.T) {
	snap := &dump.Snapshot{
		Dialect: string(dbmeta.PostgreSQL),
		Schemas: []dump.Schema{{Name: "public", Tables: []dump.Table{{Name: "t", Columns: []dump.Column{
			{Name: "f", Nullable: true, Type: dump.Type{Kind: dump.Float, Bits: 64, Raw: "double precision"}},
			{Name: "tod", Nullable: true, Type: dump.Type{Kind: dump.TimeOfDay, Raw: "time without time zone"}},
		}}}}},
	}
	l, err := dump.NewLoader(snap, "public", "t", dbmeta.PostgreSQL, dump.Policy{})
	if err != nil {
		t.Fatal(err)
	}
	tod := time.Date(0, 1, 1, 12, 34, 56, 0, time.UTC)
	got, err := l.Insert([][]any{{math.NaN(), tod}, {math.Inf(1), nil}, {math.Inf(-1), nil}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"'NaN'::double precision", "'Infinity'::double precision", "'-Infinity'::double precision", "'12:34:56'"} {
		if !strings.Contains(got, want) {
			t.Errorf("the INSERT lacks %s:\n%s", want, got)
		}
	}
}
