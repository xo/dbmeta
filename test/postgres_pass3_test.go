package test

import (
	"database/sql"
	"slices"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/models/postgres/fixture"
)

// readVia reads every row of a query through any Queryer, such as one
// connection of the pool, and fails the test on the first error.
func readVia[T any](t *testing.T, q *dbmeta.Query[T], m *dbmeta.Meta, db dbmeta.Queryer, args map[string]any) []T {
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

// onePath returns one connection whose search path holds the fixture schema
// and pg_catalog, and puts the path back when the test ends. The pool's other
// connections keep the default path, which holds neither.
func onePath(t *testing.T, db *sql.DB, path string) *sql.Conn {
	t.Helper()
	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatalf("taking a connection: %v", err)
	}
	if _, err := conn.ExecContext(t.Context(), "SET search_path TO "+path); err != nil {
		t.Fatalf("setting the search path: %v", err)
	}
	t.Cleanup(func() {
		_, _ = conn.ExecContext(t.Context(), "RESET search_path")
		_ = conn.Close()
	})
	return conn
}

// TestPass3ValuesReadBack reads the fields that D201 added to existing kinds,
// as typed values, and what each one answers on a release too old for it.
func TestPass3ValuesReadBack(t *testing.T) {
	eachPostgres(t, func(t *testing.T, db *sql.DB, m *dbmeta.Meta) {
		needStep(t, m, "tagged table")
		schema := fixture.Everything.Schema
		release := pgRelease(m)

		t.Run("schema access", func(t *testing.T) {
			got := byName(readAll(t, dbmeta.Schemas, m, db, dbmeta.Args{Name: schema}.Map()),
				func(v dbmeta.Schema) string { return v.Name })[schema]
			if !got.Access.Valid || !strings.Contains(got.Access.V, "dbmeta_fixture_role=U/") {
				t.Errorf("expected the usage grant in the access of the schema, got %+v", got.Access)
			}
			plain := byName(readAll(t, dbmeta.Schemas, m, db, dbmeta.Args{Name: "pg_catalog", WithSystem: true}.Map()),
				func(v dbmeta.Schema) string { return v.Name })["pg_catalog"]
			if !plain.Access.Valid {
				t.Errorf("expected pg_catalog to hold an access list, got %+v", plain.Access)
			}
		})

		t.Run("database locale", func(t *testing.T) {
			var name string
			if err := db.QueryRowContext(t.Context(), `SELECT current_database()`).Scan(&name); err != nil {
				t.Fatalf("reading the database name: %v", err)
			}
			got := readAll(t, dbmeta.Databases, m, db, dbmeta.Args{Name: name}.Map())
			if len(got) != 1 {
				t.Fatalf("expected the database %s, got %+v", name, got)
			}
			d := got[0]
			if !d.LocaleProvider.Valid || !slices.Contains([]string{"libc", "icu", "builtin"}, d.LocaleProvider.V) {
				t.Errorf("expected a locale provider, got %+v", d.LocaleProvider)
			}
			if release < 15 && (d.LocaleProvider.V != "libc" || d.Locale.Valid) {
				t.Errorf("expected libc and no locale below release 15, got %+v %+v", d.LocaleProvider, d.Locale)
			}
			if release < 16 && d.ICURules.Valid {
				t.Errorf("expected no ICU rules below release 16, got %+v", d.ICURules)
			}
			if d.LocaleProvider.V == "libc" && d.Locale.Valid {
				t.Errorf("expected no ICU locale for libc, got %+v", d.Locale)
			}
		})

		t.Run("extension default version", func(t *testing.T) {
			got := readAll(t, dbmeta.Extensions, m, db, dbmeta.Args{Name: "plpgsql"}.Map())
			if len(got) != 1 || !got[0].DefaultVersion.Valid || got[0].DefaultVersion.V == "" {
				t.Errorf("expected the default version of plpgsql, got %+v", got)
			}
		})

		t.Run("operator leakproof", func(t *testing.T) {
			ops := readAll(t, dbmeta.Operators, m, db, dbmeta.Args{Schema: "pg_catalog", WithSystem: true}.Map())
			find := func(name, left, right string) (dbmeta.Operator, bool) {
				for _, o := range ops {
					if o.Name == name && o.LeftType == left && o.RightType == right {
						return o, true
					}
				}
				return dbmeta.Operator{}, false
			}
			eq, ok := find("=", "integer", "integer")
			if !ok || !eq.Leakproof {
				t.Errorf("expected integer = integer to be leakproof, got %+v", eq)
			}
			plus, ok := find("+", "integer", "integer")
			if !ok || plus.Leakproof {
				t.Errorf("expected integer + integer not to be leakproof, got %+v", plus)
			}
		})

		t.Run("cast leakproof", func(t *testing.T) {
			var yes, no, none int
			for _, c := range readAll(t, dbmeta.Casts, m, db, nil) {
				switch {
				case !c.LeakProof.Valid:
					none++
					if c.Function.V != "(binary coercible)" && c.Function.V != "(with inout)" {
						t.Errorf("expected a cast with no leakproof answer to have no function, got %+v", c)
					}
				case c.LeakProof.V:
					yes++
				default:
					no++
				}
			}
			if (yes == 0 && release >= 15) || no == 0 || none == 0 {
				t.Errorf("expected leakproof, not leakproof and no function casts, got %d %d %d", yes, no, none)
			}
		})

		t.Run("sequence cache", func(t *testing.T) {
			got := byName(readAll(t, dbmeta.Sequences, m, db, dbmeta.Args{Schema: schema}.Map()),
				func(v dbmeta.Sequence) string { return v.Name })["counter"]
			switch {
			case release < 10 && got.CacheSize.Valid:
				t.Errorf("expected no cache size below release 10, got %+v", got.CacheSize)
			case release >= 10 && (!got.CacheSize.Valid || got.CacheSize.V != 5):
				t.Errorf("expected a cache of 5, got %+v", got.CacheSize)
			}
		})

		t.Run("dictionary template schema", func(t *testing.T) {
			args := dbmeta.Args{Schema: "pg_catalog", Name: "simple", WithSystem: true}.Map()
			got := readAll(t, dbmeta.TextSearchDictionaries, m, db, args)
			if len(got) != 1 || got[0].Template != "simple" || !got[0].TemplateSchema.Valid || got[0].TemplateSchema.V != "pg_catalog" {
				t.Errorf("expected the template pg_catalog.simple, got %+v", got)
			}
		})

		t.Run("domain check", func(t *testing.T) {
			got := byName(readAll(t, dbmeta.Domains, m, db, dbmeta.Args{Schema: schema}.Map()),
				func(v dbmeta.Domain) string { return v.Name })
			if c := got["positive"].Constraints; c != "CHECK (VALUE > 0)" {
				t.Errorf("expected the check of positive alone, got %q", c)
			}
			if !got["positive"].Nullable {
				t.Errorf("expected positive to allow NULL, got %+v", got["positive"])
			}
			req := got["required"]
			if req.Nullable || strings.Contains(req.Constraints, "NOT NULL") || req.Constraints != "CHECK (VALUE <> ''::text)" {
				t.Errorf("expected a not null domain whose check has no NOT NULL, got %+v", req)
			}
		})

		t.Run("partitioned types", func(t *testing.T) {
			needStep(t, m, "partitioned table")
			if release < 10 {
				return
			}
			for _, p := range readAll(t, dbmeta.Privileges, m, db, dbmeta.Args{Schema: schema, Name: "sales"}.Map()) {
				if p.Name == "sales" && p.Type != "partitioned table" {
					t.Errorf("expected a partitioned table in the privileges, got %q", p.Type)
				}
			}
			parts := byName(readAll(t, dbmeta.PartitionedTables, m, db, dbmeta.Args{Schema: schema}.Map()),
				func(v dbmeta.PartitionedTable) string { return v.Name })
			if parts["sales"].Type != "partitioned table" {
				t.Errorf("expected a partitioned table, got %q", parts["sales"].Type)
			}
			if release >= 11 && parts["sales_amount"].Type != "partitioned index" {
				t.Errorf("expected a partitioned index, got %q", parts["sales_amount"].Type)
			}
			privs := byName(readAll(t, dbmeta.Privileges, m, db, dbmeta.Args{Schema: schema}.Map()),
				func(v dbmeta.Privilege) string { return v.Name })
			if privs["sales"].Type != "partitioned table" || privs["author"].Type != "table" {
				t.Errorf("expected partitioned table and table, got %q %q", privs["sales"].Type, privs["author"].Type)
			}
		})
	})
}

