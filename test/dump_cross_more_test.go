package test

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"testing"

	mysqldriver "github.com/go-sql-driver/mysql"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
	_ "github.com/xo/dbmeta/models/cockroachdb"
	_ "github.com/xo/dbmeta/models/libsql"
	_ "github.com/xo/dbmeta/models/rqlite"
	_ "github.com/xo/dbmeta/models/singlestore"
	_ "github.com/xo/dbmeta/models/sqlserver"
	_ "github.com/xo/dbmeta/models/tidb"
	_ "github.com/xo/dbmeta/models/vitess"
)

// crossSQLServer is the schema of the cross tests, in the types of SQL Server.
var crossSQLServer = []string{
	"CREATE TABLE owner (id int PRIMARY KEY, name varchar(20) NOT NULL UNIQUE, active bit DEFAULT 1)",
	`CREATE TABLE item (id int PRIMARY KEY, owner_id int NOT NULL, price decimal(10,2), qty int DEFAULT 1,
		note varchar(40) DEFAULT 'x', made datetime2, born date, data varbinary(max), UNIQUE (owner_id, note),
		FOREIGN KEY (owner_id) REFERENCES owner (id) ON DELETE CASCADE)`,
	"CREATE INDEX item_note ON item (note)",
}

// crossSingleStore is the schema of the cross tests as SingleStore takes it. It has
// no foreign key and no unique key beyond the primary key.
var crossSingleStore = []string{
	"CREATE TABLE owner (id int PRIMARY KEY, name varchar(20) NOT NULL, active tinyint(1) DEFAULT 1)",
	`CREATE TABLE item (id int PRIMARY KEY, owner_id int NOT NULL, price decimal(10,2), qty int DEFAULT 1,
		note varchar(40) DEFAULT 'x', made datetime, born date, data blob)`,
	"CREATE INDEX item_note ON item (note)",
}

// extraCrossSides are the products that speak the SQL of another one, and the
// ones with a style of their own that came after the first four. A side is left
// out when its server is not running, so a run with fewer servers still passes.
func extraCrossSides() []crossSide {
	return []crossSide{
		{name: "cockroachdb", dialect: dbmeta.CockroachDB, create: crossPG,
			open: func(t *testing.T, role string) (*sql.DB, dump.Filter) {
				db := openFamilyWith(t, pgFamilies[1], "pgx")
				schema := map[string]string{"src": "dump_src", "dst": "dump_cross"}[role]
				drop := func() { _, _ = db.ExecContext(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") }
				drop()
				t.Cleanup(drop)
				return db, dump.Filter{Schemas: []string{schema}}
			}},
		{name: "sqlserver", dialect: dbmeta.SQLServer, create: crossSQLServer,
			open: func(t *testing.T, role string) (*sql.DB, dump.Filter) {
				name := "dbmeta_cross_" + role
				schema := "dbo"
				if role == "dst" {
					schema = "dump_cross"
				}
				return openSQLServerNamed(t, name), dump.Filter{Schemas: []string{schema}}
			}},
		{name: "rqlite", dialect: dbmeta.Rqlite, create: crossSQLite, shared: true,
			open: func(t *testing.T, _ string) (*sql.DB, dump.Filter) {
				db := openRqlite(t)
				return sharedTables(t, db), dump.Filter{Schemas: []string{"main"}}
			}},
		{name: "libsql", dialect: dbmeta.LibSQL, create: crossSQLite, shared: true,
			open: func(t *testing.T, _ string) (*sql.DB, dump.Filter) {
				db := openLibSQL(t)
				return sharedTables(t, db), dump.Filter{Schemas: []string{"main"}}
			}},
		{name: "singlestore", dialect: dbmeta.MemSQL, create: crossSingleStore, dropsUnique: true,
			open: func(t *testing.T, role string) (*sql.DB, dump.Filter) {
				name := "dbmeta_cross_" + role
				return openMySQLWireNamed(t, openSingleStore(t), "DBMETA_MEMSQL", name), dump.Filter{Schemas: []string{name}}
			}},
		{name: "vitess", dialect: dbmeta.Vitess, create: crossMySQL, shared: true,
			open: func(t *testing.T, _ string) (*sql.DB, dump.Filter) {
				db := openVitess(t)
				var schema string
				if err := db.QueryRowContext(t.Context(), "SELECT DATABASE()").Scan(&schema); err != nil || schema == "" {
					t.Skipf("the connection names no database: %v", err)
				}
				return sharedTables(t, db), dump.Filter{Schemas: []string{schema}}
			}},
		{name: "tidb", dialect: dbmeta.TiDB, create: crossMySQL,
			open: func(t *testing.T, role string) (*sql.DB, dump.Filter) {
				name := "dbmeta_cross_" + role
				return openTiDBNamed(t, name), dump.Filter{Schemas: []string{name}}
			}},
	}
}

// openTiDBNamed opens a connection to a database of the given name on the server
// that DBMETA_TIDB names, made empty and dropped at the end.
func openTiDBNamed(t *testing.T, name string) *sql.DB {
	t.Helper()
	return openMySQLWireNamed(t, openTiDB(t), "DBMETA_TIDB", name)
}

// sharedTables empties the two tables of the cross tests in a database that has
// no other, at the start and at the end.
func sharedTables(t *testing.T, db *sql.DB) *sql.DB {
	t.Helper()
	drop := func() {
		for _, s := range []string{"DROP VIEW IF EXISTS cheap", "DROP TABLE IF EXISTS item", "DROP TABLE IF EXISTS owner"} {
			_, _ = db.ExecContext(context.Background(), s)
		}
	}
	drop()
	t.Cleanup(drop)
	return db
}

// openMySQLWireNamed opens a connection to a database of the given name on a
// server that speaks the protocol of MySQL, which the variable env names, made
// empty and dropped at the end. admin is a connection to the server.
func openMySQLWireNamed(t *testing.T, admin *sql.DB, env, name string) *sql.DB {
	t.Helper()
	for _, s := range []string{"DROP DATABASE IF EXISTS " + name, "CREATE DATABASE " + name} {
		mustExec(t, admin, s)
	}
	t.Cleanup(func() { _, _ = admin.ExecContext(context.Background(), "DROP DATABASE IF EXISTS "+name) })
	cfg, err := mysqldriver.ParseDSN(os.Getenv(env))
	if err != nil {
		t.Fatalf("reading %s: %v", env, err)
	}
	cfg.DBName = name
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatalf("opening %s: %v", name, err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// openSQLServerNamed opens a connection to a database of the given name on the
// server that DBMETA_SQLSERVER names, made empty and dropped at the end.
func openSQLServerNamed(t *testing.T, name string) *sql.DB {
	t.Helper()
	admin := openSQLServer(t)
	drop := func(db *sql.DB) {
		_, _ = db.ExecContext(context.Background(), "IF DB_ID('"+name+"') IS NOT NULL BEGIN ALTER DATABASE "+name+
			" SET SINGLE_USER WITH ROLLBACK IMMEDIATE; DROP DATABASE "+name+" END")
	}
	drop(admin)
	mustExec(t, admin, "CREATE DATABASE "+name)
	t.Cleanup(func() { drop(admin) })
	u, err := url.Parse(os.Getenv("DBMETA_SQLSERVER"))
	if err != nil {
		t.Fatalf("reading DBMETA_SQLSERVER: %v", err)
	}
	q := u.Query()
	q.Set("database", name)
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlserver", u.String())
	if err != nil {
		t.Fatalf("opening %s: %v", name, err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
