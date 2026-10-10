package test

import (
	"strings"
	"testing"

	"github.com/xo/dbmeta"
)

// TestFirebirdDescribeFields reads the fields that D209 added back as typed
// values, on every release of the tier. Firebird keeps no size and no row
// count, so both stay absent.
func TestFirebirdDescribeFields(t *testing.T) {
	db := openFirebird(t)
	m := setupFirebird(t, db)
	ctx := t.Context()
	v4 := m.Version().Main().AtLeast(dbmeta.V(4, 0))
	v5 := m.Version().Main().AtLeast(dbmeta.V(5, 0))

	tables := map[string]dbmeta.Table{}
	for v, err := range dbmeta.Tables.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		tables[strings.ToUpper(v.Name)] = v
		if v.AccessMethod.Valid || v.Size.Valid || v.Rows.Valid {
			t.Errorf("%s: expected no access method, size or rows, got %+v", v.Name, v)
		}
	}
	if author := tables["AUTHOR"]; author.Owner.V != "SYSDBA" || author.Persistence.V != "permanent" || author.Options.Valid {
		t.Errorf("author: expected owner SYSDBA, permanent and no options, got %+v", author)
	}
	if scratch := tables["DBMETA_SCRATCH"]; scratch.Persistence.V != "temporary" || scratch.Options.V != "on_commit=preserve rows" {
		t.Errorf("scratch: expected temporary with on_commit, got %+v", scratch)
	}
	if recent := tables["RECENT"]; !recent.Owner.Valid || recent.Persistence.Valid || recent.Options.Valid {
		t.Errorf("recent: expected an owner and no persistence or options, got %+v", recent)
	}

	indexes := map[string]dbmeta.Index{}
	for v, err := range dbmeta.Indexes.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading indexes: %v", err)
		}
		indexes[strings.ToUpper(v.Name)] = v
		if !v.Valid.Valid || v.Owner.Valid || v.Size.Valid || v.Using.Valid || v.Definition.Valid ||
			v.Clustered.Valid || v.ReplicaIdentity.Valid || v.Options.Valid {
			t.Errorf("%s: unexpected fields %+v", v.Name, v)
		}
	}
	if pk := indexes["BOOK_PK"]; pk.ConstraintType.V != "p" || !pk.Deferrable.Valid || pk.Deferrable.V || pk.Predicate.Valid {
		t.Errorf("book_pk: expected a key index, got %+v", pk)
	}
	for _, v := range indexes {
		if v.ConstraintType.V == "f" && (!v.Deferrable.Valid || v.InitiallyDeferred.V) {
			t.Errorf("%s: expected the flags of its constraint, got %+v", v.Name, v)
		}
	}
	if free := indexes["BOOK_PUBLISHED"]; free.ConstraintType.Valid || free.Deferrable.Valid || !free.Valid.V {
		t.Errorf("book_published: expected a free index, got %+v", free)
	}
	if off := indexes["BOOK_INACTIVE"]; !off.Valid.Valid || off.Valid.V {
		t.Errorf("book_inactive: expected an invalid index, got %+v", off)
	}
	if recent := indexes["BOOK_RECENT"]; v5 {
		if recent.Predicate.V != "published IS NOT NULL" {
			t.Errorf("book_recent: expected the predicate, got %+v", recent.Predicate)
		}
	} else if recent.Name != "" {
		t.Errorf("book_recent: expected no partial index before 5.0, got %+v", recent)
	}

	var constraints int
	for v, err := range dbmeta.Constraints.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading constraints: %v", err)
		}
		if v.Enforced.Valid {
			t.Errorf("%s: expected enforced to be absent, got %+v", v.Name, v.Enforced)
		}
		constraints++
	}
	if constraints == 0 {
		t.Error("expected constraints")
	}

	for v, err := range dbmeta.Functions.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading functions: %v", err)
		}
		if v.Prosrc != v.Source {
			t.Errorf("%s: expected prosrc to match source, got %+v and %+v", v.Name, v.Prosrc, v.Source)
		}
		if strings.EqualFold(v.Name, "doubled") && !strings.Contains(v.Prosrc.V, "RETURN n * 2") {
			t.Errorf("doubled: expected the body, got %+v", v.Prosrc)
		}
		if v4 && !strings.EqualFold(v.Name, "doubled") && !strings.EqualFold(v.Name, "book_count") {
			t.Errorf("unexpected routine %s", v.Name)
		}
	}

	notNulls := map[string]string{}
	for v, err := range dbmeta.NotNulls.All(ctx, m, db, dbmeta.Args{Parent: "AUTHOR"}.Map()) {
		if err != nil {
			t.Fatalf("reading not nulls: %v", err)
		}
		if v.NoInherit || !v.Local || v.Inherited || !v.Validated {
			t.Errorf("%s: unexpected flags %+v", v.Name, v)
		}
		notNulls[v.Column] = v.Name
	}
	if notNulls["AUTHOR_ID"] == "" || notNulls["NAME"] == "" || len(notNulls) != 2 {
		t.Errorf("expected not nulls on author_id and name, got %v", notNulls)
	}
}