// TestPass3Publications reads GeneratedColumns, and the columns of \dRs+ that
// each release adds, and the connection string that only a superuser reads.
func TestPass3Publications(t *testing.T) {
	eachPostgres(t, func(t *testing.T, db *sql.DB, m *dbmeta.Meta) {
		needStep(t, m, "subscription")
		release := pgRelease(m)
		if release < 10 {
			t.Skip("release 9.6 has no publications")
		}

		pubs := byName(readAll(t, dbmeta.Publications, m, db, dbmeta.Args{Name: "dbmeta_fixture%"}.Map()),
			func(v dbmeta.Publication) string { return v.Name })
		for name, p := range pubs {
			want := "none"
			if name == "dbmeta_fixture_pub_gen" {
				want = "stored"
			}
			if !p.GeneratedColumns.Valid || p.GeneratedColumns.V != want {
				t.Errorf("expected %s to publish %s generated columns, got %+v", name, want, p.GeneratedColumns)
			}
		}
		if release >= 18 && len(pubs) < 4 {
			t.Errorf("expected the publication of generated columns, got %d publications", len(pubs))
		}

		subs := readAll(t, dbmeta.Subscriptions, m, db, dbmeta.Args{Name: "dbmeta_fixture_sub"}.Map())
		if len(subs) != 1 {
			t.Fatalf("expected the fixture subscription, got %+v", subs)
		}
		s := subs[0]
		check := func(name string, got sql.Null[bool], from int, want bool) {
			t.Helper()
			switch {
			case release < from && got.Valid:
				t.Errorf("expected no %s below release %d, got %+v", name, from, got)
			case release >= from && (!got.Valid || got.V != want):
				t.Errorf("expected %s %v, got %+v", name, want, got)
			}
		}
		checkText := func(name string, got sql.Null[string], from int, want string) {
			t.Helper()
			switch {
			case release < from && got.Valid:
				t.Errorf("expected no %s below release %d, got %+v", name, from, got)
			case release >= from && (!got.Valid || got.V != want):
				t.Errorf("expected %s %q, got %+v", name, want, got)
			}
		}
		check("binary", s.Binary, 14, true)
		streaming := "on"
		if release >= 16 {
			streaming = "parallel"
		}
		checkText("streaming", s.Streaming, 14, streaming)
		checkText("two phase", s.TwoPhase, 15, "disabled")
		check("disable on error", s.DisableOnError, 15, true)
		checkText("origin", s.Origin, 16, "none")
		check("password required", s.PasswordRequired, 16, true)
		check("run as owner", s.RunAsOwner, 16, true)
		check("failover", s.Failover, 17, false)
		checkText("skip lsn", s.SkipLSN, 15, "0/0")

		// the connection string is a kind of its own, and the fixture
		// runs as a superuser
		conns := readAll(t, dbmeta.SubscriptionConnections, m, db, dbmeta.Args{Name: "dbmeta_fixture_sub"}.Map())
		if len(conns) != 1 || !strings.Contains(conns[0].Conninfo, "host=example.invalid") {
			t.Errorf("expected the connection string of the subscription, got %+v", conns)
		}
	})
}

