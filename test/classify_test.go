package test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/xo/dbmeta"
)

// classifyProbe is a table and the statements that run against it. The
// server decides whether each statement returns rows, by whether the result
// has columns, and the test holds Classify to that (D264).
type classifyProbe struct {
	dialect dbmeta.Dialect
	env     string
	// dsn is used when env is not set, for a database that is a library.
	dsn     string
	drivers []string
	setup   []string
	cleanup []string
	stmts   []string
}

// returning holds the statements of a product with a RETURNING clause.
func returning() []string {
	table := "dbm_cls"
	return []string{
		"INSERT INTO " + table + " (a) VALUES (1) RETURNING a",
		"INSERT INTO " + table + " (a) VALUES (2)",
		"UPDATE " + table + " SET a = 3 WHERE a = 1 RETURNING a",
		"UPDATE " + table + " SET a = 4 WHERE a = 3",
		"DELETE FROM " + table + " WHERE a = 4 RETURNING a",
		"DELETE FROM " + table + " WHERE a = 2",
		"INSERT INTO " + table + " (a) VALUES (5) -- RETURNING a",
		"INSERT INTO " + table + " (a) VALUES (6) /* RETURNING a */",
		"WITH x AS (SELECT 7 AS n) INSERT INTO " + table + " (a) SELECT n FROM x",
		"WITH x AS (SELECT 8 AS n) INSERT INTO " + table + " (a) SELECT n FROM x RETURNING a",
		"WITH x AS (SELECT 9 AS n) SELECT n FROM x",
		"SELECT 1 AS a",
	}
}

var classifyProbes = []classifyProbe{
	{
		dialect: dbmeta.PostgreSQL, env: "DBMETA_POSTGRES", drivers: []string{"pgx", "postgres"},
		setup: []string{"CREATE TEMPORARY TABLE dbm_cls (a integer)"},
		stmts: append(returning(),
			"WITH x AS (INSERT INTO dbm_cls (a) VALUES (10) RETURNING a) SELECT a FROM x",
			"WITH x AS (INSERT INTO dbm_cls (a) VALUES (11) RETURNING a) INSERT INTO dbm_cls (a) SELECT a FROM x",
			"SELECT a INTO TEMPORARY dbm_cls2 FROM dbm_cls",
			"VALUES (1), (2)",
			"TABLE dbm_cls",
			"SHOW server_version",
			"EXPLAIN SELECT 1",
			"SET search_path = public",
			"MERGE INTO dbm_cls t USING (SELECT 5 AS a) s ON t.a = s.a WHEN MATCHED THEN DELETE RETURNING t.a",
			"MERGE INTO dbm_cls t USING (SELECT 5 AS a) s ON t.a = s.a WHEN MATCHED THEN DELETE"),
	},
	{
		dialect: dbmeta.CockroachDB, env: "DBMETA_COCKROACHDB", drivers: []string{"pgx"},
		setup:   []string{"CREATE TABLE dbm_cls (a integer)"},
		cleanup: []string{"DROP TABLE IF EXISTS dbm_cls"},
		stmts: append(returning(),
			"UPSERT INTO dbm_cls (a) VALUES (12) RETURNING a",
			"UPSERT INTO dbm_cls (a) VALUES (13)"),
	},
	{
		dialect: dbmeta.SQLite3, dsn: ":memory:", drivers: []string{"sqlite3", "sqlite"},
		setup: []string{"CREATE TEMPORARY TABLE dbm_cls (a integer)"},
		stmts: append(returning(),
			"PRAGMA table_info(dbm_cls)",
			"PRAGMA user_version = 1",
			"REPLACE INTO dbm_cls (a) VALUES (14)",
			"REPLACE INTO dbm_cls (a) VALUES (15) RETURNING a"),
	},
	{
		dialect: dbmeta.DuckDB, drivers: []string{"duckdb"},
		setup: []string{"CREATE TEMPORARY TABLE dbm_cls (a integer)"},
		stmts: append(returning(), "FROM dbm_cls", "SUMMARIZE dbm_cls"),
	},
	{
		dialect: dbmeta.MySQL, env: "DBMETA_MYSQL", drivers: []string{"mysql"},
		setup: []string{"CREATE TEMPORARY TABLE dbm_cls (a integer)"},
		// MariaDB has no RETURNING on UPDATE, so that case is not here.
		stmts: []string{
			"INSERT INTO dbm_cls (a) VALUES (1) RETURNING a",
			"INSERT INTO dbm_cls (a) VALUES (2)",
			"UPDATE dbm_cls SET a = 4 WHERE a = 1",
			"DELETE FROM dbm_cls WHERE a = 4 RETURNING a",
			"DELETE FROM dbm_cls WHERE a = 2",
			"INSERT INTO dbm_cls (a) VALUES (5) # RETURNING a",
			"REPLACE INTO dbm_cls (a) VALUES (6) RETURNING a",
			"REPLACE INTO dbm_cls (a) VALUES (7)",
			"SELECT 1 AS a",
			"SELECT a INTO @v FROM dbm_cls LIMIT 1",
			"SHOW TABLES",
			"DESCRIBE dbm_cls",
		},
	},
	{
		dialect: dbmeta.Firebird, env: "DBMETA_FIREBIRDSQL", drivers: []string{"firebirdsql"},
		setup:   []string{"RECREATE TABLE dbm_cls (a integer)"},
		cleanup: []string{"DROP TABLE dbm_cls"},
		stmts: []string{
			"INSERT INTO dbm_cls (a) VALUES (1) RETURNING a",
			"INSERT INTO dbm_cls (a) VALUES (2)",
			"UPDATE dbm_cls SET a = 3 WHERE a = 1 RETURNING a",
			"UPDATE dbm_cls SET a = 4 WHERE a = 3",
			"DELETE FROM dbm_cls WHERE a = 4 RETURNING a",
			"DELETE FROM dbm_cls WHERE a = 2",
			"SELECT 1 AS a FROM rdb$database",
		},
	},
	{
		dialect: dbmeta.SQLServer, env: "DBMETA_SQLSERVER", drivers: []string{"sqlserver"},
		setup: []string{"CREATE TABLE #cls (a int)", "CREATE TABLE #out (a int)"},
		stmts: []string{
			"INSERT INTO #cls (a) OUTPUT inserted.a VALUES (1)",
			"INSERT INTO #cls (a) VALUES (2)",
			"UPDATE #cls SET a = 3 OUTPUT deleted.a, inserted.a WHERE a = 1",
			"UPDATE #cls SET a = 4 WHERE a = 3",
			"DELETE FROM #cls OUTPUT deleted.a WHERE a = 4",
			"INSERT INTO #cls (a) OUTPUT inserted.a INTO #out VALUES (5)",
			"UPDATE #cls SET a = 6 OUTPUT inserted.a INTO #out WHERE a = 5",
			"DELETE FROM #cls OUTPUT deleted.a INTO #out WHERE a = 6",
			"INSERT INTO #cls (a) VALUES (7) -- OUTPUT inserted.a",
			"MERGE #cls AS t USING (SELECT 7 AS a) AS s ON t.a = s.a WHEN MATCHED THEN UPDATE SET a = 8 OUTPUT inserted.a;",
			"MERGE #cls AS t USING (SELECT 8 AS a) AS s ON t.a = s.a WHEN MATCHED THEN UPDATE SET a = 9;",
			"WITH x AS (SELECT 1 AS n) DELETE FROM #cls OUTPUT deleted.a",
			"EXEC sp_executesql N'SELECT 1 AS a'",
			"EXECUTE sp_executesql N'SELECT 1 AS a'",
			"SELECT a INTO #cls2 FROM #cls",
			"SELECT 1 AS a",
		},
	},
	{
		dialect: dbmeta.Oracle, env: "DBMETA_ORACLE", drivers: []string{"oracle"},
		setup:   []string{"CREATE TABLE dbm_cls (a number)"},
		cleanup: []string{"DROP TABLE dbm_cls PURGE"},
		// RETURNING INTO needs an output variable, so only the statements that
		// take none are here.
		stmts: []string{
			"INSERT INTO dbm_cls (a) VALUES (1)",
			"UPDATE dbm_cls SET a = 2",
			"DELETE FROM dbm_cls",
			"SELECT 1 AS a FROM dual",
			"WITH x AS (SELECT 1 AS n FROM dual) SELECT n FROM x",
		},
	},
}

