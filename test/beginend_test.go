package test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
)

// beginEndProbe is one stored program whose body sits between BEGIN and END,
// holds a semicolon, and has no quotes around it. whole is sent to the server
// as one statement.
type beginEndProbe struct {
	// setup makes what the program needs, such as a table for a trigger.
	setup []string
	// whole is the CREATE statement.
	whole string
	// cleanup removes what the probe made.
	cleanup []string
	// tooOld says why the server is too old for the form, or returns an empty
	// string. The model is true when some release takes the form, so a
	// release before it is skipped and not held to the model.
	tooOld func(ctx context.Context, conn *sql.Conn) (string, error)
}

// pgBeginAtomic is PostgreSQL's release 14, which added BEGIN ATOMIC.
func pgBeginAtomic(ctx context.Context, conn *sql.Conn) (string, error) {
	var n int
	if err := conn.QueryRowContext(ctx, "SELECT current_setting('server_version_num')::int").Scan(&n); err != nil {
		return "", fmt.Errorf("reading the release: %w", err)
	}
	if n < 140000 {
		return fmt.Sprintf("BEGIN ATOMIC came in PostgreSQL 14, and this is %d", n), nil
	}
	return "", nil
}

// cockroachBeginAtomic is CockroachDB 26.3, the first release that took it.
func cockroachBeginAtomic(ctx context.Context, conn *sql.Conn) (string, error) {
	var v string
	if err := conn.QueryRowContext(ctx, "SELECT version()").Scan(&v); err != nil {
		return "", fmt.Errorf("reading the release: %w", err)
	}
	var major, minor int
	if _, rest, ok := strings.Cut(v, " v"); ok {
		if _, err := fmt.Sscanf(rest, "%d.%d", &major, &minor); err == nil && (major < 26 || major == 26 && minor < 3) {
			return "BEGIN ATOMIC came in CockroachDB 26.3, and this is " + v, nil
		}
	}
	return "", nil
}

