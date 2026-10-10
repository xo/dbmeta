package test

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
	arfixture "github.com/xo/dbmeta/models/arangodb/fixture"
	athfixture "github.com/xo/dbmeta/models/athena/fixture"
	avfixture "github.com/xo/dbmeta/models/avatica/fixture"
	bqfixture "github.com/xo/dbmeta/models/bigquery/fixture"
	cafixture "github.com/xo/dbmeta/models/cassandra/fixture"
	chfixture "github.com/xo/dbmeta/models/clickhouse/fixture"
	cmfixture "github.com/xo/dbmeta/models/cosmos/fixture"
	cbfixture "github.com/xo/dbmeta/models/couchbase/fixture"
	crfixture "github.com/xo/dbmeta/models/cratedb/fixture"
	dbfixture "github.com/xo/dbmeta/models/databend/fixture"
	dbxfixture "github.com/xo/dbmeta/models/databricks/fixture"
	dlfixture "github.com/xo/dbmeta/models/drill/fixture"
	drfixture "github.com/xo/dbmeta/models/druid/fixture"
	dkfixture "github.com/xo/dbmeta/models/duckdb/fixture"
	esfixture "github.com/xo/dbmeta/models/elasticsearch/fixture"
	exfixture "github.com/xo/dbmeta/models/exasol/fixture"
	fbfixture "github.com/xo/dbmeta/models/firebird/fixture"
	gzfixture "github.com/xo/dbmeta/models/gizmosql/fixture"
	hafixture "github.com/xo/dbmeta/models/hana/fixture"
	hvfixture "github.com/xo/dbmeta/models/hive/fixture"
	imfixture "github.com/xo/dbmeta/models/impala/fixture"
	ixfixture "github.com/xo/dbmeta/models/influxdb/fixture"
	iqfixture "github.com/xo/dbmeta/models/influxql/fixture"
	lsfixture "github.com/xo/dbmeta/models/libsql/fixture"
	myfixture "github.com/xo/dbmeta/models/mysql/fixture"
	njfixture "github.com/xo/dbmeta/models/neo4j/fixture"
	osfixture "github.com/xo/dbmeta/models/opensearch/fixture"
	orfixture "github.com/xo/dbmeta/models/oracle/fixture"
	pgfixture "github.com/xo/dbmeta/models/postgres/fixture"
	prfixture "github.com/xo/dbmeta/models/presto/fixture"
	qdfixture "github.com/xo/dbmeta/models/questdb/fixture"
	rsfixture "github.com/xo/dbmeta/models/redshift/fixture"
	rqfixture "github.com/xo/dbmeta/models/rqlite/fixture"
	ssfixture "github.com/xo/dbmeta/models/singlestore/fixture"
	sffixture "github.com/xo/dbmeta/models/snowflake/fixture"
	slfixture "github.com/xo/dbmeta/models/solr/fixture"
	spfixture "github.com/xo/dbmeta/models/spanner/fixture"
	sqfixture "github.com/xo/dbmeta/models/sqlite3/fixture"
	msfixture "github.com/xo/dbmeta/models/sqlserver/fixture"
	srfixture "github.com/xo/dbmeta/models/surrealdb/fixture"
	tdfixture "github.com/xo/dbmeta/models/tidb/fixture"
	trfixture "github.com/xo/dbmeta/models/trino/fixture"
	vefixture "github.com/xo/dbmeta/models/vertica/fixture"
	vtfixture "github.com/xo/dbmeta/models/vitess/fixture"
	ydfixture "github.com/xo/dbmeta/models/ydb/fixture"
)

// The cross family conformance test.
//
// TestMySQLAgainstMariaDB compares two products of one family, raw, and it has
// found four faults. This does the same thing across families, which needs two
// changes: the values compared are the portable ones, and the expectation is
// checked in rather than taken from another live database.
//
// # Why a checked in expectation rather than pairwise
//
// Five databases pairwise is ten comparisons and a failure does not say which
// side is wrong. One expectation is five comparisons and every failure names
// the database.
//
// The expectation works only because the canonical projection is portable by
// construction. A raw golden has to be per product and per release,
// because PostgreSQL 9.6 and 18 disagree about raw values. Nullability and
// ordinal position do not change between releases, so one file covers every
// release of every database.
//
// It does not replace TestMySQLAgainstMariaDB. That compares raw values within
// one family and catches what this cannot: two databases that both changed the
// same way, and every difference in spelling. Goldens catch a regression and
// pairwise catches a divergence, and they are different faults.
//
// # Why this cannot make a model lie
//
// It compares canonical projections, and canonical.go can only read a model
// value. The raw values are asserted by each database's own tests, which this
// cannot weaken. See canonical.go.

