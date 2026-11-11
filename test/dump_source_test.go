package test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// sourceSide is a product that a dump only reads from (D305): how it opens, and how its fixture
// is built.
type sourceSide struct {
	name    string
	dialect dbmeta.Dialect
	open    func(*testing.T) *sql.DB
	setup   func(*testing.T, *sql.DB) *dbmeta.Meta
	// schemas are the schemas that the fixture builds, for a server that holds others.
	schemas []string
	// tables are the patterns of the tables that the fixture builds, for a server that holds others.
	tables []string
	// rows are the statements that give the fixture rows, for a product whose fixture has none.
	rows []string
}

// memoryRows gives Trino and Presto a table with rows, because the fixture of the memory connector
// has none. The table has the types that a basic dump moves.
var memoryRows = []string{
	"DROP TABLE IF EXISTS memory.dbmeta_fixture.dump_basic",
	`CREATE TABLE memory.dbmeta_fixture.dump_basic (id integer, name varchar(20), amount decimal(10, 2), flag boolean, made timestamp, score double, note varchar)`,
	`INSERT INTO memory.dbmeta_fixture.dump_basic VALUES
	  (1, 'ann', 12.50, true, TIMESTAMP '2024-02-29 13:14:15.000', 1.5, 'hello'),
	  (2, 'it''s a \ test é€', -0.25, false, TIMESTAMP '1999-12-31 23:59:59.000', -2.25, NULL),
	  (3, '', CAST(NULL AS decimal(10, 2)), true, CAST(NULL AS timestamp), CAST(NULL AS double), 'Unicode: ありがとう')`,
}

func sourceSides() []sourceSide {
	return []sourceSide{
		{"trino", dbmeta.Trino, openTrino, setupTrino, []string{"dbmeta_fixture"}, nil, memoryRows},
		{"presto", dbmeta.Presto, openPresto, setupPresto, []string{"dbmeta_fixture"}, nil, memoryRows},
		{"athena", dbmeta.Athena, openAthena, setupAthena, nil, nil, nil},
		{"hive", dbmeta.Hive, openHive, setupHive, nil, nil, nil},
		{"impala", dbmeta.Impala, openImpala, setupImpala, nil, nil, nil},
		{"maxcompute", dbmeta.MaxCompute, openMaxCompute, setupMaxCompute, nil, nil, nil},
		{"elasticsearch", dbmeta.Elasticsearch, openElasticsearch, setupElasticsearch, nil, []string{"dbmeta_%"}, nil},
		{"opensearch", dbmeta.OpenSearch, openOpenSearch, setupOpenSearch, nil, []string{"dbmeta_%"}, nil},
		{"solr", dbmeta.Solr, openSolr, setupSolr, nil, nil, nil},
		{"druid", dbmeta.Druid, openDruid, setupDruid, nil, nil, nil},
		{"drill", dbmeta.Drill, openDrill, setupDrill, nil, nil, nil},
		{"influxdb", dbmeta.InfluxDB, openInfluxDB, setupInfluxDB, nil, nil, nil},
		{"influxql", dbmeta.InfluxQL, openInfluxQL, setupInfluxQL, nil, nil, nil},
		{"tablestore", dbmeta.Tablestore, openTablestore, setupTablestore, nil, nil, nil},
		{"surrealdb", dbmeta.SurrealDB, openSurrealDB, setupSurrealDB, nil, nil, nil},
		{"arangodb", dbmeta.ArangoDB, openArangoDB, setupArangoDB, nil, nil, nil},
	}
}

