package dbmeta_test

import (
	"testing"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/duckdb"
	_ "github.com/xo/dbmeta/models/firebird"
	_ "github.com/xo/dbmeta/models/hive"
	_ "github.com/xo/dbmeta/models/impala"
	_ "github.com/xo/dbmeta/models/mysql"
	_ "github.com/xo/dbmeta/models/neo4j"
	_ "github.com/xo/dbmeta/models/oracle"
	_ "github.com/xo/dbmeta/models/postgres"
	_ "github.com/xo/dbmeta/models/spanner"
	_ "github.com/xo/dbmeta/models/sqlite3"
	_ "github.com/xo/dbmeta/models/sqlserver"
)

// TestClassify holds the tag and the rows of statements, for each dialect
// that reads a statement differently. See D264.
func TestClassify(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		d    dbmeta.Dialect
		stmt string
		tag  string
		rows bool
	}{
		// the cases that usql's QueryExecType held
		{dbmeta.PostgreSQL, "", "EXEC", false},
		{dbmeta.PostgreSQL, "select 1", "SELECT", true},
		{dbmeta.PostgreSQL, "  -- a comment\n select 1", "SELECT", true},
		{dbmeta.PostgreSQL, "(select 1) union (select 2)", "SELECT", true},
		{dbmeta.PostgreSQL, "values (1), (2)", "VALUES", true},
		{dbmeta.PostgreSQL, "table books", "TABLE", true},
		{dbmeta.PostgreSQL, "show server_version", "SHOW", true},
		{dbmeta.PostgreSQL, "explain select 1", "EXPLAIN", true},
		{dbmeta.PostgreSQL, "fetch 5 from c", "FETCH", true},
		{dbmeta.PostgreSQL, "call p()", "CALL", true},
		{dbmeta.PostgreSQL, "select * into t2 from t1", "SELECT INTO", false},
		{dbmeta.PostgreSQL, "select (select 1 into) from t", "SELECT", true},
		{dbmeta.PostgreSQL, "select 'into' as \"into\" from t", "SELECT", true},
		{dbmeta.PostgreSQL, "create table t (a int)", "CREATE TABLE", false},
		{dbmeta.PostgreSQL, "create or replace function f() returns int as $$ select 1 $$", "CREATE FUNCTION", false},
		{dbmeta.PostgreSQL, "create unique index i on t (a)", "CREATE INDEX", false},
		{dbmeta.PostgreSQL, "create temp table t as select 1", "CREATE TABLE", false},
		{dbmeta.PostgreSQL, "create text search parser p (start = s)", "CREATE TEXT SEARCH PARSER", false},
		{dbmeta.PostgreSQL, "drop procedural language plpgsql", "DROP LANGUAGE", false},
		{dbmeta.PostgreSQL, "insert into t values (1)", "INSERT", false},
		{dbmeta.PostgreSQL, "update t set a = 1", "UPDATE", false},
		{dbmeta.PostgreSQL, "delete from t", "DELETE", false},
		{dbmeta.PostgreSQL, "begin", "BEGIN", false},
		{dbmeta.PostgreSQL, "rollback to savepoint s", "ROLLBACK TO SAVEPOINT", false},
		{dbmeta.PostgreSQL, "set search_path = x", "SET", false},
		{dbmeta.PostgreSQL, "set", "SET", false},
		{dbmeta.PostgreSQL, "execute p", "EXECUTE", false},
		{dbmeta.PostgreSQL, "exec p", "EXEC", true},
		{dbmeta.PostgreSQL, "merge into t using s on true when matched then delete", "MERGE", false},
		{dbmeta.PostgreSQL, "unknown thing", "UNKNOWN", false},
		{dbmeta.SQLite3, "pragma table_info(t)", "PRAGMA", true},
		{dbmeta.SQLite3, "pragma journal_mode = wal", "PRAGMA", false},
		{dbmeta.SQLite3, "pragma x = 'a'", "PRAGMA", false},
		// RETURNING
		{dbmeta.PostgreSQL, "insert into t values (1) returning id", "INSERT", true},
		{dbmeta.PostgreSQL, "UPDATE t SET a = 1 RETURNING *", "UPDATE", true},
		{dbmeta.PostgreSQL, "delete from t returning a, b", "DELETE", true},
		{dbmeta.PostgreSQL, "merge into t using s on true when matched then delete returning t.*", "MERGE", true},
		{dbmeta.PostgreSQL, "insert into t values ('returning')", "INSERT", false},
		{dbmeta.PostgreSQL, `insert into t ("returning") values (1)`, "INSERT", false},
		{dbmeta.PostgreSQL, "insert into t values (1) -- returning id", "INSERT", false},
		{dbmeta.PostgreSQL, "insert into t values (1) /* returning id */", "INSERT", false},
		{dbmeta.PostgreSQL, "insert into t select a from (select 1 as a) x", "INSERT", false},
		{dbmeta.PostgreSQL, "create function f() returns int as $$ insert into t values (1) returning id $$ language sql", "CREATE FUNCTION", false},
		{dbmeta.PostgreSQL, "with x as (insert into t values (1) returning id) select * from x", "WITH", true},
		{dbmeta.PostgreSQL, "with x as (select 1) insert into t select * from x", "INSERT", false},
		{dbmeta.PostgreSQL, "with x as (select 1) insert into t select * from x returning a", "INSERT", true},
		{dbmeta.PostgreSQL, "with recursive x(n) as (select 1 union all select n + 1 from x) select n from x", "WITH", true},
		{dbmeta.PostgreSQL, "with x as (select 1) select * into t from x", "SELECT INTO", false},
		{dbmeta.PostgreSQL, "explain analyze insert into t values (1) returning id", "EXPLAIN", true},
		{dbmeta.SQLite3, "insert into t values (1) returning id", "INSERT", true},
		{dbmeta.SQLite3, "insert into t ([returning]) values (1)", "INSERT", false},
		{dbmeta.SQLite3, "update t set `returning` = 1", "UPDATE", false},
		{dbmeta.DuckDB, "insert into t values (1) returning *", "INSERT", true},
		{dbmeta.MySQL, "insert into t values (1) returning id", "INSERT", true},
		{dbmeta.MySQL, "delete from t returning a", "DELETE", true},
		{dbmeta.MySQL, "replace into t values (1) returning a", "REPLACE", true},
		{dbmeta.MySQL, "insert into t values (1) # returning id", "INSERT", false},
		{dbmeta.MySQL, "update t set `returning` = 1", "UPDATE", false},
		{dbmeta.Firebird, "insert into t values (1) returning id", "INSERT", true},
		{dbmeta.Spanner, "insert into t (a) values (1) then return a", "INSERT", true},
		{dbmeta.Spanner, "insert into t (a) values (1)", "INSERT", false},
		// Oracle puts the rows in variables
		{dbmeta.Oracle, "insert into t values (1) returning id into :id", "INSERT", false},
		{dbmeta.Oracle, "update t set a = 1 returning a into :a", "UPDATE", false},
		{dbmeta.Oracle, "select 1 from dual", "SELECT", true},
		{dbmeta.Oracle, "begin null; end;", "BEGIN", false},
		// SQL Server
		{dbmeta.SQLServer, "insert into t (a) output inserted.id values (1)", "INSERT", true},
		{dbmeta.SQLServer, "update t set a = 1 output deleted.a, inserted.a where id = 1", "UPDATE", true},
		{dbmeta.SQLServer, "delete from t output deleted.*", "DELETE", true},
		{dbmeta.SQLServer, "merge t using s on t.id = s.id when matched then delete output $action;", "MERGE", true},
		{dbmeta.SQLServer, "insert into t (a) output inserted.a into @tab values (1)", "INSERT", false},
		{dbmeta.SQLServer, "update t set a = 1 output inserted.a into @tab", "UPDATE", false},
		{dbmeta.SQLServer, "delete from t output deleted.* into audit", "DELETE", false},
		{dbmeta.SQLServer, "update t set a = 1 output inserted.a into @tab output inserted.a", "UPDATE", true},
		{dbmeta.SQLServer, "update t set [output] = 1", "UPDATE", false},
		{dbmeta.SQLServer, "insert into t values ('output')", "INSERT", false},
		{dbmeta.SQLServer, "exec sp_who", "EXEC", true},
		{dbmeta.SQLServer, "execute sp_who", "EXECUTE", true},
		{dbmeta.SQLServer, "with x as (select 1 a) delete from t output deleted.*", "DELETE", true},
		{dbmeta.SQLServer, "select * into #t from t", "SELECT INTO", false},
		{dbmeta.SQLServer, "create procedure p as select 1", "CREATE PROCEDURE", false},
		// SET lists the settings
		{dbmeta.Impala, "set", "SET", true},
		{dbmeta.Impala, "set all", "SET", true},
		{dbmeta.Impala, "set;", "SET", true},
		{dbmeta.Impala, "set a = 1", "SET", false},
		{dbmeta.Hive, "SET", "SET", true},
		{dbmeta.Hive, "set -v", "SET", true},
		{dbmeta.Hive, "set hive.x=1", "SET", false},
		{dbmeta.PostgreSQL, "set all", "SET", false},
		// every statement is a query
		{dbmeta.Neo4j, "create (n)", "CREATE", true},
		{dbmeta.Neo4j, "", "EXEC", true},
		// a dialect with no model
		{dbmeta.Dialect("no-such-dialect"), "select 1", "SELECT", true},
		{dbmeta.Dialect("no-such-dialect"), "insert into t values (1) returning id", "INSERT", false},
		// the two word key that usql never matched
		{dbmeta.PostgreSQL, "admin show ddl jobs", "ADMIN SHOW", true},
	} {
		tag, rows := tt.d.Classify(tt.stmt)
		if tag != tt.tag || rows != tt.rows {
			t.Errorf("%s: %q: got %q and %t, want %q and %t", tt.d, tt.stmt, tag, rows, tt.tag, tt.rows)
		}
	}
}
