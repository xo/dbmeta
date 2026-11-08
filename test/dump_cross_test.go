package test

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// crossSide is one product as a source and as a target of a dump.
type crossSide struct {
	name    string
	dialect dbmeta.Dialect
	// open returns an empty database of the product, and the filter that reads
	// the schema that the DDL builds.
	open   func(t *testing.T, role string) (*sql.DB, dump.Filter)
	create []string
	// shared says that both roles are one database, as a server of one database
	// is, so a pair of the same product runs over itself, so it is left out.
	shared bool
	// dropsUnique says that the product has no foreign key and refuses a unique key
	// that does not hold its shard key, so the copy has neither.
	dropsUnique bool
}

// table names a table of the fixture in a product, for the source or for the
// copy. The source of PostgreSQL has a schema of its own, and the copy of the
// products with schemas is in dump_cross.
func (s crossSide) table(role, name string) string {
	switch {
	case (s.dialect == dbmeta.PostgreSQL || s.dialect == dbmeta.CockroachDB) && role == "src":
		return "dump_src." + name
	case (s.dialect == dbmeta.PostgreSQL || s.dialect == dbmeta.CockroachDB || s.dialect == dbmeta.DuckDB || s.dialect == dbmeta.SQLServer) && role == "dst":
		return "dump_cross." + name
	}
	return name
}

// The tables have what every product has: integers, text, a decimal, a boolean,
// dates, bytes, a default, a primary key, a unique key, a foreign key and an index.
var (
	crossPG = []string{
		"CREATE SCHEMA dump_src",
		"CREATE TABLE dump_src.owner (id integer PRIMARY KEY, name varchar(20) NOT NULL UNIQUE, active boolean DEFAULT true)",
		`CREATE TABLE dump_src.item (id integer PRIMARY KEY, owner_id integer NOT NULL, price numeric(10,2), qty integer DEFAULT 1,
			note varchar(40) DEFAULT 'x', made timestamp, born date, data bytea, UNIQUE (owner_id, note),
			FOREIGN KEY (owner_id) REFERENCES dump_src.owner (id) ON DELETE CASCADE)`,
		"CREATE INDEX item_note ON dump_src.item (note)",
	}
	crossSQLite = []string{
		"CREATE TABLE owner (id integer PRIMARY KEY, name varchar(20) NOT NULL UNIQUE, active boolean DEFAULT 1)",
		`CREATE TABLE item (id integer PRIMARY KEY, owner_id integer NOT NULL, price decimal(10,2), qty integer DEFAULT 1,
			note varchar(40) DEFAULT 'x', made datetime, born date, data blob, UNIQUE (owner_id, note),
			FOREIGN KEY (owner_id) REFERENCES owner (id) ON DELETE CASCADE)`,
		"CREATE INDEX item_note ON item (note)",
	}
	crossDuckDB = []string{
		"CREATE TABLE owner (id integer PRIMARY KEY, name varchar NOT NULL UNIQUE, active boolean DEFAULT true)",
		`CREATE TABLE item (id integer PRIMARY KEY, owner_id integer NOT NULL, price decimal(10,2), qty integer DEFAULT 1,
			note varchar DEFAULT 'x', made timestamp, born date, data blob, UNIQUE (owner_id, note),
			FOREIGN KEY (owner_id) REFERENCES owner (id))`,
		"CREATE INDEX item_note ON item (note)",
	}
	crossMySQL = []string{
		"CREATE TABLE owner (id int PRIMARY KEY, name varchar(20) NOT NULL UNIQUE, active tinyint(1) DEFAULT 1)",
		`CREATE TABLE item (id int PRIMARY KEY, owner_id int NOT NULL, price decimal(10,2), qty int DEFAULT 1,
			note varchar(40) DEFAULT 'x', made datetime, born date, data blob, UNIQUE (owner_id, note),
			FOREIGN KEY (owner_id) REFERENCES owner (id) ON DELETE CASCADE)`,
		"CREATE INDEX item_note ON item (note)",
	}
)

func crossSides() []crossSide {
	return append(baseCrossSides(), extraCrossSides()...)
}

