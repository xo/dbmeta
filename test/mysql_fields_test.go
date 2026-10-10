package test

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/models/mysql"
	myfixture "github.com/xo/dbmeta/models/mysql/fixture"
)

// tableFacts says what a product of the MySQL family answers for the new
// fields of a table. See D205.
type tableFacts struct {
	engine  string // the engine of the fixture tables, empty when the product names none
	options bool   // CREATE_OPTIONS holds ROW_FORMAT
	size    bool   // DATA_LENGTH, INDEX_LENGTH and TABLE_ROWS hold a number
	// unknownPersistence is set for a product that cannot tell a temporary
	// table from a permanent one, so the field is absent.
	unknownPersistence bool
}

func checkTableFacts(t *testing.T, db *sql.DB, m *dbmeta.Meta, schema string, want tableFacts) {
	t.Helper()
	seen := map[string]dbmeta.Table{}
	for v, err := range dbmeta.Tables.All(t.Context(), m, db, dbmeta.Args{Schema: schema}.Map()) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		seen[v.Name] = v
	}
	for name, v := range seen {
		if want.unknownPersistence {
			if v.Persistence.Valid {
				t.Errorf("%s: persistence is %+v, want absent", name, v.Persistence)
			}
		} else if !v.Persistence.Valid || v.Persistence.V != "permanent" {
			t.Errorf("%s: persistence is %+v, want permanent", name, v.Persistence)
		}
		if v.Owner.Valid || v.RowSecurity.Valid || v.RowSecurityForced.Valid {
			t.Errorf("%s: owner and row security have no source and must be absent", name)
		}
		if v.Type == "view" {
			if v.AccessMethod.Valid || v.Size.Valid || v.Rows.Valid || v.Options.Valid {
				t.Errorf("%s: a view has no engine, size, rows or options, got %+v", name, v)
			}
			continue
		}
		if want.engine != "" && name == "book" &&
			(!v.AccessMethod.Valid || !strings.EqualFold(v.AccessMethod.V, want.engine)) {
			t.Errorf("%s: access method is %+v, want %s", name, v.AccessMethod, want.engine)
		}
	}
	book, ok := seen["book"]
	if !ok {
		t.Fatal("expected to find book")
	}
	if want.size {
		if !book.Size.Valid || book.Size.V < 0 {
			t.Errorf("book: size is %+v, want bytes", book.Size)
		}
		if !book.Rows.Valid || book.Rows.V < 0 {
			t.Errorf("book: rows is %+v, want an estimate", book.Rows)
		}
	}
	if want.options {
		packed, ok := seen["packed"]
		if !ok {
			t.Fatal("expected to find packed")
		}
		if !packed.Options.Valid || !strings.Contains(strings.ToLower(packed.Options.V), "row_format=compressed") {
			t.Errorf("packed: options is %+v, want row_format=COMPRESSED", packed.Options)
		}
		if book.Options.Valid {
			t.Errorf("book: options is %+v, want none", book.Options)
		}
	}
}

func checkIndexUsing(t *testing.T, db *sql.DB, m *dbmeta.Meta, schema string, want map[string]string) {
	t.Helper()
	got := map[string]string{}
	for v, err := range dbmeta.Indexes.All(t.Context(), m, db, dbmeta.Args{Schema: schema}.Map()) {
		if err != nil {
			t.Fatalf("reading indexes: %v", err)
		}
		if !v.Using.Valid {
			t.Errorf("%s.%s: using is absent", v.Table, v.Name)
		}
		got[v.Table+"."+v.Name] = v.Using.V
		if v.Using.V != v.Type {
			t.Errorf("%s.%s: using is %q and type is %q", v.Table, v.Name, v.Using.V, v.Type)
		}
		if v.Predicate.Valid || v.Size.Valid || v.Valid.Valid || v.Owner.Valid || v.Clustered.Valid {
			t.Errorf("%s.%s: predicate, size, valid, owner and clustered have no source and must be absent",
				v.Table, v.Name)
		}
	}
	for k, w := range want {
		if g, ok := got[k]; !ok {
			t.Errorf("%s: not listed", k)
		} else if g != w {
			t.Errorf("%s: using is %q, want %q", k, g, w)
		}
	}
}

// checkEnforced reads every constraint of the schema. MariaDB and MySQL from
// 8.0.16 answer true, and MySQL below that answers NULL (D205, D212).
func checkEnforced(t *testing.T, db *sql.DB, m *dbmeta.Meta, schema string, absent bool) {
	t.Helper()
	var n int
	for v, err := range dbmeta.Constraints.All(t.Context(), m, db, dbmeta.Args{Schema: schema}.Map()) {
		if err != nil {
			t.Fatalf("reading constraints: %v", err)
		}
		n++
		if absent {
			if v.Enforced.Valid {
				t.Errorf("%s.%s: enforced is %+v, want absent", v.Table, v.Name, v.Enforced)
			}
			continue
		}
		if !v.Enforced.Valid || !v.Enforced.V {
			t.Errorf("%s.%s: enforced is %+v, want true", v.Table, v.Name, v.Enforced)
		}
	}
	if n == 0 {
		t.Error("expected constraints")
	}
}

