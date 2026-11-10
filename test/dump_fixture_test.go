package test

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

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
	// rename gives the schemas other names, so that the copy lives in the server
	// of the source. A product whose schema is a database needs it.
	rename map[string]string
	// skips are the kinds of object that the dump reads and does not write, which
	// the snapshot lists as skipped, so that a client can warn about them.
	skips []string
	// eventTriggers says that the fixture has a trigger on a DDL event, which the
	// snapshot must hold.
	eventTriggers bool
	// rows says that the test also moves the rows of every table and compares them.
	rows bool
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
			eventTriggers: true,
			rows:          true,
			dialect:       dbmeta.PostgreSQL,
			source:        func(t *testing.T) *sql.DB { return openPostgresDatabase(t, "dump", false) },
			fresh:         func(t *testing.T) *sql.DB { return openPostgresDatabase(t, "dbmeta_dump_fixture", true) },
		},
		"sqlserver": {
			eventTriggers: true,
			rows:          true,
			dialect:       dbmeta.SQLServer,
			source: func(t *testing.T) *sql.DB {
				openSQLServer(t)
				return openSQLServerAt(t, "dump")
			},
			fresh: func(t *testing.T) *sql.DB { return openSQLServerNamed(t, "dbmeta_dump_fixture") },
		},
		"mysql": {
			rows:    true,
			dialect: dbmeta.MySQL,
			source:  func(t *testing.T) *sql.DB { return openMySQLAt(t, "dump_core") },
			fresh:   openMySQLCopy,
			schemas: []string{"dump_core", "dump_extra", "thing"},
			rename:  map[string]string{"dump_core": "cp_dump_core", "dump_extra": "cp_dump_extra", "thing": "cp_thing"},
		},
		"sqlite3": {
			rows:    true,
			dialect: dbmeta.SQLite3,
			source:  func(t *testing.T) *sql.DB { return openEmbeddedFixture(t, sqliteDrivers[0], "dump.db") },
			fresh:   func(t *testing.T) *sql.DB { return openSQLiteAt(t, sqliteDrivers[0]) },
			schemas: []string{"main"},
		},
		"oracle": {
			eventTriggers: true,
			rows:          true,
			dialect:       dbmeta.Oracle,
			source:        openOracle,
			fresh:         openOracleCopy,
			schemas:       []string{"DUMP_CORE", "DUMP_EXTRA"},
			rename:        map[string]string{"DUMP_CORE": "CP_CORE", "DUMP_EXTRA": "CP_EXTRA"},
		},
		"duckdb": {
			skips:   []string{"type struct"},
			rows:    true,
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
			for _, kind := range side.skips {
				if !slices.ContainsFunc(first.Skipped, func(sk dump.Skipped) bool { return sk.Kind == kind }) {
					t.Errorf("the snapshot does not list a skipped %s", kind)
				}
			}
			if side.eventTriggers && len(first.EventTriggers) == 0 {
				t.Errorf("the snapshot holds no trigger on a DDL event")
			}
			want := first
			filter2 := filter
			if len(side.rename) > 0 {
				if side.dialect == dbmeta.Oracle {
					want = renameOracleSchemas(t, first, side.rename)
				} else {
					want = renameFixtureSchemas(t, first, side.rename)
				}
				filter2 = dump.Filter{}
				for _, sc := range side.schemas {
					filter2.Schemas = append(filter2.Schemas, side.rename[sc])
				}
			}
			stmts := scriptOf(t, want, side.dialect)
			dst := side.fresh(t)
			var report strings.Builder
			var script strings.Builder
			// One connection runs the script, as a client does, because USE and the
			// settings belong to a connection.
			conn, err := dst.Conn(t.Context())
			if err != nil {
				t.Fatalf("taking a connection: %v", err)
			}
			defer conn.Close()
			rowsDone := !side.rows
			for _, st := range stmts {
				if st.SQL == "" {
					continue
				}
				// The rows go in after the tables and before the keys, as a client does.
				if !rowsDone && st.Stage == dump.Post {
					rowsDone = true
					copyFixtureRows(t, src, conn, first, want, side, stmts, &report)
				}
				script.WriteString(st.SQL + ";\n")
				if _, err := conn.ExecContext(t.Context(), st.SQL); err != nil {
					msg, _, _ := strings.Cut(strings.TrimSpace(err.Error()), "\n")
					fmt.Fprintf(&report, "statement failed: %s %s: %s\n", st.Kind, st.Name, msg)
				}
			}
			second, err := dump.Read(t.Context(), m, dst, filter2)
			if err != nil {
				t.Fatalf("reading the copy: %v", err)
			}
			if os.Getenv("DBMETA_SHOW_SKIPPED") != "" {
				for _, sk := range first.Skipped {
					t.Logf("skipped %s: %s.%s %s: %s", name, sk.Schema, sk.Name, sk.Kind, sk.Reason)
				}
			}
			want.Skipped, second.Skipped = nil, nil
			if side.dialect == dbmeta.SQLServer {
				// The server names the keys of a table type, and the names change.
				normalizeViews(want)
				normalizeViews(second)
			}
			wantJSON, err := want.JSON()
			if err != nil {
				t.Fatal(err)
			}
			got, err := second.JSON()
			if err != nil {
				t.Fatal(err)
			}
			if side.dialect == dbmeta.Oracle {
				// The server names a key, an index or a column that it makes itself, and
				// the names differ in each run.
				wantJSON, got = generatedNames.ReplaceAll(wantJSON, []byte("SYS_GENERATED")), generatedNames.ReplaceAll(got, []byte("SYS_GENERATED"))
				fixed := generatedNames.ReplaceAllString(report.String(), "SYS_GENERATED")
				report.Reset()
				report.WriteString(fixed)
			}
			// A second dump of the copy writes the views and the routines in the same
			// order as the first one.
			if a, b := objectOrder(stmts), objectOrder(scriptOf(t, second, side.dialect)); a != b {
				report.WriteString("the order of the views and the routines differs:\n" + lineDiff(a, b) + "\n")
			}
			if string(got) != string(wantJSON) {
				report.WriteString("the copy differs from the source:\n" + lineDiff(string(wantJSON), string(got)) + "\n")
			}
			if out := os.Getenv("DBMETA_SCRIPT_OUT"); out != "" {
				_ = os.WriteFile(filepath.Join(out, "fixture-"+name+".sql"), []byte(script.String()), 0o644)
				_ = os.WriteFile(filepath.Join(out, "fixture-"+name+"-source.json"), wantJSON, 0o644)
				_ = os.WriteFile(filepath.Join(out, "fixture-"+name+"-copy.json"), got, 0o644)
			}
			product := name
			if name == "mysql" {
				// MySQL and MariaDB share the dialect and not the answers.
				var version string
				if err := src.QueryRowContext(t.Context(), "SELECT VERSION()").Scan(&version); err == nil &&
					strings.Contains(strings.ToLower(version), "mariadb") {
					product = "mariadb"
				}
			}
			golden := filepath.Join("testdata", "dump_fixture", product+".txt")
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
	dsn := path
	if driver == "duckdb" {
		dsn += "?access_mode=read_only"
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// renameFixtureSchemas gives the schemas of a snapshot other names. A name in
// the text of a view or a routine is renamed where it stands for a database,
// which is where a backtick, a dot or the name alone shows it. The name thing is
// also a table, so it is renamed only before a dot that follows a backtick.
func renameFixtureSchemas(t *testing.T, snap *dump.Snapshot, names map[string]string) *dump.Snapshot {
	t.Helper()
	b, err := snap.JSON()
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for from, to := range names {
		if from == "thing" {
			text = strings.ReplaceAll(text, "`thing`.`", "`"+to+"`.`")
			continue
		}
		text = regexp.MustCompile(`\b`+from+`\b`).ReplaceAllString(text, to)
	}
	out, err := dump.ParseSnapshot([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	for i := range out.Schemas {
		if to, ok := names[out.Schemas[i].Name]; ok {
			out.Schemas[i].Name = to
		}
	}
	return out
}

// openMySQLCopy opens the server without a database, and drops the databases of
// the copy at the end.
func openMySQLCopy(t *testing.T) *sql.DB {
	t.Helper()
	admin := openMySQL(t)
	drop := func() {
		// The two databases refer to each other, so the keys are off for the drops.
		conn, err := admin.Conn(context.Background())
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = conn.ExecContext(context.Background(), "SET FOREIGN_KEY_CHECKS = 0")
		for _, name := range []string{"cp_dump_core", "cp_dump_extra", "cp_thing"} {
			_, _ = conn.ExecContext(context.Background(), "DROP DATABASE IF EXISTS "+name)
		}
	}
	drop()
	if os.Getenv("DBMETA_KEEP") == "" {
		t.Cleanup(drop)
	}
	return admin
}

// openOracleCopy opens the server, and drops the two users of the copy before and
// after, because Oracle calls a schema a user.
func openOracleCopy(t *testing.T) *sql.DB {
	t.Helper()
	admin := openOracle(t)
	drop := func() {
		for _, u := range []string{"CP_CORE", "CP_EXTRA"} {
			_, _ = admin.ExecContext(context.Background(), "BEGIN EXECUTE IMMEDIATE 'DROP USER "+u+" CASCADE'; EXCEPTION WHEN OTHERS THEN NULL; END;")
		}
	}
	drop()
	t.Cleanup(drop)
	return admin
}

// generatedNames matches the names that Oracle gives to what it makes itself.
var generatedNames = regexp.MustCompile(`SYS_[A-Z]+[0-9]+[A-Z0-9$]*`)

// copyFixtureRows moves the rows of every table that the script made, in the
// order that it made them, and compares the rows of the copy with the source.
// A table that fails, or that has other rows, adds a line to the report.
func copyFixtureRows(t *testing.T, src *sql.DB, dst *sql.Conn, first, want *dump.Snapshot, side fixtureSide, stmts []dump.Statement, report *strings.Builder) {
	t.Helper()
	back := map[string]string{}
	for from, to := range side.rename {
		back[to] = from
	}
	for _, st := range stmts {
		if st.Kind != "table" || st.Stage != dump.Pre {
			continue
		}
		schema, table, _ := strings.Cut(st.Name, ".")
		tb := findFixtureTable(want, schema, table)
		if tb == nil || tb.IsPartition() {
			continue
		}
		origin := schema
		if o, ok := back[schema]; ok {
			origin = o
		}
		from, err := dump.NewLoader(first, origin, table, side.dialect, dump.Policy{})
		if err != nil {
			fmt.Fprintf(report, "rows failed: %s: %v\n", st.Name, err)
			continue
		}
		to, err := dump.NewLoader(want, schema, table, side.dialect, dump.Policy{})
		if err != nil {
			fmt.Fprintf(report, "rows failed: %s: %v\n", st.Name, err)
			continue
		}
		cells := readCells(t, src, from.SelectAll())
		// A constructor of an object type names its schema, and the copy has another.
		for _, row := range cells {
			for i, v := range row {
				if text, ok := v.(string); ok {
					for old, now := range side.rename {
						text = regexp.MustCompile(`(?i)\b`+old+`\b`).ReplaceAllString(text, now)
					}
					row[i] = text
				}
			}
		}
		var failed error
		if p := to.Prologue(); p != "" {
			if _, err := dst.ExecContext(t.Context(), p); err != nil {
				failed = err
			}
		}
		for start := 0; start < len(cells) && failed == nil; start += to.BatchSize() {
			stmt, err := to.Insert(cells[start:min(start+to.BatchSize(), len(cells))])
			if err != nil {
				failed = err
				break
			}
			_, failed = dst.ExecContext(t.Context(), stmt)
		}
		if e := to.Epilogue(); e != "" {
			if _, err := dst.ExecContext(t.Context(), e); err != nil && failed == nil {
				failed = err
			}
		}
		if failed != nil {
			msg, _, _ := strings.Cut(strings.TrimSpace(failed.Error()), "\n")
			fmt.Fprintf(report, "rows failed: %s: %s\n", st.Name, msg)
			continue
		}
		got, err := connCells(t, dst, to.SelectAll())
		if err != nil {
			fmt.Fprintf(report, "rows failed: %s: reading the copy: %v\n", st.Name, err)
			continue
		}
		// A value that holds an offset comes back as the same instant in UTC.
		for _, set := range [][][]any{cells, got} {
			for _, row := range set {
				for i, v := range row {
					if tv, ok := v.(time.Time); ok {
						row[i] = tv.UTC()
					}
				}
			}
		}
		if !equalCells(cells, got) {
			if os.Getenv("DBMETA_SHOW_SKIPPED") != "" {
				t.Logf("source %v\ncopy   %v", cells, got)
			}
			fmt.Fprintf(report, "rows differ: %s: %d in the source and %d in the copy\n", st.Name, len(cells), len(got))
		}
	}
}

// findFixtureTable finds a table of a snapshot.
func findFixtureTable(s *dump.Snapshot, schema, table string) *dump.Table {
	for i := range s.Schemas {
		if s.Schemas[i].Name != schema {
			continue
		}
		for j := range s.Schemas[i].Tables {
			if s.Schemas[i].Tables[j].Name == table {
				return &s.Schemas[i].Tables[j]
			}
		}
	}
	return nil
}

// connCells reads every row of a query on one connection.
func connCells(t *testing.T, conn *sql.Conn, query string) ([][]any, error) {
	t.Helper()
	rows, err := conn.QueryContext(t.Context(), query)
	if err != nil {
		return nil, fmt.Errorf("querying: %w", err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("reading the columns: %w", err)
	}
	var out [][]any
	for rows.Next() {
		row := make([]any, len(cols))
		dest := make([]any, len(cols))
		for i := range dest {
			dest[i] = &row[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("scanning: %w", err)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// objectOrder lists the views and the routines of a script in the order that it
// writes them.
func objectOrder(stmts []dump.Statement) string {
	var out strings.Builder
	for _, st := range stmts {
		if st.Kind != "view" && st.Kind != "routine" && st.Kind != "materialized view" {
			continue
		}
		out.WriteString(st.Kind + " " + st.Name + "\n")
	}
	return out.String()
}