// update rewrites the expectation instead of comparing against it.
var update = flag.Bool("update", os.Getenv("DBMETA_UPDATE") != "", "rewrite testdata/conformance.txt and testdata/parity.txt. DBMETA_UPDATE sets it, for a run that dbrun starts")

// conformGolden is the checked in expectation.
const conformGolden = "testdata/conformance.txt"

// conformTarget is one database under test.
type conformTarget struct {
	name    string
	dialect dbmeta.Dialect
	// open returns a connection, or skips when the database is not running.
	open func(*testing.T) *sql.DB
	// schema is where the fixture built its objects.
	schema string
	// build runs the fixture and returns the meta.
	build func(*testing.T, *sql.DB) *dbmeta.Meta
}

func conformTargets() []conformTarget {
	return []conformTarget{
		{
			name: "postgres", dialect: dbmeta.PostgreSQL,
			open: open, schema: pgfixture.Everything.Schema, build: setup,
		},
		{
			name: "cockroachdb", dialect: dbmeta.CockroachDB,
			open: openCockroachDB, schema: pgFamilies[1].fixture.Schema, build: setupCockroachDB,
		},
		{
			name: "cratedb", dialect: dbmeta.CrateDB,
			open: openCrateDB, schema: crfixture.Everything.Schema, build: setupCrateDB,
		},
		{
			name: "databend", dialect: dbmeta.Databend,
			open: openDatabend, schema: dbfixture.Everything.Schema, build: setupDatabend,
		},
		{
			name: "singlestore", dialect: dbmeta.MemSQL,
			open: openSingleStore, schema: ssfixture.Everything.Schema, build: setupSingleStore,
		},
		{
			name: "impala", dialect: dbmeta.Impala,
			open: openImpala, schema: imfixture.Everything.Schema, build: setupImpala,
		},
		{
			name: "questdb", dialect: dbmeta.QuestDB,
			open: openQuestDB, schema: qdfixture.Everything.Schema, build: setupQuestDB,
		},
		{
			name: "tidb", dialect: dbmeta.TiDB,
			open: openTiDB, schema: tdfixture.Everything.Schema, build: setupTiDB,
		},
		{
			name: "vitess", dialect: dbmeta.Vitess,
			open: openVitess, schema: vtfixture.Everything.Schema, build: setupVitess,
		},
		{
			name: "mysql", dialect: dbmeta.MySQL,
			open: openMySQL, schema: myfixture.Everything.Schema, build: setupMySQL,
		},
		{
			name: "sqlite3", dialect: dbmeta.SQLite3,
			open:   func(t *testing.T) *sql.DB { return openSQLiteWith(t, "sqlite3") },
			schema: sqfixture.Everything.Schema, build: setupSQLite,
		},
		{
			name: "influxdb", dialect: dbmeta.InfluxDB,
			open: openInfluxDB, schema: ixfixture.Everything.Schema, build: setupInfluxDB,
		},
		{
			name: "influxql", dialect: dbmeta.InfluxQL,
			open: openInfluxQL, schema: iqfixture.Everything.Schema, build: setupInfluxQL,
		},
		{
			name: "rqlite", dialect: dbmeta.Rqlite,
			open: openRqlite, schema: rqfixture.Everything.Schema, build: setupRqlite,
		},
		{
			name: "libsql", dialect: dbmeta.LibSQL,
			open: openLibSQL, schema: lsfixture.Everything.Schema, build: setupLibSQL,
		},
		{
			name: "duckdb", dialect: dbmeta.DuckDB,
			open: openDuckDB, schema: dkfixture.Everything.Schema, build: setupDuckDB,
		},
		{
			name: "gizmosql", dialect: dbmeta.GizmoSQL,
			open: openGizmoSQL, schema: gzfixture.Everything.Schema, build: setupGizmoSQL,
		},
		{
			name: "sqlserver", dialect: dbmeta.SQLServer,
			open: openSQLServer, schema: msfixture.Everything.Schema, build: setupSQLServer,
		},
		{
			name: "oracle", dialect: dbmeta.Oracle,
			open: openOracle, schema: orfixture.Everything.Schema, build: setupOracle,
		},
		{
			name: "cassandra", dialect: dbmeta.Cassandra,
			open: openCassandra, schema: cafixture.Everything.Schema,
			build: setupCassandra,
		},
		{
			name: "couchbase", dialect: dbmeta.Couchbase,
			open: openCouchbase, schema: cbfixture.Everything.Schema,
			// Below 7.6 every query is too old, so there is nothing to
			// compare. TestCouchbaseTooOldBelowTheFloor covers that release.
			build: func(t *testing.T, db *sql.DB) *dbmeta.Meta {
				m := setupCouchbase(t, db)
				skipBelowFloor(t, m)
				return m
			},
		},
		{
			name: "neo4j", dialect: dbmeta.Neo4j,
			open: openNeo4j, schema: njfixture.Everything.Schema, build: setupNeo4j,
		},
		{
			name: "arangodb", dialect: dbmeta.ArangoDB,
			open: openArangoDB, schema: arfixture.Everything.Schema, build: setupArangoDB,
		},
		{
			name: "surrealdb", dialect: dbmeta.SurrealDB,
			open: openSurrealDB, schema: srfixture.Everything.Schema,
			// 2.x answers the current schema alone, so there is nothing to
			// compare. TestSurrealDBTooOld covers that release.
			build: func(t *testing.T, db *sql.DB) *dbmeta.Meta {
				m := setupSurrealDB(t, db)
				if !m.Version().Main().AtLeast(srINFO) {
					t.Skipf("the server is %s, which answers the current schema alone", m.Version())
				}
				return m
			},
		},
		{
			name: "avatica", dialect: dbmeta.Avatica,
			open: openAvatica, schema: avfixture.Everything.Schema, build: setupAvatica,
		},
		{
			name: "drill", dialect: dbmeta.Drill,
			open: openDrill, schema: dlfixture.Everything.Schema, build: setupDrill,
		},
		{
			name: "druid", dialect: dbmeta.Druid,
			open: openDruid, schema: drfixture.Everything.Schema, build: setupDruid,
		},
		{
			name: "elasticsearch", dialect: dbmeta.Elasticsearch,
			open: openElasticsearch, schema: esfixture.Everything.Schema, build: setupElasticsearch,
		},
		{
			name: "opensearch", dialect: dbmeta.OpenSearch,
			open: openOpenSearch, schema: osfixture.Everything.Schema, build: setupOpenSearch,
		},
		{
			name: "solr", dialect: dbmeta.Solr,
			open: openSolr, schema: slfixture.Everything.Schema, build: setupSolr,
		},
		{
			name: "clickhouse", dialect: dbmeta.ClickHouse,
			open: openClickHouse, schema: chfixture.Everything.Schema,
			build: setupClickHouse,
		},
		{
			name: "trino", dialect: dbmeta.Trino,
			open: openTrino, schema: trfixture.Everything.Schema,
			build: setupTrino,
		},
		{
			name: "presto", dialect: dbmeta.Presto,
			open: openPresto, schema: prfixture.Everything.Schema,
			build: setupPresto,
		},
		{
			name: "hive", dialect: dbmeta.Hive,
			open: openHive, schema: hvfixture.Everything.Schema,
			build: setupHive,
		},
		{
			name: "hana", dialect: dbmeta.HANA,
			open: openHANA, schema: hafixture.Everything.Schema,
			build: setupHANA,
		},
		{
			name: "firebird", dialect: dbmeta.Firebird,
			open: openFirebird, schema: fbfixture.Everything.Schema,
			build: setupFirebird,
		},
		{
			name: "exasol", dialect: dbmeta.Exasol,
			open: openExasol, schema: exfixture.Everything.Schema,
			build: setupExasol,
		},
		{
			name: "vertica", dialect: dbmeta.Vertica,
			open: openVertica, schema: vefixture.Everything.Schema,
			build: setupVertica,
		},
		{
			name: "snowflake", dialect: dbmeta.Snowflake,
			open: openSnowflake, schema: sffixture.Everything.Schema,
			build: setupSnowflake,
		},
		{
			name: "redshift", dialect: dbmeta.Redshift,
			open: openRedshift, schema: rsfixture.Everything.Schema,
			build: setupRedshift,
		},
		{
			name: "ydb", dialect: dbmeta.YDB,
			open: openYDB, schema: ydfixture.Everything.Schema,
			build: setupYDB,
		},
		{
			name: "bigquery", dialect: dbmeta.BigQuery,
			open: openBigQuery, schema: bqfixture.Everything.Schema,
			build: setupBigQuery,
		},
		{
			name: "athena", dialect: dbmeta.Athena,
			open: openAthena, schema: athfixture.Everything.Schema,
			build: setupAthena,
		},
		{
			name: "spanner", dialect: dbmeta.Spanner,
			open: openSpanner, schema: spfixture.Everything.Schema,
			build: setupSpanner,
		},
		{
			name: "databricks", dialect: dbmeta.Databricks,
			open: openDatabricks, schema: dbxfixture.Everything.Schema,
			build: setupDatabricks,
		},
		{
			name: "cosmos", dialect: dbmeta.Cosmos,
			open: openCosmos, schema: cmfixture.Everything.Schema,
			build: setupCosmos,
		},
	}
}

