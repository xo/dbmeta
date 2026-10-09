package test

import (
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
)

// openOracleFixtureUser opens a second connection as the owner of the fixture
// schema. USER_SEGMENTS is the only view of segments that every user can read,
// so only this user sees the size of the tables and the indexes it owns.
func openOracleFixtureUser(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("oracle", replaceUser(t, dsnOf(t, "DBMETA_ORACLE"), "dbmeta_fixture", "P4ssw0rd"))
	if err != nil {
		t.Fatalf("opening as the fixture user: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting as the fixture user: %v", err)
	}
	return db
}

// TestOracleDescribeFields reads the fields that D206 added back as typed
// values, on every release of the tier.
func TestOracleDescribeFields(t *testing.T) {
	db := openOracle(t)
	m := setupOracle(t, db)
	owner := openOracleFixtureUser(t)
	ctx := t.Context()
	v12 := m.Version().Main().AtLeast(dbmeta.V(12))

	read := func(db dbmeta.Queryer) map[string]dbmeta.Table {
		tables := make(map[string]dbmeta.Table)
		for v, err := range dbmeta.Tables.All(ctx, m, db, oraArgs()) {
			if err != nil {
				t.Fatalf("reading tables: %v", err)
			}
			tables[v.Name] = v
		}
		return tables
	}
	tables := read(db)
	own := read(owner)

	author := tables["AUTHOR"]
	if author.Owner.V != "DBMETA_FIXTURE" || author.Persistence.V != "permanent" || author.AccessMethod.V != "heap" {
		t.Errorf("expected a permanent heap table owned by the schema, got %+v", author)
	}
	if !author.Rows.Valid || author.Rows.V != 200 {
		t.Errorf("expected the 200 rows that were analyzed, got %+v", author.Rows)
	}
	if got := tables["BOOK"].Rows; got.Valid {
		t.Errorf("expected no row count for a table that was never analyzed, got %+v", got)
	}
	if author.Options.Valid {
		t.Errorf("expected no options, got %+v", author.Options)
	}
	if author.Size.Valid {
		t.Errorf("expected no size for a table of another schema, got %+v", author.Size)
	}
	if got := own["AUTHOR"].Size; !got.Valid || got.V <= 0 {
		t.Errorf("expected a size for a table of the connected user, got %+v", got)
	}
	// a table with no row has no segment yet when the segment is deferred,
	// which is a size of zero, and 11.2.0.2 Express makes the segment at once
	if got := own["BOOK"].Size; !got.Valid || got.V < 0 {
		t.Errorf("expected a size for an empty table of the connected user, got %+v", got)
	}
	if got := own["RECENT"]; got.Size.Valid || got.AccessMethod.Valid || got.Rows.Valid || got.Type != "view" {
		t.Errorf("expected a view to have no size, access method or rows, got %+v", got)
	}
	if got := tables["SCRATCH"]; got.Persistence.V != "temporary" || got.Options.V != "on_commit=delete_rows" {
		t.Errorf("expected a temporary table that deletes rows on commit, got %+v", got)
	}
	if got := tables["LOOKUP"].AccessMethod.V; got != "index organized" {
		t.Errorf("expected an index organized table, got %q", got)
	}
	if got := tables["LOGBOOK"].Options.V; got != "nologging, pctfree=20" {
		t.Errorf("expected the storage options of logbook, got %q", got)
	}
	if got := tables["AUTHOR"].RowSecurity; !got.Valid || got.V {
		t.Errorf("expected row security to be a real false, got %+v", got)
	}

	policies := make(map[string]dbmeta.Policy)
	for v, err := range dbmeta.Policies.All(ctx, m, db, oraArgs()) {
		if err != nil {
			t.Fatalf("reading policies: %v", err)
		}
		policies[v.Command] = v
	}
	if len(policies) == 0 {
		t.Logf("this release has no virtual private database, so SECRET has no policy")
	} else {
		if len(policies) != 3 {
			t.Errorf("expected a row for select, insert and update, got %v", policies)
		}
		for cmd, p := range policies {
			if p.Table != "SECRET" || p.Name != "SECRET_POLICY" || p.Permissive || p.Roles.Valid {
				t.Errorf("unexpected policy %+v", p)
			}
			if !strings.HasSuffix(p.Using.V+p.WithCheck.V, "DBMETA_FIXTURE.SECRET_CHECK") {
				t.Errorf("%s: expected the policy function, got %+v", cmd, p)
			}
		}
		if p := policies["insert"]; p.Using.Valid || !p.WithCheck.Valid {
			t.Errorf("expected insert to check and not to filter, got %+v", p)
		}
		if p := policies["select"]; !p.Using.Valid || p.WithCheck.Valid {
			t.Errorf("expected select to filter and not to check, got %+v", p)
		}
		if p := policies["update"]; !p.Using.Valid || !p.WithCheck.Valid {
			t.Errorf("expected update to filter and to check, got %+v", p)
		}
		if got := tables["SECRET"]; !got.RowSecurity.V || !got.RowSecurityForced.V {
			t.Errorf("expected row security on and forced, got %+v", got)
		}
	}

	indexes := make(map[string]dbmeta.Index)
	byTable := make(map[string]dbmeta.Index)
	for v, err := range dbmeta.Indexes.All(ctx, m, owner, oraArgs()) {
		if err != nil {
			t.Fatalf("reading indexes: %v", err)
		}
		indexes[v.Name] = v
		if v.ConstraintType.Valid {
			byTable[v.Table+" "+v.ConstraintType.V] = v
		}
	}
	pk := byTable["AUTHOR p"]
	if !pk.Primary || !pk.Valid.V || !pk.Valid.Valid || pk.Clustered.V || pk.Owner.V != "DBMETA_FIXTURE" {
		t.Errorf("expected a valid primary key index of a heap, got %+v", pk)
	}
	if !pk.Deferrable.Valid || pk.Deferrable.V || !pk.InitiallyDeferred.Valid || pk.InitiallyDeferred.V {
		t.Errorf("expected a key that is not deferrable, got %+v", pk)
	}
	if !pk.Size.Valid || pk.Size.V <= 0 {
		t.Errorf("expected a size, got %+v", pk.Size)
	}
	if got := byTable["BOOK u"]; got.Primary || !got.Unique {
		t.Errorf("expected the unique constraint index of book, got %+v", got)
	}
	if got := byTable["LOOKUP p"]; !got.Clustered.V || got.Type != "iot - top" {
		t.Errorf("expected the key index of an index organized table to hold it, got %+v", got)
	}
	plain := indexes["BOOK_PUBLISHED_IX"]
	if plain.ConstraintType.Valid || plain.Deferrable.Valid || plain.Options.Valid || plain.Predicate.Valid {
		t.Errorf("expected an index with none of the optional fields, got %+v", plain)
	}
	if !plain.Valid.V || plain.Clustered.V || plain.Persistence.V != "permanent" {
		t.Errorf("expected a valid permanent index that is not clustered, got %+v", plain)
	}
	if got := indexes["EXTRAS_LABEL_IX"]; got.Options.V != "invisible" || !got.Valid.V {
		t.Errorf("expected an invisible index that is valid, got %+v", got)
	}
	if got := indexes["EXTRAS_SHOUTED_IX"]; !got.Valid.Valid || got.Valid.V {
		t.Errorf("expected an unusable index to be not valid, got %+v", got.Valid)
	}

	enforced := make(map[string]bool)
	for v, err := range dbmeta.Constraints.All(ctx, m, db, oraArgs()) {
		if err != nil {
			t.Fatalf("reading constraints: %v", err)
		}
		if !v.Enforced.Valid {
			t.Errorf("expected %s to say whether it is enforced", v.Name)
		}
		enforced[v.Name] = v.Enforced.V
	}
	if !enforced["AUTHOR_PK"] || !enforced["BOOK_AUTHOR_FK"] || !enforced["AUTHOR_RATING_CK"] {
		t.Errorf("expected enforced constraints, got %v", enforced)
	}
	if enforced["EXTRAS_LABEL_CK"] {
		t.Errorf("expected a disabled constraint to be not enforced, got %v", enforced)
	}

	for v, err := range dbmeta.Sequences.All(ctx, m, db, oraArgs()) {
		if err != nil {
			t.Fatalf("reading sequences: %v", err)
		}
		if v.Name == "COUNTER" && v.CacheSize.V != 20 {
			t.Errorf("expected a cache of 20, got %+v", v.CacheSize)
		}
	}

	var bounds []string
	for v, err := range dbmeta.Partitions.All(ctx, m, db, oraArgs()) {
		if err != nil {
			t.Fatalf("reading partitions: %v", err)
		}
		if v.Table != "SALES" || v.PartitionSchema != v.Schema || v.Partitioned || v.DetachPending {
			t.Errorf("unexpected partition %+v", v)
		}
		bounds = append(bounds, v.Partition+" "+v.Bound.V)
	}
	if _, ok := tables["SALES"]; ok {
		want := "SALES_P1 100|SALES_P2 200|SALES_PMAX MAXVALUE"
		if strings.Join(bounds, "|") != want {
			t.Errorf("expected %q, got %v", want, bounds)
		}
	} else if len(bounds) != 0 {
		t.Errorf("expected no partitions without a SALES table, got %v", bounds)
	}

	var notNulls []dbmeta.NotNull
	var nerr error
	for v, err := range dbmeta.NotNulls.All(ctx, m, db, oraArgs()) {
		if err != nil {
			nerr = err
			break
		}
		notNulls = append(notNulls, v)
	}
	if !v12 {
		if !errors.Is(nerr, dbmeta.ErrVersionTooOld) {
			t.Errorf("expected NotNulls to need release 12, got %v", nerr)
		}
		return
	}
	if nerr != nil {
		t.Fatalf("reading not null constraints: %v", nerr)
	}
	var found bool
	for _, n := range notNulls {
		if n.Table == "AUTHOR" && n.Column == "NAME" {
			found = true
			if !n.Validated || n.NoInherit || !n.Local || n.Inherited || n.Name == "" {
				t.Errorf("unexpected not null %+v", n)
			}
		}
	}
	if !found {
		t.Errorf("expected a NOT NULL on author.name, got %+v", notNulls)
	}
}
