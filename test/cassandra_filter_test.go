package test

import (
	"strings"
	"testing"

	"github.com/xo/dbmeta"
	cafixture "github.com/xo/dbmeta/models/cassandra/fixture"
)

// caSystem lists the keyspaces every Cassandra and ScyllaDB release keeps for
// itself. The model hides these unless a caller asks for with_system.
var caSystem = []string{
	"system", "system_schema", "system_auth", "system_distributed", "system_traces",
}

// TestCassandraFilters checks that each filter narrows, which CQL cannot do in
// a statement and the Keep function of each binding does. See D200.
func TestCassandraFilters(t *testing.T) {
	db := openCassandra(t)
	ctx := t.Context()
	m := setupCassandra(t, db)
	fix := cafixture.Everything.Schema

	// schemas reads the name of every keyspace the arguments keep.
	schemas := func(args dbmeta.Args) map[string]bool {
		t.Helper()
		got := map[string]bool{}
		for v, err := range dbmeta.Schemas.Each(ctx, m, db, args) {
			if err != nil {
				t.Fatalf("reading schemas: %v", err)
			}
			got[v.Name] = true
		}
		return got
	}
	hidden := schemas(dbmeta.Args{})
	if !hidden[fix] {
		t.Errorf("expected the fixture keyspace by default, got %v", hidden)
	}
	for _, name := range caSystem {
		if hidden[name] {
			t.Errorf("expected %s to be hidden by default", name)
		}
	}
	shown := schemas(dbmeta.Args{WithSystem: true})
	for _, name := range append([]string{fix}, caSystem...) {
		if !shown[name] {
			t.Errorf("expected %s with with_system, got %v", name, shown)
		}
	}
	if got := schemas(dbmeta.Args{Schema: `dbmeta\_%`}); len(got) != 1 || !got[fix] {
		t.Errorf("expected only the fixture keyspace for a schema pattern, got %v", got)
	}
	if got := schemas(dbmeta.Args{Name: "system_%"}); len(got) != 0 {
		t.Errorf("expected no system keyspace without with_system, got %v", got)
	}
	if got := schemas(dbmeta.Args{Name: "system_%", WithSystem: true}); !got["system_schema"] || got[fix] {
		t.Errorf("expected the system keyspaces for a name pattern, got %v", got)
	}
	if got := schemas(dbmeta.Args{Schema: strings.ToUpper(fix)}); len(got) != 0 {
		t.Errorf("expected a pattern to be case sensitive, got %v", got)
	}

	// Tables: schema, name, types and with_system.
	tables := func(args dbmeta.Args) []dbmeta.Table {
		t.Helper()
		var got []dbmeta.Table
		for v, err := range dbmeta.Tables.Each(ctx, m, db, args) {
			if err != nil {
				t.Fatalf("reading tables: %v", err)
			}
			got = append(got, v)
		}
		return got
	}
	for _, v := range tables(dbmeta.Args{}) {
		for _, name := range caSystem {
			if v.Schema == name {
				t.Errorf("expected no table of %s by default, got %s", name, v.Name)
			}
		}
	}
	var seen bool
	for _, v := range tables(dbmeta.Args{Schema: "system_schema", WithSystem: true}) {
		seen = true
		if v.Schema != "system_schema" {
			t.Errorf("expected only system_schema, got %s.%s", v.Schema, v.Name)
		}
	}
	if !seen {
		t.Error("expected the tables of system_schema with with_system")
	}
	if got := tables(dbmeta.Args{Schema: "system_schema"}); len(got) != 0 {
		t.Errorf("expected no table of system_schema without with_system, got %d", len(got))
	}
	if got := tables(dbmeta.Args{Schema: fix, Name: "b%"}); len(got) != 1 || got[0].Name != "book" {
		t.Errorf("expected only book for the name b%%, got %v", got)
	}
	if got := tables(dbmeta.Args{Schema: fix, Name: "bo_k"}); len(got) != 1 {
		t.Errorf("expected _ to match one character, got %v", got)
	}
	if got := tables(dbmeta.Args{Schema: fix, Types: []string{"view"}}); len(got) != 0 {
		t.Errorf("expected no table of the type view, got %v", got)
	}
	if got := tables(dbmeta.Args{Schema: fix, Types: []string{"view", "table"}}); len(got) < 4 {
		t.Errorf("expected the fixture tables for the types view and table, got %v", got)
	}

	// Columns: parent is the table.
	var cols int
	for v, err := range dbmeta.Columns.Each(ctx, m, db, dbmeta.Args{Schema: fix, Parent: "book"}) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		cols++
		if v.Schema != fix || v.Table != "book" {
			t.Errorf("expected only book, got %s.%s", v.Schema, v.Table)
		}
	}
	if cols == 0 {
		t.Error("expected the columns of book")
	}
	for v, err := range dbmeta.Columns.Each(ctx, m, db, dbmeta.Args{Schema: fix, Parent: "book", Name: "author%"}) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		if !strings.HasPrefix(v.Name, "author") {
			t.Errorf("expected only author columns, got %s", v.Name)
		}
	}

	// A role belongs to no keyspace. A name narrows it, and a schema pattern
	// that is not empty matches nothing.
	var roles int
	for v, err := range dbmeta.Roles.Each(ctx, m, db, dbmeta.Args{Name: `dbmeta\_reader`}) {
		if err != nil {
			t.Fatalf("reading roles: %v", err)
		}
		roles++
		if v.Name != "dbmeta_reader" {
			t.Errorf("expected only dbmeta_reader, got %s", v.Name)
		}
	}
	if roles != 1 {
		t.Errorf("expected one role, got %d", roles)
	}
	for _, err := range dbmeta.Roles.Each(ctx, m, db, dbmeta.Args{Schema: "x"}) {
		t.Errorf("expected no role for a schema pattern, got %v", err)
	}
}