// TestConformance checks every database against the same expectation.
//
// Run with -update to rewrite the expectation. Read the diff: a change here is
// a change in what a consumer can rely on, and it is the reason the file is
// checked in rather than computed.
func TestConformance(t *testing.T) {
	want := readGolden(t)
	var ran int
	for _, target := range conformTargets() {
		t.Run(target.name, func(t *testing.T) {
			db := target.open(t)
			m := target.build(t, db)
			got := conformReport(t, m, db, target.schema)
			section := conformSection(target.name, m, want)
			ran++
			if *update {
				writeGolden(t, section, got)
				return
			}
			expected, ok := want[section]
			if !ok {
				t.Fatalf("no expectation for %s in %s. Run go test -update and read the diff.",
					section, conformGolden)
			}
			compareReport(t, section, expected, got)
		})
	}
	if ran == 0 {
		t.Skip("no database was reachable")
	}
}

// conformSection picks the section a server is recorded under.
//
// One section holds for every release of a product, because the canonical
// projection is portable. The exception is a fixture that cannot build a
// core object on an old release: Vertica added CHECK constraints in 9.1, so
// 7.2 has no check constraint on book to report. A section named
// product@major, or product@major.minor, wins for a server reporting that
// release, the same way parity's does. See D61.
func conformSection(name string, m *dbmeta.Meta, want map[string][]string) string {
	main := m.Version().Main()
	if main.Unknown || len(main.Parts) == 0 {
		return name
	}
	for _, n := range releaseNames(name, "", main.Parts) {
		n = strings.TrimSuffix(n, "/")
		if _, ok := want[n]; ok {
			return n
		}
	}
	return name
}

// conformPrefix is what a product's fixture writes before the name of every
// core object, where the product cannot name them plainly. The role of the
// ordinary user of Elasticsearch reads the indices that start with dbmeta, so
// the fixture calls the core index author dbmeta_author, and the report reads
// it as author.
var conformPrefix = map[dbmeta.Dialect]string{dbmeta.Elasticsearch: "dbmeta_", dbmeta.OpenSearch: "dbmeta_"}

// conformReport reads the core schema and returns the canonical answer, as
// lines, sorted so that two databases can be compared line by line.
func conformReport(t *testing.T, m *dbmeta.Meta, db *sql.DB, schema string) []string {
	t.Helper()
	ctx := t.Context()
	args := dbmeta.Args{Schema: schema}.Map()

	// Only the core objects. A database builds more than these and what it
	// builds beyond them is its own model's business.
	core := map[string]bool{
		"author": true, "book": true, "region": true, "shipment": true, "recent": true,
	}

	var out []string

	// tables, by name and kind
	var tables []string
	for v, err := range dbmeta.Tables.All(ctx, m, db, args) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		v.Name = strings.TrimPrefix(v.Name, conformPrefix[m.Dialect()])
		if !core[fold(v.Name)] {
			continue
		}
		// The kind is normalized to table or view. DuckDB says "temporary
		// table" for a temporary one and nothing here is temporary, and
		// SQLite says "virtual" for a module backed table. Anything that is
		// not a view is a table for this purpose.
		kind := "table"
		if strings.Contains(v.Type, "view") {
			kind = "view"
		}
		tables = append(tables, fmt.Sprintf("table %s %s", fold(v.Name), kind))
	}
	sort.Strings(tables)
	out = append(out, tables...)

	// columns, canonically. A database with no column catalog contributes no
	// column lines. Couchbase is one case: a document has no fixed shape, so
	// no statement lists the fields of a collection. Neo4j is the other: the
	// only list of the properties of a label reads every node (D162).
	var cols []string
	if dbmeta.Columns.Support(m) != dbmeta.Supported {
		t.Logf("no column lines: columns is %v", dbmeta.Columns.Support(m))
		return out
	}
	for v, err := range dbmeta.Columns.All(ctx, m, db, args) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		v.Table = strings.TrimPrefix(v.Table, conformPrefix[m.Dialect()])
		if !core[fold(v.Table)] {
			continue
		}
		c := canonicalizeColumn(v)
		cols = append(cols, fmt.Sprintf("column %s %s", c.key(), c))
	}
	sort.Strings(cols)
	out = append(out, cols...)

	// constraints, by table, kind and columns rather than by name.
	//
	// A database that answers neither query contributes no constraint lines.
	// ClickHouse is the case: system.constraints holds the expression a CHECK
	// asserts and there is no list of columns behind it, so it registers
	// Constraints and not ConstraintColumns. Skipping is safe, because a
	// database that used to report them loses its recorded lines and fails
	// on the diff rather than here.
	if dbmeta.Constraints.Support(m) != dbmeta.Supported ||
		dbmeta.ConstraintColumns.Support(m) != dbmeta.Supported {
		t.Logf("no constraint lines: constraints is %v and constraint columns is %v",
			dbmeta.Constraints.Support(m), dbmeta.ConstraintColumns.Support(m))
		return out
	}
	kinds := map[string]string{}
	for v, err := range dbmeta.Constraints.All(ctx, m, db, args) {
		if err != nil {
			t.Fatalf("reading constraints: %v", err)
		}
		kinds[fold(v.Table)+"\x00"+fold(v.Name)] = v.Type
	}
	var ccols []dbmeta.ConstraintColumn
	for v, err := range dbmeta.ConstraintColumns.All(ctx, m, db, args) {
		if err != nil {
			t.Fatalf("reading constraint columns: %v", err)
		}
		if core[fold(v.Table)] {
			ccols = append(ccols, v)
		}
	}
	for _, c := range canonicalizeConstraints(ccols, kinds) {
		out = append(out, "constraint "+c.String())
	}

	return out
}