func checkPartitions(t *testing.T, db *sql.DB, m *dbmeta.Meta, schema string, sizes, subpartitions bool) {
	t.Helper()
	ctx := t.Context()
	type key struct{ table, part string }
	got := map[key]dbmeta.Partition{}
	for v, err := range dbmeta.Partitions.All(ctx, m, db, dbmeta.Args{Schema: schema}.Map()) {
		if err != nil {
			t.Fatalf("reading partitions: %v", err)
		}
		got[key{v.Table, v.Partition}] = v
	}
	type partitionWant struct {
		table, part, bound string
		partitioned        bool
	}
	want := []partitionWant{
		{"sales", "p2026", "2027", false},
		{"sales", "pmax", "MAXVALUE", false},
		{"region_sales", "north", "1,2,3", false},
		{"region_sales", "south", "4", false},
	}
	if subpartitions {
		want = append(want,
			partitionWant{"ledger", "early", "100", true},
			partitionWant{"ledger", "late", "MAXVALUE", true})
	}
	for _, w := range want {
		v, ok := got[key{w.table, w.part}]
		if !ok {
			t.Errorf("%s.%s: not listed", w.table, w.part)
			continue
		}
		if v.Type != "partition" || !v.Bound.Valid || v.Bound.V != w.bound || v.Partitioned != w.partitioned {
			t.Errorf("%s.%s: got %+v", w.table, w.part, v)
		}
	}
	var subs int
	for k, v := range got {
		if k.table == "ledger" && v.Type == "subpartition" {
			subs++
			if v.Bound.Valid || v.Partitioned {
				t.Errorf("ledger.%s: a subpartition has no bound, got %+v", k.part, v)
			}
		}
	}
	if subpartitions && subs != 4 || !subpartitions && subs != 0 {
		t.Errorf("ledger has %d subpartitions, want 4", subs)
	}
	for v, err := range dbmeta.Partitions.All(ctx, m, db, map[string]any{"schema": schema, "parent": "sales"}) {
		if err != nil {
			t.Fatalf("reading the partitions of sales: %v", err)
		}
		if v.Table != "sales" {
			t.Errorf("the parent filter returned %s", v.Table)
		}
	}
	if !sizes {
		return
	}
	for v, err := range dbmeta.PartitionedTables.All(ctx, m, db, dbmeta.Args{Schema: schema}.Map()) {
		if err != nil {
			t.Fatalf("reading partitioned tables: %v", err)
		}
		if !v.TotalSize.Valid || v.TotalSize.V < 0 || v.DirectSize != v.TotalSize {
			t.Errorf("%s: sizes are %+v and %+v", v.Name, v.DirectSize, v.TotalSize)
		}
	}
}

// TestMySQLTableFacts reads the size, the rows, the engine, the persistence
// and the options of a table back as typed values.
func TestMySQLTableFacts(t *testing.T) {
	db := openMySQL(t)
	m := setupMySQL(t, db)
	checkTableFacts(t, db, m, myfixture.Everything.Schema,
		tableFacts{engine: "InnoDB", options: true, size: true})
}

// TestMySQLIndexUsing reads the access method of each index.
func TestMySQLIndexUsing(t *testing.T) {
	db := openMySQL(t)
	m := setupMySQL(t, db)
	checkIndexUsing(t, db, m, myfixture.Everything.Schema, map[string]string{
		"book.PRIMARY":         "BTREE",
		"book.book_published":  "BTREE",
		"lookup.lookup_id":     "HASH",
		"article.article_body": "FULLTEXT",
	})
}

// TestMySQLConstraintEnforced reads whether the server checks a constraint.
func TestMySQLConstraintEnforced(t *testing.T) {
	db := openMySQL(t)
	m := setupMySQL(t, db)
	checkEnforced(t, db, m, myfixture.Everything.Schema, !mysql.IsMariaDB(m.Version()) && m.Version().Get(mysql.MySQL).Compare(dbmeta.V(8, 0, 16)) < 0)
}

// TestMySQLCompressedColumn reads the compression of a column.
func TestMySQLCompressedColumn(t *testing.T) {
	db := openMySQL(t)
	m := setupMySQL(t, db)
	maria := mysql.IsMariaDB(m.Version()) && m.Version().Get(mysql.MariaDB).Compare(dbmeta.V(10, 3)) >= 0
	var seen bool
	for v, err := range dbmeta.Columns.All(t.Context(), m, db, dbmeta.Args{Schema: myfixture.Everything.Schema}.Map()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		compressed := v.Table == "archive" && v.Name == "body"
		seen = seen || compressed
		switch {
		case compressed && maria && v.Compression.V != "zlib":
			t.Errorf("archive.body: compression is %+v, want zlib", v.Compression)
		case (!compressed || !maria) && v.Compression.Valid:
			t.Errorf("%s.%s: compression is %+v, want absent", v.Table, v.Name, v.Compression)
		}
		if v.Storage.Valid || v.StatsTarget.Valid {
			t.Errorf("%s.%s: storage and stats target have no source", v.Table, v.Name)
		}
	}
	if maria && !seen {
		t.Error("expected the compressed column")
	}
}

// TestMySQLPartitions reads the partitions and their bounds, and the sizes of
// a partitioned table.
func TestMySQLPartitions(t *testing.T) {
	db := openMySQL(t)
	m := setupMySQL(t, db)
	checkPartitions(t, db, m, myfixture.Everything.Schema, true, true)
}

// TestMySQLFunctionProsrc reads the body of a routine.
func TestMySQLFunctionProsrc(t *testing.T) {
	db := openMySQL(t)
	m := setupMySQL(t, db)
	var n int
	for v, err := range dbmeta.Functions.All(t.Context(), m, db, dbmeta.Args{Schema: myfixture.Everything.Schema}.Map()) {
		if err != nil {
			t.Fatalf("reading functions: %v", err)
		}
		n++
		if v.Prosrc != v.Source || !v.Prosrc.Valid {
			t.Errorf("%s: prosrc is %+v and source is %+v", v.Name, v.Prosrc, v.Source)
		}
		if v.Leakproof {
			t.Errorf("%s: leakproof must be false", v.Name)
		}
	}
	if n == 0 {
		t.Error("expected functions")
	}
}
