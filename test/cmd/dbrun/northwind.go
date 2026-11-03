package main

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"net/url"
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
//go:embed northwind/postgres.sql northwind/postgres-hr.sql northwind/sqlserver.sql northwind/sqlserver-hr.sql northwind/oracle.sql northwind/oracle-hr.sql
var northwindFiles embed.FS

// northwindSource names the files of a product, in the order they load, and
// says how the statements of those files end.
type northwindSource struct {
	files []string
	// blocks says that the product writes the body of a procedure, a function
	// or a trigger between BEGIN and END with semicolons inside, and a name
	// between brackets, which is T-SQL.
	blocks bool
	// slash says that a line with only a slash ends each statement, which is
	// how SQL*Plus reads Oracle, and that the statements have no semicolon of
	// their own except the one that ends a PL/SQL block.
	slash bool
}

// northwindSources is every product that has the fixture.
var northwindSources = map[dbmeta.Dialect]northwindSource{
	dbmeta.PostgreSQL: {files: []string{"northwind/postgres.sql", "northwind/postgres-hr.sql"}},
	dbmeta.SQLServer:  {files: []string{"northwind/sqlserver.sql", "northwind/sqlserver-hr.sql"}, blocks: true},
	dbmeta.Oracle:     {files: []string{"northwind/oracle.sql", "northwind/oracle-hr.sql"}, slash: true},
}

// northwindDatabase is the name of the database that the fixture makes.
const northwindDatabase = "northwind"

// doNorthwind drops the database northwind and builds it again from the
// embedded SQL, so that two runs leave the same rows. It touches that database
// and the role northwind_reader, and nothing else.
//
// The files are written for a client that splits at each semicolon, so this
// function splits them the same way and sends each statement as one batch.
// SQL Server needs that, because CREATE VIEW and CREATE PROCEDURE must be the
// only statement of a batch.
func doNorthwind(ctx context.Context, r runner, t target) error {
	src, ok := northwindSources[t.Dialect]
	if !ok || t.Kind != kindContainer {
		return fmt.Errorf("the northwind fixture is for PostgreSQL, SQL Server and Oracle servers, and %s is not one", t.Name)
	}
	if !r.running(ctx, t.Name) {
		return fmt.Errorf("it is not running. Start it with: dbrun start %s", t.Name)
	}
	var statements []string
	for _, name := range src.files {
		b, err := northwindFiles.ReadFile(name)
		if err != nil {
			return fmt.Errorf("reading %s: %w", name, err)
		}
		if src.slash {
			statements = append(statements, splitSlash(string(b))...)
			continue
		}
		statements = append(statements, splitStatements(string(b), src.blocks, t.Dialect == dbmeta.PostgreSQL)...)
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	dsn := t.connectDSN()
	switch t.Dialect {
	case dbmeta.PostgreSQL:
		if err := runSteps(ctx, dsn, dialectDriver(t), []string{
			"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '" + northwindDatabase + "' AND pid <> pg_backend_pid()",
			"DROP DATABASE IF EXISTS " + northwindDatabase,
			"CREATE DATABASE " + northwindDatabase,
		}); err != nil {
			return err
		}
		u, err := url.Parse(dsn)
		if err != nil {
			return fmt.Errorf("reading the URL of %s: %w", t.Name, err)
		}
		u.Path = "/" + northwindDatabase
		dsn = u.String()
	case dbmeta.Oracle:
		if err := runSteps(ctx, dsn, dialectDriver(t), oracleNorthwindPrepare()); err != nil {
			return err
		}
		statements = slices.Insert(statements, 0, "ALTER SESSION SET CURRENT_SCHEMA = "+northwindDatabase)
	case dbmeta.SQLServer:
		if err := runSteps(ctx, dsn, dialectDriver(t), []string{
			"IF DB_ID('" + northwindDatabase + "') IS NOT NULL BEGIN ALTER DATABASE [" + northwindDatabase +
				"] SET SINGLE_USER WITH ROLLBACK IMMEDIATE; DROP DATABASE [" + northwindDatabase + "]; END",
			"CREATE DATABASE [" + northwindDatabase + "]",
		}); err != nil {
			return err
		}
	}
	db, err := sql.Open(dialectDriver(t), dsn)
	if err != nil {
		return fmt.Errorf("opening: %w", err)
	}
	defer db.Close()
	// A session setting lasts for one connection, so every statement runs on
	// the same one, which also keeps SET DATEFORMAT of the T-SQL file.
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("connecting: %w", err)
	}
	defer conn.Close()
	if t.Dialect == dbmeta.SQLServer {
		if _, err := conn.ExecContext(ctx, "USE ["+northwindDatabase+"]"); err != nil {
			return fmt.Errorf("choosing the database: %w", err)
		}
	}
	identity := identityTracker{}
	for _, s := range statements {
		for _, step := range identity.steps(s) {
			if _, err := conn.ExecContext(ctx, step); err != nil {
				return fmt.Errorf("%w\n%s", err, firstLines(step, 6))
			}
		}
	}
	var orders, customers int
	count := "SELECT (SELECT COUNT(*) FROM orders), (SELECT COUNT(*) FROM customers)"
	switch t.Dialect {
	case dbmeta.SQLServer:
		count = "SELECT (SELECT COUNT(*) FROM [Orders]), (SELECT COUNT(*) FROM [Customers])"
	case dbmeta.Oracle:
		count = "SELECT (SELECT COUNT(*) FROM " + northwindDatabase + ".orders), (SELECT COUNT(*) FROM " +
			northwindDatabase + ".customers) FROM dual"
	}
	if err := conn.QueryRowContext(ctx, count).Scan(&orders, &customers); err != nil {
		return fmt.Errorf("counting the rows: %w", err)
	}
	fmt.Printf("  %-*s northwind: %d statements, %d customers and %d orders, in the database %s\n",
		nameWidth(), t.Name, len(statements), customers, orders, northwindDatabase)
	return nil
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

// firstLines returns the first n lines of a statement, for an error message.
func firstLines(s string, n int) string {
	lines := strings.SplitN(s, "\n", n+1)
	if len(lines) > n {
		lines = append(lines[:n], "...")
	}
	return strings.Join(lines, "\n")
}

// identityInsert matches the SET that turns on the insert of identity values
// into one table.
var identityInsert = regexp.MustCompile(`(?is)^\s*SET\s+IDENTITY_INSERT\s+(\[[^\]]+\])\s+ON\b`)

// identityTracker turns identity insert off for the table that it was on for
// before it turns it on for another one. SQL Server allows one table at a time
// in a session, and the T-SQL file sets it again before every insert.
type identityTracker struct {
	current string
}

// steps returns the statements to run for one statement of the file.
func (it *identityTracker) steps(s string) []string {
	m := identityInsert.FindStringSubmatch(s)
	if m == nil {
		return []string{s}
	}
	if it.current == "" || strings.EqualFold(it.current, m[1]) {
		if it.current == "" {
			it.current = m[1]
		}
		return []string{s}
	}
	off := "SET IDENTITY_INSERT " + it.current + " OFF"
	it.current = m[1]
	return []string{off, s}
}

// splitStatements splits SQL at each semicolon that is not inside a quote, a
// name between double quotes or brackets, a comment, a dollar quoted string or,
// where blocks is true, the body of a CREATE or ALTER that sits between BEGIN
// and END. It drops a piece that holds nothing but space and comments.
//
// This is a small splitter for the files of the fixture. It is not the grammar
// of any product, and a file that needs more is written to avoid it.
func splitStatements(src string, blocks, dollar bool) []string {
	var out []string
	var (
		start  int  // where the current statement began
		has    bool // the statement holds a token that is not a comment
		depth  int  // BEGIN and CASE that END has not closed yet
		create bool // the statement is a CREATE or an ALTER
		words  int  // how many words the statement holds so far
	)
	flush := func(end int) {
		if has {
			out = append(out, strings.TrimSpace(src[start:end]))
		}
		start, has, depth, create, words = end, false, 0, false, 0
	}
	i := 0
	for i < len(src) {
		c := src[i]
		switch {
		case c == '-' && strings.HasPrefix(src[i:], "--"):
			i = indexFrom(src, i, "\n")
		case c == '/' && strings.HasPrefix(src[i:], "/*"):
			i = indexFrom(src, i+2, "*/") + 2
		case c == '\'':
			i = closeQuote(src, i, '\'')
			has = true
		case c == '"':
			i = closeQuote(src, i, '"')
			has = true
		case c == '[' && blocks:
			i = closeQuote(src, i, ']')
			has = true
		case c == '$' && dollar:
			if tag, ok := dollarTag(src[i:]); ok {
				i = indexFrom(src, i+len(tag), tag) + len(tag)
				has = true
			} else {
				i++
				has = true
			}
		case c == ';':
			if depth > 0 {
				i++
				continue
			}
			flush(i + 1)
			i++
		case isWordStart(c):
			j := i
			for j < len(src) && isWordPart(src[j]) {
				j++
			}
			w := strings.ToUpper(src[i:j])
			if words == 0 && (w == "CREATE" || w == "ALTER") {
				create = true
			}
			if blocks && create {
				switch w {
				case "BEGIN", "CASE":
					depth++
				case "END":
					if depth > 0 {
						depth--
					}
				}
			}
			words++
			has = true
			i = j
		default:
			if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
				has = true
			}
			i++
		}
	}
	flush(len(src))
	for k, s := range out {
		out[k] = strings.TrimSuffix(s, ";")
	}
	return out
}

