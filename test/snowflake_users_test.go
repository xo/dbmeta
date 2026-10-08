package test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"testing"

	"github.com/snowflakedb/gosnowflake/v2"

	"github.com/xo/dbmeta"
)

// Snowflake principals for parity (rule 16) and for ChangePassword (D56).
//
// A principal in Snowflake is a user that holds roles, and a role holds the
// grants. The test makes a role and a user for each principal, with the
// CREATE ROLE and CREATE USER grants that Ken gave the role of the test
// account. Neither is the one login of the account, which only the account's
// own user is, and the test never changes it. See D193.
//
// A made user logs in with a key pair that the test generates in memory. The
// public key goes to the server and the private key never leaves the process,
// so no secret is in a file or in a statement. The connection string is the
// administrator's with the user, the role and the key replaced, which
// gosnowflake parses and writes. The test never prints it.

// sfWarehouse is the warehouse a made user runs on. The role of the test
// account owns it, and holds USAGE on it with the grant option, so the test
// grants USAGE to each role it makes and the drop of the role removes the
// grant. The test never alters, suspends, resizes or drops the warehouse, and
// it grants nothing to a role it did not make, because dbimp uses the same
// warehouse. See D203.
const sfWarehouse = "DBMETA_WH"

// sfPad is put before every password in a test, because the password policy
// of the account refuses a short one.
const sfPad = "Zq7-Long-Pad-"

// sfLogin returns the connection string of the administrator with the
// settings of another login. It keeps the account and the database.
func sfLogin(t *testing.T, adminDSN string, change func(*gosnowflake.Config)) string {
	t.Helper()
	cfg, err := gosnowflake.ParseDSN(adminDSN)
	if err != nil {
		t.Fatalf("reading the connection string: %v", err)
	}
	cfg.Password, cfg.PrivateKey, cfg.Token = "", nil, ""
	cfg.Authenticator = gosnowflake.AuthTypeSnowflake
	cfg.Warehouse = sfWarehouse
	change(cfg)
	dsn, err := gosnowflake.DSN(cfg)
	if err != nil {
		t.Fatalf("writing the connection string: %v", err)
	}
	return dsn
}

// dropSnowflakePrincipal removes a user and its role. It runs before they are
// made as well as after, because a run that failed part way leaves them
// behind.
func dropSnowflakePrincipal(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	cleanup(t, db, `DROP USER IF EXISTS `+name)
	cleanup(t, db, `DROP ROLE IF EXISTS `+name)
}

// grantWarehouse gives a role that the test made USAGE on the warehouse, and
// revokes it when the test ends, before the role is dropped.
func grantWarehouse(t *testing.T, db *sql.DB, role string) {
	t.Helper()
	exec(t, db, `GRANT USAGE ON WAREHOUSE `+sfWarehouse+` TO ROLE `+role)
	t.Cleanup(func() { cleanup(t, db, `REVOKE USAGE ON WAREHOUSE `+sfWarehouse+` FROM ROLE `+role) })
}

// makeSnowflakePrincipal makes a role that holds the grants, and a user that
// holds the role and logs in with a key pair. It returns the connection
// string of the user.
//
// A session opens in the database and schema of the connection string, and
// Snowflake refuses a role that holds no USAGE on them. So open names only
// what the role can use: the database, the schema, or neither.
func makeSnowflakePrincipal(t *testing.T, db *sql.DB, dsn, name string, open func(*gosnowflake.Config), grants ...string) string {
	t.Helper()
	dropSnowflakePrincipal(t, db, name)
	t.Cleanup(func() { dropSnowflakePrincipal(t, db, name) })
	exec(t, db, `CREATE ROLE `+name)
	grantWarehouse(t, db, name)
	for _, grant := range grants {
		exec(t, db, grant+` TO ROLE `+name)
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating a key: %v", err)
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("writing the public key: %v", err)
	}
	exec(t, db, `CREATE USER `+name+` DEFAULT_ROLE = `+name+
		` RSA_PUBLIC_KEY = '`+
		base64.StdEncoding.EncodeToString(der)+`'`)
	exec(t, db, `GRANT ROLE `+name+` TO USER `+name)
	return sfLogin(t, dsn, func(cfg *gosnowflake.Config) {
		cfg.User, cfg.Role = name, name
		cfg.Authenticator = gosnowflake.AuthTypeJwt
		cfg.PrivateKey = key
		open(cfg)
	})
}

