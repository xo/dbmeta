package dump_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

func TestConvertReadsAValueByItsKind(t *testing.T) {
	t.Parallel()
	typ := func(k dump.Kind) dump.Type { return dump.Type{Kind: k} }
	for _, tt := range []struct {
		name string
		v    any
		kind dump.Kind
		want any
	}{
		{"nil", nil, dump.Integer, nil},
		{"int32", int32(7), dump.Integer, int64(7)},
		{"uint8", uint8(7), dump.Integer, int64(7)},
		{"bit true", []byte{1}, dump.Boolean, true},
		{"tinyint false", int64(0), dump.Boolean, false},
		{"text true", "t", dump.Boolean, true},
		{"float32", float32(1.5), dump.Float, 1.5},
		{"decimal text", []byte("12.50"), dump.Decimal, "12.50"},
		{"decimal float", 0.25, dump.Decimal, "0.25"},
		{"uuid bytes", []byte("0123456789abcdef"), dump.UUID, "30313233-3435-3637-3839-616263646566"},
		{"uuid text", "30313233343536373839616263646566", dump.UUID, "30313233-3435-3637-3839-616263646566"},
		{"bytes from text", "ab", dump.Binary, []byte("ab")},
	} {
		got, err := dump.Convert(tt.v, typ(tt.kind), typ(tt.kind))
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		if g, w := stringOf(got), stringOf(tt.want); g != w {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
	for _, tt := range []struct {
		name string
		v    any
		kind dump.Kind
	}{
		{"two is not a boolean", int64(2), dump.Boolean},
		{"two bytes are not a bit", []byte{1, 1}, dump.Boolean},
		{"text is not an integer", "abc", dump.Integer},
		{"not a decimal", "NaN", dump.Decimal},
		{"not a UUID", "xyz", dump.UUID},
		{"not a date", "yesterday", dump.Date},
	} {
		if _, err := dump.Convert(tt.v, typ(tt.kind), typ(tt.kind)); !errors.Is(err, dump.ErrBadValue) {
			t.Errorf("%s: got %v, want ErrBadValue", tt.name, err)
		}
	}
}

func stringOf(v any) string {
	if b, ok := v.([]byte); ok {
		return "bytes:" + string(b)
	}
	return fmt.Sprint(v)
}

func TestConvertReadsDatesAndTimes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		text string
		kind dump.Kind
		want string
	}{
		{"2024-02-29", dump.Date, "2024-02-29T00:00:00Z"},
		{"2024-02-29 13:14:15.5", dump.LocalTimestamp, "2024-02-29T13:14:15.5Z"},
		{"2024-02-29T13:14:15+02:00", dump.Timestamp, "2024-02-29T13:14:15+02:00"},
		{"13:14:15", dump.TimeOfDay, "0000-01-01T13:14:15Z"},
	}
	for _, c := range cases {
		got, err := dump.Convert(c.text, dump.Type{Kind: c.kind}, dump.Type{Kind: c.kind})
		if err != nil {
			t.Errorf("%s: %v", c.text, err)
			continue
		}
		tm, ok := got.(time.Time)
		if !ok || tm.Format(time.RFC3339Nano) != c.want {
			t.Errorf("%s: got %v, want %s", c.text, got, c.want)
		}
	}
}

// pgSnapshot is a small PostgreSQL schema for the loader.
func pgSnapshot() *dump.Snapshot {
	typ := func(raw string) dump.Type {
		t, err := dump.ParseType(dbmeta.PostgreSQL, raw)
		if err != nil {
			panic(err)
		}
		return t
	}
	return &dump.Snapshot{
		Version: dump.SnapshotVersion, Dialect: "postgres",
		Schemas: []dump.Schema{{Name: "public", Tables: []dump.Table{{
			Name: "t",
			Columns: []dump.Column{
				{Name: "id", Type: typ("integer"), Identity: "a"},
				{Name: "name", Type: typ("text"), Nullable: true},
				{Name: "data", Type: typ("bytea"), Nullable: true},
				{Name: "n", Type: typ("integer"), Nullable: true, Generated: "s"},
			},
			Constraints: []dump.Constraint{{Name: "t_pkey", Kind: "primary key", Columns: []string{"id"}}},
		}}}},
	}
}

func TestALoaderWritesTheLiteralsOfThePostgresTarget(t *testing.T) {
	t.Parallel()
	l, err := dump.NewLoader(pgSnapshot(), "public", "t", dbmeta.PostgreSQL, dump.Policy{})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(l.Columns(), ","); got != "id,name,data" {
		t.Errorf("the columns are %s, want id,name,data, because the generated column is computed", got)
	}
	if !l.Ordered() || l.SelectAll() != `SELECT "id", "name", "data" FROM "public"."t" ORDER BY "id"` {
		t.Errorf("SelectAll gave %q", l.SelectAll())
	}
	stmt, err := l.Insert([][]any{{int64(1), "it's a \\ test", []byte{0, 255}}, {int64(2), nil, nil}})
	if err != nil {
		t.Fatal(err)
	}
	want := `INSERT INTO "public"."t" ("id", "name", "data") OVERRIDING SYSTEM VALUE VALUES ` +
		`(1, E'it''s a \\ test', decode('00ff', 'hex')), (2, NULL, NULL)`
	if stmt != want {
		t.Errorf("got\n%s\nwant\n%s", stmt, want)
	}
	if got := l.InsertArgs(2); got != `INSERT INTO "public"."t" ("id", "name", "data") OVERRIDING SYSTEM VALUE VALUES ($1, $2, $3), ($4, $5, $6)` {
		t.Errorf("InsertArgs gave %s", got)
	}
}

func TestALoaderRefusesWhatItCannotWrite(t *testing.T) {
	t.Parallel()
	l, err := dump.NewLoader(pgSnapshot(), "public", "t", dbmeta.PostgreSQL, dump.Policy{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.Insert([][]any{{int64(1), "a\x00b", nil}}); !errors.Is(err, dump.ErrBadValue) {
		t.Errorf("a NUL in text gave %v, want ErrBadValue", err)
	}
	if _, err := l.Insert([][]any{{int64(1), "x"}}); !errors.Is(err, dump.ErrBadValue) {
		t.Errorf("a short row gave %v, want ErrBadValue", err)
	}
	if _, err := l.Insert(nil); err == nil {
		t.Error("an INSERT of no rows was written")
	}
}

func TestTheBatchHoldsTheParametersOfTheProduct(t *testing.T) {
	t.Parallel()
	l, err := dump.NewLoader(pgSnapshot(), "public", "t", dbmeta.SQLite3, dump.Policy{Default: dump.Allow{Lossy: true, Text: true}})
	if err != nil {
		t.Fatal(err)
	}
	// SQLite takes 999 variables. The generated column is a plain column in
	// another dialect, so the table has four columns and the batch is 249 rows.
	if got := l.BatchSize(); got != 249 {
		t.Errorf("the batch is %d, want 249", got)
	}
}
