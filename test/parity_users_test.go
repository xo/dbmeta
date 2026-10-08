package test

import (
	"context"
	"database/sql"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/container"
	arfixture "github.com/xo/dbmeta/models/arangodb/fixture"
)

// The principals, one maker per product.
//
// Every principal in a scene gets the same rights over the fixture schema, so
// that the only thing that varies between them is what kind of principal they
// are. A difference that comes from one having fewer grants than another says
// nothing about the model.

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

// makeTiDBGrantee makes the MySQL grantee, and connects it to the fixture
// schema, because the database the TiDB DSN names is one it cannot use.
func makeTiDBGrantee(t *testing.T, db *sql.DB, dsn, schema string) string {
	t.Helper()
	return mysqlAt(t, makeMySQLGrantee(t, db, dsn, schema), "dbmeta_grantee", parityPassword, schema)
}

// makeTiDBReader makes a user that can only read the fixture schema.
func makeTiDBReader(t *testing.T, db *sql.DB, dsn, schema string) string {
	t.Helper()
	const who = `'dbmeta_parity_reader'@'%'`
	cleanup(t, db, `DROP USER IF EXISTS `+who)
	exec(t, db, `CREATE USER `+who+` IDENTIFIED BY '`+parityPassword+`'`)
	t.Cleanup(func() { cleanup(t, db, `DROP USER IF EXISTS `+who) })
	// The star is the grant's object list, every table in the schema, and
	// not a select list. unqueryvet reads the two the same way.
	//nolint:unqueryvet // GRANT ... ON db.* is the GRANT syntax
	exec(t, db, "GRANT SELECT ON `"+schema+"`.* TO "+who)
	return mysqlAt(t, dsn, "dbmeta_parity_reader", parityPassword, schema)
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
// changing the owner. A change of owner makes the fixture teardown fail.
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

// mysqlAt returns a go-sql-driver DSN for user on the database given, which
// is empty for none. It parses the DSN with the driver's own parser rather
// than by hand.
func mysqlAt(t *testing.T, dsn, user, password, database string) string {
	t.Helper()
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	cfg.User, cfg.Passwd, cfg.DBName = user, password, database
	return cfg.FormatDSN()
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
	return replaceUser(t, dsn, "dbmeta_grantee", parityPassword)
}

// makeCassandraUser connects as the ordinary user the dbrun setup makes.
//
// The setup makes the role on every start. It can log in, it is no superuser,
// and it holds no permission. So becoming it is a change to the credentials of
// the URL (D195).
func makeCassandraUser(t *testing.T, _ *sql.DB, dsn, _ string) string {
	t.Helper()
	return replaceUser(t, dsn, container.CassandraUser, container.Password)
}

// makeClickHouseGrantee makes a user with every privilege on the fixture
// database and none anywhere else.
//
// It needs the server started with CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT, which
// container/clickhouse.go sets. Without it CREATE USER is refused and there is
// no second principal to be.
func makeClickHouseGrantee(t *testing.T, db *sql.DB, dsn, schema string) string {
	t.Helper()
	cleanup(t, db, `DROP USER IF EXISTS dbmeta_parity`)
	exec(t, db, `CREATE USER dbmeta_parity IDENTIFIED BY '`+parityPassword+`'`)
	t.Cleanup(func() { cleanup(t, db, `DROP USER IF EXISTS dbmeta_parity`) })
	// The star here is the grant's object list, every table in the database,
	// and not a select list. unqueryvet reads the two the same way.
	//nolint:unqueryvet // GRANT ... ON db.* is the CREATE GRANT syntax
	exec(t, db, `GRANT SELECT ON `+schema+`.* TO dbmeta_parity`)
	return replaceUser(t, dsn, "dbmeta_parity", parityPassword)
}

// makeCrateDBHolder returns a principal maker for a user holding privileges
// on the fixture schema and nothing else. CrateDB grants on a schema cover
// every table in it, including the ones made later.
func makeCrateDBHolder(privileges string) func(*testing.T, *sql.DB, string, string) string {
	return func(t *testing.T, db *sql.DB, dsn, schema string) string {
		t.Helper()
		cleanup(t, db, `DROP USER IF EXISTS dbmeta_parity`)
		exec(t, db, `CREATE USER dbmeta_parity WITH (password = '`+parityPassword+`')`)
		t.Cleanup(func() { cleanup(t, db, `DROP USER IF EXISTS dbmeta_parity`) })
		exec(t, db, `GRANT `+privileges+` ON SCHEMA `+schema+` TO dbmeta_parity`)
		return replaceUser(t, dsn, "dbmeta_parity", parityPassword)
	}
}

// makeQuestDBReader connects as the user of the PostgreSQL interface that
// can only read, which the dbrun entry turns on. QuestDB has no statement
// that creates a user, so becoming it is a change to the credentials of the
// DSN.
func makeQuestDBReader(t *testing.T, _ *sql.DB, dsn, _ string) string {
	t.Helper()
	return replaceUser(t, dsn, container.QuestDBUser, container.Password)
}

// makeRqliteUser connects as the user the dbrun entry declares, who can
// query and execute and nothing else. rqlite has no statement that makes a
// user, because it reads its users from a file, so becoming the user is a
// change to the credentials of the URL.
func makeRqliteUser(t *testing.T, _ *sql.DB, dsn, _ string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	u.User = url.UserPassword(container.RqliteUser, container.Password)
	return u.String()
}

// makeLibSQLUser connects as the user the dbrun entry declares, whose token
// can read and not write. sqld has no statement that makes a user, because it
// checks a signed token, so becoming the user is a change to the credentials
// of the URL (D153).
func makeLibSQLUser(t *testing.T, _ *sql.DB, dsn, _ string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	u.User = url.UserPassword(container.LibSQLUser, container.LibSQLUserToken)
	return u.String()
}

// makeCouchbaseUser connects as the ordinary user the dbrun setup makes.
//
// SQL++ has no statement that creates a user, and the setup already makes
// one with the roles a grantee has: select, insert, update and delete on the
// bucket, and the role that reads the system catalog. So becoming it is a
// change to the credentials of the URL.
func makeCouchbaseUser(t *testing.T, _ *sql.DB, dsn, _ string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	u.User = url.UserPassword(container.CouchbaseUser, container.Password)
	return u.String()
}

// makeNeo4jUser connects as the ordinary user the dbrun setup makes.
//
// The setup makes the user with the role publisher on every start, so
// becoming it is a change to the credentials of the URL. The fixture gives it
// the role dbmeta_reader as well.
func makeNeo4jUser(t *testing.T, _ *sql.DB, dsn, _ string) string {
	t.Helper()
	return replaceUser(t, dsn, container.Neo4jUser, container.Password)
}

// makeYDBUser connects as the ordinary user the dbrun setup makes.
//
// The setup makes dbmetauser and lets it read and describe the directory
// dbmeta, which holds the fixture. YDB has no containment: a user belongs to
// the cluster and a directory is only a grant scope. So becoming the user is
// a change to the credentials of the URL.
func makeYDBUser(t *testing.T, _ *sql.DB, dsn, _ string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	u.User = url.UserPassword(container.YDBUser, container.Password)
	return u.String()
}

// makeArangoDBUser connects as the ordinary user the dbrun setup makes, who
// has read and write on the database dbmeta and nothing on _system. So
// becoming it is a change to the credentials of the URL.
func makeArangoDBUser(t *testing.T, _ *sql.DB, dsn, _ string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	u.User = url.UserPassword(container.ArangoDBUser, container.Password)
	return u.String()
}

// arangoReader is the user makeArangoDBReader makes.
const arangoReader = "dbmeta_reader"

// makeArangoDBReader makes a user who can read the database dbmeta and has
// no access to the collection note, which has a schema rule. A user belongs
// to the server, and a database or a collection is only a grant scope, so
// this is a grantee with less than the ordinary user. Only the HTTP API of
// _system makes a user, and AQL makes none.
func makeArangoDBReader(t *testing.T, _ *sql.DB, dsn, _ string) string {
	t.Helper()
	ctx := t.Context()
	user := `{"user": "` + arangoReader + `", "passwd": "` + container.Password + `", "active": true}`
	for _, r := range []*arfixture.Request{
		{Method: http.MethodDelete, Path: "/_api/user/" + arangoReader},
		{Method: http.MethodPost, Path: "/_api/user", Body: user},
		{Method: http.MethodPut, Path: "/_api/user/" + arangoReader + "/database/dbmeta", Body: `{"grant": "ro"}`},
		{Method: http.MethodPut, Path: "/_api/user/" + arangoReader + "/database/dbmeta/note", Body: `{"grant": "none"}`},
	} {
		if err := arangoAPI(ctx, dsn, "_system", r); err != nil && r.Method != http.MethodDelete {
			t.Fatalf("making %s: %v", arangoReader, err)
		}
	}
	t.Cleanup(func() {
		//nolint:errcheck // removing the user is best effort
		arangoAPI(context.WithoutCancel(ctx), dsn, "_system",
			&arfixture.Request{Method: http.MethodDelete, Path: "/_api/user/" + arangoReader})
	})
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	u.User = url.UserPassword(arangoReader, container.Password)
	return u.String()
}

// makeSurrealDBUser connects as the ordinary user the dbrun setup makes, an
// EDITOR on the database dbmeta. A user defined on a database names its level
// in the URL, because the driver sends the headers that sign it in there
// (dbimp D51).
func makeSurrealDBUser(t *testing.T, _ *sql.DB, dsn, _ string) string {
	t.Helper()
	return surrealDBPrincipal(t, dsn, container.SurrealDBUser, "database")
}

// makeSurrealDBViewer makes a VIEWER on the database dbmeta, which reads
// everything there and changes nothing.
func makeSurrealDBViewer(t *testing.T, db *sql.DB, dsn, _ string) string {
	t.Helper()
	makeSurrealDBSystemUser(t, db, "dbmeta_viewer", "DATABASE", "VIEWER")
	return surrealDBPrincipal(t, dsn, "dbmeta_viewer", "database")
}

// makeSurrealDBNamespaceUser makes an EDITOR on the namespace dbmeta, which
// is above the database and below the root.
func makeSurrealDBNamespaceUser(t *testing.T, db *sql.DB, dsn, _ string) string {
	t.Helper()
	makeSurrealDBSystemUser(t, db, "dbmeta_ns_editor", "NAMESPACE", "EDITOR")
	return surrealDBPrincipal(t, dsn, "dbmeta_ns_editor", "namespace")
}

// makeSurrealDBSystemUser defines a system user with one role on a level,
// and removes it when the test ends.
func makeSurrealDBSystemUser(t *testing.T, db *sql.DB, name, level, role string) {
	t.Helper()
	ctx := t.Context()
	exec(t, db, "DEFINE USER OVERWRITE "+name+" ON "+level+" PASSWORD '"+container.Password+"' ROLES "+role)
	t.Cleanup(func() {
		//nolint:errcheck // removing the user is best effort
		db.ExecContext(context.WithoutCancel(ctx), "REMOVE USER IF EXISTS "+name+" ON "+level)
	})
}

// surrealDBPrincipal is dsn with the credentials of user, signed in at level.
func surrealDBPrincipal(t *testing.T, dsn, user, level string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	u.User = url.UserPassword(user, container.Password)
	q := u.Query()
	q.Set("auth", level)
	u.RawQuery = q.Encode()
	return u.String()
}

// makeTrinoPrincipal names a different principal on the connection.
//
// Trino has no users to create. A client states who it is on every request
// and the server takes it, because the image configures no authenticator, so
// becoming somebody else is a change to the DSN and nothing more. A password
// cannot go with it: Trino refuses username and password authentication over
// plain HTTP.
//
// With no access control plugin the server then allows that principal
// everything, so this target is expected to find no difference at all. That
// is the measurement rather than a gap in it, and D61 wants it written down
// either way.
func makeTrinoPrincipal(t *testing.T, _ *sql.DB, dsn, _ string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	u.User = url.User("dbmeta_other")
	return u.String()
}

// makePrestoPrincipal names a different principal on the connection, the same
// way makeTrinoPrincipal does. Presto takes the user from the DSN and the
// image configures no authenticator.
func makePrestoPrincipal(t *testing.T, _ *sql.DB, dsn, _ string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	u.User = url.User("dbmeta_other")
	return u.String()
}

// makeFirebirdGrantee creates a user with SELECT on one table.
//
// A Firebird user belongs to the server rather than to the database, so
// CREATE USER writes to the security database that every database on the
// server shares. That is why the fixture creates none and this does, and why
// it is dropped again whatever the test finds.
//
// Every user management statement runs on a connection of its own and that
// connection is then closed. Two faults make it necessary, and both were
// measured rather than guessed.
//
// The first is that Firebird has no DROP USER ... IF EXISTS before 5.0, so
// the tidying drop fails on a clean server, and nakagami/firebirdsql then
// returns that same error for every later user management statement on the
// connection:
//
//	DROP USER dbmeta_absent  -> record not found for user: DBMETA_ABSENT
//	CREATE USER dbmeta_p1    -> record not found for user: DBMETA_ABSENT
//	SELECT COUNT(*) ...      -> <nil>
//	CREATE USER dbmeta_p2    -> record not found for user: DBMETA_ABSENT
//
// The second is worse and it is why the successful CREATE USER is moved as
// well. After a CREATE USER, a later read of SEC$USERS on the same
// connection is answered with EOF: the server drops the attachment, and the
// pool's next connection then fails to hand shake at all, so every query
// after it reports a protocol error. It takes a few statements in between to
// become reliable, which is why it looked intermittent before it was pinned
// down:
//
//	CREATE USER ...          -> <nil>
//	... ten metadata queries -> <nil>
//	SELECT FROM SEC$USERS    -> EOF
//
// dbmeta issues no user management statement at any time, so nothing a
// consumer reads is affected and the Roles query keeps SEC$USERS. A test that
// creates a principal has to keep the two apart, and this does.
func makeFirebirdGrantee(t *testing.T, db *sql.DB, dsn, _ string) string {
	t.Helper()
	firebirdApart(t, dsn, `DROP USER dbmeta_parity`)
	firebirdApart(t, dsn, `CREATE USER dbmeta_parity PASSWORD '`+parityPassword+`'`)
	t.Cleanup(func() { firebirdApart(t, dsn, `DROP USER dbmeta_parity`) })
	// A grant is ordinary SQL and belongs on the connection under test.
	exec(t, db, `GRANT SELECT ON author TO dbmeta_parity`)
	return firebirdUser(t, dsn, "dbmeta_parity", parityPassword)
}

// firebirdApart runs one statement on a connection it then closes, so that
// neither fault above can reach the connection the test measures.
func firebirdApart(t *testing.T, dsn, stmt string) {
	t.Helper()
	db, err := sql.Open("firebirdsql", dsn)
	if err != nil {
		t.Logf("opening for %s: %v", stmt, err)
		return
	}
	defer db.Close()
	if _, err := db.ExecContext(context.WithoutCancel(t.Context()), stmt); err != nil {
		t.Logf("running %s: %v", stmt, err)
	}
}

// firebirdUser swaps the principal in a Firebird DSN.
//
// replaceUser cannot, because a Firebird DSN carries no scheme and url.Parse
// then reads the user name as one. This adds the scheme that dburl uses,
// edits the user and takes the scheme off again.
func firebirdUser(t *testing.T, dsn, user, password string) string {
	t.Helper()
	u, err := url.Parse("firebird://" + dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	u.User = url.UserPassword(user, password)
	return strings.TrimPrefix(u.String(), "firebird://")
}

// makeHANAGrantee creates a user with SELECT on one table.
//
// NO FORCE_FIRST_PASSWORD_CHANGE is required rather than tidy: without it
// HANA marks the password as needing a change and the new user cannot run a
// statement until it has changed one, so every query reports the same error
// and the comparison says nothing.
func makeHANAGrantee(t *testing.T, db *sql.DB, dsn, schema string) string {
	t.Helper()
	cleanup(t, db, `DROP USER dbmeta_parity CASCADE`)
	exec(t, db, `CREATE USER dbmeta_parity PASSWORD "`+parityPassword+
		`" NO FORCE_FIRST_PASSWORD_CHANGE`)
	t.Cleanup(func() { cleanup(t, db, `DROP USER dbmeta_parity CASCADE`) })
	exec(t, db, `GRANT SELECT ON SCHEMA `+schema+` TO dbmeta_parity`)
	return replaceUser(t, dsn, "dbmeta_parity", parityPassword)
}

// makeHivePrincipal names a different principal on the connection.
//
// Hive has no users to create. The image configures no authorization, so the
// client states who it is and HiveServer2 takes it: the SASL exchange
// happens and nothing is validated. A password still has to be non empty,
// which is the one thing the exchange checks.
//
// With no authorization configured the server then allows that principal
// everything, so this target is expected to find no difference. That is the
// measurement rather than a gap in it.
func makeHivePrincipal(t *testing.T, _ *sql.DB, dsn, _ string) string {
	t.Helper()
	return replaceUser(t, dsn, "dbmeta_other", parityPassword)
}

// makeExasolOwner makes a user that owns the fixture schema.
//
// An Exasol schema's owner owns every object in it, so changing the schema's
// owner is the whole of it. The schema goes back to SYS before the user is
// dropped, because DROP USER ... CASCADE drops every schema the user owns.
// Without that step, the fixture's own teardown finds the adapter its virtual
// schema needs already gone.
func makeExasolOwner(t *testing.T, db *sql.DB, dsn, schema string) string {
	t.Helper()
	cleanup(t, db, `DROP USER IF EXISTS dbmeta_owner CASCADE`)
	exec(t, db, `CREATE USER dbmeta_owner IDENTIFIED BY "`+parityPassword+`"`)
	t.Cleanup(func() {
		cleanup(t, db, `ALTER SCHEMA `+schema+` CHANGE OWNER SYS`)
		cleanup(t, db, `DROP USER IF EXISTS dbmeta_owner CASCADE`)
	})
	exec(t, db, `GRANT CREATE SESSION TO dbmeta_owner`)
	exec(t, db, `ALTER SCHEMA `+schema+` CHANGE OWNER dbmeta_owner`)
	return exasolUser(t, dsn, "dbmeta_owner", parityPassword)
}

// makeExasolGrantee makes a user that can read the schema and owns nothing.
func makeExasolGrantee(t *testing.T, db *sql.DB, dsn, schema string) string {
	t.Helper()
	cleanup(t, db, `DROP USER IF EXISTS dbmeta_grantee CASCADE`)
	exec(t, db, `CREATE USER dbmeta_grantee IDENTIFIED BY "`+parityPassword+`"`)
	t.Cleanup(func() { cleanup(t, db, `DROP USER IF EXISTS dbmeta_grantee CASCADE`) })
	exec(t, db, `GRANT CREATE SESSION TO dbmeta_grantee`)
	exec(t, db, `GRANT SELECT ON SCHEMA `+schema+` TO dbmeta_grantee`)
	return exasolUser(t, dsn, "dbmeta_grantee", parityPassword)
}

// exasolUser rewrites the user and password of an Exasol DSN, which is not a
// URL but exa:host:port followed by key=value pairs separated by semicolons.
//
// The password goes last. The driver reads a backslash before a semicolon as
// an escaped separator, so a password ending in a backslash swallows the
// semicolon after it. At the end there is none. A password containing a
// semicolon is not written here at all: that is the driver's escaping, and
// test/password_test.go leaves that case out.
func exasolUser(t *testing.T, dsn, user, password string) string {
	t.Helper()
	var kept []string
	var found int
	for p := range strings.SplitSeq(dsn, ";") {
		key, _, ok := strings.Cut(p, "=")
		switch {
		case ok && strings.EqualFold(key, "user"):
			kept = append(kept, "user="+user)
			found++
		case ok && strings.EqualFold(key, "password"):
			found++
		default:
			kept = append(kept, p)
		}
	}
	if found != 2 {
		t.Fatalf("expected a user and a password in %s", dsn)
	}
	return strings.Join(append(kept, "password="+password), ";")
}

// makeVerticaOwner makes a user that owns every object in the fixture
// schema.
//
// From 10.1, ALTER SCHEMA ... OWNER TO ... CASCADE hands over the schema and
// its objects together. 7.2 and 9.1 have no ALTER SCHEMA ... OWNER, and
// refuse it as a syntax error, so there each table, view and sequence is
// handed over on its own and the user is granted usage on the schema, which
// it cannot own. Ownership goes back to the administrator before the user is
// dropped, because DROP USER ... CASCADE drops what the user owns.
func makeVerticaOwner(t *testing.T, db *sql.DB, dsn, schema string) string {
	t.Helper()
	cleanup(t, db, `DROP USER IF EXISTS dbmeta_owner CASCADE`)
	exec(t, db, `CREATE USER dbmeta_owner IDENTIFIED BY '`+parityPassword+`'`)
	if _, err := db.ExecContext(t.Context(),
		`ALTER SCHEMA `+schema+` OWNER TO dbmeta_owner CASCADE`); err == nil {
		t.Cleanup(func() {
			cleanup(t, db, `ALTER SCHEMA `+schema+` OWNER TO dbmeta CASCADE`)
			cleanup(t, db, `DROP USER IF EXISTS dbmeta_owner CASCADE`)
		})
		return replaceUser(t, dsn, "dbmeta_owner", parityPassword)
	}
	objects := verticaObjects(t, db, schema)
	t.Cleanup(func() {
		for _, o := range objects {
			cleanup(t, db, `ALTER `+o+` OWNER TO dbmeta`)
		}
		cleanup(t, db, `DROP USER IF EXISTS dbmeta_owner CASCADE`)
	})
	exec(t, db, `GRANT USAGE ON SCHEMA `+schema+` TO dbmeta_owner`)
	for _, o := range objects {
		exec(t, db, `ALTER `+o+` OWNER TO dbmeta_owner`)
	}
	return replaceUser(t, dsn, "dbmeta_owner", parityPassword)
}

// verticaObjects names every table, view and sequence in a schema, each with
// the word ALTER needs before it.
func verticaObjects(t *testing.T, db *sql.DB, schema string) []string {
	t.Helper()
	rows, err := db.QueryContext(t.Context(), `SELECT 'TABLE ' || table_schema || '.' || table_name
FROM v_catalog.tables WHERE table_schema = ?
UNION ALL SELECT 'VIEW ' || table_schema || '.' || table_name
FROM v_catalog.views WHERE table_schema = ?
UNION ALL SELECT 'SEQUENCE ' || sequence_schema || '.' || sequence_name
FROM v_catalog.sequences WHERE sequence_schema = ? AND identity_table_name IS NULL`,
		schema, schema, schema)
	if err != nil {
		t.Fatalf("listing the objects in %s: %v", schema, err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var o string
		if err := rows.Scan(&o); err != nil {
			t.Fatalf("reading the objects in %s: %v", schema, err)
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("listing the objects in %s: %v", schema, err)
	}
	return out
}

// makeVerticaGrantee makes a user that can read the schema and owns nothing.
func makeVerticaGrantee(t *testing.T, db *sql.DB, dsn, schema string) string {
	t.Helper()
	cleanup(t, db, `DROP USER IF EXISTS dbmeta_grantee CASCADE`)
	exec(t, db, `CREATE USER dbmeta_grantee IDENTIFIED BY '`+parityPassword+`'`)
	t.Cleanup(func() { cleanup(t, db, `DROP USER IF EXISTS dbmeta_grantee CASCADE`) })
	exec(t, db, `GRANT USAGE ON SCHEMA `+schema+` TO dbmeta_grantee`)
	exec(t, db, `GRANT SELECT ON ALL TABLES IN SCHEMA `+schema+` TO dbmeta_grantee`)
	return replaceUser(t, dsn, "dbmeta_grantee", parityPassword)
}

// openCockroachDB returns the administrator connection to the server named by
// DBMETA_COCKROACHDB, or skips.
func openCockroachDB(t *testing.T) *sql.DB {
	t.Helper()
	return openFamilyWith(t, pgFamilies[1], "pgx")
}

// setupCockroachDB builds the CockroachDB fixture and returns the meta.
func setupCockroachDB(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	return setupFamily(t, pgFamilies[1], db)
}

// makeCockroachDBOwner makes a role that owns the schema and every relation
// the fixture built in it, as makePostgresOwner does. The relations are found
// and changed from Go rather than in a DO block, so that nothing here depends
// on CockroachDB's PL/pgSQL.
//
// CockroachDB refuses to change the owner of a sequence that a serial column
// owns, as PostgreSQL does, so those are left to follow their table.
func makeCockroachDBOwner(t *testing.T, db *sql.DB, dsn, schema string) string {
	t.Helper()
	dropCockroachDBRole(t, db, "dbmeta_owner")
	exec(t, db, `CREATE ROLE dbmeta_owner LOGIN PASSWORD '`+parityPassword+`'`)
	t.Cleanup(func() { dropCockroachDBRole(t, db, "dbmeta_owner") })
	exec(t, db, `ALTER SCHEMA `+schema+` OWNER TO dbmeta_owner`)
	for _, s := range cockroachDBOwnerChanges(t, db, schema) {
		exec(t, db, s)
	}
	return replaceUser(t, dsn, "dbmeta_owner", parityPassword)
}

// cockroachDBOwnerChanges lists the statements that give dbmeta_owner every
// relation of the schema.
func cockroachDBOwnerChanges(t *testing.T, db *sql.DB, schema string) []string {
	t.Helper()
	rows, err := db.QueryContext(t.Context(), `SELECT c.relname, c.relkind FROM pg_catalog.pg_class c
	JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
	WHERE n.nspname = $1 AND c.relkind IN ('r', 'v', 'm', 'S')
	AND NOT (c.relkind = 'S' AND EXISTS (
		SELECT 1 FROM pg_catalog.pg_depend d
		WHERE d.objid = c.oid AND d.deptype IN ('a', 'i')))`, schema)
	if err != nil {
		t.Fatalf("listing the relations of %s: %v", schema, err)
	}
	defer rows.Close()
	var stmts []string
	for rows.Next() {
		var name, kind string
		if err := rows.Scan(&name, &kind); err != nil {
			t.Fatalf("reading a relation of %s: %v", schema, err)
		}
		what := map[string]string{"r": "TABLE", "v": "VIEW", "m": "MATERIALIZED VIEW", "S": "SEQUENCE"}[kind]
		stmts = append(stmts, `ALTER `+what+` `+schema+`.`+name+` OWNER TO dbmeta_owner`)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("listing the relations of %s: %v", schema, err)
	}
	return stmts
}

// makeCockroachDBGrantee makes a role that can read the schema and owns
// nothing, as makePostgresGrantee does.
func makeCockroachDBGrantee(t *testing.T, db *sql.DB, dsn, schema string) string {
	t.Helper()
	dropCockroachDBRole(t, db, "dbmeta_grantee")
	exec(t, db, `CREATE ROLE dbmeta_grantee LOGIN PASSWORD '`+parityPassword+`'`)
	t.Cleanup(func() { dropCockroachDBRole(t, db, "dbmeta_grantee") })
	exec(t, db, `GRANT USAGE ON SCHEMA `+schema+` TO dbmeta_grantee`)
	exec(t, db, `GRANT SELECT ON ALL TABLES IN SCHEMA `+schema+` TO dbmeta_grantee`)
	return replaceUser(t, dsn, "dbmeta_grantee", parityPassword)
}

// dropCockroachDBRole removes a role and whatever it owns, from Go rather
// than in a DO block, before the role is made and after.
func dropCockroachDBRole(t *testing.T, db *sql.DB, role string) {
	t.Helper()
	var n int
	ctx := context.WithoutCancel(t.Context())
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM pg_catalog.pg_roles WHERE rolname = $1`, role).Scan(&n); err != nil || n == 0 {
		return
	}
	cleanup(t, db, `REASSIGN OWNED BY `+role+` TO current_user`)
	cleanup(t, db, `DROP OWNED BY `+role)
	cleanup(t, db, `DROP ROLE `+role)
}

// makeDatabendHolder makes a user whose role holds privileges on the
// fixture's database, and connects as it. Databend grants a privilege on a
// database to a role and not to a user, so the user takes a role of its own
// as its default.
func makeDatabendHolder(privileges string) func(*testing.T, *sql.DB, string, string) string {
	return func(t *testing.T, db *sql.DB, dsn, schema string) string {
		t.Helper()
		drop := func() {
			cleanup(t, db, `DROP USER IF EXISTS dbmeta_parity`)
			cleanup(t, db, `DROP ROLE IF EXISTS dbmeta_parity_role`)
		}
		drop()
		exec(t, db, `CREATE ROLE dbmeta_parity_role`)
		exec(t, db, `GRANT `+privileges+` ON `+schema+`.* TO ROLE dbmeta_parity_role`)
		exec(t, db, `CREATE USER dbmeta_parity IDENTIFIED BY '`+parityPassword+
			`' WITH DEFAULT_ROLE = 'dbmeta_parity_role'`)
		exec(t, db, `GRANT ROLE dbmeta_parity_role TO dbmeta_parity`)
		t.Cleanup(drop)
		return replaceUser(t, dsn, "dbmeta_parity", parityPassword)
	}
}
