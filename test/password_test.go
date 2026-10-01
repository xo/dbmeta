package test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

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

// TestChangePasswordPostgres sets each password and logs in with it, on each
// product of the PostgreSQL family. CockroachDB takes the postgres model's
// statement (D123), and this is what proves it parses there.
func TestChangePasswordPostgres(t *testing.T) {
	for _, f := range pgFamilies {
		t.Run(f.name, func(t *testing.T) {
			db := openFamilyWith(t, f, f.drivers[0])
			ctx := t.Context()
			q, err := f.dialect.Quoting(ctx, db)
			if err != nil {
				t.Fatalf("reading the quoting state: %v", err)
			}
			if !q.BackslashEscapes.Valid {
				t.Fatal("expected the server to report standard_conforming_strings")
			}
			t.Logf("standard_conforming_strings gives BackslashEscapes=%v", q.BackslashEscapes.V)

			const user = "dbmeta_pw"
			exec(t, db, `DROP ROLE IF EXISTS `+user)
			exec(t, db, `CREATE ROLE `+user+` LOGIN`)
			t.Cleanup(func() { cleanup(t, db, `DROP ROLE IF EXISTS `+user) })

			for _, c := range hostilePasswords {
				t.Run(c.name, func(t *testing.T) {
					stmt, err := f.dialect.ChangePassword(
						dbmeta.PasswordChange{User: user, Password: c.password}, q)
					if err != nil {
						t.Fatalf("building the statement: %v", err)
					}
					exec(t, db, stmt)
					// The statement ran. Now prove it set what was asked, by
					// using it.
					dsn := replaceUser(t, dsnOf(t, f.env), user, c.password)
					// Every driver of the product, because a password is
					// escaped into a statement here and then parsed out of a
					// DSN by the driver, and drivers parse a DSN differently.
					for _, driver := range f.drivers {
						login(t, driver, dsn, `SELECT current_user`, user)
					}
				})
			}
		})
	}
}

// TestChangePasswordMySQL does the same on MariaDB or MySQL, which is the
// product where quote doubling alone is not enough.
func TestChangePasswordMySQL(t *testing.T) {
	changePasswordMySQLFamily(t, dbmeta.MySQL, openMySQL(t), "DBMETA_MYSQL")
}

// TestChangePasswordTiDB does the same on TiDB, which takes the mysql model's
// statement and reads sql_mode the same way.
func TestChangePasswordTiDB(t *testing.T) {
	changePasswordMySQLFamily(t, dbmeta.TiDB, openTiDB(t), "DBMETA_TIDB")
}

