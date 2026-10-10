package test

import (
	"database/sql"
	"os"
	"testing"

	"github.com/xo/dbmeta"
	rsfixture "github.com/xo/dbmeta/models/redshift/fixture"
)

// TestRedshiftSpectrum reads the external table that the Glue data catalog
// holds, as the administrator and as a user that has USAGE on the external
// schema (D221). It skips when the namespace has no default IAM role, which is
// when the fixture makes no external schema.
func TestRedshiftSpectrum(t *testing.T) {
	db := openRedshift(t)
	m := setupRedshift(t, db)
	if !hasRedshiftSpectrum(t, db) {
		t.Skip("the namespace has no default IAM role, so the fixture made no external schema")
	}
	grantee := makeRedshiftGrantee(t, db, os.Getenv("DBMETA_REDSHIFT"), rsfixture.Everything.Schema)
	user, err := sql.Open("pgx", grantee)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { user.Close() })

	args := dbmeta.Args{Schema: rsfixture.Everything.External}.Map()
	for who, conn := range map[string]*sql.DB{"administrator": db, "grantee": user} {
		var tables []dbmeta.Table
		for v, err := range dbmeta.Tables.All(t.Context(), m, conn, args) {
			if err != nil {
				t.Fatalf("%s: reading tables: %v", who, err)
			}
			tables = append(tables, v)
		}
		if len(tables) != 1 || tables[0].Name != "spectrum_t" || tables[0].Type != "external table" ||
			!tables[0].Owner.Valid || tables[0].Size.Valid || tables[0].Rows.Valid ||
			!tables[0].Options.Valid {
			t.Fatalf("%s: expected the one external table spectrum_t with an owner and options, got %+v", who, tables)
		}
		types := map[string]string{}
		for v, err := range dbmeta.Columns.All(t.Context(), m, conn, dbmeta.Args{
			Schema: rsfixture.Everything.External, Parent: "spectrum_t"}.Map()) {
			if err != nil {
				t.Fatalf("%s: reading columns: %v", who, err)
			}
			types[v.Name] = v.DataType
			if !v.Nullable || v.PrimaryKey || v.Comment.Valid {
				t.Errorf("%s: %s: expected a nullable column with no key and no comment, got %+v", who, v.Name, v)
			}
		}
		if types["id"] != "int" || types["v"] != "string" || len(types) != 2 {
			t.Errorf("%s: expected id int and v string, got %v", who, types)
		}
	}
}
