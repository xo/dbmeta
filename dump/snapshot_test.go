package dump_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/xo/dbmeta/dump"
)

func sample() *dump.Snapshot {
	def := "0"
	return &dump.Snapshot{
		Version: dump.SnapshotVersion, Dialect: "postgres", Server: "PostgreSQL 18",
		Schemas: []dump.Schema{{
			Name: "app",
			Tables: []dump.Table{{
				Name: "child",
				Columns: []dump.Column{
					{Name: "id", Type: dump.Type{Kind: dump.Integer, Bits: 32, Raw: "integer"}},
					{Name: "n", Type: dump.Type{Kind: dump.Decimal, Precision: 10, Scale: 2, Raw: "numeric(10,2)"}, Nullable: true, Default: &def},
				},
				Constraints: []dump.Constraint{
					{Name: "child_pkey", Kind: "primary key", Columns: []string{"id"}, Definition: "PRIMARY KEY (id)"},
					{Name: "child_fk", Kind: "foreign key", Columns: []string{"id"},
						Ref: &dump.Reference{Schema: "app", Table: "parent", Columns: []string{"id"}}},
				},
				Indexes: []dump.Index{{Name: "child_n", Columns: []dump.IndexColumn{{Name: "n", Descending: true}}}},
			}},
			Views: []dump.View{{Name: "v", Definition: "SELECT 1"}},
		}},
		Skipped: []dump.Skipped{{Schema: "app", Name: "m", Kind: "materialized view", Reason: "not read"}},
	}
}

func TestSnapshotJSONRoundTrips(t *testing.T) {
	t.Parallel()
	want := sample()
	data, err := want.JSON()
	if err != nil {
		t.Fatal(err)
	}
	got, err := dump.ParseSnapshot(data)
	if err != nil {
		t.Fatalf("ParseSnapshot: %v\n%s", err, data)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	again, err := got.JSON()
	if err != nil || string(again) != string(data) {
		t.Errorf("the same snapshot wrote different bytes: %v", err)
	}
	if !strings.HasSuffix(string(data), "\n") || !strings.Contains(string(data), `"version": 1`) {
		t.Errorf("unexpected bytes: %s", data)
	}
}

func TestSnapshotNamesAreTheFormat(t *testing.T) {
	t.Parallel()
	data, err := sample().JSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		`"version"`, `"dialect"`, `"server"`, `"schemas"`, `"tables"`, `"columns"`, `"nullable"`, `"default"`,
		`"constraints"`, `"kind"`, `"ref"`, `"indexes"`, `"descending"`, `"views"`, `"definition"`, `"skipped"`,
		`"precision"`, `"scale"`, `"raw"`,
	} {
		if !strings.Contains(string(data), key) {
			t.Errorf("the format has no key %s", key)
		}
	}
}

func TestParseSnapshotRefusesAVersionItDoesNotKnow(t *testing.T) {
	t.Parallel()
	_, err := dump.ParseSnapshot([]byte(`{"version": 99, "dialect": "x", "schemas": []}`))
	if !errors.Is(err, dump.ErrUnknownVersion) || !strings.Contains(err.Error(), "99") {
		t.Errorf("got %v, want ErrUnknownVersion naming 99", err)
	}
	if _, err := dump.ParseSnapshot([]byte(`{"dialect": "x"}`)); !errors.Is(err, dump.ErrUnknownVersion) {
		t.Errorf("a file with no version gave %v, want ErrUnknownVersion", err)
	}
	if _, err := dump.ParseSnapshot([]byte(`{"version": 1, "dialect": "x", "schemas": [], "surprise": 1}`)); err == nil {
		t.Error("a file with an unknown field was accepted")
	}
}
