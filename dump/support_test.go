package dump_test

import (
	"errors"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
	_ "github.com/xo/dbmeta/models/arangodb"
	_ "github.com/xo/dbmeta/models/athena"
	_ "github.com/xo/dbmeta/models/drill"
	_ "github.com/xo/dbmeta/models/druid"
	_ "github.com/xo/dbmeta/models/duckdb"
	_ "github.com/xo/dbmeta/models/elasticsearch"
	_ "github.com/xo/dbmeta/models/hive"
	_ "github.com/xo/dbmeta/models/impala"
	_ "github.com/xo/dbmeta/models/influxdb"
	_ "github.com/xo/dbmeta/models/influxql"
	_ "github.com/xo/dbmeta/models/maxcompute"
	_ "github.com/xo/dbmeta/models/mysql"
	_ "github.com/xo/dbmeta/models/opensearch"
	_ "github.com/xo/dbmeta/models/oracle"
	_ "github.com/xo/dbmeta/models/postgres"
	_ "github.com/xo/dbmeta/models/presto"
	_ "github.com/xo/dbmeta/models/redshift"
	_ "github.com/xo/dbmeta/models/solr"
	_ "github.com/xo/dbmeta/models/sqlite3"
	_ "github.com/xo/dbmeta/models/surrealdb"
	_ "github.com/xo/dbmeta/models/tablestore"
	_ "github.com/xo/dbmeta/models/trino"
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
	c := dump.Support(dbmeta.Redshift)
	if c.Target || !errors.Is(c.TargetWhy, dump.ErrNoTypes) {
		t.Errorf("Redshift target: got %+v, want no type functions", c)
	}
	if c.Source || !errors.Is(c.SourceWhy, dump.ErrNoTypes) {
		t.Errorf("Redshift source: got %+v, want no type functions", c)
	}
	missing := dump.Support(dbmeta.Dialect("no-such-product"))
	if missing.Source || missing.Target || !errors.Is(missing.SourceWhy, dbmeta.ErrModelNotBuilt) {
		t.Errorf("an unknown dialect: got %+v, want model not built", missing)
	}
}

func TestSupportNamesTheProductsThatAreOnlyASource(t *testing.T) {
	t.Parallel()
	for _, d := range []dbmeta.Dialect{
		dbmeta.Trino, dbmeta.Presto, dbmeta.Athena, dbmeta.Hive, dbmeta.Impala, dbmeta.MaxCompute,
		dbmeta.Elasticsearch, dbmeta.OpenSearch, dbmeta.Solr, dbmeta.Druid, dbmeta.Drill, dbmeta.InfluxDB,
		dbmeta.InfluxQL, dbmeta.Tablestore, dbmeta.SurrealDB, dbmeta.ArangoDB,
	} {
		c := dump.Support(d)
		if !c.Source || c.Target || c.SourceWhy != nil || c.TargetWhy == nil {
			t.Errorf("%s: got %+v, want a source and no target", d, c)
		}
	}
}
