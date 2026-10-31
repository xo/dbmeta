package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/xo/dbmeta"
)

// examplesFixture is the data fixture that a README can print the same way on
// every base database (D244). It is two plain tables in the default schema of
// the user in the URL, so that `select * from authors` works with no schema, and
// it is not the fixture of D53, which exists for the metadata readers and lives
// in a schema of its own.
//
// Every statement is the same on every product except the drop, because a drop
// of a table that is not there needs a different form on each. The foreign key
// is a table constraint, because MySQL and MariaDB read a column constraint and
// then ignore it.
var examplesFixture = struct {
	create []string
	rows   []string
}{
	create: []string{
		`CREATE TABLE authors (
  author_id INTEGER PRIMARY KEY,
  name VARCHAR(100) NOT NULL
)`,
		`CREATE TABLE books (
  book_id INTEGER PRIMARY KEY,
  author_id INTEGER,
  title VARCHAR(200) NOT NULL,
  FOREIGN KEY (author_id) REFERENCES authors (author_id)
)`,
	},
	rows: []string{
		`INSERT INTO authors (author_id, name) VALUES (1, 'Isaac Asimov')`,
		`INSERT INTO authors (author_id, name) VALUES (2, 'Stephen King')`,
		`INSERT INTO books (book_id, author_id, title) VALUES (1, 1, 'Foundation')`,
		`INSERT INTO books (book_id, author_id, title) VALUES (2, 1, 'I, Robot')`,
		`INSERT INTO books (book_id, author_id, title) VALUES (3, 2, 'Carrie')`,
		`INSERT INTO books (book_id, author_id, title) VALUES (4, 2, 'Cujo')`,
	},
}

// examplesDrop drops the two tables, the books first because they refer to the
// authors. Oracle has DROP TABLE IF EXISTS only from 23ai, so its drop is a block
// that ignores the one error, ORA-00942, and works on every release.
func examplesDrop(d dbmeta.Dialect) ([]string, bool) {
	switch d {
	case dbmeta.PostgreSQL, dbmeta.MySQL, dbmeta.SQLServer, dbmeta.SQLite3, dbmeta.DuckDB:
		return []string{"DROP TABLE IF EXISTS books", "DROP TABLE IF EXISTS authors"}, true
	case dbmeta.Oracle:
		drop := func(table string) string {
			return "BEGIN EXECUTE IMMEDIATE 'DROP TABLE " + table +
				"'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -942 THEN RAISE; END IF; END;"
		}
		return []string{drop("books"), drop("authors")}, true
	}
	return nil, false
}

// examplesPrepare is what runs before the drop. The URL of a MySQL or a MariaDB
// server names no database, so there is no default schema to put the tables in.
// The fixture makes the database examples and uses it, and a caller connects to
// that database to read the tables unqualified.
func examplesPrepare(d dbmeta.Dialect) []string {
	if d == dbmeta.MySQL {
		return []string{"CREATE DATABASE IF NOT EXISTS examples", "USE examples"}
	}
	return nil
}

// doFixture drops and rebuilds the examples fixture on one server, so that two
// runs leave the same rows. It touches the tables authors and books of the
// default schema of the user in the DSN, and nothing else.
func doFixture(ctx context.Context, r runner, t target, o options) error {
	if o.fixture != "examples" {
		return fmt.Errorf("no fixture called %q. The only one is examples", o.fixture)
	}
	if t.Kind == kindHosted {
		return errors.New("a hosted service is not a place to drop and make tables")
	}
	drop, ok := examplesDrop(t.Dialect)
	if !ok {
		return fmt.Errorf("the examples fixture has no form for %s", t.Dialect)
	}
	steps := slices.Concat(examplesPrepare(t.Dialect), drop, examplesFixture.create, examplesFixture.rows)
	if t.Kind == kindEmbedded {
		return fixtureEmbedded(ctx, t, steps)
	}
	if !r.running(ctx, t.Name) {
		return fmt.Errorf("it is not running. Start it with: dbrun start %s", t.Name)
	}
	driver, ok := drivers[t.Dialect]
	if !ok {
		return fmt.Errorf("no driver for %s", t.Dialect)
	}
	db, err := sql.Open(driver, t.connectDSN())
	if err != nil {
		return fmt.Errorf("opening: %w", err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	// USE lasts for one connection, so every step runs on the same one.
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
	var authors, books int
	if err := conn.QueryRowContext(ctx, "SELECT (SELECT COUNT(*) FROM authors), (SELECT COUNT(*) FROM books)"+fromDual(t.Dialect)).
		Scan(&authors, &books); err != nil {
		return fmt.Errorf("counting the rows: %w", err)
	}
	note := ""
	if t.Dialect == dbmeta.MySQL {
		note = ", in the database examples"
	}
	fmt.Printf("  %-*s examples: %d authors and %d books%s\n", nameWidth(), t.Name, authors, books, note)
	return nil
}

// fromDual ends a SELECT with no table, which Oracle needs and no other product
// accepts.
func fromDual(d dbmeta.Dialect) string {
	if d == dbmeta.Oracle {
		return " FROM dual"
	}
	return ""
}

// fixtureEmbedded runs the steps in an embedded database with usql, so that
// dbrun links no cgo driver. usql must be on the path, as it must for dbrun
// usql.
func fixtureEmbedded(ctx context.Context, t target, steps []string) error {
	if _, err := exec.LookPath("usql"); err != nil {
		return errors.New("usql is not on the path")
	}
	if err := extractSamples(t); err != nil {
		return err
	}
	f, err := os.CreateTemp("", "dbrun-examples-*.sql")
	if err != nil {
		return fmt.Errorf("making the script: %w", err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(strings.Join(steps, ";\n") + ";\n"); err != nil {
		f.Close()
		return fmt.Errorf("writing the script: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("writing the script: %w", err)
	}
	out, err := exec.CommandContext(ctx, "usql", "--file", f.Name(), t.URL).CombinedOutput()
	if err != nil {
		return fmt.Errorf("running usql: %w\n%s", err, out)
	}
	fmt.Printf("  %-*s examples: 2 authors and 4 books\n", nameWidth(), t.Name)
	return nil
}
