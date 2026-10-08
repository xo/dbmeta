package test

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/models/postgres/fixture"
)

// pgRelease is the PostgreSQL release a meta claims, and 0 when it claims
// none. CockroachDB claims one, so the release says what catalog it imitates.
func pgRelease(m *dbmeta.Meta) int {
	if parts := m.Version().Main().Parts; len(parts) > 0 {
		return int(parts[0])
	}
	return 0
}

// TestDescribeFieldsReadBack reads the fields that the describe commands of
// psql print about a relation, an index, a column and a function, as typed
// values. Each one is NULL on a release too old to have it. See D198.
func TestDescribeFieldsReadBack(t *testing.T) {
	eachPostgres(t, func(t *testing.T, db *sql.DB, m *dbmeta.Meta) {
		ctx := t.Context()
		schema := fixture.Everything.Schema

		t.Run("tables", func(t *testing.T) {
			needStep(t, m, "scratch table")
			tables := map[string]dbmeta.Table{}
			for v, err := range dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Schema: schema}.Map()) {
				if err != nil {
					t.Fatalf("reading tables: %v", err)
				}
				tables[v.Name] = v
			}
			scratch := tables["scratch"]
			if !scratch.Owner.Valid || scratch.Owner.V == "" {
				t.Errorf("expected an owner, got %+v", scratch.Owner)
			}
			if got := scratch.Persistence; !got.Valid || got.V != "unlogged" {
				t.Errorf("expected scratch to be unlogged, got %+v", got)
			}
			if got := tables["author"].Persistence; !got.Valid || got.V != "permanent" {
				t.Errorf("expected author to be permanent, got %+v", got)
			}
			if got := tables["author"].Size; !got.Valid || got.V <= 0 {
				t.Errorf("expected author to take bytes, got %+v", got)
			}
			if got := tables["recent"].Type; got != "view" {
				t.Errorf("expected recent to be a view, got %q", got)
			}
			if m.Dialect() != dbmeta.PostgreSQL {
				return
			}
			// ANALYZE ran on author with 200 rows, and nothing else ever
			// analyzed region. CLUSTER stores a count, so scratch is no example.
			if got := tables["author"].Rows; !got.Valid || got.V != 200 {
				t.Errorf("expected 200 rows estimated for author, got %+v", got)
			}
			neverAnalyzed := int64(-1)
			if pgRelease(m) < 14 {
				neverAnalyzed = 0
			}
			if got := tables["region"].Rows; !got.Valid || got.V != neverAnalyzed {
				t.Errorf("expected %d for a table never analyzed, got %+v", neverAnalyzed, got)
			}
			if pgRelease(m) >= 12 {
				if got := tables["author"].AccessMethod; !got.Valid || got.V != "heap" {
					t.Errorf("expected the heap access method, got %+v", got)
				}
				if got := tables["recent"].AccessMethod; got.Valid {
					t.Errorf("expected a view to have no access method, got %+v", got)
				}
			} else if got := tables["author"].AccessMethod; got.Valid {
				t.Errorf("expected no access method before release 12, got %+v", got)
			}
			if pgRelease(m) >= 10 {
				if got := tables["sales"].Type; got != "partitioned table" {
					t.Errorf("expected sales to be a partitioned table, got %q", got)
				}
			}
		})

		t.Run("indexes", func(t *testing.T) {
			needStep(t, m, "scratch table")
			indexes := map[string]dbmeta.Index{}
			for v, err := range dbmeta.Indexes.All(ctx, m, db, dbmeta.Args{Schema: schema}.Map()) {
				if err != nil {
					t.Fatalf("reading indexes: %v", err)
				}
				indexes[v.Name] = v
			}
			key, stamp, owned := indexes["scratch_key"], indexes["scratch_stamp"], indexes["scratch_payload_unique"]
			if !key.Owner.Valid || key.Owner.V == "" {
				t.Errorf("expected an owner, got %+v", key.Owner)
			}
			if got := key.Persistence; !got.Valid || got.V != "unlogged" {
				t.Errorf("expected an unlogged index, got %+v", got)
			}
			if got := key.Size; !got.Valid || got.V <= 0 {
				t.Errorf("expected an index to take bytes, got %+v", got)
			}
			for name, got := range map[string]sql.Null[bool]{
				"scratch_key valid":       key.Valid,
				"scratch_stamp valid":     stamp.Valid,
				"scratch_key identity":    key.ReplicaIdentity,
				"scratch_stamp clustered": stamp.Clustered,
			} {
				if !got.Valid || !got.V {
					t.Errorf("expected %s to be true, got %+v", name, got)
				}
			}
			if got := key.Clustered; !got.Valid || got.V {
				t.Errorf("expected scratch_key not to be clustered, got %+v", got)
			}
			if got := stamp.ReplicaIdentity; !got.Valid || got.V {
				t.Errorf("expected scratch_stamp not to be the replica identity, got %+v", got)
			}
			if key.Predicate.Valid {
				t.Errorf("expected no predicate on a full index, got %q", key.Predicate.V)
			}
			// only an index that a constraint owns has these two
			if key.Deferrable.Valid || key.InitiallyDeferred.Valid {
				t.Errorf("expected no constraint on scratch_key, got %+v %+v", key.Deferrable, key.InitiallyDeferred)
			}
			if !owned.Deferrable.Valid || !owned.Deferrable.V || !owned.InitiallyDeferred.Valid || !owned.InitiallyDeferred.V {
				t.Errorf("expected a deferrable constraint index, got %+v %+v", owned.Deferrable, owned.InitiallyDeferred)
			}
			if pk := indexes["author_pkey"]; !pk.Deferrable.Valid || pk.Deferrable.V {
				t.Errorf("expected a primary key that is not deferrable, got %+v", pk.Deferrable)
			}
			needStep(t, m, "book partial index")
			if got := indexes["book_recent"].Predicate; !got.Valid || !strings.Contains(got.V, "published >") {
				t.Errorf("expected the predicate of book_recent, got %+v", got)
			}
		})

		t.Run("columns", func(t *testing.T) {
			needStep(t, m, "scratch table")
			cols := map[string]dbmeta.Column{}
			a := dbmeta.Args{Schema: schema, Parent: "scratch"}.Map()
			for v, err := range dbmeta.Columns.All(ctx, m, db, a) {
				if err != nil {
					t.Fatalf("reading columns: %v", err)
				}
				cols[v.Name] = v
			}
			payload, id := cols["payload"], cols["scratch_id"]
			if got := payload.Storage; !got.Valid || got.V != "external" {
				t.Errorf("expected external storage, got %+v", got)
			}
			if got := id.Storage; !got.Valid || got.V != "plain" {
				t.Errorf("expected plain storage, got %+v", got)
			}
			if got := payload.StatsTarget; !got.Valid || got.V != 500 {
				t.Errorf("expected a statistics target of 500, got %+v", got)
			}
			if got := id.StatsTarget; got.Valid {
				t.Errorf("expected the default statistics target to be absent, got %+v", got)
			}
			if m.Dialect() != dbmeta.PostgreSQL {
				return
			}
			if pgRelease(m) >= 14 {
				if got := payload.Compression; !got.Valid || got.V != "pglz" {
					t.Errorf("expected pglz compression, got %+v", got)
				}
			} else if payload.Compression.Valid {
				t.Errorf("expected no compression before release 14, got %+v", payload.Compression)
			}
			if id.Compression.Valid {
				t.Errorf("expected the default compression to be absent, got %+v", id.Compression)
			}
		})

		t.Run("functions", func(t *testing.T) {
			needStep(t, m, "leakproof function")
			fns := map[string]dbmeta.Function{}
			for v, err := range dbmeta.Functions.All(ctx, m, db, dbmeta.Args{Schema: schema}.Map()) {
				if err != nil {
					t.Fatalf("reading functions: %v", err)
				}
				fns[v.Name] = v
			}
			if !fns["same"].Leakproof {
				t.Error("expected same to be leakproof")
			}
			if fns["touch"].Leakproof {
				t.Error("expected touch not to be leakproof")
			}
			if got := fns["same"].Prosrc; !got.Valid || got.V != "SELECT $1" {
				t.Errorf("expected the body of a SQL function, got %+v", got)
			}
			// Source holds prosrc for internal and C functions only
			if got := fns["touch"].Prosrc; !got.Valid || !strings.Contains(got.V, "RETURN NEW") {
				t.Errorf("expected the body of a PL/pgSQL function, got %+v", got)
			}
			if got := fns["touch"].Source; got.Valid {
				t.Errorf("expected Source to stay empty for PL/pgSQL, got %+v", got)
			}
		})
	})
}
