package test

import (
	"testing"

	tdfixture "github.com/xo/dbmeta/models/tidb/fixture"
)

// TestTiDBNewFields reads the fields that the shared statements fill on TiDB.
// TiDB ignores a hash index, a storage engine and a row format, so what it
// reports is what it keeps. A global temporary table is a base table in the
// catalog, so TiDB has no source for the persistence. See D205.
func TestTiDBNewFields(t *testing.T) {
	db := openTiDB(t)
	m := setupTiDB(t, db)
	schema := tdfixture.Everything.Schema

	checkTableFacts(t, db, m, schema, tableFacts{engine: "InnoDB", size: true, unknownPersistence: true})
	checkIndexUsing(t, db, m, schema, map[string]string{
		"book.PRIMARY":        "BTREE",
		"book.book_published": "BTREE",
		"lookup.lookup_id":    "BTREE",
	})
	checkEnforced(t, db, m, schema, true)
	checkPartitions(t, db, m, schema, true, false)
}