// splitSlash splits SQL at each line that holds only a slash, and drops a
// piece that holds nothing but space and comments. A statement keeps what it
// ends with, so a PL/SQL block keeps its final semicolon.
func splitSlash(src string) []string {
	var out []string
	var buf []string
	flush := func() {
		text := strings.TrimSpace(strings.Join(buf, "\n"))
		buf = nil
		for l := range strings.SplitSeq(text, "\n") {
			if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "--") {
				out = append(out, text)
				return
			}
		}
	}
	for l := range strings.SplitSeq(src, "\n") {
		if strings.TrimSpace(l) == "/" {
			flush()
			continue
		}
		buf = append(buf, l)
	}
	flush()
	return out
}

// indexFrom returns the index of sep at or after from, or the end of the text.
func indexFrom(s string, from int, sep string) int {
	if from > len(s) {
		return len(s)
	}
	if k := strings.Index(s[from:], sep); k >= 0 {
		return from + k
	}
	return len(s)
}

// closeQuote returns the index after the quote that closes the one at i. A
// doubled closing character is part of the text.
func closeQuote(s string, i int, closer byte) int {
	for j := i + 1; j < len(s); j++ {
		if s[j] != closer {
			continue
		}
		if j+1 < len(s) && s[j+1] == closer {
			j++
			continue
		}
		return j + 1
	}
	return len(s)
}

// dollarTag returns the tag at the start of s, such as $$ or $body$.
func dollarTag(s string) (string, bool) {
	for j := 1; j < len(s); j++ {
		switch {
		case s[j] == '$':
			return s[:j+1], true
		case isWordPart(s[j]) && (j != 1 || s[j] < '0' || s[j] > '9'):
		default:
			return "", false
		}
	}
	return "", false
}

func isWordStart(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '@' || c == '#'
}

func isWordPart(c byte) bool { return isWordStart(c) || c >= '0' && c <= '9' }