// compareReport reports every line that differs, in both directions.
func compareReport(t *testing.T, name string, want, got []string) {
	t.Helper()
	inWant := map[string]bool{}
	for _, l := range want {
		inWant[l] = true
	}
	inGot := map[string]bool{}
	for _, l := range got {
		inGot[l] = true
	}
	for _, l := range want {
		if !inGot[l] {
			t.Errorf("%s does not report: %s", name, l)
		}
	}
	for _, l := range got {
		if !inWant[l] {
			t.Errorf("%s reports, and the expectation does not have: %s", name, l)
		}
	}
}

// readGolden reads the expectation, as a map from database name to lines.
//
// The file is one section per database, because a database cannot build every
// object of every other. A line outside a section is an error rather than a
// default, so a typo in a section header fails rather than silently applying
// to nothing.
func readGolden(t *testing.T) map[string][]string {
	t.Helper()
	return readGoldenAt(t, conformGolden)
}

// readGoldenAt reads one expectation file.
func readGoldenAt(t *testing.T, path string) map[string][]string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		if *update {
			return map[string][]string{}
		}
		t.Fatalf("reading %s: %v. Run go test -update to write it.", path, err)
	}
	out := map[string][]string{}
	var section string
	for line := range strings.SplitSeq(string(body), "\n") {
		line = strings.TrimRight(line, " \t")
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
		case strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]"):
			section = line[1 : len(line)-1]
			if _, seen := out[section]; seen {
				t.Fatalf("%s: the section [%s] appears twice", path, section)
			}
			out[section] = nil
		case section == "":
			t.Fatalf("%s: a line before any section: %q", path, line)
		default:
			out[section] = append(out[section], line)
		}
	}
	return out
}

// writeGolden rewrites one section, leaving the others as they are. A database
// that is not running keeps its recorded answer rather than losing it.
func writeGolden(t *testing.T, name string, lines []string) {
	t.Helper()
	writeGoldenAt(t, conformGolden, goldenHeader, name, lines)
}

