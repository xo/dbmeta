package test

import (
	"strings"
	"testing"

	"github.com/xo/dbmeta"
	hafixture "github.com/xo/dbmeta/models/hana/fixture"
)

// TestHANADescribeFields reads the fields that D209 added back as typed
// values. The size and the row count stay absent, because the monitoring view
// that has them cannot be filtered at a cost that follows the rows returned.
func TestHANADescribeFields(t *testing.T) {
	db := openHANA(t)
	m := setupHANA(t, db)
	ctx := t.Context()

	tables := map[string]dbmeta.Table{}
	for v, err := range dbmeta.Tables.All(ctx, m, db, haArgs()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		tables[v.Name] = v
	}
	for _, v := range tables {
		if v.Size.Valid || v.Rows.Valid {
			t.Errorf("%s: expected no size and no rows, got %+v", v.Name, v)
		}
	}
	author := tables["AUTHOR"]
	if author.Owner.V != "SYSTEM" || author.Persistence.V != "permanent" || author.AccessMethod.V != "column" || author.Options.Valid {
		t.Errorf("author: expected SYSTEM, permanent, column and no options, got %+v", author)
	}
	if ledger := tables["LEDGER"]; ledger.AccessMethod.V != "row" {
		t.Errorf("ledger: expected the row store, got %+v", ledger)
	}
	if scratch := tables["SCRATCH"]; scratch.Persistence.V != "temporary" || scratch.Options.V != "on_commit=delete" {
		t.Errorf("scratch: expected temporary with on_commit, got %+v", scratch)
	}
	if cache := tables["CACHE"]; cache.Persistence.V != "unlogged" {
		t.Errorf("cache: expected unlogged, got %+v", cache)
	}
	if queue := tables["QUEUE"]; queue.Options.V != "auto_merge=off, load_unit=page" {
		t.Errorf("queue: expected auto_merge=off and load_unit=page, got %+v", queue)
	}
	if recent := tables["RECENT"]; recent.Owner.V != "SYSTEM" || recent.Persistence.Valid || recent.AccessMethod.Valid || recent.Size.Valid || recent.Rows.Valid || recent.Options.Valid {
		t.Errorf("recent: expected an owner and nothing else, got %+v", recent)
	}

	for v, err := range dbmeta.Columns.All(ctx, m, db, dbmeta.Args{Schema: hafixture.Everything.Schema, Parent: "AUTHOR"}.Map()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		if v.Compression.V != "default" || v.Storage.Valid || v.StatsTarget.Valid {
			t.Errorf("author.%s: expected the default compression only, got %+v", v.Name, v)
		}
	}

	indexes := map[string]dbmeta.Index{}
	for v, err := range dbmeta.Indexes.All(ctx, m, db, haArgs()) {
		if err != nil {
			t.Fatalf("reading indexes: %v", err)
		}
		indexes[v.Name] = v
		if v.Owner.V != "SYSTEM" || v.Using.V != strings.ToLower(v.Type) {
			t.Errorf("%s: expected the owner and the kind, got %+v", v.Name, v)
		}
		if v.Valid.Valid || v.Clustered.Valid || v.Predicate.Valid || v.Size.Valid {
			t.Errorf("%s: unexpected fields %+v", v.Name, v)
		}
	}
	if uq := indexes["BOOK_TITLE_UQ"]; uq.ConstraintType.V != "u" {
		t.Errorf("book_title_uq: expected u, got %+v", uq)
	}
	if free := indexes["BOOK_PUBLISHED"]; free.ConstraintType.Valid {
		t.Errorf("book_published: expected a free index, got %+v", free)
	}
	var primary int
	for _, v := range indexes {
		if v.Primary {
			primary++
			if v.ConstraintType.V != "p" {
				t.Errorf("%s: expected p, got %+v", v.Name, v)
			}
		}
	}
	if primary == 0 {
		t.Error("expected primary key indexes")
	}

	enforced := map[string]sqlBool{}
	for v, err := range dbmeta.Constraints.All(ctx, m, db, haArgs()) {
		if err != nil {
			t.Fatalf("reading constraints: %v", err)
		}
		enforced[v.Name] = sqlBool{v.Enforced.Valid, v.Enforced.V}
	}
	if got := enforced["BOOK_AUTHOR_FK"]; !got.valid || !got.value {
		t.Errorf("book_author_fk: expected enforced, got %+v", got)
	}
	if got := enforced["PENDING_AUTHOR_FK"]; !got.valid || got.value {
		t.Errorf("pending_author_fk: expected not enforced, got %+v", got)
	}
	if got := enforced["BOOK_TITLE_CK"]; got.valid {
		t.Errorf("book_title_ck: expected enforced to be absent, got %+v", got)
	}

	bounds := map[string]string{}
	for v, err := range dbmeta.Partitions.All(ctx, m, db, dbmeta.Args{Schema: hafixture.Everything.Schema, Parent: "AUDIT"}.Map()) {
		if err != nil {
			t.Fatalf("reading partitions: %v", err)
		}
		if v.Table != "AUDIT" || v.Type != "partition" || v.Partitioned {
			t.Errorf("unexpected partition %+v", v)
		}
		bounds[v.Partition] = v.Bound.V
	}
	want := map[string]string{"1": "0 <= VALUES < 100", "2": "100 <= VALUES < 200", "3": "OTHERS"}
	for k, v := range want {
		if bounds[k] != v {
			t.Errorf("audit partition %s: expected %q, got %q", k, v, bounds[k])
		}
	}
	hashed := 0
	for v, err := range dbmeta.Partitions.All(ctx, m, db, dbmeta.Args{Schema: hafixture.Everything.Schema, Parent: "ARCHIVE"}.Map()) {
		if err != nil {
			t.Fatalf("reading partitions: %v", err)
		}
		if !strings.HasPrefix(v.Bound.V, "HASH ") || !strings.HasSuffix(v.Bound.V, " OF 4") {
			t.Errorf("archive: expected a hash bound, got %+v", v)
		}
		hashed++
	}
	if hashed != 4 {
		t.Errorf("expected 4 hash partitions of archive, got %d", hashed)
	}
}

// sqlBool is a nullable boolean that prints in a test message.
type sqlBool struct{ valid, value bool }
