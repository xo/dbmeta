package dump_test

import (
	"strings"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// partitionSnapshot is a PostgreSQL schema with a partitioned table, one partition
// that repeats the key and the index of its parent as PostgreSQL reports them, and
// a materialized view with an index.
func partitionSnapshot() *dump.Snapshot {
	col := func(name, raw string) dump.Column {
		typ, err := dump.ParseType(dbmeta.PostgreSQL, raw)
		if err != nil {
			panic(err)
		}
		return dump.Column{Name: name, Type: typ}
	}
	key := func(table string) dump.Constraint {
		return dump.Constraint{Name: table + "_pkey", Kind: "primary key", Columns: []string{"event_id", "happened"},
			Definition: "PRIMARY KEY (event_id, happened)"}
	}
	return &dump.Snapshot{
		Version: dump.SnapshotVersion, Dialect: "postgres",
		Schemas: []dump.Schema{{
			Name: "app",
			Tables: []dump.Table{
				{
					Name:         "events",
					Partitioning: &dump.Partitioning{Strategy: "range", Expression: "happened"},
					Columns:      []dump.Column{col("event_id", "integer"), col("happened", "date")},
					Constraints:  []dump.Constraint{key("events")},
					Indexes: []dump.Index{{Name: "events_pkey", Unique: true, Primary: true, Constraint: "p",
						Definition: "CREATE UNIQUE INDEX events_pkey ON ONLY app.events USING btree (event_id, happened)"}, {
						Name: "events_happened", Columns: []dump.IndexColumn{{Name: "happened"}},
						Definition: "CREATE INDEX events_happened ON ONLY app.events USING btree (happened)"}},
				},
				{
					Name:        "events_2025",
					PartitionOf: &dump.PartitionOf{Schema: "app", Table: "events", Bound: "FOR VALUES FROM ('2025-01-01') TO ('2026-01-01')"},
					Columns:     []dump.Column{col("event_id", "integer"), col("happened", "date")},
					Constraints: []dump.Constraint{key("events_2025")},
					Indexes: []dump.Index{{Name: "events_2025_pkey", Unique: true, Primary: true, Constraint: "p",
						Definition: "CREATE UNIQUE INDEX events_2025_pkey ON app.events_2025 USING btree (event_id, happened)"}, {
						Name: "events_2025_happened_idx", Columns: []dump.IndexColumn{{Name: "happened"}},
						Definition: "CREATE INDEX events_2025_happened_idx ON app.events_2025 USING btree (happened)"}, {
						Name: "events_2025_local", Columns: []dump.IndexColumn{{Name: "event_id"}},
						Definition: "CREATE INDEX events_2025_local ON app.events_2025 USING btree (event_id)"}},
				},
			},
			Views: []dump.View{{
				Name: "counts", Materialized: true, Definition: " SELECT happened, count(*) AS n FROM app.events GROUP BY happened;",
				Indexes: []dump.Index{{Name: "counts_happened", Unique: true, Columns: []dump.IndexColumn{{Name: "happened"}},
					Definition: "CREATE UNIQUE INDEX counts_happened ON app.counts USING btree (happened)"}},
			}},
		}},
	}
}

// scriptText writes a script for a target and returns each statement.
func scriptText(t *testing.T, s *dump.Snapshot, target dbmeta.Dialect) []dump.Statement {
	t.Helper()
	var out []dump.Statement
	for st, err := range dump.Script(s, target, dump.Policy{}) {
		if err != nil {
			t.Fatalf("writing the script for %s: %v", target, err)
		}
		out = append(out, st)
	}
	return out
}

func TestAPartitionedTableIsWrittenWithItsPartitions(t *testing.T) {
	t.Parallel()
	stmts := scriptText(t, partitionSnapshot(), dbmeta.PostgreSQL)
	var tables, indexes, constraints, views []string
	for _, st := range stmts {
		switch st.Kind {
		case "table":
			tables = append(tables, st.SQL)
		case "index":
			indexes = append(indexes, st.SQL)
		case "constraint":
			constraints = append(constraints, st.SQL)
		case "materialized view":
			views = append(views, st.SQL)
		}
	}
	if len(tables) != 2 || !strings.HasSuffix(tables[0], ") PARTITION BY RANGE (happened)") {
		t.Errorf("the tables are %q, and the first is the parent with PARTITION BY", tables)
	}
	if len(tables) == 2 && tables[1] != `CREATE TABLE "app"."events_2025" PARTITION OF "app"."events" FOR VALUES FROM ('2025-01-01') TO ('2026-01-01')` {
		t.Errorf("the partition is %s", tables[1])
	}
	if len(constraints) != 0 {
		t.Errorf("the constraints are %q, and the partition has the key of its parent", constraints)
	}
	// The parent index is made on every partition, and the partition keeps only the
	// index that its parent does not have.
	want := []string{
		"CREATE INDEX events_happened ON app.events USING btree (happened)",
		"CREATE INDEX events_2025_local ON app.events_2025 USING btree (event_id)",
		"CREATE UNIQUE INDEX counts_happened ON app.counts USING btree (happened)",
	}
	if strings.Join(indexes, "\n") != strings.Join(want, "\n") {
		t.Errorf("the indexes are\n%s\nwant\n%s", strings.Join(indexes, "\n"), strings.Join(want, "\n"))
	}
	if len(views) != 1 || views[0] != `CREATE MATERIALIZED VIEW "app"."counts" AS SELECT happened, count(*) AS n FROM app.events GROUP BY happened WITH DATA` {
		t.Errorf("the materialized view is %q", views)
	}
}

func TestAMaterializedViewComesAfterTheRowsAndBeforeItsIndex(t *testing.T) {
	t.Parallel()
	view, index := -1, -1
	for i, st := range scriptText(t, partitionSnapshot(), dbmeta.PostgreSQL) {
		switch {
		case st.Kind == "materialized view":
			view = i
			if st.Stage != dump.Post {
				t.Errorf("the materialized view is in the %s stage, and it reads rows that the data stage loads", st.Stage)
			}
		case st.Kind == "index" && strings.HasSuffix(st.Name, ".counts_happened"):
			index = i
		}
	}
	if view < 0 || index < view {
		t.Errorf("the view is statement %d and its index is %d", view, index)
	}
}

func TestAnotherProductGetsOneTableAndAPlainView(t *testing.T) {
	t.Parallel()
	for _, target := range []dbmeta.Dialect{dbmeta.SQLite3, dbmeta.MySQL} {
		var tables []string
		var notes []string
		var view dump.Statement
		for _, st := range scriptText(t, partitionSnapshot(), target) {
			switch st.Kind {
			case "table":
				tables = append(tables, st.Name)
				notes = append(notes, st.Note)
			case "materialized view":
				view = st
			case "index", "constraint":
				if strings.Contains(st.Name, "events_2025") {
					t.Errorf("%s: the statement %s %s is for a partition, which this product has no table for", target, st.Kind, st.Name)
				}
			}
		}
		if len(tables) != 1 || tables[0] != "app.events" {
			t.Errorf("%s: the tables are %q, and the partition is not a table of its own", target, tables)
		}
		if len(notes) == 1 && !strings.Contains(notes[0], "split into partitions") {
			t.Errorf("%s: the note of the table is %q", target, notes[0])
		}
		if strings.Contains(strings.Join(tables, " "), "PARTITION") {
			t.Errorf("%s: a table is partitioned", target)
		}
		if !strings.HasPrefix(view.SQL, "CREATE VIEW ") || !strings.Contains(view.Note, "plain view") {
			t.Errorf("%s: the materialized view is written as %q with the note %q", target, view.SQL, view.Note)
		}
	}
}

func TestAPartitionIsMarked(t *testing.T) {
	t.Parallel()
	for _, tb := range partitionSnapshot().Schemas[0].Tables {
		if got, want := tb.IsPartition(), tb.Name == "events_2025"; got != want {
			t.Errorf("IsPartition of %s is %t, want %t", tb.Name, got, want)
		}
	}
}

// typeSnapshot has an extension, two domains where one is built on the other, and a
// composite type that has an attribute of a domain. The names are in an order that
// the statements must not follow.
func typeSnapshot() *dump.Snapshot {
	attr := func(name, raw string) dump.Column {
		typ, err := dump.ParseType(dbmeta.PostgreSQL, raw)
		if err != nil {
			panic(err)
		}
		return dump.Column{Name: name, Type: typ, Nullable: true}
	}
	zero := "0"
	return &dump.Snapshot{
		Version: dump.SnapshotVersion, Dialect: "postgres",
		Extensions: []dump.Extension{{Name: "citext", Schema: "public", Comment: "case insensitive text"}},
		Schemas: []dump.Schema{{
			Name: "app",
			Domains: []dump.Domain{
				{Name: "big", Type: "app.small", Constraints: "CHECK (VALUE > 10)"},
				{Name: "small", Type: "integer", NotNull: true, Default: &zero, Constraints: "CHECK (VALUE >= 0)", Comment: "not negative"},
			},
			Composites: []dump.Composite{{Name: "a_point", Comment: "a place",
				Columns: []dump.Column{attr("x", "app.big"), attr("label", "text")}}},
		}},
	}
}

func TestDomainsAndCompositeTypesAreWrittenInTheOrderOfUse(t *testing.T) {
	t.Parallel()
	var got []string
	for _, st := range scriptText(t, typeSnapshot(), dbmeta.PostgreSQL) {
		switch st.Kind {
		case "extension", "type", "comment":
			got = append(got, st.SQL)
		}
	}
	want := []string{
		`CREATE EXTENSION IF NOT EXISTS "citext" WITH SCHEMA "public"`,
		`CREATE DOMAIN "app"."small" AS integer DEFAULT 0 NOT NULL CHECK (VALUE >= 0)`,
		`CREATE DOMAIN "app"."big" AS app.small CHECK (VALUE > 10)`,
		`CREATE TYPE "app"."a_point" AS ("x" app.big, "label" text)`,
		`COMMENT ON EXTENSION "citext" IS 'case insensitive text'`,
		`COMMENT ON DOMAIN "app"."small" IS 'not negative'`,
		`COMMENT ON TYPE "app"."a_point" IS 'a place'`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("the statements are\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestAnotherProductGetsNoExtensionDomainOrCompositeType(t *testing.T) {
	t.Parallel()
	for _, st := range scriptText(t, typeSnapshot(), dbmeta.SQLite3) {
		switch st.Kind {
		case "extension", "type":
			t.Errorf("the script for SQLite has the %s %s", st.Kind, st.Name)
		}
	}
}
