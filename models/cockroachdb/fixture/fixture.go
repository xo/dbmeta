// Package fixture holds known good CockroachDB schemas.
//
// A fixture is a schema that contains one of everything the metadata queries
// read, so that a test asks a real server for metadata and gets an answer
// worth checking. Read the postgres fixture package for what a fixture is and
// what a caller can depend on. The types are that package's.
//
// CockroachDB builds most of the PostgreSQL fixture as it is, so [Everything]
// is the PostgreSQL fixture step by step. It keeps each step that CockroachDB
// builds, gates on CockroachDB's own release each step that only newer
// releases build, writes again each step whose syntax CockroachDB writes
// differently, and leaves out each step that CockroachDB cannot build at all.
// The steps and the reasons were measured on 24.3.36, 26.2.7 and 26.3.2, on
// 2026-09-29. See D123.
package fixture

import (
	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/models/cockroachdb"
	pgfixture "github.com/xo/dbmeta/models/postgres/fixture"
)

// v263 is CockroachDB 26.3, the first release measured to build a domain and
// a comment on a sequence or a function. 26.2 refuses all three.
var v263 = dbmeta.V(26, 3)

// left names each PostgreSQL step that CockroachDB cannot build, with the
// reason. The queries that read what the step builds answer no rows.
var left = map[string]string{
	// REVOKE from the owner is refused: "user root does not have privileges
	// over table revoked_privs". An owner keeps its privileges.
	"revoke": "an owner cannot revoke its own privileges",
	// CockroachDB partitions a table by its primary key, with its own
	// syntax, and shows no partition in pg_catalog.pg_inherits.
	"partitioned table": "PARTITION BY names no partitions, and pg_inherits holds none",
	"partition":         "PARTITION OF is not CockroachDB syntax",
	// CockroachDB has changefeeds and no publications.
	"publication": "CockroachDB has no publications",
	// CREATE STATISTICS takes columns and no expression.
	"expression statistics": "CockroachDB has no statistics on an expression",
	// CockroachDB refuses CREATE TEXT SEARCH CONFIGURATION and CREATE
	// COLLATION.
	"text search configuration": "CockroachDB cannot create a text search configuration",
	"collation with rules":      "CockroachDB cannot create a collation",
	// These steps were added for the describe commands of psql. They have
	// not been run on CockroachDB.
	"book covering index":                "not measured on CockroachDB",
	"book partial index":                 "not measured on CockroachDB",
	"fixture role":                       "not measured on CockroachDB",
	"role setting for every database":    "not measured on CockroachDB",
	"default privilege for every schema": "not measured on CockroachDB",
	"scratch table":                      "not measured on CockroachDB",
	"scratch storage and statistics":     "not measured on CockroachDB",
	"scratch compression":                "not measured on CockroachDB",
	"scratch replica identity index":     "not measured on CockroachDB",
	"scratch replica identity":           "not measured on CockroachDB",
	"scratch clustered index":            "not measured on CockroachDB",
	"scratch cluster":                    "not measured on CockroachDB",
	"scratch deferrable constraint":      "not measured on CockroachDB",
	"leakproof function":                 "not measured on CockroachDB",
}

// since gates a PostgreSQL step on a CockroachDB release, so that an older
// release skips it.
func since(ver dbmeta.Version, s pgfixture.Step) pgfixture.Step {
	query := s.Stmt[0][0].Query
	return pgfixture.Step{Name: s.Name, Stmt: dbmeta.Stmt{{{Key: cockroachdb.Release, Min: ver, Query: query}}}}
}

// Everything is a schema containing one of every object kind the CockroachDB
// queries read, built from the PostgreSQL fixture of the same name.
var Everything = func() pgfixture.Fixture {
	pg := pgfixture.Everything
	f := pgfixture.Fixture{
		Name:   pg.Name,
		Schema: pg.Schema,
		// The PostgreSQL teardown drops its publication first, and
		// CockroachDB has none to drop.
		Teardown: []pgfixture.Step{{Name: "drop schema", Stmt: dbmeta.Always(`DROP SCHEMA IF EXISTS ` + pg.Schema + ` CASCADE`)}},
	}
	for _, s := range pg.Setup {
		if _, skip := left[s.Name]; skip {
			continue
		}
		switch s.Name {
		case "domain", "sequence comment", "function comment":
			s = since(v263, s)
		case "author table":
			// Before 26.3 there is no domain, and rating is an integer.
			s.Stmt = dbmeta.Stmt{{
				{Key: cockroachdb.Release, Min: v263, Query: s.Stmt[0][0].Query},
				{Key: cockroachdb.Release, Query: `CREATE TABLE dbmeta_fixture.author (
	author_id serial PRIMARY KEY,
	name text NOT NULL,
	rating integer,
	shade dbmeta_fixture.colour DEFAULT 'red'
)`},
			}}
		case "extended statistics":
			// CockroachDB names the statistics without a schema.
			s.Stmt = dbmeta.Always(`CREATE STATISTICS book_stats ON author_id, published FROM dbmeta_fixture.book`)
		}
		f.Setup = append(f.Setup, s)
	}
	return f
}()

// Left says why [Everything] has no step of this name that the PostgreSQL
// fixture has, and is empty when it has one.
func Left(step string) string { return left[step] }