// beginEndProbes holds one probe for each dialect that a server here can
// answer for. The expected answer is never written here. It is
// Syntax.BeginEndBlocks of the model, so the test holds the model to the
// server (D249).
var beginEndProbes = map[dbmeta.Dialect]beginEndProbe{
	dbmeta.SQLite3: sqliteTrigger,
	dbmeta.LibSQL:  sqliteTrigger,
	dbmeta.Rqlite:  sqliteTrigger,
	dbmeta.MySQL:   mysqlProcedure,
	dbmeta.TiDB:    mysqlProcedure,
	// SingleStore writes the keyword AS before BEGIN, and echoes rows with ECHO.
	dbmeta.MemSQL: {
		setup:   []string{"CREATE DATABASE IF NOT EXISTS dbm_d", "USE dbm_d"},
		whole:   "CREATE PROCEDURE dbm_p() AS BEGIN ECHO SELECT 1; ECHO SELECT 2; END",
		cleanup: []string{"DROP DATABASE IF EXISTS dbm_d"},
	},
	dbmeta.Vitess: mysqlProcedure,
	dbmeta.SQLServer: {
		whole:   "CREATE PROCEDURE dbm_p AS BEGIN SELECT 1; SELECT 2; END",
		cleanup: []string{"DROP PROCEDURE IF EXISTS dbm_p"},
	},
	dbmeta.Oracle: {
		whole:   "CREATE OR REPLACE PROCEDURE dbm_begin_p AS n NUMBER; BEGIN n := 1; n := 2; END;",
		cleanup: []string{"DROP PROCEDURE dbm_begin_p"},
	},
	// PostgreSQL 14 added a function body between BEGIN ATOMIC and END.
	dbmeta.PostgreSQL: {
		whole:   "CREATE FUNCTION dbm_f() RETURNS integer LANGUAGE sql BEGIN ATOMIC SELECT 1; SELECT 2; END",
		cleanup: []string{"DROP FUNCTION IF EXISTS dbm_f()"},
		tooOld:  pgBeginAtomic,
	},
	dbmeta.CockroachDB: {
		whole:   "CREATE FUNCTION dbm_f() RETURNS integer LANGUAGE sql BEGIN ATOMIC SELECT 1; SELECT 2; END",
		cleanup: []string{"DROP FUNCTION IF EXISTS dbm_f()"},
		tooOld:  cockroachBeginAtomic,
	},
	dbmeta.CrateDB: {
		whole: "CREATE FUNCTION dbm_f() RETURNS integer LANGUAGE sql BEGIN ATOMIC SELECT 1; SELECT 2; END",
	},
	dbmeta.Firebird: {
		whole:   "CREATE PROCEDURE dbm_p AS DECLARE i INTEGER; BEGIN i = 1; i = 2; END",
		cleanup: []string{"DROP PROCEDURE dbm_p"},
	},
	dbmeta.HANA: {
		whole:   "CREATE PROCEDURE dbm_p() AS BEGIN SELECT 1 FROM DUMMY; SELECT 2 FROM DUMMY; END",
		cleanup: []string{"DROP PROCEDURE dbm_p"},
	},
	dbmeta.Exasol: {
		setup:   []string{"CREATE SCHEMA IF NOT EXISTS dbm_s", "OPEN SCHEMA dbm_s"},
		whole:   "CREATE OR REPLACE FUNCTION dbm_f() RETURN INT IS res INT; BEGIN res := 1; RETURN res; END dbm_f;",
		cleanup: []string{"DROP SCHEMA IF EXISTS dbm_s CASCADE"},
	},
	dbmeta.Vertica: {
		whole:   "CREATE PROCEDURE dbm_p() AS BEGIN SELECT 1; SELECT 2; END",
		cleanup: []string{"DROP PROCEDURE IF EXISTS dbm_p()"},
	},
	dbmeta.Trino: {
		whole:   "CREATE FUNCTION memory.default.dbm_f() RETURNS integer BEGIN DECLARE x integer DEFAULT 1; RETURN x; END",
		cleanup: []string{"DROP FUNCTION IF EXISTS memory.default.dbm_f()"},
	},
	dbmeta.Presto: {
		whole:   "CREATE FUNCTION dbm_f() RETURNS integer BEGIN DECLARE x integer DEFAULT 1; RETURN x; END",
		cleanup: []string{"DROP FUNCTION IF EXISTS dbm_f()"},
	},
	dbmeta.ClickHouse: {
		whole: "CREATE PROCEDURE dbm_p() AS BEGIN SELECT 1; SELECT 2; END",
	},
	dbmeta.DuckDB: {
		whole: "CREATE PROCEDURE dbm_p() AS BEGIN SELECT 1; SELECT 2; END",
	},
	dbmeta.Databend: {
		whole:   "CREATE PROCEDURE dbm_p() RETURNS int LANGUAGE SQL AS BEGIN LET x := 1; RETURN x; END",
		cleanup: []string{"DROP PROCEDURE IF EXISTS dbm_p()"},
	},
}

var (
	sqliteTrigger = beginEndProbe{
		setup:   []string{"CREATE TABLE dbm_t (a integer)"},
		whole:   "CREATE TRIGGER dbm_trg AFTER INSERT ON dbm_t BEGIN UPDATE dbm_t SET a = 1; DELETE FROM dbm_t WHERE a = 2; END",
		cleanup: []string{"DROP TABLE IF EXISTS dbm_t"},
	}
	mysqlProcedure = beginEndProbe{
		whole:   "CREATE PROCEDURE dbm_p() BEGIN SELECT 1; SELECT 2; END",
		cleanup: []string{"DROP PROCEDURE IF EXISTS dbm_p"},
	}
)

