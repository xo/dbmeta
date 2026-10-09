package test

import (
	"testing"

	"github.com/xo/dbmeta"
	ssfixture "github.com/xo/dbmeta/models/singlestore/fixture"
)

// TestSingleStoreNewFields reads the fields that the shared statements fill on
// SingleStore. Its access method is the storage type, and it has no source for
// create options, so the options are absent. See D205.
func TestSingleStoreNewFields(t *testing.T) {
	db := openSingleStore(t)
	m := setupSingleStore(t, db)
	schema := ssfixture.Everything.Schema

	checkTableFacts(t, db, m, schema, tableFacts{engine: "COLUMNSTORE", size: true})
	checkIndexUsing(t, db, m, schema, map[string]string{
		"book.PRIMARY":        "COLUMNSTORE HASH",
		"book.book_published": "COLUMNSTORE HASH",
		"region.PRIMARY":      "BTREE",
		"article.body":        "FULLTEXT",
		"author.PRIMARY":      "COLUMNSTORE HASH",
		"author.__SHARDKEY":   "SHARD",
	})
	checkEnforced(t, db, m, schema, true)

	var storage = map[string]string{}
	for v, err := range dbmeta.Tables.All(t.Context(), m, db, dbmeta.Args{Schema: schema}.Map()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		storage[v.Name] = v.AccessMethod.V
	}
	if storage["author"] != "COLUMNSTORE" || storage["region"] != "INMEMORY_ROWSTORE" {
		t.Errorf("storage types are %v", storage)
	}

	for v, err := range dbmeta.Functions.All(t.Context(), m, db, dbmeta.Args{Schema: schema}.Map()) {
		if err != nil {
			t.Fatalf("reading functions: %v", err)
		}
		if v.Prosrc != v.Source {
			t.Errorf("%s: prosrc is %+v and source is %+v", v.Name, v.Prosrc, v.Source)
		}
		if v.Leakproof {
			t.Errorf("%s: leakproof must be false", v.Name)
		}
	}
}
