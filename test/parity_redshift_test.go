package test

import (
	"context"
	"database/sql"
	"testing"

	rsfixture "github.com/xo/dbmeta/models/redshift/fixture"
)

// redshiftFixtureTables is every relation the fixture builds, which ALTER TABLE
// can give to another user. Redshift has no DO block and no REASSIGN OWNED,
// so the owner is handed back by name.
var redshiftFixtureTables = []string{"author", "book", "region", "shipment", "recent"}

// dropRedshiftUser removes a user. A user cannot be dropped while it owns an
// object or holds a grant, and Redshift has no DROP OWNED, so the grants are
// revoked and the ownership is handed back to the administrator first. It
// runs before the user is made as well as after, because a run that failed
// part way leaves the user behind.
func dropRedshiftUser(t *testing.T, db *sql.DB, user, schema string) {
	t.Helper()
	// A cleanup runs after t.Context is canceled.
	ctx := context.WithoutCancel(t.Context())
	var exists bool
	err := db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_user WHERE usename = $1)`, user).Scan(&exists)
	if err != nil {
		t.Logf("looking for the user %s: %v", user, err)
		return
	}
	if !exists {
		return
	}
	var admin string
	if err := db.QueryRowContext(ctx, `SELECT current_user`).Scan(&admin); err != nil {
		t.Logf("reading the administrator: %v", err)
		return
	}
	for _, table := range redshiftFixtureTables {
		cleanup(t, db, `ALTER TABLE `+schema+`.`+table+` OWNER TO `+admin)
	}
	cleanup(t, db, `ALTER SCHEMA `+schema+` OWNER TO `+admin)
	cleanup(t, db, `REVOKE SELECT ON svv_table_info FROM `+user)
	cleanup(t, db, `REVOKE ALL ON ALL TABLES IN SCHEMA `+schema+` FROM `+user)
	cleanup(t, db, `REVOKE ALL ON SCHEMA `+schema+` FROM `+user)
	if hasRedshiftSpectrum(t, db) {
		cleanup(t, db, `REVOKE ALL ON SCHEMA `+rsfixture.Everything.External+` FROM `+user)
	}
	cleanup(t, db, `DROP USER IF EXISTS `+user)
}

// hasRedshiftSpectrum reports whether the fixture made its external schema.
func hasRedshiftSpectrum(t *testing.T, db *sql.DB) bool {
	t.Helper()
	var found bool
	err := db.QueryRowContext(context.WithoutCancel(t.Context()),
		`SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = $1)`,
		rsfixture.Everything.External).Scan(&found)
	if err != nil {
		t.Fatalf("looking for the external schema: %v", err)
	}
	return found
}

// makeRedshiftOwner gives the fixture schema and its tables to a new user.
func makeRedshiftOwner(t *testing.T, db *sql.DB, dsn, schema string) string {
	t.Helper()
	dropRedshiftUser(t, db, "dbmeta_owner", schema)
	exec(t, db, `CREATE USER dbmeta_owner PASSWORD '`+parityPassword+`'`)
	t.Cleanup(func() { dropRedshiftUser(t, db, "dbmeta_owner", schema) })
	grantTableInfo(t, db, "dbmeta_owner")
	exec(t, db, `ALTER SCHEMA `+schema+` OWNER TO dbmeta_owner`)
	for _, table := range redshiftFixtureTables {
		exec(t, db, `ALTER TABLE `+schema+`.`+table+` OWNER TO dbmeta_owner`)
	}
	return replaceUser(t, dsn, "dbmeta_owner", parityPassword)
}

// makeRedshiftGrantee makes a user that can read the schema and owns nothing.
func makeRedshiftGrantee(t *testing.T, db *sql.DB, dsn, schema string) string {
	t.Helper()
	dropRedshiftUser(t, db, "dbmeta_grantee", schema)
	exec(t, db, `CREATE USER dbmeta_grantee PASSWORD '`+parityPassword+`'`)
	t.Cleanup(func() { dropRedshiftUser(t, db, "dbmeta_grantee", schema) })
	grantTableInfo(t, db, "dbmeta_grantee")
	exec(t, db, `GRANT USAGE ON SCHEMA `+schema+` TO dbmeta_grantee`)
	exec(t, db, `GRANT SELECT ON ALL TABLES IN SCHEMA `+schema+` TO dbmeta_grantee`)
	// The external schema exists only when the namespace has a default IAM
	// role (D221). A user reads its tables once it has USAGE on the schema.
	if hasRedshiftSpectrum(t, db) {
		exec(t, db, `GRANT USAGE ON SCHEMA `+rsfixture.Everything.External+` TO dbmeta_grantee`)
	}
	return replaceUser(t, dsn, "dbmeta_grantee", parityPassword)
}

// makeRedshiftStranger makes a user with no grant on the fixture at all.
// Redshift hides nothing in pg_catalog from it. The SVV views show it only the
// rows that concern it, which is what the golden file records (D204).
func makeRedshiftStranger(t *testing.T, db *sql.DB, dsn, schema string) string {
	t.Helper()
	dropRedshiftUser(t, db, "dbmeta_stranger", schema)
	exec(t, db, `CREATE USER dbmeta_stranger PASSWORD '`+parityPassword+`'`)
	t.Cleanup(func() { dropRedshiftUser(t, db, "dbmeta_stranger", schema) })
	grantTableInfo(t, db, "dbmeta_stranger")
	return replaceUser(t, dsn, "dbmeta_stranger", parityPassword)
}

// grantTableInfo lets a user read SVV_TABLE_INFO, which Redshift refuses to
// every user who is not a superuser. The Tables kind reads it for the size, the
// rows and the options, so a user without the grant gets an error from the
// whole kind (D212).
func grantTableInfo(t *testing.T, db *sql.DB, user string) {
	t.Helper()
	exec(t, db, `GRANT SELECT ON svv_table_info TO `+user)
}