// changePasswordMySQLFamily sets each password on a server that speaks
// MySQL's protocol and logs in with it.
func changePasswordMySQLFamily(t *testing.T, d dbmeta.Dialect, db *sql.DB, env string) {
	t.Helper()
	ctx := t.Context()
	q, err := d.Quoting(ctx, db)
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
			stmt, err := d.ChangePassword(
				dbmeta.PasswordChange{User: account, Password: c.password}, q)
			if err != nil {
				t.Fatalf("building the statement: %v", err)
			}
			exec(t, db, stmt)
			// No database, because the one a TiDB DSN names is one the new
			// user cannot use.
			dsn := mysqlAt(t, dsnOf(t, env), "dbmeta_pw", c.password, "")
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
// canceled. It drops the cancellation rather than reaching for a background
// context, and it reports rather than fails, because a test that passed must
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

// TestChangePasswordExasol sets each password and logs in with it, and then
// has the user change its own password, which needs the current one.
//
// Exasol takes a password as a quoted identifier, so the escaping is a
// doubled double quote and nothing else. The password that contains a
// semicolon is left out. The statement handles it, and the Exasol DSN is
// key=value pairs separated by semicolons with an escaping of its own, which
// is the driver's business rather than this model's.
func TestChangePasswordExasol(t *testing.T) {
	db := openExasol(t)
	// Exasol needs no session state to escape a password.
	q, err := dbmeta.Exasol.Quoting(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the quoting state: %v", err)
	}
	if q.BackslashEscapes.Valid {
		t.Errorf("expected Exasol to report no quoting state, got %v", q.BackslashEscapes)
	}
	// Created unquoted, so the catalog records it upper cased, and that is
	// the name the statement quotes.
	const user = "DBMETA_PW"
	exec(t, db, `DROP USER IF EXISTS dbmeta_pw`)
	exec(t, db, `CREATE USER dbmeta_pw IDENTIFIED BY "Start-P4ss!x"`)
	t.Cleanup(func() { cleanup(t, db, `DROP USER IF EXISTS dbmeta_pw`) })
	exec(t, db, `GRANT CREATE SESSION TO dbmeta_pw`)
	base := dsnOf(t, "DBMETA_EXASOL")
	current := "Start-P4ss!x"
	for _, c := range hostilePasswords {
		t.Run(c.name, func(t *testing.T) {
			if strings.Contains(c.password, ";") {
				t.Skip("an Exasol DSN separates its pairs with a semicolon")
			}
			stmt, err := dbmeta.Exasol.ChangePassword(
				dbmeta.PasswordChange{User: user, Password: c.password}, q)
			if err != nil {
				t.Fatalf("building the statement: %v", err)
			}
			exec(t, db, stmt)
			login(t, "exasol", exasolUser(t, base, user, c.password), `SELECT CURRENT_USER`, user)
			current = c.password
		})
	}

	// The user changing its own password. Without ALTER USER it has to name
	// the current password, and a statement without it is refused, which is
	// what makes REPLACE worth writing.
	self := openAt(t, "exasol", exasolUser(t, base, user, current))
	const next = `Next-P4ss!x"'`
	bare, err := dbmeta.Exasol.ChangePassword(dbmeta.PasswordChange{User: user, Password: next}, q)
	if err != nil {
		t.Fatalf("building the statement: %v", err)
	}
	if _, err := self.ExecContext(t.Context(), bare); err == nil {
		t.Error("expected a user without ALTER USER to need its current password")
	}
	stmt, err := dbmeta.Exasol.ChangePassword(
		dbmeta.PasswordChange{User: user, Password: next, Old: current}, q)
	if err != nil {
		t.Fatalf("building the statement: %v", err)
	}
	if _, err := self.ExecContext(t.Context(), stmt); err != nil {
		t.Fatalf("changing its own password: %v", err)
	}
	login(t, "exasol", exasolUser(t, base, user, next), `SELECT CURRENT_USER`, user)
}

// TestChangePasswordVertica sets each password and logs in with it, under
// both settings of standard_conforming_strings, and then has the user change
// its own password with the current one.
//
// The setting is per session, so the statement is built and run on one
// connection that set it.
func TestChangePasswordVertica(t *testing.T) {
	db := openVertica(t)
	const user = "dbmeta_pw"
	exec(t, db, `DROP USER IF EXISTS `+user)
	exec(t, db, `CREATE USER `+user+` IDENTIFIED BY 'Start-P4ss!x'`)
	t.Cleanup(func() { cleanup(t, db, `DROP USER IF EXISTS `+user) })
	base := dsnOf(t, "DBMETA_VERTICA")
	current := "Start-P4ss!x"
	for _, setting := range []string{"ON", "OFF"} {
		conn, err := db.Conn(t.Context())
		if err != nil {
			t.Fatalf("taking a connection: %v", err)
		}
		if _, err := conn.ExecContext(t.Context(), `SET STANDARD_CONFORMING_STRINGS TO `+setting); err != nil {
			t.Fatalf("setting standard_conforming_strings: %v", err)
		}
		q, err := dbmeta.Vertica.Quoting(t.Context(), conn)
		if err != nil {
			t.Fatalf("reading the quoting state: %v", err)
		}
		if q.BackslashEscapes.V != (setting == "OFF") {
			t.Fatalf("standard_conforming_strings %s read as BackslashEscapes=%v", setting, q.BackslashEscapes)
		}
		for _, c := range hostilePasswords {
			t.Run(setting+"/"+c.name, func(t *testing.T) {
				stmt, err := dbmeta.Vertica.ChangePassword(
					dbmeta.PasswordChange{User: user, Password: c.password}, q)
				if err != nil {
					t.Fatalf("building the statement: %v", err)
				}
				if _, err := conn.ExecContext(t.Context(), stmt); err != nil {
					t.Fatalf("running %s: %v", stmt, err)
				}
				login(t, "vertica", replaceUser(t, base, user, c.password), `SELECT CURRENT_USER()`, user)
				current = c.password
			})
		}
		if err := conn.Close(); err != nil {
			t.Fatalf("returning the connection: %v", err)
		}
	}

	// The user changing its own password, which needs the current one.
	self := openAt(t, "vertica", replaceUser(t, base, user, current))
	q, err := dbmeta.Vertica.Quoting(t.Context(), self)
	if err != nil {
		t.Fatalf("reading the quoting state: %v", err)
	}
	const next = `Next-P4ss!x'`
	stmt, err := dbmeta.Vertica.ChangePassword(
		dbmeta.PasswordChange{User: user, Password: next, Old: current}, q)
	if err != nil {
		t.Fatalf("building the statement: %v", err)
	}
	if _, err := self.ExecContext(t.Context(), stmt); err != nil {
		t.Fatalf("changing its own password: %v", err)
	}
	login(t, "vertica", replaceUser(t, base, user, next), `SELECT CURRENT_USER()`, user)
}

// TestChangePasswordCrateDB sets each password and logs in with it, and then
// has the user change its own password.
//
// CrateDB reads a string literal the way PostgreSQL does with
// standard_conforming_strings on, and it reports the setting, so the quoting
// state comes from the server as it does for PostgreSQL. A user can change
// its own password without naming the current one.
func TestChangePasswordCrateDB(t *testing.T) {
	db := openCrateDB(t)
	q, err := dbmeta.CrateDB.Quoting(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the quoting state: %v", err)
	}
	if !q.BackslashEscapes.Valid {
		t.Fatal("expected the server to report standard_conforming_strings")
	}
	const user = "dbmeta_pw"
	exec(t, db, `DROP USER IF EXISTS `+user)
	exec(t, db, `CREATE USER `+user+` WITH (password = 'Start-P4ss!x')`)
	t.Cleanup(func() { cleanup(t, db, `DROP USER IF EXISTS `+user) })
	base := dsnOf(t, "DBMETA_CRATEDB")
	current := "Start-P4ss!x"
	for _, c := range hostilePasswords {
		t.Run(c.name, func(t *testing.T) {
			stmt, err := dbmeta.CrateDB.ChangePassword(
				dbmeta.PasswordChange{User: user, Password: c.password}, q)
			if err != nil {
				t.Fatalf("building the statement: %v", err)
			}
			exec(t, db, stmt)
			login(t, "pgx", replaceUser(t, base, user, c.password), `SELECT current_user`, user)
			current = c.password
		})
	}

	self := openAt(t, "pgx", replaceUser(t, base, user, current))
	const next = `Next-P4ss!x"'\`
	stmt, err := dbmeta.CrateDB.ChangePassword(dbmeta.PasswordChange{User: user, Password: next}, q)
	if err != nil {
		t.Fatalf("building the statement: %v", err)
	}
	if _, err := self.ExecContext(t.Context(), stmt); err != nil {
		t.Fatalf("changing its own password: %v", err)
	}
	login(t, "pgx", replaceUser(t, base, user, next), `SELECT current_user`, user)
}

// TestChangePasswordClickHouse sets each password and logs in with it, and
// then sets one for a user whose name holds a backslash and a backtick.
//
// A backslash escapes the next character in a ClickHouse string literal and
// inside backticks, whatever the session says, so the model doubles it in
// both. A name that ends in a backslash is the case that can escape its own
// closing backtick.
func TestChangePasswordClickHouse(t *testing.T) {
	db := openClickHouse(t)
	base := dsnOf(t, "DBMETA_CLICKHOUSE")
	for _, u := range []struct{ name, create string }{
		{"dbmeta_pw", "`dbmeta_pw`"},
		{"dbmeta`pw\\", "`dbmeta``pw\\\\`"},
	} {
		t.Run(u.name, func(t *testing.T) {
			exec(t, db, `DROP USER IF EXISTS `+u.create)
			exec(t, db, `CREATE USER `+u.create+` IDENTIFIED BY 'Start-P4ss!x'`)
			t.Cleanup(func() { cleanup(t, db, `DROP USER IF EXISTS `+u.create) })
			for _, c := range hostilePasswords {
				t.Run(c.name, func(t *testing.T) {
					stmt, err := dbmeta.ClickHouse.ChangePassword(
						dbmeta.PasswordChange{User: u.name, Password: c.password}, dbmeta.Quoting{})
					if err != nil {
						t.Fatalf("building the statement: %v", err)
					}
					exec(t, db, stmt)
					login(t, "clickhouse", replaceUser(t, base, u.name, c.password), `SELECT currentUser()`, u.name)
				})
			}
		})
	}
}

