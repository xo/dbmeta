package dbmeta_test

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/xo/dbmeta"

	dkfixture "github.com/xo/dbmeta/models/duckdb/fixture"
	fbfixture "github.com/xo/dbmeta/models/firebird/fixture"
	myfixture "github.com/xo/dbmeta/models/mysql/fixture"
	pgfixture "github.com/xo/dbmeta/models/postgres/fixture"
	sqfixture "github.com/xo/dbmeta/models/sqlite3/fixture"
	msfixture "github.com/xo/dbmeta/models/sqlserver/fixture"
)

// Every model ships a fixture, and nothing checked them against each other
// until this existed. They drifted: 32 steps on PostgreSQL, 19 on MariaDB, 14
// on SQLite and 12 on DuckDB, and three of the four built a region and a
// shipment table while SQLite did not.
//
// That drift is why a cross family comparison could not be written. Comparing
// two databases on schemas that are not the same schema reports the fixtures,
// not the models.
//
// This test needs no database. It reads the SQL each fixture would run.

// coreObjects are the objects every fixture must build, whatever the database.
// They are the schema a cross family comparison reads, so a fixture that skips
// one takes that database out of the comparison silently.
//
// Add to this list only when every database can build the object. Anything one
// database cannot build belongs in that model's fixture alone, and the model's
// own tests cover it.
var coreObjects = []string{
	// a table with a single column key, a not null column, a nullable column
	// and a column with a default
	"author",
	// a table with a single column key, a foreign key, a unique constraint
	// and a check constraint
	"book",
	// an index somebody created, as against one a constraint created
	"book_published",
	// a table with a composite primary key
	"region",
	// a table with a composite foreign key into it
	"shipment",
	// a view
	"recent",
}

// newest returns a version set that meets every gate, for one product.
//
// The keys matter. Setting both "mariadb" and "mysql" makes the check
// constraint step ambiguous, because a fragment for each product then applies
// and nothing chooses between them. That is ErrAmbiguousFragment working, and
// it is why this asks each fixture about one product at a time.
func newest(keys ...string) dbmeta.VersionSet {
	var s dbmeta.VersionSet
	unknown := dbmeta.Version{Unknown: true}
	s.Set("", unknown)
	for _, key := range keys {
		s.Set(key, unknown)
	}
	return s
}

// sqlOf returns the SQL a step resolved to, and the empty string for a step
// the server cannot run, which cannot happen here because the version is
// unknown and an unknown version is newer than every known one.
func sqlOf(t *testing.T, query string, err error) string {
	t.Helper()
	switch {
	case errors.Is(err, dbmeta.ErrVersionTooOld), errors.Is(err, dbmeta.ErrNotSupported):
		return ""
	case err != nil:
		t.Fatalf("resolving a fixture step: %v", err)
	}
	return query + "\n"
}

// fixtureText returns everything one fixture would run, as one string.
type fixtureText struct {
	name  string
	steps int
	sql   string
}

func allFixtures(t *testing.T) []fixtureText {
	t.Helper()
	// Every fixture package declares its own Step type, so this cannot be a
	// loop over an interface without exporting one. Five literals are cheaper
	// than an interface nobody else needs.
	var out []fixtureText
	gather := func(name string, versions dbmeta.VersionSet, stmts []dbmeta.Stmt) {
		var b strings.Builder
		for _, stmt := range stmts {
			query, err := stmt.Build(versions)
			b.WriteString(sqlOf(t, query, err))
		}
		out = append(out, fixtureText{name, len(stmts), b.String()})
	}
	var pg, my, sq, dk, ms, fb []dbmeta.Stmt
	for _, s := range pgfixture.Everything.Setup {
		pg = append(pg, s.Stmt)
	}
	for _, s := range myfixture.Everything.Setup {
		my = append(my, s.Stmt)
	}
	for _, s := range sqfixture.Everything.Setup {
		sq = append(sq, s.Stmt)
	}
	for _, s := range dkfixture.Everything.Setup {
		dk = append(dk, s.Stmt)
	}
	for _, s := range msfixture.Everything.Setup {
		ms = append(ms, s.Stmt)
	}
	for _, s := range fbfixture.Everything.Setup {
		fb = append(fb, s.Stmt)
	}
	gather("postgres", newest(), pg)
	// One product at a time, because asking about both at once is ambiguous.
	gather("mariadb", newest("mariadb"), my)
	gather("mysql", newest("mysql"), my)
	gather("sqlite3", newest(), sq)
	gather("duckdb", newest(), dk)
	gather("sqlserver", newest(), ms)
	gather("firebird", newest(), fb)
	return out
}

// TestEveryFixtureBuildsTheCoreObjects is the guard that makes a cross family
// comparison possible. A fixture that stops building one of these takes its
// database out of the comparison, and without this it would do so silently.
func TestEveryFixtureBuildsTheCoreObjects(t *testing.T) {
	t.Parallel()
	for _, f := range allFixtures(t) {
		for _, object := range coreObjects {
			if !createsObject(f.sql, object) {
				t.Errorf("the %s fixture does not build %q.\n"+
					"Every fixture builds the core objects, so that a cross family "+
					"comparison reads the same schema everywhere. Add it, or take it "+
					"out of coreObjects and say why.", f.name, object)
			}
		}
	}
}

// TestFixturesAgreeOnTheSchemaName checks the one name a caller passes in.
func TestFixturesAgreeOnTheSchemaName(t *testing.T) {
	t.Parallel()
	for name, schema := range map[string]string{
		"postgres":  pgfixture.Everything.Schema,
		"mysql":     myfixture.Everything.Schema,
		"duckdb":    dkfixture.Everything.Schema,
		"sqlserver": msfixture.Everything.Schema,
	} {
		if schema != "dbmeta_fixture" {
			t.Errorf("the %s fixture uses the schema %q, and the others use dbmeta_fixture",
				name, schema)
		}
	}
	// SQLite is the exception and it is not drift. It has one schema per file
	// and it is called main, so the fixture cannot put its objects anywhere
	// else.
	if got := sqfixture.Everything.Schema; got != "main" {
		t.Errorf("expected the SQLite fixture to use main, got %q", got)
	}
}

// createsObject reports whether the SQL creates an object of that name.
//
// It must match a CREATE rather than the name anywhere, because a name appears
// in the statements that reference it as well as the one that makes it. The
// first version of this looked for the name anywhere, and renaming the region
// table did not fail the test: shipment's foreign key still said
// REFERENCES region(country, area).
func createsObject(query, object string) bool {
	pattern := `(?is)\bCREATE\s+(?:OR\s+REPLACE\s+)?(?:TEMP(?:ORARY)?\s+)?` +
		`(?:UNIQUE\s+)?(?:MATERIALIZED\s+)?(?:AGGREGATE\s+)?` +
		`(?:TABLE|VIEW|INDEX|SEQUENCE|TYPE|SCHEMA|MACRO|FUNCTION|PROCEDURE|TRIGGER|SERVER)\s+` +
		`(?:IF\s+NOT\s+EXISTS\s+)?(?:[a-z_0-9]+\.)?` + regexp.QuoteMeta(object) + `\b`
	return regexp.MustCompile(pattern).MatchString(query)
}
