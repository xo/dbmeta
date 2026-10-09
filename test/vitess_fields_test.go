package test

import (
	"testing"

	"github.com/xo/dbmeta"
	vtfixture "github.com/xo/dbmeta/models/vitess/fixture"
)

// TestVitessNewFields reads the fields that the shared statements fill on
// Vitess. vtgate passes the statement to the MySQL of one tablet, so the
// answers are MySQL's, and the schema is the keyspace. See D205.
func TestVitessNewFields(t *testing.T) {
	db := openVitess(t)
	m := setupVitess(t, db)
	schema := vtfixture.Everything.Schema

	checkTableFacts(t, db, m, schema, tableFacts{engine: "InnoDB", options: true, size: true})
	checkIndexUsing(t, db, m, schema, map[string]string{
		"book.PRIMARY":         "BTREE",
		"book.book_published":  "BTREE",
		"lookup.lookup_id":     "HASH",
		"article.article_body": "FULLTEXT",
	})
	checkEnforced(t, db, m, schema, false)
	checkPartitions(t, db, m, schema, true, true)
	for v, err := range dbmeta.Partitions.All(t.Context(), m, db, dbmeta.Args{Schema: schema}.Map()) {
		if err != nil {
			t.Fatalf("reading partitions: %v", err)
		}
		if v.Schema != schema || v.PartitionSchema != schema {
			t.Errorf("%s.%s: the schema is %q and the partition schema %q, want the keyspace", v.Table, v.Partition, v.Schema, v.PartitionSchema)
		}
	}
}
