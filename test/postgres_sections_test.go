package test

import (
	"database/sql"
	"slices"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/models/postgres/fixture"
)

// readAll reads every row of a query, and fails the test on the first error.
func readAll[T any](t *testing.T, q *dbmeta.Query[T], m *dbmeta.Meta, db *sql.DB, args map[string]any) []T {
	t.Helper()
	var out []T
	for v, err := range q.All(t.Context(), m, db, args) {
		if err != nil {
			t.Fatalf("reading %s: %v", q.Name(), err)
		}
		out = append(out, v)
	}
	return out
}

// byName indexes rows by a name that the caller picks.
func byName[T any](rows []T, name func(T) string) map[string]T {
	out := make(map[string]T, len(rows))
	for _, v := range rows {
		out[name(v)] = v
	}
	return out
}

// TestSectionsReadBack reads every field and every kind that D199 added for
// the sections of psql's \d+ name, as typed values. A release too old for the
// source answers NULL or refuses the kind, and the test says which.
func TestSectionsReadBack(t *testing.T) {
	eachPostgres(t, func(t *testing.T, db *sql.DB, m *dbmeta.Meta) {
		schema := fixture.Everything.Schema
		in := dbmeta.Args{Schema: schema}.Map
		release := pgRelease(m)

		t.Run("table options and row security", func(t *testing.T) {
			needStep(t, m, "ledger view")
			tables := byName(readAll(t, dbmeta.Tables, m, db, in()), func(v dbmeta.Table) string { return v.Name })
			if got := tables["ledger"].Options; !got.Valid || got.V != "fillfactor=70, toast.autovacuum_enabled=false" {
				t.Errorf("expected the options of ledger with the TOAST option, got %+v", got)
			}
			if got := tables["ledger_view"].Options; !got.Valid ||
				!strings.Contains(got.V, "security_barrier=true") || !strings.Contains(got.V, "check_option=local") {
				t.Errorf("expected the options of a view, got %+v", got)
			}
			if got := tables["author"].Options; got.Valid {
				t.Errorf("expected no options on author, got %+v", got)
			}
			for name, want := range map[string]bool{"document": true, "ledger": false} {
				if got := tables[name].RowSecurity; !got.Valid || got.V != want {
					t.Errorf("expected row security %v on %s, got %+v", want, name, got)
				}
			}
			if got := tables["document"].RowSecurityForced; !got.Valid || !got.V {
				t.Errorf("expected forced row security on document, got %+v", got)
			}
			if got := tables["ledger"].RowSecurityForced; !got.Valid || got.V {
				t.Errorf("expected no forced row security on ledger, got %+v", got)
			}
		})

		t.Run("composite type", func(t *testing.T) {
			needStep(t, m, "composite type")
			// a composite type is listed only when the caller names it
			for _, v := range readAll(t, dbmeta.Tables, m, db, in()) {
				if v.Type == "composite type" {
					t.Errorf("expected no composite type without types, got %s", v.Name)
				}
			}
			args := dbmeta.Args{Schema: schema, Name: "point", Types: []string{"composite type"}}.Map()
			got := readAll(t, dbmeta.Tables, m, db, args)
			if len(got) != 1 || got[0].Type != "composite type" {
				t.Fatalf("expected the composite type point, got %+v", got)
			}
			cols := readAll(t, dbmeta.Columns, m, db, dbmeta.Args{Schema: schema, Parent: "point"}.Map())
			var names []string
			for _, c := range cols {
				names = append(names, c.Name)
			}
			if !slices.Equal(names, []string{"x", "y"}) {
				t.Errorf("expected the attributes x and y, got %v", names)
			}
		})

		t.Run("index options and include columns", func(t *testing.T) {
			needStep(t, m, "ledger index with options")
			indexes := byName(readAll(t, dbmeta.Indexes, m, db, in()), func(v dbmeta.Index) string { return v.Name })
			if got := indexes["ledger_note"].Options; !got.Valid || got.V != "fillfactor=60" {
				t.Errorf("expected the options of ledger_note, got %+v", got)
			}
			if got := indexes["author_pkey"].Options; got.Valid {
				t.Errorf("expected no options on author_pkey, got %+v", got)
			}
			if release < 11 {
				for _, v := range readAll(t, dbmeta.IndexColumns, m, db, in()) {
					if v.Include {
						t.Errorf("expected no include column below release 11, got %s", v.Index)
					}
				}
				return
			}
			for index, want := range map[string]map[string]bool{
				"book_covering":   {"author_id": false, "title": true},
				"ledger_covering": {"id": false, "note": true},
			} {
				args := dbmeta.Args{Schema: schema, Name: index}.Map()
				cols := readAll(t, dbmeta.IndexColumns, m, db, args)
				if len(cols) != len(want) {
					t.Fatalf("expected %d columns in %s, got %+v", len(want), index, cols)
				}
				for _, c := range cols {
					if !c.Name.Valid || c.Include != want[c.Name.V] {
						t.Errorf("expected include %v on %s of %s, got %+v", want[c.Name.V], c.Name.V, index, c)
					}
				}
			}
		})

		t.Run("partitions", func(t *testing.T) {
			needStep(t, m, "partition")
			if release < 10 {
				if got := dbmeta.Partitions.Support(m); got != dbmeta.TooOld {
					t.Errorf("expected Partitions to be too old below release 10, got %v", got)
				}
				return
			}
			parts := byName(readAll(t, dbmeta.Partitions, m, db,
				dbmeta.Args{Schema: schema, Parent: "sales"}.Map()), func(v dbmeta.Partition) string { return v.Partition })
			p26 := parts["sales_2026"]
			if p26.Table != "sales" || p26.PartitionSchema != schema || p26.Type != "table" {
				t.Errorf("expected sales_2026 to be a table partition of sales, got %+v", p26)
			}
			if got := p26.Bound; !got.Valid || got.V != "FOR VALUES FROM ('2026-01-01') TO ('2027-01-01')" {
				t.Errorf("expected the bound of sales_2026, got %+v", got)
			}
			if got := p26.Constraint; !got.Valid || !strings.Contains(got.V, "sold_on") {
				t.Errorf("expected the partition constraint of sales_2026, got %+v", got)
			}
			if p26.Partitioned || p26.DetachPending {
				t.Errorf("expected sales_2026 to be a leaf that is not detaching, got %+v", p26)
			}
			if _, ok := parts["sales_2027"]; !ok {
				t.Errorf("expected sales_2027, got %v", parts)
			}
			wantParts := 2
			if release >= 11 {
				wantParts = 3
				if got := parts["sales_other"].Bound; !got.Valid || got.V != "DEFAULT" {
					t.Errorf("expected the default partition, got %+v", got)
				}
			}
			if len(parts) != wantParts {
				t.Errorf("expected %d partitions of sales, got %v", wantParts, parts)
			}
			// a partition is found by its own name too, which is how psql
			// describes it
			one := readAll(t, dbmeta.Partitions, m, db, dbmeta.Args{Schema: schema, Name: "sales_2027"}.Map())
			if len(one) != 1 || one[0].Table != "sales" {
				t.Errorf("expected sales_2027 to name sales as its parent, got %+v", one)
			}
			if release >= 11 {
				nested := readAll(t, dbmeta.Partitions, m, db, dbmeta.Args{Schema: schema, Name: "region_sales_east"}.Map())
				if len(nested) != 1 || !nested[0].Partitioned {
					t.Errorf("expected region_sales_east to have partitions of its own, got %+v", nested)
				}
			}
		})

		t.Run("partitioned tables and indexes", func(t *testing.T) {
			needStep(t, m, "partitioned table")
			if release < 10 {
				return
			}
			rows := byName(readAll(t, dbmeta.PartitionedTables, m, db, in()), func(v dbmeta.PartitionedTable) string { return v.Name })
			sales := rows["sales"]
			if sales.Expression != "RANGE (sold_on)" || sales.Table.Valid {
				t.Errorf("expected the key of sales and no table, got %+v", sales)
			}
			if release < 12 {
				if sales.DirectSize.Valid || sales.TotalSize.Valid {
					t.Errorf("expected no sizes below release 12, got %+v", sales)
				}
			} else {
				if !sales.DirectSize.Valid || !sales.TotalSize.Valid || sales.DirectSize.V < 0 || sales.TotalSize.V < sales.DirectSize.V {
					t.Errorf("expected sizes of the partitions, got %+v %+v", sales.DirectSize, sales.TotalSize)
				}
			}
			if release < 11 {
				return
			}
			idx := rows["sales_amount"]
			if idx.Type != "index" || !idx.Table.Valid || idx.Table.V != schema+".sales" {
				t.Errorf("expected a partitioned index on sales, got %+v", idx)
			}
			if !idx.AccessMethod.Valid || idx.AccessMethod.V != "btree" {
				t.Errorf("expected the btree access method, got %+v", idx.AccessMethod)
			}
			if release >= 12 && !idx.TotalSize.Valid {
				t.Errorf("expected the total size of the index, got %+v", idx.TotalSize)
			}
		})

		t.Run("inheritance", func(t *testing.T) {
			needStep(t, m, "ledger child")
			parents := readAll(t, dbmeta.Inherits, m, db, map[string]any{"schema": schema, "name": "ledger_child"})
			if len(parents) != 1 {
				t.Fatalf("expected one parent of ledger_child, got %+v", parents)
			}
			p := parents[0]
			if p.Parent != "ledger" || p.ParentSchema != schema || p.Ordinal != 1 || p.Partition || p.Type != "table" {
				t.Errorf("expected ledger_child to inherit from ledger, got %+v", p)
			}
			children := readAll(t, dbmeta.Inherits, m, db, map[string]any{"parent_schema": schema, "parent": "ledger"})
			if len(children) != 1 || children[0].Name != "ledger_child" {
				t.Errorf("expected ledger_child as the only child of ledger, got %+v", children)
			}
			if release < 10 {
				return
			}
			// a partition is a child too, and the flag says so
			sales := readAll(t, dbmeta.Inherits, m, db, map[string]any{"schema": schema, "name": "sales_2026"})
			if len(sales) != 1 || !sales[0].Partition {
				t.Errorf("expected sales_2026 to be a partition, got %+v", sales)
			}
		})

		t.Run("policies", func(t *testing.T) {
			needStep(t, m, "document read policy")
			pols := byName(readAll(t, dbmeta.Policies, m, db,
				dbmeta.Args{Schema: schema, Parent: "document"}.Map()), func(v dbmeta.Policy) string { return v.Name })
			want := 3
			if release < 10 {
				want = 2
			}
			if len(pols) != want {
				t.Fatalf("expected %d policies, got %+v", want, pols)
			}
			read, write := pols["document_read"], pols["document_write"]
			if read.Command != "select" || !read.Permissive {
				t.Errorf("expected a permissive select policy, got %+v", read)
			}
			if !read.Roles.Valid || read.Roles.V != "dbmeta_fixture_role" {
				t.Errorf("expected the role of the policy, got %+v", read.Roles)
			}
			if !read.Using.Valid || !strings.Contains(read.Using.V, "owner_name") || read.WithCheck.Valid {
				t.Errorf("expected a USING and no WITH CHECK, got %+v %+v", read.Using, read.WithCheck)
			}
			if write.Command != "update" || write.Roles.Valid {
				t.Errorf("expected an update policy for public, got %+v", write)
			}
			if !write.WithCheck.Valid || write.WithCheck.V != "(document_id > 0)" {
				t.Errorf("expected the WITH CHECK of the policy, got %+v", write.WithCheck)
			}
			if release >= 10 {
				if limit := pols["document_limit"]; limit.Permissive || limit.Command != "all" {
					t.Errorf("expected a restrictive policy for all commands, got %+v", limit)
				}
			}
		})

		t.Run("rules", func(t *testing.T) {
			needStep(t, m, "ledger disable rule")
			rules := byName(readAll(t, dbmeta.Rules, m, db,
				dbmeta.Args{Schema: schema, Parent: "ledger"}.Map()), func(v dbmeta.Rule) string { return v.Name })
			if len(rules) != 2 {
				t.Fatalf("expected two rules, got %+v", rules)
			}
			log, skip := rules["ledger_log"], rules["ledger_skip"]
			if log.Event != "insert" || log.Enabled != "enabled" || log.Instead ||
				!strings.Contains(log.Definition, "NOTIFY ledger_changed") || strings.HasSuffix(log.Definition, ";") {
				t.Errorf("expected the rule ledger_log, got %+v", log)
			}
			if skip.Event != "delete" || skip.Enabled != "disabled" || !skip.Instead {
				t.Errorf("expected the rule ledger_skip, got %+v", skip)
			}
			// the rule that makes a view is the view's definition
			if got := readAll(t, dbmeta.Rules, m, db, dbmeta.Args{Schema: schema, Parent: "recent"}.Map()); len(got) != 0 {
				t.Errorf("expected no rule on a view, got %+v", got)
			}
		})

		t.Run("not null constraints", func(t *testing.T) {
			needStep(t, m, "ledger table")
			if release < 18 {
				if got := dbmeta.NotNulls.Support(m); got != dbmeta.TooOld {
					t.Errorf("expected NotNulls to be too old below release 18, got %v", got)
				}
				return
			}
			rows := byName(readAll(t, dbmeta.NotNulls, m, db,
				dbmeta.Args{Schema: schema, Parent: "ledger"}.Map()), func(v dbmeta.NotNull) string { return v.Name })
			note := rows["ledger_note_present"]
			if note.Column != "note" || note.NoInherit || !note.Local || note.Inherited || !note.Validated {
				t.Errorf("expected the named constraint on note, got %+v", note)
			}
			if stamp := rows["ledger_stamp_present"]; !stamp.NoInherit || stamp.Column != "stamp" {
				t.Errorf("expected a NO INHERIT constraint on stamp, got %+v", stamp)
			}
			if id := rows["ledger_id_not_null"]; id.Column != "id" {
				t.Errorf("expected the generated name of the constraint on id, got %v", rows)
			}
			child := readAll(t, dbmeta.NotNulls, m, db, dbmeta.Args{Schema: schema, Parent: "ledger_child"}.Map())
			if len(child) == 0 {
				t.Fatal("expected ledger_child to hold the inherited constraints")
			}
			for _, v := range child {
				if !v.Inherited || v.Local {
					t.Errorf("expected an inherited constraint, got %+v", v)
				}
			}
			// Constraints still leaves them out, as D49 says
			for _, v := range readAll(t, dbmeta.Constraints, m, db, dbmeta.Args{Schema: schema, Parent: "ledger"}.Map()) {
				t.Errorf("expected no constraint on ledger, got %+v", v)
			}
		})

		t.Run("exclusion constraint", func(t *testing.T) {
			needStep(t, m, "booking table")
			got := readAll(t, dbmeta.Constraints, m, db, dbmeta.Args{Schema: schema, Parent: "booking"}.Map())
			var exclusion *dbmeta.Constraint
			for i := range got {
				if got[i].Type == "exclusion" {
					exclusion = &got[i]
				}
			}
			if exclusion == nil {
				t.Fatalf("expected an exclusion constraint, got %+v", got)
			}
			want := "EXCLUDE USING gist (int4range(booking_id, booking_id + 1) WITH &&)"
			if !exclusion.Definition.Valid || exclusion.Definition.V != want {
				t.Errorf("expected %q, got %+v", want, exclusion.Definition)
			}
		})

		t.Run("publications of a table", func(t *testing.T) {
			needStep(t, m, "publication of every table")
			if release < 10 {
				return
			}
			args := func(implicit bool) map[string]any {
				return map[string]any{"schema": schema, "parent": "ledger", "with_implicit": implicit}
			}
			named := readAll(t, dbmeta.PublicationTables, m, db, map[string]any{"schema": schema, "parent": "ledger"})
			if release < 15 {
				if len(named) != 0 {
					t.Errorf("expected no publication to name ledger below release 15, got %+v", named)
				}
			} else {
				if len(named) != 1 || named[0].Publication != "dbmeta_fixture_pub_rows" || named[0].Via.V != "table" {
					t.Fatalf("expected one publication that names ledger, got %+v", named)
				}
				if named[0].Columns != "id, note" || !named[0].Where.Valid || named[0].Where.V != "(id > 0)" {
					t.Errorf("expected the column list and the row filter, got %+v", named[0])
				}
			}
			all := byName(readAll(t, dbmeta.PublicationTables, m, db, args(true)),
				func(v dbmeta.PublicationTable) string { return v.Publication })
			if got := all["dbmeta_fixture_pub_all"]; got.Via.V != "all tables" || got.Name != "ledger" {
				t.Errorf("expected ledger through the publication of every table, got %+v", got)
			}
			if release >= 15 {
				if got := all["dbmeta_fixture_pub_schema"]; got.Via.V != "schema" || got.Name != "ledger" {
					t.Errorf("expected ledger through the publication of its schema, got %+v", got)
				}
				if got := all["dbmeta_fixture_pub_rows"]; got.Via.V != "table" {
					t.Errorf("expected the publication that names ledger, got %+v", got)
				}
			}
		})

		t.Run("statistics objects", func(t *testing.T) {
			needStep(t, m, "extended statistics")
			if m.Dialect() != dbmeta.PostgreSQL {
				t.Skipf("%s has its own statistics objects, which take no parent", m.Dialect())
			}
			if release < 10 {
				return
			}
			stats := byName(readAll(t, dbmeta.ExtendedStats, m, db,
				dbmeta.Args{Schema: schema, Parent: "book"}.Map()), func(v dbmeta.ExtendedStat) string { return v.Name.V })
			bs := stats["book_stats"]
			if bs.Table != "book" || !bs.Ndistinct || !bs.Dependencies {
				t.Errorf("expected book_stats on book, got %+v", bs)
			}
			if release >= 13 {
				if bs.StatsTarget.V != 200 || !bs.StatsTarget.Valid {
					t.Errorf("expected a target of 200, got %+v", bs.StatsTarget)
				}
			} else if bs.StatsTarget.Valid {
				t.Errorf("expected no target below release 13, got %+v", bs.StatsTarget)
			}
			if release >= 14 {
				ts := stats["title_stats"]
				if ts.StatsTarget.Valid || !ts.Definition.Valid || !strings.Contains(ts.Definition.V, "lower(title)") {
					t.Errorf("expected title_stats with the default target, got %+v", ts)
				}
			}
			if all := readAll(t, dbmeta.ExtendedStats, m, db, dbmeta.Args{Schema: schema, Parent: "author"}.Map()); len(all) != 0 {
				t.Errorf("expected no statistics object on author, got %+v", all)
			}
		})

		t.Run("foreign table", func(t *testing.T) {
			needStep(t, m, "foreign table")
			got := readAll(t, dbmeta.ForeignTables, m, db, dbmeta.Args{Schema: schema, Name: "remote_ledger"}.Map())
			if len(got) != 1 || got[0].Server != "dbmeta_fixture_server" {
				t.Fatalf("expected remote_ledger on its server, got %+v", got)
			}
			if !got[0].Options.Valid || !strings.Contains(got[0].Options.V, "table_name=ledger") {
				t.Errorf("expected the options of the foreign table, got %+v", got[0].Options)
			}
			tables := readAll(t, dbmeta.Tables, m, db, dbmeta.Args{Schema: schema, Name: "remote_ledger"}.Map())
			if len(tables) != 1 || tables[0].Type != "foreign table" {
				t.Errorf("expected a foreign table, got %+v", tables)
			}
		})

		t.Run("view definition", func(t *testing.T) {
			needStep(t, m, "ledger view")
			views := byName(readAll(t, dbmeta.Views, m, db, in()), func(v dbmeta.View) string { return v.Name })
			for _, name := range []string{"ledger_view", "recent", "author_count"} {
				if got := views[name].Definition; !got.Valid || !strings.Contains(got.V, "SELECT") {
					t.Errorf("expected the definition of %s, got %+v", name, got)
				}
			}
			if got := views["ledger_view"].CheckOption; !got.Valid || got.V != "local" {
				t.Errorf("expected a local check option, got %+v", got)
			}
		})
	})
}
