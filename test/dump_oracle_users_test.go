package test

import (
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// TestOracleDumpLeavesOutUsersThatOwnNothing requires that a dump of every schema
// holds no schema without an object, such as OPS$ORACLE of the image.
func TestOracleDumpLeavesOutUsersThatOwnNothing(t *testing.T) {
	db := openOracle(t)
	m := metaFor(t, dbmeta.Oracle, db)
	snap, err := dump.Read(t.Context(), m, db, dump.Filter{})
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	var n int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM all_users WHERE username = 'OPS$ORACLE'").Scan(&n); err != nil {
		t.Fatal(err)
	}
	for _, s := range snap.Schemas {
		if s.Name == "OPS$ORACLE" {
			t.Errorf("the snapshot has the schema OPS$ORACLE, which owns nothing (the server has it: %d)", n)
		}
	}
}
