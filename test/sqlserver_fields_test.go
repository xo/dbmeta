package test

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
)

// TestSQLServerDescribeFields reads the fields that D206 added back as typed
// values, on every release of the tier.
func TestSQLServerDescribeFields(t *testing.T) {
	db := openSQLServer(t)
	m := setupSQLServer(t, db)
	ctx := t.Context()
	v13 := m.Version().Main().AtLeast(dbmeta.V(13))

	tables := make(map[string]dbmeta.Table)
	for v, err := range dbmeta.Tables.All(ctx, m, db, msArgs()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		tables[v.Name] = v
	}
	author := tables["author"]
	if !author.Owner.Valid || author.Owner.V != "dbo" {
		t.Errorf("expected the owner of the schema, dbo, got %+v", author.Owner)
	}
	if author.Persistence.V != "permanent" {
		t.Errorf("expected permanent, got %+v", author.Persistence)
	}
	// the primary key makes the clustered index
	if author.AccessMethod.V != "clustered" {
		t.Errorf("expected a clustered table, got %+v", author.AccessMethod)
	}
	if !author.Size.Valid || author.Size.V <= 0 || author.Size.V%8192 != 0 {
		t.Errorf("expected a size that is a number of pages, got %+v", author.Size)
	}
	if !author.Rows.Valid || author.Rows.V != 200 {
		t.Errorf("expected 200 rows, got %+v", author.Rows)
	}
	if author.Options.Valid {
		t.Errorf("expected no options, got %+v", author.Options)
	}
	if v13 && (!author.RowSecurity.Valid || author.RowSecurity.V) {
		t.Errorf("expected row security to be a real false, got %+v", author.RowSecurity)
	}
	// a table with no primary key and no index is a heap
	if got := tables["sales"].AccessMethod.V; got != "heap" {
		t.Errorf("expected sales to be a heap, got %q", got)
	}
	if got := tables["sales"].Options; !got.Valid || got.V != "data_compression=page" {
		t.Errorf("expected the compression of sales, got %+v", got)
	}
	if got := tables["sales"].Rows; got.V != 4 {
		t.Errorf("expected 4 rows in sales, got %+v", got)
	}
	// a view has no size, no rows and no storage
	recent := tables["recent"]
	if recent.Size.Valid || recent.Rows.Valid || recent.AccessMethod.Valid {
		t.Errorf("expected a view to have no size, rows or access method, got %+v", recent)
	}
	if recent.Persistence.V != "permanent" || recent.Owner.V != "dbo" {
		t.Errorf("expected a permanent view owned by dbo, got %+v", recent)
	}
	if v13 {
		secret := tables["secret"]
		if !secret.RowSecurity.V || !secret.RowSecurityForced.V {
			t.Errorf("expected row security on and forced, got %+v", secret)
		}
		if !secret.RowSecurity.Valid || !secret.RowSecurityForced.Valid {
			t.Errorf("expected row security to be a value, got %+v", secret)
		}
	}

	indexes := make(map[string]dbmeta.Index)
	for v, err := range dbmeta.Indexes.All(ctx, m, db, msArgs()) {
		if err != nil {
			t.Fatalf("reading indexes: %v", err)
		}
		indexes[v.Name] = v
	}
	pk := indexes["author_pk"]
	if pk.ConstraintType.V != "p" || !pk.Clustered.V || !pk.Valid.V {
		t.Errorf("expected a valid clustered primary key, got %+v", pk)
	}
	if !pk.Deferrable.Valid || pk.Deferrable.V || !pk.InitiallyDeferred.Valid {
		t.Errorf("expected a key that is not deferrable, got %+v", pk)
	}
	if indexes["book_title_unique"].ConstraintType.V != "u" {
		t.Errorf("expected a unique constraint, got %+v", indexes["book_title_unique"])
	}
	plain := indexes["book_published"]
	filtered := indexes["author_rating"]
	if plain.ConstraintType.Valid || plain.Deferrable.Valid || plain.Predicate.Valid || plain.Options.Valid {
		t.Errorf("expected an index with none of the optional fields, got %+v", plain)
	}
	if plain.Clustered.V || !plain.Valid.V || plain.Persistence.V != "permanent" {
		t.Errorf("expected a valid permanent index that is not clustered, got %+v", plain)
	}
	if !filtered.Size.Valid || filtered.Size.V <= 0 {
		t.Errorf("expected a size, got %+v", filtered.Size)
	}
	if !strings.Contains(filtered.Predicate.V, "rating") || !filtered.Predicate.Valid {
		t.Errorf("expected the filter of the index, got %+v", filtered.Predicate)
	}
	if filtered.Options.V != "fillfactor=70" {
		t.Errorf("expected the fill factor, got %+v", filtered.Options)
	}
	if off := indexes["author_name_off"]; !off.Valid.Valid || off.Valid.V {
		t.Errorf("expected a disabled index to be not valid, got %+v", off.Valid)
	}

	includes := make(map[string]bool)
	for v, err := range dbmeta.IndexColumns.All(ctx, m, db, msArgs()) {
		if err != nil {
			t.Fatalf("reading index columns: %v", err)
		}
		if v.Index == "author_rating" {
			includes[v.Name.V] = v.Include
		}
	}
	if includes["rating"] || !includes["name"] {
		t.Errorf("expected name to be an included column and rating a key column, got %v", includes)
	}

	enforced := make(map[string]bool)
	for v, err := range dbmeta.Constraints.All(ctx, m, db, msArgs()) {
		if err != nil {
			t.Fatalf("reading constraints: %v", err)
		}
		if !v.Enforced.Valid {
			t.Errorf("expected %s to say whether it is enforced", v.Name)
		}
		enforced[v.Name] = v.Enforced.V
	}
	if !enforced["author_pk"] || !enforced["book_author_fk"] || !enforced["title_not_empty"] {
		t.Errorf("expected enforced constraints, got %v", enforced)
	}
	if enforced["extras_title_check"] {
		t.Errorf("expected a constraint set to NOCHECK to be not enforced, got %v", enforced)
	}

	if m.Version().Main().AtLeast(dbmeta.V(11)) {
		for v, err := range dbmeta.Sequences.All(ctx, m, db, msArgs()) {
			if err != nil {
				t.Fatalf("reading sequences: %v", err)
			}
			if v.Name == "counter" && v.CacheSize.V != 20 {
				t.Errorf("expected a cache of 20, got %+v", v.CacheSize)
			}
		}
	}

	var sales []dbmeta.PartitionedTable
	for v, err := range dbmeta.PartitionedTables.All(ctx, m, db, msArgs()) {
		if err != nil {
			t.Fatalf("reading partitioned tables: %v", err)
		}
		sales = append(sales, v)
	}
	if len(sales) != 1 || sales[0].Owner != "dbo" || sales[0].AccessMethod.V != "heap" ||
		!sales[0].TotalSize.Valid || sales[0].TotalSize.V != tables["sales"].Size.V {
		t.Errorf("expected the partitioned table sales with its owner and size, got %+v", sales)
	}
}

