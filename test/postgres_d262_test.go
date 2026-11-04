package test

import (
	"context"
	"database/sql"
	"slices"
	"testing"

	"github.com/xo/dbmeta"
)

// TestPostgresDescribeFactsForUsql holds the facts that usql needs to print what psql
// prints, to a server. It builds a schema that is not on the search path, with
// one object of each kind, reads each fact, and drops the schema. See D262.
func TestPostgresDescribeFactsForUsql(t *testing.T) {
	f := pgFamilies[0]
	for _, driver := range f.drivers {
		t.Run(driver, func(t *testing.T) {
			db := openFamilyWith(t, f, driver)
			m := setupFamily(t, f, db)
			describeFactsForUsql(t, db, m)
		})
	}
}

func describeFactsForUsql(t *testing.T, db *sql.DB, m *dbmeta.Meta) {
	t.Helper()
	ctx := t.Context()
	var num int
	if err := db.QueryRowContext(ctx, "SELECT current_setting('server_version_num')::int").Scan(&num); err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	// identity columns arrived in release 10, partitioning in 10 and a
	// partitioned index in 11, and a generated column in 12
	identity, partitioned, generated := num >= 100000, num >= 110000, num >= 120000
	tableT := "CREATE TABLE dbm_desc.t (id serial PRIMARY KEY, name text)"
	switch {
	case generated:
		tableT = "CREATE TABLE dbm_desc.t (id integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY, name text, double integer GENERATED ALWAYS AS (id * 2) STORED)"
	case identity:
		tableT = "CREATE TABLE dbm_desc.t (id integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY, name text)"
	}
	steps := []string{
		"DROP SCHEMA IF EXISTS dbm_desc CASCADE",
		"DROP TABLE IF EXISTS public.dbm_desc_visible",
		"CREATE SCHEMA dbm_desc",
		tableT,
		"CREATE INDEX t_lower ON dbm_desc.t (lower(name))",
		"CREATE TABLE dbm_desc.serial_t (id serial PRIMARY KEY)",
		"COMMENT ON CONSTRAINT t_pkey ON dbm_desc.t IS 'the key of t'",
		"CREATE TABLE public.dbm_desc_visible (a integer)",
	}
	bg := context.WithoutCancel(ctx)
	t.Cleanup(func() {
		db.ExecContext(bg, "DROP SCHEMA IF EXISTS dbm_desc CASCADE")       //nolint:errcheck // best effort
		db.ExecContext(bg, "DROP TABLE IF EXISTS public.dbm_desc_visible") //nolint:errcheck // best effort
	})
	if partitioned {
		steps = append(steps,
			"CREATE TABLE dbm_desc.events (event_id bigint, happened date, PRIMARY KEY (event_id, happened)) PARTITION BY RANGE (happened)",
			"CREATE TABLE dbm_desc.events_2025 PARTITION OF dbm_desc.events FOR VALUES FROM ('2025-01-01') TO ('2026-01-01')",
			"CREATE TABLE dbm_desc.events_2026 PARTITION OF dbm_desc.events FOR VALUES FROM ('2026-01-01') TO ('2027-01-01')")
	}
	for _, s := range steps {
		if _, err := db.ExecContext(ctx, s); err != nil {
			t.Fatalf("%v\n%s", err, s)
		}
	}
	// the schema is not on the search path, and public is
	tables := readAll(t, dbmeta.Tables, m, db, dbmeta.Args{Schema: "dbm_desc", Name: "t"}.Map())
	if len(tables) != 1 || !tables[0].Visible.Valid || tables[0].Visible.V {
		t.Errorf("a table in a schema that is not on the search path is visible: %+v", tables)
	}
	tables = readAll(t, dbmeta.Tables, m, db, dbmeta.Args{Schema: "public", Name: "dbm_desc_visible"}.Map())
	if len(tables) != 1 || !tables[0].Visible.Valid || !tables[0].Visible.V {
		t.Errorf("a table in public is not visible: %+v", tables)
	}
	// the column of an index that holds an expression is a column of its own
	cols := readAll(t, dbmeta.IndexColumns, m, db, dbmeta.Args{Schema: "dbm_desc", Name: "t_lower"}.Map())
	if len(cols) != 1 || cols[0].ColumnName.V != "lower" || cols[0].ColumnType.V != "text" || cols[0].Storage.V != "extended" {
		t.Errorf("the index column of lower(name) is %+v", cols)
	}
	// identity and serial sequences
	seqs := byName(readAll(t, dbmeta.Sequences, m, db, dbmeta.Args{Schema: "dbm_desc"}.Map()), func(s dbmeta.Sequence) string { return s.Name })
	if s := seqs["t_id_seq"]; identity && (!s.Identity.Valid || !s.Identity.V) {
		t.Errorf("the sequence of an identity column is %+v", s)
	}
	if s := seqs["serial_t_id_seq"]; !s.Identity.Valid || s.Identity.V {
		t.Errorf("the sequence of a serial column is %+v", s)
	}
	if partitioned {
		// a partitioned index has partitions, and an index partition has a parent
		idx := byName(readAll(t, dbmeta.Indexes, m, db, dbmeta.Args{Schema: "dbm_desc", Parent: "events%"}.Map()), func(i dbmeta.Index) string { return i.Name })
		if i := idx["events_pkey"]; !i.Partitioned.Valid || !i.Partitioned.V {
			t.Errorf("the index of a partitioned table is not partitioned: %+v", i)
		}
		if i := idx["events_2025_pkey"]; !i.Partitioned.Valid || i.Partitioned.V {
			t.Errorf("the index of a partition is partitioned: %+v", i)
		}
		parts := readAll(t, dbmeta.Partitions, m, db, dbmeta.Args{Schema: "dbm_desc", Parent: "events_pkey", WithIndexes: true}.Map())
		if len(parts) != 2 {
			t.Errorf("the partitioned index has %d partitions, and the expected number is 2", len(parts))
		}
		if got := readAll(t, dbmeta.Partitions, m, db, dbmeta.Args{Schema: "dbm_desc", Parent: "events_pkey"}.Map()); len(got) != 0 {
			t.Errorf("the partitions of an index are read without the flag: %+v", got)
		}
		inh := readAll(t, dbmeta.Inherits, m, db, dbmeta.Args{Schema: "dbm_desc", Name: "events_2025_pkey", WithIndexes: true}.Map())
		if len(inh) != 1 || inh[0].Parent != "events_pkey" || !inh[0].Partition {
			t.Errorf("the parent of an index partition is %+v", inh)
		}
	}
	// the generated expression has no extra parentheses
	columns := byName(readAll(t, dbmeta.Columns, m, db, dbmeta.Args{Schema: "dbm_desc", Parent: "t"}.Map()), func(c dbmeta.Column) string { return c.Name })
	if got := columns["double"].Default.V; generated && got != "id * 2" {
		t.Errorf("the generated expression is %q, and the expected text has no parentheses of its own", got)
	}
	// the comment on a constraint, which psql lists with \dd
	var found bool
	for _, c := range readAll(t, dbmeta.Comments, m, db, dbmeta.Args{Schema: "dbm_desc"}.Map()) {
		if c.Type == "table constraint" && c.Name == "t_pkey" && c.Comment == "the key of t" && c.Visible.Valid && !c.Visible.V {
			found = true
		}
	}
	if !found {
		t.Error("the comment on the constraint t_pkey is not in Comments")
	}
	// the settings of a name that differs in case, and where a value came from
	settings := readAll(t, dbmeta.Settings, m, db, map[string]any{"name": "%style%"})
	var names []string
	for _, s := range settings {
		names = append(names, s.Name)
		if !s.Source.Valid {
			t.Errorf("the setting %s has no source", s.Name)
		}
	}
	if !slices.Contains(names, "DateStyle") {
		t.Errorf("the settings that match %%style%% are %v, and the expected list has DateStyle", names)
	}
	// the languages that have no call handler
	languages := func(args map[string]any) []string {
		var out []string
		for _, l := range readAll(t, dbmeta.Languages, m, db, args) {
			out = append(out, l.Name)
		}
		return out
	}
	if got := languages(nil); slices.Contains(got, "c") || slices.Contains(got, "internal") {
		t.Errorf("the languages with no system objects are %v", got)
	}
	if got := languages(dbmeta.Args{WithSystem: true}.Map()); !slices.Contains(got, "c") || !slices.Contains(got, "internal") || !slices.Contains(got, "sql") {
		t.Errorf("the languages with the system objects are %v", got)
	}
	// the memberships of the system roles
	for _, g := range readAll(t, dbmeta.RoleGrants, m, db, nil) {
		if len(g.Role) > 3 && g.Role[:3] == "pg_" {
			t.Errorf("the memberships of %s are read with no system objects", g.Role)
		}
	}
	// the kinds of token of the default parser
	tokens := readAll(t, dbmeta.TextSearchParserTokenTypes, m, db, dbmeta.Args{Schema: "pg_catalog", Name: "default", WithSystem: true}.Map())
	var aliases []string
	for _, tk := range tokens {
		aliases = append(aliases, tk.Alias)
	}
	if len(tokens) != 23 || !slices.Contains(aliases, "asciiword") || !slices.Contains(aliases, "email") {
		t.Errorf("the token types of the default parser are %v", aliases)
	}
	// aggregates sort by the argument types, as functions do
	prev := ""
	for _, fn := range readAll(t, dbmeta.Functions, m, db, dbmeta.Args{Schema: "pg_catalog", Name: "avg", WithSystem: true}.Map()) {
		if a := fn.ArgTypes.V; a < prev {
			t.Errorf("the arguments %q follow %q", a, prev)
		} else {
			prev = a
		}
	}
}
