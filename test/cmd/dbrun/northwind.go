package main

import (
	"bytes"
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/container"
)

// northwindFiles holds the SQL of the northwind fixture (D254). For each
// product there is the Northwind database, and a second file with a schema
// named hr that holds what Northwind lacks.
//
//go:embed northwind/postgres.sql northwind/postgres-hr.sql northwind/sqlserver.sql northwind/sqlserver-hr.sql northwind/oracle.sql northwind/oracle-hr.sql northwind/mysql.sql northwind/mysql-hr.sql northwind/mysql-only-hr.sql northwind/mariadb-only-hr.sql northwind/sqlite3.sql northwind/sqlite3-hr.sql northwind/duckdb.sql northwind/duckdb-hr.sql northwind/firebird.sql northwind/hana.sql
var northwindFiles embed.FS

// northwindSource names the files of a product, in the order they load, the
// statement that counts the rows, and what runs before them.
type northwindSource struct {
	files []string
	// more names files that only one flavor of the dialect applies, by the
	// product, and they load after files.
	more map[string][]string
	// count is a query that returns the number of orders and the number of
	// customers, so that a load is checked by what it left.
	count string
	// header is a statement that goes at the top of every file, such as the
	// one that chooses the schema on Oracle.
	header string
}

// northwindSources is every product that has the fixture. usql applies every
// file, so a file is what usql reads for that product and not what its own
// command line client reads.
var northwindSources = map[dbmeta.Dialect]northwindSource{
	dbmeta.PostgreSQL: {
		files: []string{"northwind/postgres.sql", "northwind/postgres-hr.sql"},
		count: "SELECT (SELECT COUNT(*) FROM orders), (SELECT COUNT(*) FROM customers)",
	},
	dbmeta.SQLServer: {
		files: []string{"northwind/sqlserver.sql", "northwind/sqlserver-hr.sql"},
		count: "SELECT (SELECT COUNT(*) FROM [Orders]), (SELECT COUNT(*) FROM [Customers])",
	},
	dbmeta.Oracle: {
		files:  []string{"northwind/oracle.sql", "northwind/oracle-hr.sql"},
		count:  "SELECT (SELECT COUNT(*) FROM northwind.orders), (SELECT COUNT(*) FROM northwind.customers) FROM dual",
		header: "ALTER SESSION SET CURRENT_SCHEMA = northwind;\n",
	},
	dbmeta.MySQL: {
		files: []string{"northwind/mysql.sql", "northwind/mysql-hr.sql"},
		more: map[string][]string{
			"mysql":   {"northwind/mysql-only-hr.sql"},
			"mariadb": {"northwind/mariadb-only-hr.sql"},
		},
		count: "SELECT (SELECT COUNT(*) FROM orders), (SELECT COUNT(*) FROM customers)",
	},
	dbmeta.SQLite3: {
		files: []string{"northwind/sqlite3.sql", "northwind/sqlite3-hr.sql"},
		count: "SELECT (SELECT COUNT(*) FROM orders), (SELECT COUNT(*) FROM customers)",
	},
	dbmeta.DuckDB: {
		files: []string{"northwind/duckdb.sql", "northwind/duckdb-hr.sql"},
		count: "SELECT (SELECT COUNT(*) FROM orders), (SELECT COUNT(*) FROM customers)",
	},
	dbmeta.Firebird: {
		files: []string{"northwind/firebird.sql"},
		count: "SELECT (SELECT COUNT(*) FROM orders), (SELECT COUNT(*) FROM customers) FROM rdb$database",
	},
	dbmeta.HANA: {
		files:  []string{"northwind/hana.sql"},
		header: "SET SCHEMA northwind;\n",
		count:  "SELECT (SELECT COUNT(*) FROM northwind.orders), (SELECT COUNT(*) FROM northwind.customers) FROM dummy",
	},
}

// northwindDatabase is the name of the database that the fixture makes.
const northwindDatabase = "northwind"