// TestPass3Options reads the options of a foreign data wrapper, a server, a
// user mapping and a foreign table in rows, with the text that psql prints.
func TestPass3Options(t *testing.T) {
	eachPostgres(t, func(t *testing.T, db *sql.DB, m *dbmeta.Meta) {
		needStep(t, m, "foreign data wrapper options")
		schema := fixture.Everything.Schema
		type row struct{ kind, name, option, value, quoted string }
		var got []row
		for _, v := range readAll(t, dbmeta.ForeignOptions, m, db, dbmeta.Args{Name: "dbmeta_fixture%"}.Map()) {
			got = append(got, row{v.Kind, v.Name.V, v.Option, v.Value, v.Quoted})
		}
		want := []row{
			{"foreign data wrapper", "dbmeta_fixture_fdw", "debug", "on", "debug 'on'"},
			{"foreign server", "dbmeta_fixture_server", "host", "example.invalid", "host 'example.invalid'"},
			{"foreign server", "dbmeta_fixture_server", "note", "a, b=c", "note 'a, b=c'"},
			{"user mapping", "dbmeta_fixture_role", "user", "remote", `"user" 'remote'`},
			{"user mapping", "dbmeta_fixture_role", "password", "secret", "password 'secret'"},
		}
		for _, w := range want {
			if !slices.Contains(got, w) {
				t.Errorf("expected the option %+v, got %+v", w, got)
			}
		}
		tableArgs := dbmeta.Args{Schema: schema, Name: "remote_ledger"}.Map()
		tableArgs["kind"] = "foreign table"
		var tableOptions []string
		for _, v := range readAll(t, dbmeta.ForeignOptions, m, db, tableArgs) {
			if !v.Schema.Valid || v.Schema.V != schema || !v.Server.Valid || v.Server.V != "dbmeta_fixture_server" {
				t.Errorf("expected the schema and the server of the table, got %+v", v)
			}
			tableOptions = append(tableOptions, v.Quoted)
		}
		if !slices.Equal(tableOptions, []string{`schema_name 'public'`, `table_name 'ledger'`, `"user" 'x, y'`}) {
			t.Errorf("expected the options of the foreign table in order, got %v", tableOptions)
		}
		// the text of the old field is unchanged, and cannot be split back
		tables := readAll(t, dbmeta.ForeignTables, m, db, dbmeta.Args{Schema: schema}.Map())
		if len(tables) != 1 || !tables[0].Options.Valid || !strings.Contains(tables[0].Options.V, "user=x, y") {
			t.Errorf("expected the catalog text in ForeignTable.Options, got %+v", tables)
		}
		// a server filter and a kind that holds no such object
		only := dbmeta.Args{Server: "dbmeta_fixture_server"}.Map()
		only["kind"] = "user mapping"
		if rows := readAll(t, dbmeta.ForeignOptions, m, db, only); len(rows) != 2 {
			t.Errorf("expected the two options of the user mapping, got %+v", rows)
		}
		none := dbmeta.Args{Schema: "no_such_schema"}.Map()
		if rows := readAll(t, dbmeta.ForeignOptions, m, db, none); len(rows) != 0 {
			t.Errorf("expected no option in a schema that does not exist, got %+v", rows)
		}
	})
}

