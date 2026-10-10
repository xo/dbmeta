package test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"net/url"
	"strings"
	"testing"

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
// administrator's with the user, the role, the warehouse and the key replaced.
// The test never prints it.

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

// sfLogin returns the connection string of the administrator with the user, the
// key, the role and the warehouse of another login. It keeps the host. The
// path of the URL holds the database and the schema, and open names the ones
// that the role can use. The password of the URL is the private key, as the
// base64url text of its PKCS8 DER bytes, which is the form of the driver of
// dbimp (D213). The test never prints the result.
func sfLogin(t *testing.T, adminDSN, user, role string, key *rsa.PrivateKey, open func(path []string) []string) string {
	t.Helper()
	u, err := url.Parse(adminDSN)
	if err != nil {
		t.Fatalf("reading the connection string: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("writing the private key: %v", err)
	}
	u.User = url.UserPassword(user, base64.RawURLEncoding.EncodeToString(der))
	u.Path = "/" + strings.Join(open(strings.Split(strings.Trim(u.Path, "/"), "/")), "/")
	q := url.Values{}
	q.Set("role", role)
	q.Set("warehouse", sfWarehouse)
	u.RawQuery = q.Encode()
	return u.String()
}

// sfKey makes the key pair of a made user. The public key goes to the server
// as the text that RSA_PUBLIC_KEY takes.
func sfKey(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating a key: %v", err)
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("writing the public key: %v", err)
	}
	return key, base64.StdEncoding.EncodeToString(der)
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
// Snowflake refuses a role that holds no USAGE on them. So open takes the
// database and the schema of the administrator and returns only what the role
// can use: the database, the schema, or neither.
func makeSnowflakePrincipal(t *testing.T, db *sql.DB, dsn, name string, open func([]string) []string, grants ...string) string {
	t.Helper()
	dropSnowflakePrincipal(t, db, name)
	t.Cleanup(func() { dropSnowflakePrincipal(t, db, name) })
	exec(t, db, `CREATE ROLE `+name)
	grantWarehouse(t, db, name)
	for _, grant := range grants {
		exec(t, db, grant+` TO ROLE `+name)
	}
	key, public := sfKey(t)
	exec(t, db, `CREATE USER `+name+` DEFAULT_ROLE = `+name+` RSA_PUBLIC_KEY = '`+public+`'`)
	exec(t, db, `GRANT ROLE `+name+` TO USER `+name)
	return sfLogin(t, dsn, name, name, key, open)
}

// makeSnowflakeGrantee makes a role that can read the fixture schema and one
// of its tables, and owns nothing.
func makeSnowflakeGrantee(t *testing.T, db *sql.DB, dsn, schema string) string {
	t.Helper()
	var database string
	if err := db.QueryRowContext(t.Context(), `SELECT CURRENT_DATABASE()`).Scan(&database); err != nil {
		t.Fatalf("reading the database: %v", err)
	}
	return makeSnowflakePrincipal(t, db, dsn, "DBMETA_GRANTEE", func(path []string) []string { return path },
		`GRANT USAGE ON DATABASE `+database,
		`GRANT USAGE ON SCHEMA `+database+`.`+schema,
		`GRANT SELECT ON TABLE `+database+`.`+schema+`.AUTHOR`)
}

// makeSnowflakeStranger makes a role with no grant on the fixture at all.
// INFORMATION_SCHEMA shows a role only the objects it holds a privilege on,
// and the golden file records what that leaves.
func makeSnowflakeStranger(t *testing.T, db *sql.DB, dsn, _ string) string {
	t.Helper()
	return makeSnowflakePrincipal(t, db, dsn, "DBMETA_STRANGER", func([]string) []string { return nil })
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
	return makeSnowflakePrincipal(t, db, dsn, "DBMETA_VISITOR", func(path []string) []string { return path[:1] }, `GRANT USAGE ON DATABASE `+database)
}

// sfHasPassword reads whether a user holds a password, from the column
// has_password of SHOW USERS. It reads the columns by name, because SHOW has
// no fixed list that a test can count on.
func sfHasPassword(t *testing.T, db *sql.DB, user string) bool {
	t.Helper()
	rows, err := db.QueryContext(t.Context(), `SHOW USERS LIKE '`+user+`'`)
	if err != nil {
		t.Fatalf("showing the user: %v", err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		t.Fatalf("reading the columns: %v", err)
	}
	if !rows.Next() {
		t.Fatalf("expected a row for user %s", user)
	}
	vals := make([]sql.NullString, len(cols))
	dest := make([]any, len(cols))
	for i := range vals {
		dest[i] = &vals[i]
	}
	if err := rows.Scan(dest...); err != nil {
		t.Fatalf("scanning the user: %v", err)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reading the user: %v", err)
	}
	for i, c := range cols {
		if strings.EqualFold(c, "has_password") {
			return strings.EqualFold(vals[i].String, "true")
		}
	}
	t.Fatalf("expected the column has_password in %v", cols)
	return false
}

// TestChangePasswordSnowflake runs the statement for each password and reads
// back that the user holds one.
//
// The driver of dbimp reads the password of a URL only as a private key, so
// the test cannot log in with a password. D213 says where the login was
// measured.
//
// Snowflake reads a backslash in a string as an escape, so the statement
// escapes both it and the quote, and a user name between double quotes keeps
// its case. The user is made by the test and is not the account's own login.
func TestChangePasswordSnowflake(t *testing.T) {
	db := openSnowflake(t)
	const user = "DBMETA_PW"
	dropSnowflakePrincipal(t, db, user)
	// TYPE = LEGACY_SERVICE is the kind of user that a password alone can
	// log in, with no second factor. The user holds no password until the
	// statement sets one.
	exec(t, db, `CREATE ROLE `+user)
	t.Cleanup(func() { dropSnowflakePrincipal(t, db, user) })
	exec(t, db, `CREATE USER `+user+` MUST_CHANGE_PASSWORD = FALSE`+
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
			exec(t, db, `ALTER USER `+user+` UNSET PASSWORD`)
			if sfHasPassword(t, db, user) {
				t.Fatalf("expected %s to hold no password before the statement", user)
			}
			exec(t, db, stmt)
			if !sfHasPassword(t, db, user) {
				t.Errorf("expected %s to hold a password after the statement", user)
			}
		})
	}
}