// doNorthwind drops the database northwind and builds it again from the
// embedded SQL, so that two runs leave the same rows. It touches that database
// and the role northwind_reader, and nothing else.
//
// usql applies the files. Preparing the database is the one thing it does not
// do, because a database cannot be dropped from inside itself, so dbrun runs
// those statements on a connection of its own first.
func doNorthwind(ctx context.Context, r runner, t target) error {
	src, ok := northwindSources[t.Dialect]
	if !ok || (t.Kind != kindContainer && t.Kind != kindEmbedded) {
		return fmt.Errorf("the northwind fixture is not written for %s", t.Name)
	}
	if t.Kind == kindContainer && !r.running(ctx, t.Name) {
		return fmt.Errorf("it is not running. Start it with: dbrun start %s", t.Name)
	}
	if _, err := exec.LookPath("usql"); err != nil {
		return errors.New("usql is not on the path")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if err := northwindPrepare(ctx, t); err != nil {
		return err
	}
	target, err := northwindURL(t)
	if err != nil {
		return err
	}
	files := slices.Concat(src.files, src.more[t.Product])
	for _, name := range files {
		b, err := northwindFiles.ReadFile(name)
		if err != nil {
			return fmt.Errorf("reading %s: %w", name, err)
		}
		if err := usqlFile(ctx, target, src.header+string(b)); err != nil {
			return fmt.Errorf("applying %s: %w", name, err)
		}
	}
	out, err := usqlOutput(ctx, target, src.count)
	if err != nil {
		return fmt.Errorf("counting the rows: %w", err)
	}
	var orders, customers int
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if _, err := fmt.Sscanf(lines[len(lines)-1], "%d|%d", &orders, &customers); err != nil {
		return fmt.Errorf("reading the count %q: %w", out, err)
	}
	where := "the database " + northwindDatabase
	if t.Kind == kindEmbedded {
		where = "the file " + northwindFile(t)
	}
	fmt.Printf("  %-*s northwind: %d files, %d customers and %d orders, in %s\n",
		nameWidth(), t.Name, len(files), customers, orders, where)
	return nil
}

// northwindPrepare removes what an earlier run made and makes what the files
// expect to find.
func northwindPrepare(ctx context.Context, t target) error {
	dsn := t.connectDSN()
	switch t.Dialect {
	case dbmeta.PostgreSQL:
		return runSteps(ctx, dsn, dialectDriver(t), []string{
			"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '" + northwindDatabase + "' AND pid <> pg_backend_pid()",
			"DROP DATABASE IF EXISTS " + northwindDatabase,
			"CREATE DATABASE " + northwindDatabase,
		})
	case dbmeta.Oracle:
		return runSteps(ctx, dsn, dialectDriver(t), oracleNorthwindPrepare())
	case dbmeta.MySQL:
		return runSteps(ctx, dsn, dialectDriver(t), []string{
			"DROP DATABASE IF EXISTS " + northwindDatabase,
			"DROP DATABASE IF EXISTS hr",
			"DROP ROLE IF EXISTS northwind_reader",
			"CREATE DATABASE " + northwindDatabase + " CHARACTER SET utf8mb4",
		})
	case dbmeta.SQLServer:
		return runSteps(ctx, dsn, dialectDriver(t), []string{
			"IF DB_ID('" + northwindDatabase + "') IS NOT NULL BEGIN ALTER DATABASE [" + northwindDatabase +
				"] SET SINGLE_USER WITH ROLLBACK IMMEDIATE; DROP DATABASE [" + northwindDatabase + "]; END",
			"CREATE DATABASE [" + northwindDatabase + "]",
		})
	case dbmeta.Firebird:
		return firebirdNorthwindPrepare(ctx, t)
	case dbmeta.HANA:
		// The first drop fails when there is no schema, which is not a fault.
		_ = runSteps(ctx, dsn, dialectDriver(t), []string{"DROP SCHEMA " + northwindDatabase + " CASCADE"})
		return runSteps(ctx, dsn, dialectDriver(t), []string{"CREATE SCHEMA " + northwindDatabase})
	case dbmeta.SQLite3, dbmeta.DuckDB:
		// An embedded database is a file, and the fixture owns a file of its
		// own, so that a run leaves nothing of the one before.
		file := northwindFile(t)
		if err := os.Remove(file); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("removing %s: %w", file, err)
		}
	}
	return nil
}

// firebirdCreate matches the statements of the fixture that make an object that
// a drop removes.
var firebirdCreate = regexp.MustCompile(`(?im)^create\s+(table|view|procedure)\s+(\w+)`)