// TestPass3ColumnPrivileges reads the entries of the column privileges in
// rows, and the text the old field kept.
func TestPass3ColumnPrivileges(t *testing.T) {
	eachPostgres(t, func(t *testing.T, db *sql.DB, m *dbmeta.Meta) {
		needStep(t, m, "column privilege for a role")
		schema := fixture.Everything.Schema
		got := readAll(t, dbmeta.ColumnPrivileges, m, db, dbmeta.Args{Schema: schema, Parent: "scratch"}.Map())
		if len(got) != 2 {
			t.Fatalf("expected two entries, got %+v", got)
		}
		payload, stamp := got[0], got[1]
		if payload.Column != "payload" || payload.Ordinal != 1 || !payload.Grantee.Valid ||
			payload.Grantee.V != "dbmeta_fixture_role" || payload.Privileges != "rw" ||
			!payload.Grantor.Valid || !strings.HasPrefix(payload.Access, "dbmeta_fixture_role=rw/") {
			t.Errorf("expected the entry of the role on payload, got %+v", payload)
		}
		if stamp.Column != "stamp" || stamp.Grantee.Valid || stamp.Privileges != "r" || !strings.HasPrefix(stamp.Access, "=r/") {
			t.Errorf("expected the entry of public on stamp, got %+v", stamp)
		}
		// the old field holds the same entries as one text
		privs := byName(readAll(t, dbmeta.Privileges, m, db, dbmeta.Args{Schema: schema, Name: "scratch"}.Map()),
			func(v dbmeta.Privilege) string { return v.Name })
		if c := privs["scratch"].ColumnAccess; !c.Valid || !strings.Contains(c.V, "payload:dbmeta_fixture_role=rw/") {
			t.Errorf("expected the old text to stay, got %+v", c)
		}
		one := dbmeta.Args{Schema: schema, Parent: "scratch", Name: "stamp"}.Map()
		if rows := readAll(t, dbmeta.ColumnPrivileges, m, db, one); len(rows) != 1 {
			t.Errorf("expected one entry for stamp, got %+v", rows)
		}
	})
}