// TestASourceOnlyDumpReadsATable builds the fixture of each product that a dump only reads from,
// reads every table with its rows, writes the table and the rows into PostgreSQL, and requires
// the same number of rows. A type that PostgreSQL has no equivalent for becomes text.
func TestASourceOnlyDumpReadsATable(t *testing.T) {
	policy := dump.Policy{Default: dump.Allow{Lossy: true, Text: true}}
	pg := basicSides()[0]
	for _, src := range sourceSides() {
		t.Run(src.name, func(t *testing.T) {
			if c := dump.Support(src.dialect); !c.Source || c.Target {
				t.Fatalf("the support of %s is %+v, and want a source and no target", src.name, c)
			}
			db := src.open(t)
			m := src.setup(t, db)
			for _, stmt := range src.rows {
				if _, err := db.ExecContext(t.Context(), stmt); err != nil && !strings.HasPrefix(stmt, "DROP") {
					t.Fatalf("%s: %v", stmt, err)
				}
			}
			t.Cleanup(func() {
				_, _ = db.ExecContext(context.WithoutCancel(t.Context()), "DROP TABLE IF EXISTS memory.dbmeta_fixture.dump_basic")
			})
			all, err := dump.Read(t.Context(), m, db, dump.Filter{Schemas: src.schemas, Tables: src.tables})
			if err != nil {
				t.Fatalf("reading: %v", err)
			}
			tables, total := 0, 0
			for _, sc := range all.Schemas {
				for _, tb := range sc.Tables {
					tables++
					t.Run(sc.Name+"."+tb.Name, func(t *testing.T) {
						total += copySourceTable(t, m, db, sc.Name, tb.Name, pg, policy)
					})
				}
			}
			if tables == 0 {
				t.Errorf("the fixture of %s has no table", src.name)
			}
			if total == 0 {
				t.Errorf("the fixture of %s has no row, so the test copied nothing", src.name)
			}
			t.Logf("%s: %d tables, %d rows", src.name, tables, total)
		})
	}
}

// copySourceTable copies one table into PostgreSQL and compares the number of rows.
func copySourceTable(t *testing.T, m *dbmeta.Meta, db *sql.DB, schema, table string, pg basicSide, policy dump.Policy) int {
	t.Helper()
	snap, err := dump.Read(t.Context(), m, db, dump.Filter{Schemas: []string{schema}, Tables: []string{table}})
	if err != nil {
		t.Fatalf("reading %s.%s: %v", schema, table, err)
	}
	// The loader of the original snapshot names the table as the source does, and the loader of
	// the renamed one writes into PostgreSQL.
	read, err := dump.NewLoader(snap, schema, table, dbmeta.PostgreSQL, policy)
	if errors.Is(err, dbmeta.ErrNotFound) {
		t.Skipf("nothing to move: %v", err)
	}
	if err != nil {
		t.Fatalf("preparing the loader of the source: %v", err)
	}
	renameSchemas(snap, "dump_cross")
	pgDB, _ := pg.open(t, "dst")
	mustExec(t, pgDB, `DROP SCHEMA IF EXISTS "dump_cross" CASCADE`)
	for st, err := range dump.Script(snap, dbmeta.PostgreSQL, policy) {
		if err != nil {
			t.Skipf("writing the script: %v", err)
		}
		if st.Stage == dump.Pre && st.SQL != "" {
			mustExec(t, pgDB, st.SQL)
		}
	}
	l, err := dump.NewLoader(snap, "dump_cross", table, dbmeta.PostgreSQL, policy)
	if err != nil {
		t.Fatalf("preparing the loader: %v", err)
	}
	query := read.SelectAll()
	rows := readCells(t, db, query)
	for i := range rows {
		rows[i] = rows[i][read.LeadingColumns():]
	}
	for start := 0; start < len(rows); start += l.BatchSize() {
		stmt, err := l.Insert(rows[start:min(start+l.BatchSize(), len(rows))])
		if err != nil {
			t.Fatalf("writing the INSERT: %v", err)
		}
		if _, err := pgDB.ExecContext(context.Background(), stmt); err != nil {
			t.Fatalf("%s: %v", strings.SplitN(stmt, "VALUES", 2)[0], err)
		}
	}
	var n int
	if err := pgDB.QueryRowContext(t.Context(), `SELECT count(*) FROM "dump_cross".`+quotePG(table)).Scan(&n); err != nil {
		t.Fatalf("counting: %v", err)
	}
	if n != len(rows) {
		t.Errorf("%s.%s has %d rows in the source and %d in the copy (%s)", schema, table, len(rows), n, query)
	}
	return n
}

func quotePG(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }
