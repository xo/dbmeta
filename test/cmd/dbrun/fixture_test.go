package main

import (
	"strings"
	"testing"

	"github.com/xo/dbmeta"
)

// TestTheExamplesFixtureDropsTheBooksFirst makes sure that each dialect the
// fixture knows drops the books before the authors, because the books refer to
// them, and that a dialect it does not know is refused.
func TestTheExamplesFixtureDropsTheBooksFirst(t *testing.T) {
	for _, d := range []dbmeta.Dialect{
		dbmeta.PostgreSQL, dbmeta.MySQL, dbmeta.SQLServer, dbmeta.Oracle, dbmeta.SQLite3, dbmeta.DuckDB,
	} {
		drop, ok := examplesDrop(d)
		if !ok || len(drop) != 2 {
			t.Fatalf("%s: expected two drops, got %q", d, drop)
		}
		if !strings.Contains(strings.ToLower(drop[0]), "books") || !strings.Contains(strings.ToLower(drop[1]), "authors") {
			t.Errorf("%s: expected the books first, got %q", d, drop)
		}
	}
	if _, ok := examplesDrop(dbmeta.Snowflake); ok {
		t.Error("expected no examples fixture for Snowflake")
	}
}