// TestPass3Indexes reads the text of an index and the constraint that owns
// it, as psql prints them in the footer of \d name.
func TestPass3Indexes(t *testing.T) {
	eachPostgres(t, func(t *testing.T, db *sql.DB, m *dbmeta.Meta) {
		needStep(t, m, "tagged table")
		schema := fixture.Everything.Schema
		release := pgRelease(m)
		indexes := byName(readAll(t, dbmeta.Indexes, m, db, dbmeta.Args{Schema: schema}.Map()),
			func(v dbmeta.Index) string { return v.Name })

		for name, want := range map[string]string{
			"tagged_doc_path":    `gin (doc jsonb_path_ops)`,
			"tagged_doc":         `gin (doc)`,
			"tagged_label":       `btree (label COLLATE "C" text_pattern_ops)`,
			"book_published":     `btree (published DESC)`,
			"booking_no_overlap": `gist (int4range(booking_id, booking_id + 1))`,
		} {
			ix := indexes[name]
			if !ix.Using.Valid || ix.Using.V != want {
				t.Errorf("expected %s to use %q, got %+v", name, want, ix.Using)
			}
			full := "CREATE " + map[bool]string{true: "UNIQUE ", false: ""}[ix.Unique] + "INDEX " + name + " ON " + schema + "." + ix.Table + " USING " + want
			if !ix.Definition.Valid || ix.Definition.V != full {
				t.Errorf("expected the statement %q, got %+v", full, ix.Definition)
			}
		}

		for name, typ := range map[string]string{
			"author_pkey":            "p",
			"book_title_key":         "u",
			"tagged_label_key":       "u",
			"booking_no_overlap":     "x",
			"book_published":         "",
			"tagged_doc_path":        "",
			"scratch_key":            "",
			"scratch_payload_unique": "u",
		} {
			ix, ok := indexes[name]
			if !ok {
				t.Errorf("expected the index %s", name)
				continue
			}
			if typ == "" {
				if ix.ConstraintType.Valid || ix.ConstraintDefinition.Valid || ix.ConstraintPeriod.Valid {
					t.Errorf("expected no constraint on %s, got %+v", name, ix)
				}
				continue
			}
			if !ix.ConstraintType.Valid || ix.ConstraintType.V != typ || !ix.ConstraintDefinition.Valid {
				t.Errorf("expected the constraint type %s on %s, got %+v", typ, name, ix)
			}
			if !ix.ConstraintPeriod.Valid || ix.ConstraintPeriod.V {
				t.Errorf("expected no WITHOUT OVERLAPS on %s, got %+v", name, ix.ConstraintPeriod)
			}
		}
		if d := indexes["booking_no_overlap"].ConstraintDefinition; d.V != "EXCLUDE USING gist (int4range(booking_id, booking_id + 1) WITH &&)" {
			t.Errorf("expected the exclusion constraint text, got %q", d.V)
		}
		if d := indexes["book_title_key"].ConstraintDefinition; d.V != "UNIQUE (title)" {
			t.Errorf("expected the unique constraint text, got %q", d.V)
		}
		_ = release

		// the table is on the search path of one connection only
		if tv := indexes["book_published"].TableVisible; !tv.Valid || tv.V {
			t.Errorf("expected the table to be off the default path, got %+v", tv)
		}
		conn := onePath(t, db, schema+", pg_catalog")
		onPath := byName(readVia(t, dbmeta.Indexes, m, conn, dbmeta.Args{Schema: schema}.Map()),
			func(v dbmeta.Index) string { return v.Name })
		if tv := onPath["book_published"].TableVisible; !tv.Valid || !tv.V {
			t.Errorf("expected the table to be on the path, got %+v", tv)
		}
	})
}