// firebirdNorthwindPrepare drops what the fixture made. Firebird has no
// schemas, and the file is the database, so the fixture shares it with
// everything else in it. It drops the views, procedures and tables that its own
// files create and nothing else. A drop that a dependency refuses is tried
// again after the others.
func firebirdNorthwindPrepare(ctx context.Context, t target) error {
	src := northwindSources[t.Dialect]
	var drops []string
	for _, name := range src.files {
		b, err := northwindFiles.ReadFile(name)
		if err != nil {
			return fmt.Errorf("reading %s: %w", name, err)
		}
		for _, m := range firebirdCreate.FindAllStringSubmatch(string(b), -1) {
			drops = append(drops, "DROP "+strings.ToUpper(m[1])+" "+m[2])
		}
	}
	slices.Reverse(drops)
	db, err := sql.Open(dialectDriver(t), t.connectDSN())
	if err != nil {
		return fmt.Errorf("opening: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for range 10 {
		var left []string
		for _, d := range drops {
			if _, err := db.ExecContext(ctx, d); err != nil && !strings.Contains(err.Error(), "does not exist") &&
				!strings.Contains(err.Error(), "Table unknown") && !strings.Contains(err.Error(), "not found") {
				left = append(left, d)
			}
		}
		if len(left) == 0 {
			return nil
		}
		drops = left
	}
	return fmt.Errorf("dropping the objects of an earlier run left %d behind, such as %s", len(drops), drops[0])
}

// northwindURL is the URL that usql opens for the database of the fixture.
func northwindURL(t target) (string, error) {
	if t.Kind == kindEmbedded {
		return string(t.Dialect) + ":" + northwindFile(t), nil
	}
	u, err := url.Parse(t.URL)
	if err != nil {
		return "", fmt.Errorf("reading the URL of %s: %w", t.Name, err)
	}
	switch t.Dialect {
	case dbmeta.PostgreSQL, dbmeta.MySQL:
		u.Path = "/" + northwindDatabase
	case dbmeta.SQLServer:
		q := u.Query()
		q.Set("database", northwindDatabase)
		u.RawQuery = q.Encode()
	}
	return u.String(), nil
}

// usqlFile runs a script with usql. usql goes on after a statement that fails
// and exits with 1, so a failure shows as the lines that usql wrote to the
// error output.
func usqlFile(ctx context.Context, target, script string) error {
	f, err := os.CreateTemp("", "northwind-*.sql")
	if err != nil {
		return fmt.Errorf("making a script: %w", err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(script); err != nil {
		f.Close()
		return fmt.Errorf("writing the script: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("writing the script: %w", err)
	}
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "usql", "-q", target, "-f", f.Name())
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("usql: %w\n%s", err, firstLines(stderr.String(), 12))
	}
	return nil
}

// usqlOutput runs one query with usql and returns its rows, with a bar between
// the columns.
func usqlOutput(ctx context.Context, target, query string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "usql", "-q", "-t", "-A", target, "-c", query)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("usql: %w\n%s", err, firstLines(stderr.String(), 6))
	}
	return stdout.String(), nil
}

// northwindFile is the file of the fixture for an embedded database. It sits
// beside the file that dbrun keeps for the product and has a name of its own,
// so that the fixture never replaces what another test wrote.
func northwindFile(t target) string {
	return filepath.Join(filepath.Dir(t.DSN), northwindDatabase+filepath.Ext(t.DSN))
}

// oracleNorthwindPrepare drops and makes the two users that hold the fixture,
// because Oracle calls a schema a user. The drop ignores the error for a user
// that is not there, ORA-01918, and for a role that is not there, ORA-01919.
func oracleNorthwindPrepare() []string {
	drop := func(kind, name string, code int) string {
		return fmt.Sprintf("BEGIN EXECUTE IMMEDIATE 'DROP %s %s%s'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -%d THEN RAISE; END IF; END;",
			kind, name, map[string]string{"USER": " CASCADE"}[kind], code)
	}
	steps := []string{drop("USER", northwindDatabase, 1918), drop("USER", "hr", 1918), drop("ROLE", "northwind_reader", 1919)}
	for _, user := range []string{northwindDatabase, "hr"} {
		steps = append(steps,
			fmt.Sprintf(`CREATE USER %s IDENTIFIED BY "%s"`, user, container.Password),
			"GRANT CREATE SESSION, CREATE TABLE, CREATE VIEW, CREATE SEQUENCE, CREATE PROCEDURE, CREATE TRIGGER, "+
				"CREATE TYPE, CREATE SYNONYM, CREATE MATERIALIZED VIEW, UNLIMITED TABLESPACE TO "+user)
	}
	return steps
}

// dialectDriver is the name that sql.Open takes for a server.
func dialectDriver(t target) string { return drivers[t.Dialect] }

// runSteps runs each statement on one connection to the server.
func runSteps(ctx context.Context, dsn, driver string, steps []string) error {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return fmt.Errorf("opening: %w", err)
	}
	defer db.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("connecting: %w", err)
	}
	defer conn.Close()
	for _, s := range steps {
		if _, err := conn.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("%w\n%s", err, s)
		}
	}
	return nil
}

// firstLines returns the first n lines of a text, for an error message.
func firstLines(s string, n int) string {
	lines := strings.SplitN(strings.TrimSpace(s), "\n", n+1)
	if len(lines) > n {
		lines = append(lines[:n], "...")
	}
	return strings.Join(lines, "\n")
}