func TestClassifyIsMeasured(t *testing.T) {
	for _, p := range classifyProbes {
		for _, driver := range p.drivers {
			t.Run(string(p.dialect)+"/"+driver, func(t *testing.T) {
				dsn := p.dsn
				if p.env != "" {
					if dsn = os.Getenv(p.env); dsn == "" {
						t.Skipf("set %s to run against a server", p.env)
					}
				}
				measureClassify(t, p, driver, dsn)
			})
		}
	}
}

func measureClassify(t *testing.T, p classifyProbe, driver, dsn string) {
	t.Helper()
	ctx := context.WithoutCancel(t.Context())
	db := openAt(t, driver, dsn)
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	defer conn.Close()
	for _, q := range p.cleanup {
		conn.ExecContext(ctx, q)       //nolint:errcheck // what an earlier run left is not a fault
		defer conn.ExecContext(ctx, q) //nolint:errcheck // a cleanup at the end is best effort
	}
	for _, q := range p.setup {
		if _, err := conn.ExecContext(ctx, q); err != nil {
			t.Fatalf("making the probe table: %v\n%s", err, q)
		}
	}
	for _, q := range p.stmts {
		rows, err := conn.QueryContext(ctx, q)
		if err != nil {
			t.Errorf("%s: the server refused it: %v", q, err)
			continue
		}
		cols, err := readColumns(rows)
		if err != nil {
			t.Errorf("%s: reading the result: %v", q, err)
			continue
		}
		tag, got := p.dialect.Classify(q)
		// DuckDB answers an INSERT, UPDATE or DELETE with one column, Count.
		want := len(cols) > 0 && (p.dialect != dbmeta.DuckDB || len(cols) != 1 || cols[0] != "Count")
		if got != want {
			t.Errorf("%s: Classify says %q and rows %t, and the server returned %d columns", q, tag, got, len(cols))
		}
	}
}

// readColumns reads every row of a result and returns its column names.
func readColumns(rows *sql.Rows) ([]string, error) {
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	for rows.Next() {
	}
	return cols, rows.Err()
}