// TestPass3Families checks the operator classes, families, operators and
// support functions against what psql 18 prints for them. See D201.
func TestPass3Families(t *testing.T) {
	eachPostgres(t, func(t *testing.T, db *sql.DB, m *dbmeta.Meta) {
		needStep(t, m, "tagged table")

		t.Run("operator classes", func(t *testing.T) {
			// two classes take jsonb in gin, and both are rows
			args := dbmeta.Args{AccessMethod: "gin"}.Map()
			args["type"] = "jsonb"
			got := readAll(t, dbmeta.OperatorClasses, m, db, args)
			if len(got) != 2 || got[0].Name != "jsonb_ops" || got[1].Name != "jsonb_path_ops" {
				t.Fatalf("expected jsonb_ops and jsonb_path_ops, got %+v", got)
			}
			if !got[0].Default || got[1].Default {
				t.Errorf("expected jsonb_ops to be the default, got %+v", got)
			}
			for _, c := range got {
				if !c.Visible.Valid || !c.Visible.V || !c.FamilyVisible.Valid || !c.FamilyVisible.V ||
					c.FamilySchema != "pg_catalog" || c.Schema != "pg_catalog" {
					t.Errorf("expected a visible class and family in pg_catalog, got %+v", c)
				}
			}
			// every class of the catalog is a row, as psql lists them
			var n int
			if err := db.QueryRowContext(t.Context(), `SELECT count(*) FROM pg_catalog.pg_opclass`).Scan(&n); err != nil {
				t.Fatalf("counting the classes: %v", err)
			}
			if all := readAll(t, dbmeta.OperatorClasses, m, db, nil); len(all) != n {
				t.Errorf("expected %d operator classes, got %d", n, len(all))
			}
		})

		t.Run("operator families", func(t *testing.T) {
			args := dbmeta.Args{AccessMethod: "btree", Name: "integer_ops"}.Map()
			got := readAll(t, dbmeta.OperatorFamilies, m, db, args)
			if len(got) != 1 {
				t.Fatalf("expected one family, got %+v", got)
			}
			// the list of types is in the order psql's own statement gives
			var want string
			err := db.QueryRowContext(t.Context(), `SELECT pg_catalog.string_agg(pg_catalog.format_type(oc.opcintype, NULL), ', ')
FROM pg_catalog.pg_opclass oc
WHERE oc.opcfamily = (SELECT f.oid FROM pg_catalog.pg_opfamily f JOIN pg_catalog.pg_am am ON am.oid = f.opfmethod
	WHERE f.opfname = 'integer_ops' AND am.amname = 'btree')`).Scan(&want)
			if err != nil {
				t.Fatalf("reading the types of integer_ops: %v", err)
			}
			if !got[0].AppliesTo.Valid || got[0].AppliesTo.V != want {
				t.Errorf("expected the types in the order %q, got %+v", want, got[0].AppliesTo)
			}
			if !got[0].Visible.Valid || !got[0].Visible.V {
				t.Errorf("expected a visible family, got %+v", got[0].Visible)
			}
			byType := dbmeta.Args{AccessMethod: "gin"}.Map()
			byType["type"] = "jsonb"
			fams := readAll(t, dbmeta.OperatorFamilies, m, db, byType)
			if len(fams) != 2 {
				t.Errorf("expected the two gin families that take jsonb, got %+v", fams)
			}
		})

		t.Run("operators", func(t *testing.T) {
			rows := readAll(t, dbmeta.OperatorFamilyOperators, m, db, dbmeta.Args{AccessMethod: "btree", Name: "integer_ops"}.Map())
			if len(rows) == 0 {
				t.Fatal("expected the operators of integer_ops")
			}
			// the rows whose two types are one type come first, as in psql
			sameDone := false
			for _, r := range rows {
				same := r.LeftType == r.RightType
				if sameDone && same {
					t.Errorf("expected the same type rows first, got %+v", rows)
					break
				}
				if !same {
					sameDone = true
				}
			}
			first := rows[0]
			if first.LeftType != first.RightType || first.Strategy != 1 || first.Purpose != "search" ||
				first.SortFamily.Valid || !first.Leakproof || !first.Visible.Valid || !first.Visible.V ||
				first.FamilySchema != "pg_catalog" {
				t.Errorf("expected the first operator of integer_ops to be < of one type, got %+v", first)
			}
			// an ordering operator names the family it sorts by
			var ordering int
			for _, r := range readAll(t, dbmeta.OperatorFamilyOperators, m, db, dbmeta.Args{AccessMethod: "gist", Name: "point_ops"}.Map()) {
				if r.Purpose == "ordering" {
					ordering++
					if !r.SortFamily.Valid || r.SortFamily.V != "float_ops" {
						t.Errorf("expected an ordering operator to sort by float_ops, got %+v", r)
					}
				} else if r.SortFamily.Valid {
					t.Errorf("expected a search operator to have no sort family, got %+v", r)
				}
			}
			if ordering == 0 {
				t.Errorf("expected an ordering operator in point_ops")
			}
		})

		t.Run("support functions", func(t *testing.T) {
			rows := readAll(t, dbmeta.OperatorFamilyFunctions, m, db, dbmeta.Args{AccessMethod: "btree", Name: "integer_ops"}.Map())
			if len(rows) == 0 {
				t.Fatal("expected the support functions of integer_ops")
			}
			first := rows[0]
			if first.LeftType != first.RightType || first.Number != 1 || first.FunctionName == "" ||
				!strings.HasPrefix(first.Function, first.FunctionName+"(") || !first.Visible.V || first.FamilySchema != "pg_catalog" {
				t.Errorf("expected the name and the signature of the first function, got %+v", first)
			}
		})
	})
}