// TestChangePasswordCassandra sets each password and logs in with it.
//
// CQL has no backslash escape, so only the quote is doubled, and CQL has no
// function that names the current role, so a login is proved by a read.
//
// Cassandra 5 refuses to change a role's password within five seconds of
// the last change, and counts from when the role was made. So each password
// gets a role of its own, all of them are made first, and the test waits
// once before it changes any.
func TestChangePasswordCassandra(t *testing.T) {
	db := openCassandra(t)
	base := dsnOf(t, "DBMETA_CQL")
	users := make([]string, len(hostilePasswords))
	for i := range hostilePasswords {
		users[i] = fmt.Sprintf("dbmeta_pw%d", i)
		cleanup(t, db, `DROP ROLE IF EXISTS `+users[i])
		exec(t, db, `CREATE ROLE `+users[i]+` WITH PASSWORD = 'Start-P4ss!x' AND LOGIN = true`)
		t.Cleanup(func() { cleanup(t, db, `DROP ROLE IF EXISTS `+users[i]) })
	}
	time.Sleep(6 * time.Second)
	for i, c := range hostilePasswords {
		t.Run(c.name, func(t *testing.T) {
			stmt, err := dbmeta.Cassandra.ChangePassword(
				dbmeta.PasswordChange{User: users[i], Password: c.password}, dbmeta.Quoting{})
			if err != nil {
				t.Fatalf("building the statement: %v", err)
			}
			exec(t, db, stmt)
			login(t, "cql", cqlUser(t, base, users[i], c.password), `SELECT release_version FROM system.local`, "")
		})
	}
}