// makeSnowflakeGrantee makes a role that can read the fixture schema and one
// of its tables, and owns nothing.
func makeSnowflakeGrantee(t *testing.T, db *sql.DB, dsn, schema string) string {
	t.Helper()
	var database string
	if err := db.QueryRowContext(t.Context(), `SELECT CURRENT_DATABASE()`).Scan(&database); err != nil {
		t.Fatalf("reading the database: %v", err)
	}
	return makeSnowflakePrincipal(t, db, dsn, "DBMETA_GRANTEE", func(*gosnowflake.Config) {},
		`GRANT USAGE ON DATABASE `+database,
		`GRANT USAGE ON SCHEMA `+database+`.`+schema,
		`GRANT SELECT ON TABLE `+database+`.`+schema+`.AUTHOR`)
}

// makeSnowflakeStranger makes a role with no grant on the fixture at all.
// INFORMATION_SCHEMA shows a role only the objects it holds a privilege on,
// and the golden file records what that leaves.
func makeSnowflakeStranger(t *testing.T, db *sql.DB, dsn, _ string) string {
	t.Helper()
	return makeSnowflakePrincipal(t, db, dsn, "DBMETA_STRANGER", func(cfg *gosnowflake.Config) {
		cfg.Database, cfg.Schema = "", ""
	})
}

// makeSnowflakeVisitor makes a role that can use the database and nothing in
// it, so it reads the INFORMATION_SCHEMA of the database and sees no object
// of the fixture.
func makeSnowflakeVisitor(t *testing.T, db *sql.DB, dsn, _ string) string {
	t.Helper()
	var database string
	if err := db.QueryRowContext(t.Context(), `SELECT CURRENT_DATABASE()`).Scan(&database); err != nil {
		t.Fatalf("reading the database: %v", err)
	}
	return makeSnowflakePrincipal(t, db, dsn, "DBMETA_VISITOR", func(cfg *gosnowflake.Config) {
		cfg.Schema = ""
	}, `GRANT USAGE ON DATABASE `+database)
}

// sfPasswordLogin opens a connection as a user with a password, in the role of
// the same name.
func sfPasswordLogin(t *testing.T, adminDSN, user, password string) *sql.DB {
	t.Helper()
	dsn := sfLogin(t, adminDSN, func(cfg *gosnowflake.Config) {
		cfg.User, cfg.Password, cfg.Role = user, password, user
		// The role holds no grant on the database of the account.
		cfg.Database, cfg.Schema = "", ""
	})
	return openAt(t, "snowflake", dsn)
}

// TestChangePasswordSnowflake sets each password and logs in with it.
//
// Snowflake reads a backslash in a string as an escape, so the statement
// escapes both it and the quote, and a user name between double quotes keeps
// its case. The user is made by the test and is not the account's own login.
func TestChangePasswordSnowflake(t *testing.T) {
	db := openSnowflake(t)
	adminDSN := dsnOf(t, "DBMETA_SNOWFLAKE")
	const user = "DBMETA_PW"
	const start = "Start-P4ss!x"
	dropSnowflakePrincipal(t, db, user)
	// TYPE = LEGACY_SERVICE is the kind of user that a password alone can
	// log in, with no second factor.
	exec(t, db, `CREATE ROLE `+user)
	t.Cleanup(func() { dropSnowflakePrincipal(t, db, user) })
	grantWarehouse(t, db, user)
	exec(t, db, `CREATE USER `+user+` PASSWORD = '`+start+`' MUST_CHANGE_PASSWORD = FALSE`+
		` TYPE = LEGACY_SERVICE DEFAULT_ROLE = `+user)
	exec(t, db, `GRANT ROLE `+user+` TO USER `+user)
	t.Cleanup(func() { dropSnowflakePrincipal(t, db, user) })
	for _, c := range hostilePasswords {
		t.Run(c.name, func(t *testing.T) {
			// The password policy of the account refuses a short password,
			// so a prefix pads it and the hostile part stays at the end.
			password := sfPad + c.password
			stmt, err := dbmeta.Snowflake.ChangePassword(
				dbmeta.PasswordChange{User: user, Password: password}, dbmeta.Quoting{})
			if err != nil {
				t.Fatalf("building the statement: %v", err)
			}
			exec(t, db, stmt)
			who := sfPasswordLogin(t, adminDSN, user, password)
			var got string
			if err := who.QueryRowContext(t.Context(), `SELECT CURRENT_USER()`).Scan(&got); err != nil {
				t.Fatalf("asking who it is: %v", err)
			}
			if got != user {
				t.Errorf("expected to log in as %s, got %s", user, got)
			}
		})
	}
}