// TestPass3Parsers reads the five functions of a text search parser.
func TestPass3Parsers(t *testing.T) {
	eachPostgres(t, func(t *testing.T, db *sql.DB, m *dbmeta.Meta) {
		needStep(t, m, "tagged table")
		args := dbmeta.Args{Schema: "pg_catalog", Name: "default", WithSystem: true}.Map()
		got := readAll(t, dbmeta.TextSearchParserFunctions, m, db, args)
		var methods, functions []string
		for _, v := range got {
			methods = append(methods, v.Method)
			functions = append(functions, v.Function)
			if v.Schema != "pg_catalog" || v.Parser != "default" || !v.Comment.Valid || v.Comment.V != "(internal)" {
				t.Errorf("expected an internal function of pg_catalog.default, got %+v", v)
			}
		}
		if !slices.Equal(methods, []string{"start", "token", "end", "headline", "lextype"}) {
			t.Errorf("expected the five methods in order, got %v", methods)
		}
		if !slices.Equal(functions, []string{"prsd_start", "prsd_nexttoken", "prsd_end", "prsd_headline", "prsd_lextype"}) {
			t.Errorf("expected the functions of the default parser, got %v", functions)
		}
	})
}

// TestPass3Visible reads the flags that say psql prints a name with no
// schema, with the fixture schema on the search path of one connection.
func TestPass3Visible(t *testing.T) {
	eachPostgres(t, func(t *testing.T, db *sql.DB, m *dbmeta.Meta) {
		needStep(t, m, "ledger child")
		schema := fixture.Everything.Schema
		release := pgRelease(m)
		check := func(t *testing.T, name string, got sql.Null[bool], want bool) {
			t.Helper()
			if !got.Valid || got.V != want {
				t.Errorf("expected %s to be %v, got %+v", name, want, got)
			}
		}
		conn := onePath(t, db, schema+", pg_catalog")
		for _, c := range []struct {
			name string
			db   dbmeta.Queryer
			want bool
		}{{"off the path", db, false}, {"on the path", conn, true}} {
			t.Run(c.name, func(t *testing.T) {
				inh := readVia(t, dbmeta.Inherits, m, c.db, dbmeta.Args{Schema: schema, Name: "ledger_child"}.Map())
				if len(inh) != 1 {
					t.Fatalf("expected one parent, got %+v", inh)
				}
				check(t, "Inherit.Visible", inh[0].Visible, c.want)
				check(t, "Inherit.ParentVisible", inh[0].ParentVisible, c.want)
				fam := readVia(t, dbmeta.OperatorFamilies, m, c.db, dbmeta.Args{AccessMethod: "btree", Name: "integer_ops"}.Map())
				check(t, "OperatorFamily.Visible", fam[0].Visible, true)
				if release < 10 {
					return
				}
				stats := readVia(t, dbmeta.ExtendedStats, m, c.db, dbmeta.Args{Schema: schema, Parent: "book"}.Map())
				if len(stats) == 0 {
					t.Fatalf("expected statistics on book")
				}
				for _, s := range stats {
					check(t, "ExtendedStat.TableVisible", s.TableVisible, c.want)
				}
				parts := readVia(t, dbmeta.Partitions, m, c.db, dbmeta.Args{Schema: schema, Parent: "sales"}.Map())
				if len(parts) == 0 {
					t.Fatalf("expected the partitions of sales")
				}
				check(t, "Partition.TableVisible", parts[0].TableVisible, c.want)
				check(t, "Partition.PartitionVisible", parts[0].PartitionVisible, c.want)
				pt := byName(readVia(t, dbmeta.PartitionedTables, m, c.db, dbmeta.Args{Schema: schema}.Map()),
					func(v dbmeta.PartitionedTable) string { return v.Name })
				check(t, "PartitionedTable.ParentVisible", pt["region_sales_east"].ParentVisible, c.want)
				if release >= 11 {
					check(t, "PartitionedTable.TableVisible", pt["sales_amount"].TableVisible, c.want)
				}
			})
		}
	})
}