// TestChangePasswordOracle sets each password and logs in with it, for a user
// named plainly and one named in lower case between double quotes, and then
// has the plain user change its own password with the current one.
//
// Oracle takes the password between double quotes and has no escape for one,
// so the password that holds one is refused rather than set.
func TestChangePasswordOracle(t *testing.T) {
	db := openOracle(t)
	base := dsnOf(t, "DBMETA_ORACLE")
	for _, u := range []struct{ name, create, stored string }{
		{"dbmeta_pw", "dbmeta_pw", "DBMETA_PW"},
		{`"dbmeta_qpw"`, `"dbmeta_qpw"`, "dbmeta_qpw"},
	} {
		t.Run(u.stored, func(t *testing.T) {
			cleanup(t, db, `DROP USER `+u.create)
			exec(t, db, `CREATE USER `+u.create+` IDENTIFIED BY "Start-P4ss!x"`)
			t.Cleanup(func() { cleanup(t, db, `DROP USER `+u.create) })
			exec(t, db, `GRANT CREATE SESSION TO `+u.create)
			current := "Start-P4ss!x"
			for _, c := range hostilePasswords {
				t.Run(c.name, func(t *testing.T) {
					stmt, err := dbmeta.Oracle.ChangePassword(
						dbmeta.PasswordChange{User: u.name, Password: c.password}, dbmeta.Quoting{})
					if strings.Contains(c.password, `"`) {
						if !errors.Is(err, dbmeta.ErrInvalidPassword) {
							t.Errorf("expected %v for a double quote, got %v", dbmeta.ErrInvalidPassword, err)
						}
						return
					}
					if err != nil {
						t.Fatalf("building the statement: %v", err)
					}
					exec(t, db, stmt)
					// A login names the user the way a statement does, so a
					// name in lower case is between double quotes there too.
					login(t, "oracle", replaceUser(t, base, u.name, c.password), `SELECT USER FROM dual`, u.stored)
					current = c.password
				})
			}
			// The user changing its own password, with the current one.
			self := openAt(t, "oracle", replaceUser(t, base, u.name, current))
			const next = `Next-P4ss!x'\`
			stmt, err := dbmeta.Oracle.ChangePassword(
				dbmeta.PasswordChange{User: u.name, Password: next, Old: current}, dbmeta.Quoting{})
			if err != nil {
				t.Fatalf("building the statement: %v", err)
			}
			if _, err := self.ExecContext(t.Context(), stmt); err != nil {
				t.Fatalf("changing its own password: %v", err)
			}
			login(t, "oracle", replaceUser(t, base, u.name, next), `SELECT USER FROM dual`, u.stored)
		})
	}
}

// TestChangePasswordDatabend sets each password and logs in with it. Databend
// names a user with a string literal, and a backslash escapes the next
// character in one, so a user whose name has a backslash is changed too.
// Databend refuses a quote in a user name.
func TestChangePasswordDatabend(t *testing.T) {
	db := openDatabend(t)
	base := dsnOf(t, "DBMETA_DATABEND")
	for _, u := range []struct{ name, create string }{
		{"dbmeta_pw", `'dbmeta_pw'`},
		{`dbmeta\pw`, `'dbmeta\\pw'`},
	} {
		t.Run(u.name, func(t *testing.T) {
			cleanup(t, db, `DROP USER IF EXISTS `+u.create)
			exec(t, db, `CREATE USER `+u.create+` IDENTIFIED BY 'Start-P4ss!x'`)
			t.Cleanup(func() { cleanup(t, db, `DROP USER IF EXISTS `+u.create) })
			for _, c := range hostilePasswords {
				t.Run(c.name, func(t *testing.T) {
					stmt, err := dbmeta.Databend.ChangePassword(
						dbmeta.PasswordChange{User: u.name, Password: c.password}, dbmeta.Quoting{})
					if err != nil {
						t.Fatalf("building the statement: %v", err)
					}
					exec(t, db, stmt)
					login(t, "databend", replaceUser(t, base, u.name, c.password), `SELECT current_user()`, "pw")
				})
			}
		})
	}
}

// TestChangePasswordSingleStore sets each password and logs in with it.
// SingleStore takes MySQL's statement, which the mysql model builds.
func TestChangePasswordSingleStore(t *testing.T) {
	db := openSingleStore(t)
	changePasswordMySQLFamily(t, dbmeta.MemSQL, db, "DBMETA_MEMSQL")
}
