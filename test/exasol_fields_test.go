package test

import (
	"strings"
	"testing"

	"github.com/xo/dbmeta"
)

// TestExasolDescribeFields reads the fields that D209 added back as typed
// values. Exasol has no access method, no unlogged table and no partition
// object, so those stay absent.
func TestExasolDescribeFields(t *testing.T) {
	db := openExasol(t)
	m := setupExasol(t, db)
	ctx := t.Context()

	tables := map[string]dbmeta.Table{}
	for v, err := range dbmeta.Tables.All(ctx, m, db, exArgs()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		tables[v.Name] = v
		if v.AccessMethod.Valid {
			t.Errorf("%s: expected no access method, got %+v", v.Name, v.AccessMethod)
		}
	}
	author := tables["AUTHOR"]
	if !author.Owner.Valid || author.Persistence.V != "permanent" || author.Rows.V != 3 || !author.Size.Valid || author.Size.V <= 0 {
		t.Errorf("author: expected an owner, permanent, 3 rows and a size, got %+v", author)
	}
	if author.Options.Valid {
		t.Errorf("author: expected no options, got %+v", author.Options)
	}
	if archive := tables["ARCHIVE"]; archive.Options.V != "distribute_by=ARCHIVE_ID, partition_by=FILED_YEAR" || archive.Rows.V != 0 {
		t.Errorf("archive: expected the keys and no rows, got %+v", archive)
	}
	if recent := tables["RECENT"]; !recent.Owner.Valid || recent.Persistence.Valid || recent.Size.Valid || recent.Rows.Valid || recent.Options.Valid {
		t.Errorf("recent: expected an owner and nothing else, got %+v", recent)
	}

	var indexes int
	for v, err := range dbmeta.Indexes.All(ctx, m, db, exArgs()) {
		if err != nil {
			t.Fatalf("reading indexes: %v", err)
		}
		indexes++
		if !v.Owner.Valid || !v.Size.Valid || v.Size.V <= 0 || v.Persistence.Valid || v.Valid.Valid || v.Using.Valid {
			t.Errorf("%s: unexpected fields %+v", v.Name, v)
		}
	}
	if indexes == 0 {
		t.Error("expected indexes")
	}

	enforced := map[string]bool{}
	for v, err := range dbmeta.Constraints.All(ctx, m, db, exArgs()) {
		if err != nil {
			t.Fatalf("reading constraints: %v", err)
		}
		if !v.Enforced.Valid {
			t.Errorf("%s: expected enforced to be present, got %+v", v.Name, v)
		}
		enforced[v.Name] = v.Enforced.V
	}
	if !enforced["BOOK_AUTHOR_FK"] || enforced["LEDGER_AUTHOR_FK"] {
		t.Errorf("expected book_author_fk enforced and ledger_author_fk not, got %v", enforced)
	}

	notNulls := map[string]string{}
	for v, err := range dbmeta.NotNulls.All(ctx, m, db, dbmeta.Args{Schema: "DBMETA_FIXTURE", Parent: "AUTHOR"}.Map()) {
		if err != nil {
			t.Fatalf("reading not nulls: %v", err)
		}
		if v.NoInherit || !v.Local || v.Inherited || !v.Validated || !strings.HasPrefix(v.Name, "SYS_") {
			t.Errorf("%s: unexpected %+v", v.Name, v)
		}
		notNulls[v.Column] = v.Name
	}
	if len(notNulls) != 2 || notNulls["AUTHOR_ID"] == "" || notNulls["NAME"] == "" {
		t.Errorf("expected not nulls on author_id and name, got %v", notNulls)
	}
}
