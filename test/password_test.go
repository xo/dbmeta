package test

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
)

// Changing a password end to end, which is the only way to know the escaping
// is right.
//
// A unit test can check that the text looks correct. It cannot catch the fault
// this exists for, which is a password that terminates its own literal and
// lets the rest of the statement run. So each case sets a real password on a
// real server and then opens a new connection with it. If the escaping is
// wrong the login fails, or the statement does something else entirely.
//
// The passwords are chosen to break a naive escaper: a trailing backslash is
// the one that broke MariaDB with quote doubling alone, and the rest cover the
// quote, both together, and a password that looks like the end of a statement.
var hostilePasswords = []struct{ name, password string }{
	{"plain", "Pl41nP4ss!x"},
	{"a quote", `a'b-P4ss!x`},
	{"a trailing backslash", `P4ss!x\`},
	{"a quote and a backslash", `a'b\-P4ss!x`},
	{"two backslashes", `P4ss!x\\`},
	{"looks like the end of a statement", `P4ss!x'; SELECT 1; --`},
	{"a double quote", `a"b-P4ss!x`},
}

// TestChangePasswordPostgres sets each password and logs in with it.
func TestChangePasswordPostgres(t *testing.T) {
	db := open(t)
	ctx := t.Context()
	q, err := dbmeta.PostgreSQL.Quoting(ctx, db)
	if err != nil {
		t.Fatalf("reading the quoting state: %v", err)
	}
	if !q.BackslashEscapes.Valid {
		t.Fatal("expected PostgreSQL to report standard_conforming_strings")
	}
	t.Logf("standard_conforming_strings gives BackslashEscapes=%v", q.BackslashEscapes.V)

	const user = "dbmeta_pw"
	exec(t, db, `DROP ROLE IF EXISTS `+user)
	exec(t, db, `CREATE ROLE `+user+` LOGIN`)
	t.Cleanup(func() { cleanup(t, db, `DROP ROLE IF EXISTS `+user) })

	for _, c := range hostilePasswords {
		t.Run(c.name, func(t *testing.T) {
			stmt, err := dbmeta.PostgreSQL.ChangePassword(
				dbmeta.PasswordChange{User: user, Password: c.password}, q)
			if err != nil {
				t.Fatalf("building the statement: %v", err)
			}
			exec(t, db, stmt)
			// The statement ran. Now prove it set what was asked, by using it.
			dsn := replaceUser(t, dsnOf(t, "DBMETA_POSTGRES"), user, c.password)
			login(t, "pgx", dsn, `SELECT current_user`, user)
		})
	}
}

// TestChangePasswordMySQL does the same on MariaDB or MySQL, which is the
// product where quote doubling alone is not enough.
func TestChangePasswordMySQL(t *testing.T) {
	db := openMySQL(t)
	ctx := t.Context()
	q, err := dbmeta.MySQL.Quoting(ctx, db)
	if err != nil {
		t.Fatalf("reading the quoting state: %v", err)
	}
	if !q.BackslashEscapes.Valid {
		t.Fatal("expected the server to report sql_mode")
	}
	t.Logf("sql_mode gives BackslashEscapes=%v", q.BackslashEscapes.V)
	if !q.BackslashEscapes.V {
		t.Log("NO_BACKSLASH_ESCAPES is set, so this run does not exercise the interesting case")
	}

	const account = "dbmeta_pw@%"
	exec(t, db, "DROP USER IF EXISTS 'dbmeta_pw'@'%'")
	exec(t, db, "CREATE USER 'dbmeta_pw'@'%' IDENTIFIED BY 'start-P4ss!x'")
	t.Cleanup(func() { cleanup(t, db, "DROP USER IF EXISTS 'dbmeta_pw'@'%'") })

	for _, c := range hostilePasswords {
		t.Run(c.name, func(t *testing.T) {
			stmt, err := dbmeta.MySQL.ChangePassword(
				dbmeta.PasswordChange{User: account, Password: c.password}, q)
			if err != nil {
				t.Fatalf("building the statement: %v", err)
			}
			exec(t, db, stmt)
			base := dsnOf(t, "DBMETA_MYSQL")
			at := strings.Index(base, "@")
			dsn := "dbmeta_pw:" + c.password + base[at:]
			login(t, "mysql", dsn, `SELECT CURRENT_USER()`, "dbmeta_pw")
		})
	}
}

// TestChangePasswordSQLServer does the same where no session state is needed.
func TestChangePasswordSQLServer(t *testing.T) {
	db := openSQLServer(t)
	q, err := dbmeta.SQLServer.Quoting(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the quoting state: %v", err)
	}
	// T-SQL has no such setting, and reporting none is the right answer
	// rather than reporting a guess.
	if q.BackslashEscapes.Valid {
		t.Errorf("expected SQL Server to report no quoting state, got %v", q.BackslashEscapes)
	}

	const login2 = "dbmeta_pw"
	exec(t, db, `IF EXISTS (SELECT 1 FROM sys.server_principals WHERE name = '`+login2+`') DROP LOGIN `+login2)
	exec(t, db, `CREATE LOGIN `+login2+` WITH PASSWORD = N'Start-P4ss!x', CHECK_POLICY = OFF`)
	t.Cleanup(func() {
		cleanup(t, db, `IF EXISTS (SELECT 1 FROM sys.server_principals WHERE name = '`+login2+`') DROP LOGIN `+login2)
	})

	for _, c := range hostilePasswords {
		t.Run(c.name, func(t *testing.T) {
			stmt, err := dbmeta.SQLServer.ChangePassword(
				dbmeta.PasswordChange{User: login2, Password: c.password}, q)
			if err != nil {
				t.Fatalf("building the statement: %v", err)
			}
			exec(t, db, stmt)
			dsn := replaceUser(t, dsnOf(t, "DBMETA_SQLSERVER"), login2, c.password)
			login(t, "sqlserver", dsn, `SELECT SUSER_NAME()`, login2)
		})
	}
}

// exec runs a statement and fails the test when it does not run.
func exec(t *testing.T, db *sql.DB, stmt string) {
	t.Helper()
	if _, err := db.ExecContext(t.Context(), stmt); err != nil {
		t.Fatalf("running %s: %v", stmt, err)
	}
}

// cleanup runs a statement after the test has ended, when t.Context is already
// cancelled. It drops the cancellation rather than reaching for a background
// context, and it reports rather than fails, because a test that passed should
// not fail on tidying up after itself.
func cleanup(t *testing.T, db *sql.DB, stmt string) {
	t.Helper()
	ctx := context.WithoutCancel(t.Context())
	if _, err := db.ExecContext(ctx, stmt); err != nil {
		t.Logf("cleaning up with %s: %v", stmt, err)
	}
}

// login opens a new connection with the password just set and checks who it
// connected as. This is the assertion the whole file exists for.
func login(t *testing.T, driver, dsn, who, want string) {
	t.Helper()
	db, err := sql.Open(driver, dsn)
	if err != nil {
		t.Fatalf("opening a connection with the new password: %v", err)
	}
	defer db.Close()
	var got string
	if err := db.QueryRowContext(t.Context(), who).Scan(&got); err != nil {
		t.Fatalf("connecting with the new password: %v", err)
	}
	if !strings.Contains(got, want) {
		t.Errorf("connected as %q, expected %q", got, want)
	}
}

// replaceUser rewrites the user and password of a URL style connection string.
func replaceUser(t *testing.T, dsn, user, password string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing %s: %v", dsn, err)
	}
	u.User = url.UserPassword(user, password)
	return u.String()
}

func dsnOf(t *testing.T, name string) string {
	t.Helper()
	dsn := os.Getenv(name)
	if dsn == "" {
		t.Skipf("set %s to run against a real server", name)
	}
	return dsn
}