// TestPass3ReusedKinds checks the two items that D201 answers with a kind that
// already existed, so that the structured form and the text agree.
func TestPass3ReusedKinds(t *testing.T) {
	eachPostgres(t, func(t *testing.T, db *sql.DB, m *dbmeta.Meta) {
		needStep(t, m, "document read policy")
		schema := fixture.Everything.Schema

		// Type.Elements joins the labels, and EnumValues has one row for each
		var labels []string
		for _, v := range readAll(t, dbmeta.EnumValues, m, db, dbmeta.Args{Schema: schema, Name: "colour"}.Map()) {
			if v.Ordinal != int64(len(labels)+1) {
				t.Errorf("expected the labels in order, got %+v", v)
			}
			labels = append(labels, v.Label)
		}
		types := byName(readAll(t, dbmeta.Types, m, db, dbmeta.Args{Schema: schema}.Map()),
			func(v dbmeta.Type) string { return v.Name })
		if want := strings.Join(labels, ", "); types[schema+".colour"].Elements != want || len(labels) != 3 {
			t.Errorf("expected the elements %q, got %+v", want, types[schema+".colour"])
		}

		// Privilege.Policies names the policies, and Policies has them whole
		var names []string
		for _, v := range readAll(t, dbmeta.Policies, m, db, dbmeta.Args{Schema: schema, Parent: "document"}.Map()) {
			names = append(names, v.Name)
		}
		privs := byName(readAll(t, dbmeta.Privileges, m, db, dbmeta.Args{Schema: schema, Name: "document"}.Map()),
			func(v dbmeta.Privilege) string { return v.Name })
		if got := privs["document"].Policies; !got.Valid || got.V != strings.Join(names, ", ") || len(names) < 2 {
			t.Errorf("expected the policies %v in the text, got %+v", names, got)
		}
	})
}
