package test

import (
	"database/sql"
	"net/url"
	"strings"
	"testing"
)

// The principals, one maker per product.
//
// Every principal in a scene gets the same rights over the fixture schema, so
// that the only thing that varies between them is what kind of principal they
// are. A difference that came from one having fewer grants than another would
// say nothing about the model.

// oracleFixturePassword is what models/oracle/fixture creates its user with.
// The fixture user is already a local user owning every object, which is the
// principal this test wants, so it is reused rather than made again.
const oracleFixturePassword = "P4ssw0rd"

// makePostgresOwner gives the fixture schema and its objects to a new role.
func makePostgresOwner(t *testing.T, db *sql.DB, dsn, schema string) string {
	t.Helper()
	dropPostgresRole(t, db, "dbmeta_owner")
	exec(t, db, `CREATE ROLE dbmeta_owner LOGIN PASSWORD '`+parityPassword+`'`)
	t.Cleanup(func() { dropPostgresRole(t, db, "dbmeta_owner") })
	exec(t, db, `ALTER SCHEMA `+schema+` OWNER TO dbmeta_owner`)
	// PostgreSQL has no ALTER ALL TABLES, and ALTER TABLE is what changes the
	// owner of a view, a materialized view and a sequence as well.
	//
	// The sequence behind a serial or an identity column is left alone. It
	// belongs to the column rather than to the schema, and changing its owner
	// on its own is refused with "cannot change owner of sequence". It
	// follows the table, which this does change.
	exec(t, db, `DO $$DECLARE r record; BEGIN
	FOR r IN SELECT n.nspname, c.relname FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = '`+schema+`' AND c.relkind IN ('r', 'v', 'm', 'S', 'p')
		AND NOT (c.relkind = 'S' AND EXISTS (
			SELECT 1 FROM pg_depend d
			WHERE d.classid = 'pg_class'::regclass AND d.objid = c.oid
			AND d.deptype IN ('a', 'i')))
	LOOP
		EXECUTE format('ALTER TABLE %I.%I OWNER TO dbmeta_owner', r.nspname, r.relname);
	END LOOP;
END$$`)
	return replaceUser(t, dsn, "dbmeta_owner", parityPassword)
}

// makePostgresGrantee makes a role that can read the schema and owns nothing.
func makePostgresGrantee(t *testing.T, db *sql.DB, dsn, schema string) string {
	t.Helper()
	dropPostgresRole(t, db, "dbmeta_grantee")
	exec(t, db, `CREATE ROLE dbmeta_grantee LOGIN PASSWORD '`+parityPassword+`'`)
	t.Cleanup(func() { dropPostgresRole(t, db, "dbmeta_grantee") })
	exec(t, db, `GRANT USAGE ON SCHEMA `+schema+` TO dbmeta_grantee`)
	exec(t, db, `GRANT SELECT ON ALL TABLES IN SCHEMA `+schema+` TO dbmeta_grantee`)
	return replaceUser(t, dsn, "dbmeta_grantee", parityPassword)
}

// dropPostgresRole removes a role and whatever it owns.
//
// A role cannot be dropped while it owns anything or holds any grant, and
// REASSIGN OWNED followed by DROP OWNED is the only way to be sure. It runs
// before the role is made as well as after, because a run that failed part
// way leaves the role behind.
func dropPostgresRole(t *testing.T, db *sql.DB, role string) {
	t.Helper()
	cleanup(t, db, `DO $$BEGIN
	IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '`+role+`') THEN
		EXECUTE 'REASSIGN OWNED BY `+role+` TO ' || current_user;
		EXECUTE 'DROP OWNED BY `+role+`';
		EXECUTE 'DROP ROLE `+role+`';
	END IF;
END$$`)
}

// makeMySQLGrantee makes a user with every privilege on the fixture database
// and none anywhere else. MySQL has no containment, so this is the only
// principal below root that exists.
func makeMySQLGrantee(t *testing.T, db *sql.DB, dsn, schema string) string {
	t.Helper()
	const who = `'dbmeta_grantee'@'%'`
	cleanup(t, db, `DROP USER IF EXISTS `+who)
	exec(t, db, `CREATE USER `+who+` IDENTIFIED BY '`+parityPassword+`'`)
	t.Cleanup(func() { cleanup(t, db, `DROP USER IF EXISTS `+who) })
	exec(t, db, "GRANT ALL PRIVILEGES ON `"+schema+"`.* TO "+who)
	return mysqlUser(t, dsn, "dbmeta_grantee", parityPassword)
}

// makeSQLServerLogin makes a server login and maps it to a database user,
// which is the ordinary SQL Server model.
func makeSQLServerLogin(t *testing.T, db *sql.DB, dsn, schema string) string {
	t.Helper()
	dropSQLServerLogin(t, db, "dbmeta_login")
	exec(t, db, `CREATE LOGIN dbmeta_login WITH PASSWORD = '`+parityPassword+`'`)
	t.Cleanup(func() { dropSQLServerLogin(t, db, "dbmeta_login") })
	exec(t, db, `CREATE USER dbmeta_login FOR LOGIN dbmeta_login`)
	grantSQLServerSchema(t, db, "dbmeta_login", schema)
	return replaceUser(t, dsn, "dbmeta_login", parityPassword)
}

