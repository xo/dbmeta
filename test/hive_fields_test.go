package test

import (
	"testing"

	"github.com/xo/dbmeta"
	hvfixture "github.com/xo/dbmeta/models/hive/fixture"
)

// TestHiveDescribeFields reads the fields that D209 added back as typed
// values. Hive keeps its statistics as table parameters, so the rows and the
// size are there for a table that was written to and absent for a view.
func TestHiveDescribeFields(t *testing.T) {
	db := openHive(t)
	m := setupHive(t, db)
	ctx := t.Context()
	args := dbmeta.Args{Schema: hvfixture.Everything.Schema}.Map()

	tables := map[string]dbmeta.Table{}
	for v, err := range dbmeta.Tables.All(ctx, m, db, args) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		tables[v.Name] = v
		if v.Options.Valid {
			t.Errorf("%s: expected no options, got %+v", v.Name, v.Options)
		}
	}
	ledger := tables["ledger"]
	if ledger.Owner.V == "" || ledger.Persistence.V != "permanent" || ledger.Rows.V != 2 || ledger.Size.V <= 0 ||
		ledger.AccessMethod.V != "org.apache.hadoop.hive.ql.io.orc.OrcInputFormat" {
		t.Errorf("ledger: expected an owner, permanent, 2 rows, a size and the ORC format, got %+v", ledger)
	}
	if recent := tables["recent"]; !recent.Owner.Valid || recent.Persistence.Valid || recent.AccessMethod.Valid || recent.Size.Valid || recent.Rows.Valid {
		t.Errorf("recent: expected an owner and nothing else, got %+v", recent)
	}

	partitions := map[string]bool{}
	for v, err := range dbmeta.Partitions.All(ctx, m, db, dbmeta.Args{Schema: hvfixture.Everything.Schema, Parent: "archive"}.Map()) {
		if err != nil {
			t.Fatalf("reading partitions: %v", err)
		}
		if v.Table != "archive" || v.Type != "partition" || v.Partitioned || v.Bound.V != v.Partition {
			t.Errorf("unexpected partition %+v", v)
		}
		partitions[v.Partition] = true
	}
	if len(partitions) != 2 || !partitions["year=2025"] || !partitions["year=2026"] {
		t.Errorf("expected the partitions year=2025 and year=2026, got %v", partitions)
	}

	enforced := map[string]bool{}
	for v, err := range dbmeta.Constraints.All(ctx, m, db, args) {
		if err != nil {
			t.Fatalf("reading constraints: %v", err)
		}
		if !v.Enforced.Valid {
			t.Errorf("%s: expected enforced to be present", v.Name)
		}
		enforced[v.Table+"."+v.Name] = v.Enforced.V
	}
	if !enforced["vault.vault_ck"] || enforced["book.book_title_ck"] {
		t.Errorf("expected vault_ck enforced and book_title_ck not, got %v", enforced)
	}

	var vault, other int
	for v, err := range dbmeta.NotNulls.All(ctx, m, db, args) {
		if err != nil {
			t.Fatalf("reading not nulls: %v", err)
		}
		if v.NoInherit || !v.Local || v.Inherited {
			t.Errorf("%s: unexpected %+v", v.Name, v)
		}
		if v.Table == "vault" {
			vault++
			if v.Column != "vault_id" || v.Validated {
				t.Errorf("vault: expected vault_id enabled and not validated, got %+v", v)
			}
		} else {
			other++
		}
	}
	if vault != 1 || other == 0 {
		t.Errorf("expected one not null on vault and some elsewhere, got %d and %d", vault, other)
	}
}