// TestBeginEndBlocksIsMeasured sends a stored program with an unquoted body
// that holds a semicolon, as one statement, to every server that is running.
// The server accepts it or refuses it, and the answer must be what
// Syntax.BeginEndBlocks of the model says (D249). The piece before the first
// semicolon is sent alone first, and the answer is logged, so the record shows what a
// client sends when it splits there.
func TestBeginEndBlocksIsMeasured(t *testing.T) {
	for _, tg := range alignTargets {
		probe, ok := beginEndProbes[tg.dialect]
		if !ok {
			continue
		}
		for _, driver := range tg.drivers {
			t.Run(string(tg.dialect)+"/"+driver, func(t *testing.T) {
				dsn, ok := tg.dsns[driver]
				if !ok {
					if dsn = os.Getenv(tg.env); dsn == "" {
						t.Skipf("set %s to run against a server", tg.env)
					}
				}
				if tg.setup != nil {
					undo, err := tg.setup(t.Context(), dsn)
					if err != nil {
						t.Fatalf("setting up the probe: %v", err)
					}
					defer undo()
				}
				if tg.prep != nil {
					var err error
					if dsn, err = tg.prep(t.Context(), dsn); err != nil {
						t.Fatalf("preparing the DSN: %v", err)
					}
				}
				measureBeginEnd(t, tg, probe, driver, dsn)
			})
		}
	}
}

func measureBeginEnd(t *testing.T, tg alignTarget, probe beginEndProbe, driver, dsn string) {
	t.Helper()
	ctx := context.WithoutCancel(t.Context())
	syntax, ok := tg.dialect.Syntax()
	if !ok {
		t.Fatalf("%s has no model", tg.dialect)
	}
	db := openAt(t, driver, dsn)
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	defer conn.Close()
	if probe.tooOld != nil {
		why, err := probe.tooOld(ctx, conn)
		if err != nil {
			t.Fatal(err)
		}
		if why != "" {
			t.Skip(why)
		}
	}
	for _, q := range tg.before {
		if _, err := conn.ExecContext(ctx, q); err != nil {
			t.Fatalf("preparing the session: %v", err)
		}
	}
	for _, q := range tg.after {
		defer conn.ExecContext(ctx, q) //nolint:errcheck // a cleanup at the end is best effort
	}
	for _, q := range probe.cleanup {
		//nolint:errcheck // what an earlier run left is not a fault
		conn.ExecContext(ctx, q)
		defer conn.ExecContext(ctx, q) //nolint:errcheck // a cleanup at the end is best effort
	}
	for _, q := range probe.setup {
		if _, err := conn.ExecContext(ctx, q); err != nil {
			t.Fatalf("making what the program needs: %v", err)
		}
	}
	piece, _, _ := strings.Cut(probe.whole, ";")
	if _, perr := conn.ExecContext(ctx, piece+";"); perr == nil {
		t.Logf("the piece before the first semicolon is accepted alone: %q", piece)
	} else {
		t.Logf("the piece before the first semicolon is refused alone: %v", perr)
	}

	_, werr := conn.ExecContext(ctx, probe.whole)
	if accepted := werr == nil; accepted != syntax.BeginEndBlocks {
		t.Errorf("the server accepts the whole program is %t (%v), and the model says %t", accepted, werr, syntax.BeginEndBlocks)
	}
	t.Logf("whole program: accepted=%t err=%v", werr == nil, werr)
}

// TestOracleRunsAnonymousBlocks sends each block that starts with BEGIN or
// DECLARE as one statement, and holds Syntax.AnonymousBlocks of the model to
// the answer (D252). A slash on a line of its own is not sent, because it is
// a word of SQL*Plus and the server never reads it.
func TestOracleRunsAnonymousBlocks(t *testing.T) {
	dsn := os.Getenv("DBMETA_ORACLE")
	if dsn == "" {
		t.Skip("set DBMETA_ORACLE to run against a server")
	}
	syntax, _ := dbmeta.Oracle.Syntax()
	if !syntax.AnonymousBlocks {
		t.Fatal("the model says that Oracle has no anonymous block")
	}
	db := openAt(t, "oracle", dsn)
	for _, q := range []string{
		"begin\n  null;\n  commit;\nend;",
		"declare x number;\nbegin\n  x := 1;\nend;",
		"begin\n  for i in 1..2 loop\n    null;\n  end loop;\n  null;\nend;",
	} {
		if _, err := db.ExecContext(t.Context(), q); err != nil {
			t.Errorf("the server refuses %q: %v", q, err)
		}
	}
	// The piece before the first semicolon is not a block.
	if _, err := db.ExecContext(t.Context(), "begin\n  null;"); err == nil {
		t.Error("the server accepted half a block")
	}
}
