package test

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mysqldriver "github.com/go-sql-driver/mysql"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// fixtureSide is one server that holds the dump fixture of D291, and how to
// open the fixture, an empty database to copy it to, and the schemas to read.
type fixtureSide struct {
	dialect dbmeta.Dialect
	// source opens the database that dbrun fixture dump made, or skips.
	source func(t *testing.T) *sql.DB
	// fresh opens an empty database of its own for the copy.
	fresh func(t *testing.T) *sql.DB
	// schemas are the schemas to read. None means every schema that is not a
	// system schema.
	schemas []string
}

// TestDumpFixtureRoundTrips reads the dump fixture, writes the script of its
// snapshot, runs the script on an empty database, and compares the snapshot of
// the copy with the snapshot of the source. Run dbrun fixture dump for the
// product first.
//
// The fixture holds objects that a dump does not write yet, so the test does
// not stop at a statement that fails. It writes down each statement that failed
// and each difference between the two snapshots, and compares that text with
// testdata/dump_fixture/<product>.txt. An empty file means a perfect copy. Run
// go test -update and read the diff, because a line that goes away means that
// the dump got better, and a new line means that it got worse.
func TestDumpFixtureRoundTrips(t *testing.T) {
	sides := map[string]fixtureSide{
		"postgres": {
			dialect: dbmeta.PostgreSQL,
			source:  func(t *testing.T) *sql.DB { return openPostgresDatabase(t, "dump", false) },
			fresh:   func(t *testing.T) *sql.DB { return openPostgresDatabase(t, "dbmeta_dump_fixture", true) },
		},
		"sqlserver": {
			dialect: dbmeta.SQLServer,
			source: func(t *testing.T) *sql.DB {
				openSQLServer(t)
				return openSQLServerAt(t, "dump")
			},
			fresh: func(t *testing.T) *sql.DB { return openSQLServerNamed(t, "dbmeta_dump_fixture") },
		},
		"mysql": {
			dialect: dbmeta.MySQL,
			source:  func(t *testing.T) *sql.DB { return openMySQLAt(t, "dump_core") },
			fresh:   func(t *testing.T) *sql.DB { return openMySQLNamed(t, "dbmeta_dump_core") },
			schemas: []string{"dump_core", "dump_extra", "thing"},
		},
		"duckdb": {
			dialect: dbmeta.DuckDB,
			source:  func(t *testing.T) *sql.DB { return openEmbeddedFixture(t, "duckdb", "dump.duckdb") },
			fresh:   openDuckDBAt,
		},
	}
	for name, side := range sides {
		t.Run(name, func(t *testing.T) {
			src := side.source(t)
			m := metaFor(t, side.dialect, src)
			filter := dump.Filter{Schemas: side.schemas}
			first, err := dump.Read(t.Context(), m, src, filter)
			if err != nil {
				t.Fatalf("reading: %v", err)
			}
			stmts := scriptOf(t, first, side.dialect)
			dst := side.fresh(t)
			var report strings.Builder
			var script strings.Builder
			for _, st := range stmts {
				if st.SQL == "" {
					continue
				}
				script.WriteString(st.SQL + ";\n")
				if _, err := dst.ExecContext(t.Context(), st.SQL); err != nil {
					msg, _, _ := strings.Cut(strings.TrimSpace(err.Error()), "\n")
					fmt.Fprintf(&report, "statement failed: %s %s: %s\n", st.Kind, st.Name, msg)
				}
			}
			second, err := dump.Read(t.Context(), m, dst, filter)
			if err != nil {
				t.Fatalf("reading the copy: %v", err)
			}
			first.Skipped, second.Skipped = nil, nil
			if side.dialect == dbmeta.SQLServer {
				// The server names the keys of a table type, and the names change.
				normalizeViews(first)
				normalizeViews(second)
			}
			want, err := first.JSON()
			if err != nil {
				t.Fatal(err)
			}
			got, err := second.JSON()
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != string(want) {
				report.WriteString("the copy differs from the source:\n" + lineDiff(string(want), string(got)) + "\n")
			}
			if out := os.Getenv("DBMETA_SCRIPT_OUT"); out != "" {
				_ = os.WriteFile(filepath.Join(out, "fixture-"+name+".sql"), []byte(script.String()), 0o644)
			}
			golden := filepath.Join("testdata", "dump_fixture", name+".txt")
			if *update {
				if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(golden, []byte(report.String()), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			b, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("reading %s: %v. Run go test -update", golden, err)
			}
			if string(b) != report.String() {
				t.Errorf("the copy of the fixture changed. Run go test -update and read the diff of %s\n%s",
					golden, lineDiff(string(b), report.String()))
			}
		})
	}
}

// openPostgresDatabase opens a database of the server that DBMETA_POSTGRES
// names. With fresh it makes the database empty and drops it at the end.
func openPostgresDatabase(t *testing.T, name string, fresh bool) *sql.DB {
	t.Helper()
	admin := open(t)
	if fresh {
		for _, s := range []string{"DROP DATABASE IF EXISTS " + name, "CREATE DATABASE " + name} {
			mustExec(t, admin, s)
		}
		t.Cleanup(func() {
			_, _ = admin.ExecContext(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
		})
	}
	u, err := url.Parse(os.Getenv("DBMETA_POSTGRES"))
	if err != nil {
		t.Fatalf("reading DBMETA_POSTGRES: %v", err)
	}
	u.Path = "/" + name
	db, err := sql.Open(postgresDrivers[0], u.String())
	if err != nil {
		t.Fatalf("opening %s: %v", name, err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// openMySQLAt opens a database of the server that DBMETA_MYSQL names, as it is.
func openMySQLAt(t *testing.T, name string) *sql.DB {
	t.Helper()
	openMySQL(t)
	cfg, err := mysqldriver.ParseDSN(os.Getenv("DBMETA_MYSQL"))
	if err != nil {
		t.Fatalf("reading DBMETA_MYSQL: %v", err)
	}
	cfg.DBName = name
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatalf("opening %s: %v", name, err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// openEmbeddedFixture opens a file that dbrun keeps for an embedded database.
func openEmbeddedFixture(t *testing.T, driver, file string) *sql.DB {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home directory: %v", err)
	}
	path := filepath.Join(home, ".local", "share", "dbmeta", "embedded", file)
	if _, err := os.Stat(path); err != nil {
		t.Skipf("no %s: run dbrun fixture dump %s", path, driver)
	}
	db, err := sql.Open(driver, path+"?access_mode=read_only")
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
