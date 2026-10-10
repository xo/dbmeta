package test

import (
	"testing"

	"github.com/xo/dbmeta"
	imfixture "github.com/xo/dbmeta/models/impala/fixture"
)

// TestImpalaDescribeFields reads the fields that D209 added back as typed
// values. The walk reads them from the DESCRIBE FORMATTED it already runs, so
// the fixture's COMPUTE STATS on author gives it rows and a size.
func TestImpalaDescribeFields(t *testing.T) {
	db := openImpala(t)
	m := setupImpala(t, db)
	ctx := t.Context()
	args := dbmeta.Args{Schema: imfixture.Everything.Schema}.Map()

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
	author := tables["author"]
	if !author.Owner.Valid || author.Persistence.V != "permanent" || author.Rows.V != 3 || author.Size.V <= 0 || !author.AccessMethod.Valid {
		t.Errorf("author: expected an owner, permanent, 3 rows, a size and a format, got %+v", author)
	}
	if book := tables["book"]; book.Rows.Valid || book.Size.Valid && book.Size.V < 0 {
		t.Errorf("book: expected no row count before COMPUTE STATS, got %+v", book)
	}
	if recent := tables["recent"]; !recent.Owner.Valid || recent.Persistence.Valid || recent.AccessMethod.Valid || recent.Size.Valid || recent.Rows.Valid {
		t.Errorf("recent: expected an owner and nothing else, got %+v", recent)
	}
}
