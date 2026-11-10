package dump

import "testing"

func TestSQLiteChecksAreFoundInTheTextOfATable(t *testing.T) {
	t.Parallel()
	def := `CREATE TABLE t (
  id integer primary key,
  qty integer not null check (qty > 0),
  kind text CONSTRAINT kind_ok CHECK (kind IN ('a', 'b)')),
  "check (x)" text, -- check (y)
  note text /* check (z) */,
  CONSTRAINT "two words" CHECK (qty < (10 * 2))
)`
	got := sqliteChecks(def)
	want := []Constraint{
		{Kind: "check", Definition: "CHECK (qty > 0)"},
		{Kind: "check", Name: "kind_ok", Definition: "CHECK (kind IN ('a', 'b)'))"},
		{Kind: "check", Name: "two words", Definition: "CHECK (qty < (10 * 2))"},
	}
	if len(got) != len(want) {
		t.Fatalf("found %d checks, and want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Name != want[i].Name || got[i].Kind != want[i].Kind || got[i].Definition != want[i].Definition {
			t.Errorf("check %d is %+v, and want %+v", i, got[i], want[i])
		}
	}
}
