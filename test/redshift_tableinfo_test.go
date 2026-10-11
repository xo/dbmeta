package test

import (
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/xo/dbmeta"
	rsfixture "github.com/xo/dbmeta/models/redshift/fixture"
)

// TestRedshiftTableInfoIsReadOnRequest reads Tables as a user that was not
// granted SELECT on SVV_TABLE_INFO. The default statement does not name the
// view, so the user reads every table with no size, rows or options. Asking
// for them is refused with the error 42501, and the grant lets the same user
// read them (D232).
func TestRedshiftTableInfoIsReadOnRequest(t *testing.T) {
	db := openRedshift(t)
	m := setupRedshift(t, db)
	schema := rsfixture.Everything.Schema
	dropRedshiftUser(t, db, "dbmeta_untabled", schema)
	exec(t, db, `CREATE USER dbmeta_untabled PASSWORD '`+parityPassword+`'`)
	t.Cleanup(func() { dropRedshiftUser(t, db, "dbmeta_untabled", schema) })
	exec(t, db, `GRANT USAGE ON SCHEMA `+schema+` TO dbmeta_untabled`)
	exec(t, db, `GRANT SELECT ON ALL TABLES IN SCHEMA `+schema+` TO dbmeta_untabled`)
	user, err := sql.Open("pgx", replaceUser(t, os.Getenv("DBMETA_REDSHIFT"), "dbmeta_untabled", parityPassword))
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { user.Close() })

	args := dbmeta.Args{Schema: schema}
	var n int
	for v, err := range dbmeta.Tables.All(t.Context(), m, user, args.Map()) {
		if err != nil {
			t.Fatalf("reading tables with no grant: %v", err)
		}
		if v.Size.Valid || v.Rows.Valid || v.Options.Valid {
			t.Errorf("%s: expected no size, rows or options by default, got %+v", v.Name, v)
		}
		n++
	}
	if n == 0 {
		t.Error("expected tables for a user with no grant")
	}

	args.WithTableInfo = true
	var failed error
	for _, err := range dbmeta.Tables.All(t.Context(), m, user, args.Map()) {
		if err != nil {
			failed = err
			break
		}
	}
	var pgErr *pgconn.PgError
	if !errors.As(failed, &pgErr) || pgErr.Code != "42501" || !strings.Contains(failed.Error(), "svv_table_info") {
		t.Errorf("expected the error 42501 on svv_table_info with no grant, got %v", failed)
	}

	grantTableInfo(t, db, "dbmeta_untabled")
	var sized int
	for v, err := range dbmeta.Tables.All(t.Context(), m, user, args.Map()) {
		if err != nil {
			t.Fatalf("reading tables after the grant: %v", err)
		}
		if v.Size.Valid {
			sized++
		}
	}
	if sized == 0 {
		t.Error("expected a size for a table after the grant")
	}
}
