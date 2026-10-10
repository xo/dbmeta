package test

import (
	"database/sql"
	"os"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
	rsfixture "github.com/xo/dbmeta/models/redshift/fixture"
)

// TestRedshiftTablesNeedSelectOnTableInfo reads Tables as a user that was not
// granted SELECT on SVV_TABLE_INFO. The whole kind fails for it, which is the
// cost of D212, and the grant is what lets the same user read the rows.
func TestRedshiftTablesNeedSelectOnTableInfo(t *testing.T) {
	db := openRedshift(t)
	m := setupRedshift(t, db)
	schema := rsfixture.Everything.Schema
	dropRedshiftUser(t, db, "dbmeta_untabled", schema)
	exec(t, db, `CREATE USER dbmeta_untabled PASSWORD '`+parityPassword+`'`)
	t.Cleanup(func() { dropRedshiftUser(t, db, "dbmeta_untabled", schema) })
	user, err := sql.Open("pgx", replaceUser(t, os.Getenv("DBMETA_REDSHIFT"), "dbmeta_untabled", parityPassword))
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { user.Close() })

	args := dbmeta.Args{Schema: schema}.Map()
	var failed error
	for _, err := range dbmeta.Tables.All(t.Context(), m, user, args) {
		if err != nil {
			failed = err
			break
		}
	}
	if failed == nil || !strings.Contains(failed.Error(), "svv_table_info") {
		t.Errorf("expected tables to be refused for a user with no grant, got %v", failed)
	}

	grantTableInfo(t, db, "dbmeta_untabled")
	var n int
	for _, err := range dbmeta.Tables.All(t.Context(), m, user, args) {
		if err != nil {
			t.Fatalf("reading tables after the grant: %v", err)
		}
		n++
	}
	if n == 0 {
		t.Error("expected tables after the grant")
	}
}