// makeSQLServerContained makes a user whose password lives in the database
// and which has no login at the server. It works only because the scene set
// CONTAINMENT to PARTIAL.
func makeSQLServerContained(t *testing.T, db *sql.DB, dsn, schema string) string {
	t.Helper()
	cleanup(t, db, `IF DATABASE_PRINCIPAL_ID('dbmeta_cuser') IS NOT NULL DROP USER dbmeta_cuser`)
	exec(t, db, `CREATE USER dbmeta_cuser WITH PASSWORD = '`+parityPassword+`'`)
	t.Cleanup(func() {
		cleanup(t, db, `IF DATABASE_PRINCIPAL_ID('dbmeta_cuser') IS NOT NULL DROP USER dbmeta_cuser`)
	})
	grantSQLServerSchema(t, db, "dbmeta_cuser", schema)
	return replaceUser(t, dsn, "dbmeta_cuser", parityPassword)
}

// grantSQLServerSchema gives a principal the same rights over the fixture
// schema that the other principals have. CONTROL is ownership without
// changing the owner, which would make the fixture teardown fail.
func grantSQLServerSchema(t *testing.T, db *sql.DB, who, schema string) {
	t.Helper()
	exec(t, db, `GRANT CONTROL ON SCHEMA::`+schema+` TO `+who)
	exec(t, db, `GRANT VIEW DEFINITION ON SCHEMA::`+schema+` TO `+who)
}

// dropSQLServerLogin removes the database user and then the server login,
// which is the order SQL Server requires.
func dropSQLServerLogin(t *testing.T, db *sql.DB, who string) {
	t.Helper()
	cleanup(t, db, `IF DATABASE_PRINCIPAL_ID('`+who+`') IS NOT NULL DROP USER `+who)
	cleanup(t, db, `IF SUSER_ID('`+who+`') IS NOT NULL DROP LOGIN `+who)
}

// prepareSQLServerContained makes a database that accepts a contained user.
//
// Containment cannot be set on master, which is where the other SQL Server
// tests build the fixture, so the contained principal needs a database of its
// own and the administrator has to be compared inside the same one.
func prepareSQLServerContained(t *testing.T, admin *sql.DB, adminDSN string) string {
	t.Helper()
	const name = "dbmeta_contained"
	exec(t, admin, `EXEC sp_configure 'contained database authentication', 1`)
	exec(t, admin, `RECONFIGURE`)
	dropSQLServerDatabase(t, admin, name)
	exec(t, admin, `CREATE DATABASE `+name)
	t.Cleanup(func() { dropSQLServerDatabase(t, admin, name) })
	exec(t, admin, `ALTER DATABASE `+name+` SET CONTAINMENT = PARTIAL`)
	return replaceDatabase(t, adminDSN, name)
}

// dropSQLServerDatabase removes a database, disconnecting whoever is in it.
func dropSQLServerDatabase(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	cleanup(t, db, `IF DB_ID('`+name+`') IS NOT NULL BEGIN`+
		` ALTER DATABASE `+name+` SET SINGLE_USER WITH ROLLBACK IMMEDIATE;`+
		` DROP DATABASE `+name+`; END`)
}

// makeOracleLocal returns the fixture user, which is already a local user in
// the pluggable database and already owns every object the fixture built.
// Nothing has to be created, so the connection is not used.
func makeOracleLocal(t *testing.T, _ *sql.DB, dsn, schema string) string {
	t.Helper()
	return replaceUser(t, dsn, strings.ToLower(schema), oracleFixturePassword)
}

// mysqlUser rewrites the credentials of a MySQL DSN.
//
// The go-sql-driver DSN is not a URL, so it cannot be parsed like the others.
// Everything before the first @ is the credentials and the rest is untouched.
func mysqlUser(t *testing.T, dsn, user, password string) string {
	t.Helper()
	at := strings.IndexByte(dsn, '@')
	if at < 0 {
		t.Fatalf("no credentials in %s", dsn)
	}
	return user + ":" + password + dsn[at:]
}

// replaceDatabase points a SQL Server DSN at another database.
func replaceDatabase(t *testing.T, dsn, name string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	q := u.Query()
	q.Set("database", name)
	u.RawQuery = q.Encode()
	return u.String()
}

// makeCassandraGrantee makes a role with every permission on the fixture
// keyspace and none anywhere else.
//
// It needs the image this repository builds. The published one runs
// AllowAllAuthenticator, where CREATE ROLE is accepted and means nothing and
// there is no second principal to be.
func makeCassandraGrantee(t *testing.T, db *sql.DB, dsn, schema string) string {
	t.Helper()
	cleanup(t, db, `DROP ROLE IF EXISTS dbmeta_grantee`)
	exec(t, db, `CREATE ROLE dbmeta_grantee WITH PASSWORD = '`+parityPassword+
		`' AND LOGIN = true`)
	t.Cleanup(func() { cleanup(t, db, `DROP ROLE IF EXISTS dbmeta_grantee`) })
	exec(t, db, `GRANT ALL PERMISSIONS ON KEYSPACE `+schema+` TO dbmeta_grantee`)
	return cqlUser(t, dsn, "dbmeta_grantee", parityPassword)
}

// cqlUser rewrites the credentials of a go-cql-driver DSN.
//
// The DSN is a host list and then query options, which is neither a URL nor
// the MySQL shape, so it gets its own helper. The user and the password are
// options rather than a userinfo part.
func cqlUser(t *testing.T, dsn, user, password string) string {
	t.Helper()
	host, rawQuery, _ := strings.Cut(dsn, "?")
	q, err := url.ParseQuery(rawQuery)
	if err != nil {
		t.Fatalf("parsing the options of %s: %v", dsn, err)
	}
	q.Set("username", user)
	q.Set("password", password)
	return host + "?" + q.Encode()
}
