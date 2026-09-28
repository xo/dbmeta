package container

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/url"
)

// The PostgREST releases dbrun starts.
//
// dbmeta has no PostgREST model. The releases are here so that dbrun can
// start a server for the tests of the PostgREST driver in
// github.com/xo/dbimp, which reads the HTTP interface with a bearer token. No
// dialect is named yet, because dbimp settles the name with the driver. See
// D118.
//
// # The range
//
// docker.io/postgrest/postgrest builds each release tag once, so the rule in
// D112 applies: the newest release of each of the last two lines. Checked on
// 2026-09-28, that is v16.4, of 2026-09-24, and v14.18, of 2026-09-12. There
// is no 15. PostgREST is under the MIT licence.
//
// # The image is built here
//
// The PostgREST image holds one program and no shell. The Containerfile in
// test/cmd/dbrun/image copies it onto PostgreSQL 18, and the command starts
// both, so the release of the entry is PostgREST's and PostgreSQL is fixed.
//
// # The users
//
// PostgREST logs in to PostgreSQL as authenticator and takes the role that a
// signed token names. The command makes the roles before PostgREST starts,
// because it cannot connect before authenticator exists, and every step is
// safe to run twice. dbmeta_admin owns the schema dbmeta, and [PostgRESTUser]
// may read its tables. There is no anonymous role, so a request with no token
// is refused. A token is an HS256 JWT signed with [postgrestSecret], which
// [postgrestToken] computes, so each principal's DSN carries its token as the
// password.

// PostgRESTUser may read the tables of the schema dbmeta.
const PostgRESTUser = "dbmeta_user"

// postgrestSecret signs every token. PostgREST needs at least 32 characters.
const postgrestSecret = Password + "-postgrest-secret-for-dbmeta-tests"

// postgrestToken is a token that names a role, with no expiry.
func postgrestToken(role string) string {
	enc := base64.RawURLEncoding
	body := enc.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`)) + "." +
		enc.EncodeToString([]byte(`{"role":"`+role+`"}`))
	m := hmac.New(sha256.New, []byte(postgrestSecret))
	m.Write([]byte(body))
	return body + "." + enc.EncodeToString(m.Sum(nil))
}

// postgrestServe starts PostgreSQL, makes the roles and the schema, and starts
// PostgREST.
var postgrestServe = `set -e
docker-entrypoint.sh postgres &
until pg_isready -q -h 127.0.0.1 -U postgres; do sleep 1; done
psql -q -h 127.0.0.1 -U postgres -v ON_ERROR_STOP=1 <<'SQL'
DO $$ BEGIN
	IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'authenticator') THEN
		CREATE ROLE authenticator LOGIN NOINHERIT;
	END IF;
	IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'dbmeta_admin') THEN
		CREATE ROLE dbmeta_admin NOLOGIN;
	END IF;
	IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '` + PostgRESTUser + `') THEN
		CREATE ROLE ` + PostgRESTUser + ` NOLOGIN;
	END IF;
END $$;
ALTER ROLE authenticator PASSWORD '` + Password + `';
CREATE SCHEMA IF NOT EXISTS dbmeta AUTHORIZATION dbmeta_admin;
GRANT USAGE ON SCHEMA dbmeta TO ` + PostgRESTUser + `;
ALTER DEFAULT PRIVILEGES FOR ROLE dbmeta_admin IN SCHEMA dbmeta GRANT SELECT ON TABLES TO ` + PostgRESTUser + `;
GRANT dbmeta_admin, ` + PostgRESTUser + ` TO authenticator;
SQL
exec postgrest`

// postgrest is PostgREST with PostgreSQL, built here.
var postgrest = product{
	name:  "postgrest",
	image: "localhost/dbmeta/postgrest",
	port:  3000,
	env: map[string]string{
		"POSTGRES_PASSWORD": Password,
		"PGRST_DB_URI":      "postgres://authenticator:" + url.QueryEscape(Password) + "@127.0.0.1:5432/postgres",
		"PGRST_DB_SCHEMAS":  "dbmeta",
		"PGRST_JWT_SECRET":  postgrestSecret,
		"PGRST_SERVER_PORT": "3000",
		"PGRST_DB_POOL":     "4",
	},
	runFlags: []string{"--entrypoint", "/bin/bash"},
	args:     []string{"-c", postgrestServe},
	// The image has bash and no curl, so the check goes through bash's
	// /dev/tcp, with the administrator's token.
	ready: bashRequest(3000, "GET", "/", "", map[string]string{"Authorization": "Bearer " + postgrestToken("dbmeta_admin")}, 200),
	dsn:   keyHTTP("dbmeta_admin", postgrestToken("dbmeta_admin")),
	users: []Principal{{Role: User, User: PostgRESTUser, dsn: keyHTTP(PostgRESTUser, postgrestToken(PostgRESTUser))}},
}

// PostgREST is every PostgREST release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. See D119.
var PostgREST = list{}.add(postgrest, Staged, "14.18", "16.4")
