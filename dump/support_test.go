package dump_test

import (
	"errors"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
	_ "github.com/xo/dbmeta/models/duckdb"
	_ "github.com/xo/dbmeta/models/firebird"
	_ "github.com/xo/dbmeta/models/mysql"
	_ "github.com/xo/dbmeta/models/oracle"
	_ "github.com/xo/dbmeta/models/postgres"
	_ "github.com/xo/dbmeta/models/sqlite3"
)

func TestSupportNamesTheDialectsThatCanBeDumped(t *testing.T) {
	t.Parallel()
	for _, d := range []dbmeta.Dialect{dbmeta.PostgreSQL, dbmeta.SQLite3, dbmeta.DuckDB, dbmeta.MySQL, dbmeta.Oracle} {
		c := dump.Support(d)
		if !c.Source || !c.Target || c.SourceWhy != nil || c.TargetWhy != nil {
			t.Errorf("%s: got %+v, want a source and a target", d, c)
		}
	}
}

func TestSupportGivesTheReasonForTheOthers(t *testing.T) {
	t.Parallel()
	c := dump.Support(dbmeta.Firebird)
	if c.Target || !errors.Is(c.TargetWhy, dump.ErrNoTypes) {
		t.Errorf("Firebird target: got %+v, want no type functions", c)
	}
	if c.Source || !errors.Is(c.SourceWhy, dump.ErrNoTypes) {
		t.Errorf("Firebird source: got %+v, want no type functions", c)
	}
	missing := dump.Support(dbmeta.Dialect("no-such-product"))
	if missing.Source || missing.Target || !errors.Is(missing.SourceWhy, dbmeta.ErrModelNotBuilt) {
		t.Errorf("an unknown dialect: got %+v, want model not built", missing)
	}
}