// TestSQLServerPartitions reads the partitions of the table of three
// partitions and the interval of each.
func TestSQLServerPartitions(t *testing.T) {
	db := openSQLServer(t)
	m := setupSQLServer(t, db)
	var got []string
	for v, err := range dbmeta.Partitions.All(t.Context(), m, db, msArgs()) {
		if err != nil {
			t.Fatalf("reading partitions: %v", err)
		}
		if v.Table != "sales" || v.PartitionSchema != v.Schema || v.Type != "partition" ||
			v.Partitioned || v.DetachPending {
			t.Errorf("unexpected partition %+v", v)
		}
		got = append(got, v.Partition+" "+v.Bound.V)
	}
	want := []string{"1 [MINVALUE, 100)", "2 [100, 200)", "3 [200, MAXVALUE)"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("expected %v, got %v", want, got)
	}
}

// TestSQLServerPolicies reads the predicates of the security policy.
func TestSQLServerPolicies(t *testing.T) {
	db := openSQLServer(t)
	m := setupSQLServer(t, db)
	if !m.Version().Main().AtLeast(dbmeta.V(13)) {
		t.Skip("security policies arrived in SQL Server 2016")
	}
	type row struct{ command, using, check bool }
	got := make(map[string]row)
	enabled := make(map[string]sql.Null[bool])
	for v, err := range dbmeta.Policies.All(t.Context(), m, db, msArgs()) {
		if err != nil {
			t.Fatalf("reading policies: %v", err)
		}
		if v.Table != "secret" || v.Permissive || v.Roles.Valid {
			t.Errorf("unexpected policy %+v", v)
		}
		// the state is the policy's, so every predicate of it agrees (D211)
		if prev, ok := enabled[v.Name]; ok && prev != v.Enabled {
			t.Errorf("%s: the predicates disagree about the state, %+v and %+v", v.Name, prev, v.Enabled)
		}
		enabled[v.Name] = v.Enabled
		if v.Name != "secret_policy" {
			continue
		}
		got[v.Command] = row{true, v.Using.Valid, v.WithCheck.Valid}
		if !strings.Contains(v.Using.V+v.WithCheck.V, "secret_check") {
			t.Errorf("expected the predicate to call secret_check, got %+v", v)
		}
	}
	// the policy that is switched off is a row, with a real false
	wantEnabled := map[string]sql.Null[bool]{
		"secret_policy":     {V: true, Valid: true},
		"secret_policy_off": {V: false, Valid: true},
	}
	if len(enabled) != len(wantEnabled) {
		t.Fatalf("expected %v, got %v", wantEnabled, enabled)
	}
	for k, w := range wantEnabled {
		if enabled[k] != w {
			t.Errorf("%s: expected enabled %+v, got %+v", k, w, enabled[k])
		}
	}
	want := map[string]row{
		"select": {true, true, false},
		"insert": {true, false, true},
		"delete": {true, true, false},
	}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s: expected %v, got %v", k, w, got[k])
		}
	}
}
