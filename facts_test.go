package dbmeta_test

import (
	"os"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/all"
)

// TestPlaceholderForms asserts the placeholder of every dialect that usql
// copies rows into, and the form of each of the four styles.
func TestPlaceholderForms(t *testing.T) {
	t.Parallel()
	tests := []struct {
		d    dbmeta.Dialect
		want string
	}{
		{dbmeta.MySQL, "?"},
		{dbmeta.TiDB, "?"},
		{dbmeta.MemSQL, "?"},
		{dbmeta.Vitess, "?"},
		{dbmeta.SQLite3, "?"},
		{dbmeta.DuckDB, "?"},
		{dbmeta.CSVQ, "?"},
		{dbmeta.Exasol, "?"},
		{dbmeta.Impala, "?"},
		{dbmeta.LibSQL, "?"},
		{dbmeta.Rqlite, "?"},
		{dbmeta.ClickHouse, "?"},
		{dbmeta.Trino, "?"},
		{dbmeta.Presto, "?"},
		{dbmeta.Snowflake, "?"},
		{dbmeta.Athena, "?"},
		{dbmeta.BigQuery, "?"},
		{dbmeta.Databricks, "?"},
		{dbmeta.Oracle, ":1"},
		{dbmeta.SQLServer, "@p1"},
		{dbmeta.PostgreSQL, "$1"},
		{dbmeta.CockroachDB, "$1"},
		{dbmeta.CrateDB, "$1"},
	}
	for _, tt := range tests {
		got, ok := tt.d.Placeholder(1)
		if !ok || got != tt.want {
			t.Errorf("%s.Placeholder(1) = %q, %t, want %q", tt.d, got, ok, tt.want)
		}
	}
	if got, _ := dbmeta.Oracle.Placeholder(12); got != ":12" {
		t.Errorf("Oracle.Placeholder(12) = %q, want :12", got)
	}
	if _, ok := dbmeta.DynamoDB.Placeholder(1); ok {
		t.Error("DynamoDB.Placeholder(1) answered, want no answer")
	}
}

// TestEveryModelHasAPlaceholder fails when a model leaves Placeholder nil. A
// model that binds by name is the exception.
func TestEveryModelHasAPlaceholder(t *testing.T) {
	t.Parallel()
	for _, d := range dbmeta.Dialects() {
		if info, _ := d.Info(); info.Named {
			// A named dialect binds by name and Placeholder is not called.
			continue
		}
		if _, ok := d.Placeholder(1); !ok {
			t.Errorf("the model for %s sets no Placeholder", d)
		}
	}
}

// TestFlags asserts every row of the inventory of D230, and that no other
// dialect carries a flag.
func TestFlags(t *testing.T) {
	t.Parallel()
	flagged := func(set []dbmeta.Dialect) map[dbmeta.Dialect]bool {
		m := make(map[dbmeta.Dialect]bool)
		for _, d := range set {
			m[d] = true
		}
		return m
	}
	flags := []struct {
		name string
		get  func(dbmeta.Dialect) bool
		want map[dbmeta.Dialect]bool
	}{
		{"EveryStatementIsAQuery", dbmeta.Dialect.EveryStatementIsAQuery, flagged([]dbmeta.Dialect{
			dbmeta.ArangoDB, dbmeta.Neo4j, dbmeta.SurrealDB, dbmeta.InfluxDB, dbmeta.InfluxQL,
		})},
		{"WritesNeedAutocommit", dbmeta.Dialect.WritesNeedAutocommit, flagged([]dbmeta.Dialect{
			dbmeta.Trino, dbmeta.Presto,
		})},
		{"ScanTypes", dbmeta.Dialect.ScanTypes, flagged([]dbmeta.Dialect{
			dbmeta.MySQL, dbmeta.TiDB, dbmeta.Vitess, dbmeta.MemSQL, dbmeta.Databend,
		})},
	}
	for _, f := range flags {
		for _, d := range dbmeta.Dialects() {
			if got := f.get(d); got != f.want[d] {
				t.Errorf("%s.%s = %t, want %t", d, f.name, got, f.want[d])
			}
		}
		for d := range f.want {
			if _, ok := d.Info(); !ok {
				t.Errorf("%s has no model, and %s is set on a model", d, f.name)
			}
		}
	}
}

// TestProduct asserts the static version text, for a dialect with no model.
func TestProduct(t *testing.T) {
	t.Parallel()
	for d, want := range map[dbmeta.Dialect]string{
		dbmeta.DynamoDB: "Amazon DynamoDB",
		dbmeta.Pinot:    "Apache Pinot",
	} {
		if got, ok := d.Product(); !ok || got != want {
			t.Errorf("%s.Product() = %q, %t, want %q", d, got, ok, want)
		}
	}
	for _, d := range dbmeta.Dialects() {
		if q, _, ok := d.VersionQuery(); ok {
			if got, has := d.Product(); has {
				t.Errorf("%s has a version statement %q and the text %q", d, q, got)
			}
		}
	}
	if _, ok := dbmeta.PostgreSQL.Product(); ok {
		t.Error("PostgreSQL.Product() answered, want no answer")
	}
}

// TestFactDialectsAreDeclared fails when a dialect that a fact names is not
// in the Dialect block.
func TestFactDialectsAreDeclared(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile("dialect.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []dbmeta.Dialect{dbmeta.CSVQ, dbmeta.DynamoDB, dbmeta.Pinot} {
		if !strings.Contains(string(src), ` Dialect = "`+string(d)+`"`) {
			t.Errorf("dialect %q is not declared in dialect.go", d)
		}
		if _, ok := d.Info(); ok {
			t.Errorf("%s has a model now, so its row moves into the model", d)
		}
	}
}