// writeGoldenAt rewrites one section of one expectation file.
func writeGoldenAt(t *testing.T, path, header, name string, lines []string) {
	t.Helper()
	sections := readGoldenAt(t, path)
	sections[name] = lines
	names := make([]string, 0, len(sections))
	for n := range sections {
		names = append(names, n)
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString(header)
	for _, n := range names {
		b.WriteString("\n[" + n + "]\n")
		for _, l := range sections[n] {
			b.WriteString(l + "\n")
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote the %s section of %s", name, path)
}

const goldenHeader = `# What every database answers about the core fixture schema, canonically.
#
# This is checked in on purpose. A change here is a change in what a consumer
# can rely on across databases, so it belongs in a diff that somebody reads.
#
# Only the portable facts are here. A type spelling, a rendered default and a
# rendered constraint definition are per product and are dropped, and
# canonical.go records every one with the reason. The raw values are asserted
# by each database's own tests.
#
# One section per database, because a database cannot build every object
# another can. Rewrite one with:
#
#	go test -run TestConformance -update ./...
#
# and read the diff.
`

// TestCanonicalFieldsAreRecorded is the guard that keeps canonical.go honest.
//
// The canonical projection drops fields, and dropping a field is how a
// comparison stops noticing a difference. Every dropped field has to be named
// in canonicalFields with a reason, and this checks that by reflection rather
// than by trust: a field on the model that is not on the canonical struct and
// not in the list fails.
//
// Widening the list is then a change somebody reads, which is the whole point.
func TestCanonicalFieldsAreRecorded(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		model, canonical any
		prefix           string
	}{
		{dbmeta.Column{}, canonicalColumn{}, "Column."},
		{dbmeta.ConstraintColumn{}, canonicalConstraint{}, "ConstraintColumn."},
	} {
		kept := map[string]bool{}
		for f := range reflect.TypeOf(c.canonical).Fields() {
			kept[f.Name] = true
		}
		for f := range reflect.TypeOf(c.model).Fields() {
			name := f.Name
			if kept[name] {
				continue
			}
			if _, ok := canonicalFields[c.prefix+name]; ok {
				continue
			}
			// a field every model spells the same way, dropped for every kind
			if _, ok := canonicalFields["*."+name]; ok {
				continue
			}
			t.Errorf("%s%s is dropped by the canonical projection and is not in "+
				"canonicalFields.\nAdd it with the reason, so that narrowing the "+
				"comparison is a change somebody reads.", c.prefix, name)
		}
	}
	// and nothing in the list that no longer exists
	for name := range canonicalFields {
		if strings.HasPrefix(name, "*.") || strings.Contains(name, "Has") {
			continue
		}
		parts := strings.SplitN(name, ".", 2)
		var mv reflect.Type
		switch parts[0] {
		case "Column":
			mv = reflect.TypeFor[dbmeta.Column]()
		case "Constraint":
			mv = reflect.TypeFor[dbmeta.Constraint]()
		case "ConstraintColumn":
			mv = reflect.TypeFor[dbmeta.ConstraintColumn]()
		default:
			t.Errorf("canonicalFields names %q and there is no such type", name)
			continue
		}
		if _, ok := mv.FieldByName(parts[1]); !ok {
			t.Errorf("canonicalFields names %q and there is no such field. "+
				"A stale entry hides a field that is now dropped silently.", name)
		}
	}
}

// TestConformanceAgreementHolds reports how much the databases agree and fails
// when they agree less than they did.
//
// The number is not a target. It is a ratchet: a change that makes two
// databases disagree about something they used to agree on is either a fault or
// a fact worth writing down, and this makes somebody decide which.
func TestConformanceAgreementHolds(t *testing.T) {
	t.Parallel()
	sections := readGolden(t)
	if len(sections) < 2 {
		t.Skip("fewer than two databases recorded")
	}
	all := make([]string, 0, len(sections))
	relational := make([]string, 0, len(sections))
	for n := range sections {
		// A release's own section is that product again, not another
		// database to agree with.
		if strings.Contains(n, "@") {
			continue
		}
		all = append(all, n)
		if _, out := agreementExcluded[n]; !out {
			relational = append(relational, n)
		}
	}
	sort.Strings(all)
	sort.Strings(relational)

	// Measured when this was written, with four relational databases. Raise
	// it when they agree on more. Lower it only with a reason.
	const relationalFloor = 23
	got := agreedLines(sections, relational)
	if len(got) < relationalFloor {
		t.Errorf("the relational databases agree on %d lines and used to agree on %d.\n"+
			"Something that was uniform is not any more. Find out whether it is a "+
			"fault or a fact, and either fix it or lower the floor and say why.",
			len(got), relationalFloor)
	}
	t.Logf("%d of the canonical lines are identical across %v", len(got), relational)

	// And the same question with every database, which is a smaller number
	// and a different fact. See agreementExcluded.
	// 6 with Cassandra and 4 once ClickHouse joined, which is what a database
	// with no constraint catalog costs. Lower it only with a reason.
	const everyFloor = 4
	every := agreedLines(sections, all)
	if len(every) < everyFloor {
		t.Errorf("every database agrees on %d lines and used to agree on %d",
			len(every), everyFloor)
	}
	t.Logf("%d are identical across %v", len(every), all)
}

// agreementExcluded names the databases left out of the main agreement count,
// with the reason each is out.
//
// The number exists to notice a database that stops agreeing with the others.
// A database that was never going to agree, because the product has no such
// object, only drags the floor down until it notices nothing. Counting these
// two takes the relational agreement from 23 lines to 14 and the whole set
// from 6 to 4.
//
// Neither is a fault and both are in docs/COVERAGE.md. They are still checked
// against their own recorded sections, which is where a real regression in
// either shows.
var agreementExcluded = map[string]string{
	"cosmos": "not relational: a document has no declared attribute, so there is no column" +
		" catalog, and a container has no key, no constraint and no view that a statement" +
		" reads, so the section holds the four containers alone (D228)",
	"athena": "no constraint of any kind: a Glue table has no key, no unique constraint and no" +
		" check, so the section has no constraint line. Every column is nullable and has no" +
		" default, because a Glue column has neither, so the key columns read nullable where" +
		" PostgreSQL reads not null (D222)",
	"bigquery": "no unique constraint and no check constraint: BigQuery has a primary key and a" +
		" foreign key only, so the lines for the unique title and the check of book are missing." +
		" The key columns of the core tables have no default, because a BigQuery key is a plain" +
		" column and an identity column reports no default. The columns of a view are nullable," +
		" where MySQL says they are not (D220)",
	"databricks": "no unique constraint and no check constraint in the catalog: Databricks has a" +
		" primary key and a foreign key only, and a CHECK constraint of Delta is a table property" +
		" that no relation lists, so the lines for the unique title and the check of book are" +
		" missing. INFORMATION_SCHEMA reports no default, so author.shade reads none. The columns" +
		" of a view read not nullable where they come from a NOT NULL column, as Snowflake's do" +
		" (D224)",
	"spanner": "no unique constraint: Spanner keeps a unique key as a unique index, so the" +
		" unique title of book is an index and the section has no line for it. The other" +
		" lines agree, and the check constraint has no columns to list (D216)",
	"influxdb": "not relational: a measurement has no key, no constraint and no view, and" +
		" every one has a time column, so the section holds the four measurements and" +
		" their tags and fields alone",
	"arangodb": "not relational: a collection's columns are the properties of its" +
		" schema rule, which has no key, so no column reads a primary key, AQL lists" +
		" no view, and a rule is one check with no columns behind it, so there are no" +
		" constraint lines",
	"drill": "not relational: a table has no key and no constraint, and the view recent has" +
		" columns of type ANY that are always nullable, so no column reads a primary key," +
		" there are no constraint lines, and the type names are Drill's own (D178)",
	"druid": "not relational: a datasource has no key, no constraint and no view, and every" +
		" one has the column __time, so no column reads a primary key, there are no" +
		" constraint lines, and the section holds the four datasources and their columns" +
		" alone (D171)",
	"elasticsearch": "not relational: an index has no key, no constraint and no NOT NULL," +
		" so no column reads a primary key or NOT NULL and there are no constraint lines," +
		" and SYS COLUMNS sorts the fields of a mapping by name, so the ordinal is the" +
		" position in that order. The section holds the four indices, the alias and their" +
		" fields alone (D177)",
	"opensearch": "not relational: an index has no key, no constraint and no NOT NULL, so no" +
		" column reads a primary key or NOT NULL and there are no constraint lines, SQL cannot" +
		" tell the alias from an index so there is no view, DESCRIBE lists an object and a" +
		" nested field as columns, and the driver cannot read DESCRIBE on 2.19.6 so the" +
		" columns are those of 3.9.0. The section holds the four indices and their fields" +
		" alone (D181)",
	"snowflake": "an IDENTITY column has no default in the catalog, so author_id reads no" +
		" default where PostgreSQL's serial column reads one. A view column reads as not" +
		" nullable where it comes from a NOT NULL table column, so the view recent reads" +
		" not nullable where PostgreSQL reads nullable (D193). The keys agree since D203" +
		" read them through SHOW",
	"solr": "not relational: a collection has no key, no constraint and no view in SQL, every" +
		" column reads nullable and none reads a primary key, and every collection has the" +
		" columns _version_, _root_, _text_, _nest_path_, _query_, score and id, so the" +
		" section holds the four collections and their columns alone (D179)",
	"influxql": "not relational, for the same reason as influxdb: a measurement has" +
		" no key, no constraint and no view, and every one has a time column, so the" +
		" section holds the four measurements and their tags and fields alone (D165)",
	"cassandra": "not relational: its Tables query returns no view, because a" +
		" materialized view is in another catalog table and CQL has no UNION," +
		" and its column ordinal is a position within the primary key because" +
		" the catalog keeps no declaration order",
	"couchbase": "not relational: a document has no fixed shape, so there is" +
		" no column catalog and no constraint, and the section holds the four" +
		" collections alone",
	"clickhouse": "no constraint catalog: there is no primary key constraint," +
		" no foreign key and no unique constraint, and system.constraints holds" +
		" the expression a CHECK asserts rather than the columns behind it, so" +
		" the section has no constraint lines at all",
	"cratedb": "no foreign key and no unique constraint: CrateDB has neither" +
		" on any release, so the two foreign keys and the unique constraint on" +
		" book.title the relational databases agree on cannot be built. A check" +
		" constraint is in pg_constraint with no columns behind it in" +
		" key_column_usage, so the check on book.title has no line either",
	"databend": "no key of any kind: Databend has no primary key, no foreign" +
		" key and no unique constraint on any release, so no column reads a key" +
		" where the relational databases agree, and the one constraint line is" +
		" the CHECK on book.title",
	"exasol": "no unique constraint: Exasol refuses UNIQUE and CHECK as not" +
		" supported on every release, so the unique constraint on book.title" +
		" the relational databases agree on cannot be built. It agrees on the" +
		" other 22 lines, and it reports each NOT NULL as a named constraint" +
		" of its own, which is how Exasol keeps them",
	"impala": "no constraint Impala lists: a primary key and a foreign key are" +
		" information Impala keeps and SHOW does not list, and a Parquet table" +
		" refuses NOT NULL, so every column reads nullable and none a key",
	"neo4j": "not relational: a label has no column catalog, because the only list of its" +
		" properties reads every node, and the report compares no constraint where it" +
		" has no column, so the section holds the four labels alone. There is no" +
		" foreign key and no view",
	"presto": "a query engine rather than a store, which is the same reason as" +
		" trino. It also keeps no NOT NULL, because its memory connector refuses" +
		" one on the newest release there is, so every column reads nullable" +
		" where the relational databases agree",
	"questdb": "no constraint and no NOT NULL: QuestDB has no primary key, no" +
		" foreign key, no unique constraint, no check and no NOT NULL on any" +
		" release, so every column reads nullable and none a key where the" +
		" relational databases agree, and there are no constraint lines",
	"singlestore": "no foreign key: SingleStore refuses one on every release," +
		" so the two foreign keys the relational databases agree on cannot be" +
		" built. It agrees on the other 21 lines",
	"ydb": "no column catalog and no view: a .sys view lists the tables, and" +
		" the columns of each are in its schema, which only a gRPC call per" +
		" table reads, and no .sys view says which paths are views, so the" +
		" section holds the four tables alone (D161)",
	"surrealdb": "no primary key column and no foreign key: the record id is the key of" +
		" every table and no DEFINE FIELD names it, a composite key is a UNIQUE index," +
		" and a link to another table is a field of the type record<t> that the server" +
		" does not check. The ordinal of a column is its place in the order of the" +
		" names, because SurrealDB records no position (D164)",
	"trino": "a query engine rather than a store: it has no constraint of any" +
		" kind at any release, so every column reads primary_key=false where" +
		" the relational databases agree on the key, and there are no" +
		" constraint lines to compare",
}

// agreedLines returns the lines every named section has.
func agreedLines(sections map[string][]string, names []string) map[string]bool {
	if len(names) == 0 {
		return map[string]bool{}
	}
	common := map[string]bool{}
	for _, l := range sections[names[0]] {
		common[l] = true
	}
	for _, n := range names[1:] {
		here := map[string]bool{}
		for _, l := range sections[n] {
			here[l] = true
		}
		for l := range common {
			if !here[l] {
				delete(common, l)
			}
		}
	}
	return common
}
