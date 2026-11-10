package dump_test

import (
	"strings"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
	_ "github.com/xo/dbmeta/models/duckdb"
	_ "github.com/xo/dbmeta/models/mysql"
	_ "github.com/xo/dbmeta/models/sqlite3"
)

// twoSchemas is a snapshot of two schemas, each with one table.
func twoSchemas(d dbmeta.Dialect) *dump.Snapshot {
	typ, err := dump.ParseType(d, "integer")
	if err != nil {
		panic(err)
	}
	typ.Raw = "integer"
	table := dump.Table{Name: "authors", Columns: []dump.Column{{Name: "id", Type: typ}}}
	return &dump.Snapshot{
		Version: dump.SnapshotVersion, Dialect: string(d),
		Schemas: []dump.Schema{{Name: "app", Tables: []dump.Table{table}}, {Name: "extra", Tables: []dump.Table{table}}},
	}
}

// joined is the text of a script.
func joined(t *testing.T, s *dump.Snapshot, target dbmeta.Dialect) string {
	t.Helper()
	var b strings.Builder
	for _, st := range scriptText(t, s, target) {
		b.WriteString(st.SQL + ";\n")
	}
	return b.String()
}

// TestATargetWithOneSchemaOrAnotherSourceNeverNamesTheSchema holds that a dump of
// two schemas of DuckDB to SQLite or to MySQL writes plain names, because SQLite
// has one schema and a dump to another product puts every table in its one
// database.
func TestATargetWithOneSchemaOrAnotherSourceNeverNamesTheSchema(t *testing.T) {
	for _, target := range []dbmeta.Dialect{dbmeta.SQLite3, dbmeta.MySQL} {
		if got := joined(t, twoSchemas(dbmeta.DuckDB), target); strings.Contains(got, "app`.") || strings.Contains(got, `"app".`) {
			t.Errorf("the script for %s names the schema:\n%s", target, got)
		}
	}
}

// TestADumpOfSeveralMySQLDatabasesNamesEachOne holds that the same product keeps
// the name of every database.
func TestADumpOfSeveralMySQLDatabasesNamesEachOne(t *testing.T) {
	got := joined(t, twoSchemas(dbmeta.MySQL), dbmeta.MySQL)
	for _, want := range []string{"CREATE DATABASE `app`", "CREATE TABLE `app`.`authors`", "CREATE TABLE `extra`.`authors`"} {
		if !strings.Contains(got, want) {
			t.Errorf("the script has no %q:\n%s", want, got)
		}
	}
}

// TestAForeignKeyToAnotherMySQLDatabaseNamesIt holds that a dump of one database
// names the database of a table that a foreign key points at, and that the read of a
// table names its database.
func TestAForeignKeyToAnotherMySQLDatabaseNamesIt(t *testing.T) {
	s := twoSchemas(dbmeta.MySQL)
	s.Schemas = s.Schemas[:1]
	s.Schemas[0].Tables[0].Constraints = []dump.Constraint{{
		Name: "fk", Kind: "foreign key", Columns: []string{"id"},
		Ref: &dump.Reference{Schema: "extra", Table: "egg", Columns: []string{"id"}},
	}}
	got := joined(t, s, dbmeta.MySQL)
	if !strings.Contains(got, "REFERENCES `extra`.`egg`") {
		t.Errorf("the foreign key does not name the other database:\n%s", got)
	}
	l, err := dump.NewLoader(s, "app", "authors", dbmeta.MySQL, dump.Policy{})
	if err != nil {
		t.Fatal(err)
	}
	if want := "FROM `app`.`authors`"; !strings.Contains(l.SelectAll(), want) {
		t.Errorf("the read is %q, and it has no %q", l.SelectAll(), want)
	}
}