func baseCrossSides() []crossSide {
	return []crossSide{
		{name: "postgres", dialect: dbmeta.PostgreSQL, create: crossPG,
			open: func(t *testing.T, role string) (*sql.DB, dump.Filter) {
				db := openPostgresWith(t, postgresDrivers[0])
				schema := map[string]string{"src": "dump_src", "dst": "dump_cross"}[role]
				drop := func() { _, _ = db.ExecContext(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") }
				drop()
				t.Cleanup(drop)
				return db, dump.Filter{Schemas: []string{schema}}
			}},
		{name: "sqlite3", dialect: dbmeta.SQLite3, create: crossSQLite,
			open: func(t *testing.T, _ string) (*sql.DB, dump.Filter) {
				return openSQLiteAt(t, "sqlite3"), dump.Filter{Schemas: []string{"main"}}
			}},
		{name: "duckdb", dialect: dbmeta.DuckDB, create: crossDuckDB,
			open: func(t *testing.T, role string) (*sql.DB, dump.Filter) {
				if role == "dst" {
					return openDuckDBAt(t), dump.Filter{Schemas: []string{"dump_cross"}}
				}
				return openDuckDBAt(t), dump.Filter{Schemas: []string{"main"}}
			}},
		{name: "mysql", dialect: dbmeta.MySQL, create: crossMySQL,
			open: func(t *testing.T, role string) (*sql.DB, dump.Filter) {
				name := "dbmeta_cross_" + role
				return openMySQLNamed(t, name), dump.Filter{Schemas: []string{name}}
			}},
	}
}

// shape writes what a snapshot says about its tables as one string, without the
// parts that differ by product: the schema, the names of constraints, the
// lengths of strings and the text of defaults.
func shape(s *dump.Snapshot) string {
	var lines []string
	for _, sc := range s.Schemas {
		for _, t := range sc.Tables {
			pk := map[string]bool{}
			for _, c := range t.Constraints {
				if c.Kind == "primary key" {
					for _, name := range c.Columns {
						pk[name] = true
					}
				}
			}
			for _, c := range t.Columns {
				// MariaDB gives the text NULL as the default of a nullable column that has
				// none, which TestMySQLNullDefault pins, so that is no default here.
				// A serial column reads a sequence, which is auto numbering and not a default.
				// CockroachDB numbers a serial with unique_rowid().
				hasDefault := c.Default != nil && !strings.EqualFold(*c.Default, "NULL") &&
					!strings.HasPrefix(strings.ToLower(*c.Default), "nextval(") &&
					!strings.EqualFold(*c.Default, "unique_rowid()")
				lines = append(lines, fmt.Sprintf("%s.%s kind=%s null=%v default=%v", t.Name, c.Name, c.Type.Kind, c.Nullable || pk[c.Name], hasDefault))
			}
			for _, c := range t.Constraints {
				text := t.Name + " " + c.Kind + " (" + strings.Join(c.Columns, ",") + ")"
				if c.Ref != nil {
					text += " -> " + c.Ref.Table + " (" + strings.Join(c.Ref.Columns, ",") + ")"
				}
				lines = append(lines, text)
			}
			for _, ix := range t.Indexes {
				if !ix.Unique && !ix.Primary && ix.Constraint == "" {
					lines = append(lines, t.Name+" index "+ix.Name)
				}
			}
		}
	}
	slices.Sort(lines)
	return strings.Join(lines, "\n")
}

// TestAScriptCrossesDialects reads the same tables from each product, writes the
// script for each other product, runs it on an empty database, and requires the
// same shape. A decimal is allowed to be Lossy where the target has no exact one.
func TestAScriptCrossesDialects(t *testing.T) {
	policy := dump.Policy{Default: dump.Allow{Lossy: true}}
	for _, src := range crossSides() {
		for _, dst := range crossSides() {
			if src.name == dst.name {
				continue
			}
			t.Run(src.name+" to "+dst.name, func(t *testing.T) {
				srcDB, srcFilter := src.open(t, "src")
				for _, s := range src.create {
					mustExec(t, srcDB, s)
				}
				srcMeta := metaFor(t, src.dialect, srcDB)
				snap, err := dump.Read(t.Context(), srcMeta, srcDB, srcFilter)
				if err != nil {
					t.Fatalf("reading the source: %v", err)
				}
				renameSchemas(snap, "dump_cross")
				want := shape(snap)
				if dst.dropsUnique {
					want = withoutLines(want, " unique (", " foreign key ")
				}

				var stmts []dump.Statement
				for st, err := range dump.Script(snap, dst.dialect, policy) {
					if err != nil {
						t.Fatalf("writing the script: %v", err)
					}
					stmts = append(stmts, st)
				}
				dstDB, dstFilter := dst.open(t, "dst")
				runScript(t, dstDB, stmts)
				dstMeta := metaFor(t, dst.dialect, dstDB)
				copied, err := dump.Read(t.Context(), dstMeta, dstDB, dstFilter)
				if err != nil {
					t.Fatalf("reading the copy: %v", err)
				}
				if got := shape(copied); got != want {
					t.Errorf("the copy differs from the source:\n%s", lineDiff(want, got))
				}
			})
		}
	}
}

// renameSchemas puts every table of a snapshot in one schema of the given name,
// and points the foreign keys at it, so that a copy has a schema of its own.
func renameSchemas(s *dump.Snapshot, name string) {
	if len(s.Schemas) == 0 {
		return
	}
	merged := dump.Schema{Name: name}
	for _, sc := range s.Schemas {
		merged.Tables = append(merged.Tables, sc.Tables...)
		merged.Views = append(merged.Views, sc.Views...)
	}
	for i := range merged.Tables {
		for j := range merged.Tables[i].Constraints {
			if ref := merged.Tables[i].Constraints[j].Ref; ref != nil {
				ref.Schema = name
			}
		}
	}
	s.Schemas = []dump.Schema{merged}
}

// withoutLines removes the lines of a shape that hold a word.
func withoutLines(shape string, words ...string) string {
	var keep []string
	for line := range strings.SplitSeq(shape, "\n") {
		if !slices.ContainsFunc(words, func(w string) bool { return strings.Contains(line, w) }) {
			keep = append(keep, line)
		}
	}
	return strings.Join(keep, "\n")
}
