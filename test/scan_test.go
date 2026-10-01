package test

import (
	"database/sql"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
)

// scanEveryQuery reads every query that m supports through the query's own
// Scan, and reports each one that fails.
//
// A smoke test that reads column names cannot find the fault this finds: a
// NULL arriving in a field that cannot hold one. That fault shipped in the
// CockroachDB model, where the databases and tablespaces queries failed on
// every row and every test passed. So each query is read with the system
// objects included, which gives it the most rows there are to scan.
//
// Every query is named here once. A query the model supports and this does
// not name fails the test, so a new kind cannot be left out.
func scanEveryQuery(t *testing.T, m *dbmeta.Meta, db *sql.DB) {
	t.Helper()
	scanEveryQueryWith(t, m, db, true)
}

// scanEveryQueryWith is scanEveryQuery, with the system objects included only
// when system is true.
func scanEveryQueryWith(t *testing.T, m *dbmeta.Meta, db *sql.DB, system bool) {
	t.Helper()
	all := dbmeta.Args{WithSystem: system}.Map()
	drains := map[string]func() (int, error){
		dbmeta.Tables.Name():                  scanOne(t, all, dbmeta.Tables, m, db),
		dbmeta.Schemas.Name():                 scanOne(t, all, dbmeta.Schemas, m, db),
		dbmeta.Columns.Name():                 scanOne(t, all, dbmeta.Columns, m, db),
		dbmeta.Indexes.Name():                 scanOne(t, all, dbmeta.Indexes, m, db),
		dbmeta.Databases.Name():               scanOne(t, all, dbmeta.Databases, m, db),
		dbmeta.Tablespaces.Name():             scanOne(t, all, dbmeta.Tablespaces, m, db),
		dbmeta.AccessMethods.Name():           scanOne(t, all, dbmeta.AccessMethods, m, db),
		dbmeta.Languages.Name():               scanOne(t, all, dbmeta.Languages, m, db),
		dbmeta.Conversions.Name():             scanOne(t, all, dbmeta.Conversions, m, db),
		dbmeta.Casts.Name():                   scanOne(t, all, dbmeta.Casts, m, db),
		dbmeta.Collations.Name():              scanOne(t, all, dbmeta.Collations, m, db),
		dbmeta.LargeObjects.Name():            scanOne(t, all, dbmeta.LargeObjects, m, db),
		dbmeta.EventTriggers.Name():           scanOne(t, all, dbmeta.EventTriggers, m, db),
		dbmeta.Settings.Name():                scanOne(t, all, dbmeta.Settings, m, db),
		dbmeta.Functions.Name():               scanOne(t, all, dbmeta.Functions, m, db),
		dbmeta.Aggregates.Name():              scanOne(t, all, dbmeta.Aggregates, m, db),
		dbmeta.Types.Name():                   scanOne(t, all, dbmeta.Types, m, db),
		dbmeta.Domains.Name():                 scanOne(t, all, dbmeta.Domains, m, db),
		dbmeta.Operators.Name():               scanOne(t, all, dbmeta.Operators, m, db),
		dbmeta.Roles.Name():                   scanOne(t, all, dbmeta.Roles, m, db),
		dbmeta.RoleSettings.Name():            scanOne(t, all, dbmeta.RoleSettings, m, db),
		dbmeta.RoleGrants.Name():              scanOne(t, all, dbmeta.RoleGrants, m, db),
		dbmeta.Privileges.Name():              scanOne(t, all, dbmeta.Privileges, m, db),
		dbmeta.DefaultACLs.Name():             scanOne(t, all, dbmeta.DefaultACLs, m, db),
		dbmeta.ForeignDataWrappers.Name():     scanOne(t, all, dbmeta.ForeignDataWrappers, m, db),
		dbmeta.ForeignServers.Name():          scanOne(t, all, dbmeta.ForeignServers, m, db),
		dbmeta.UserMappings.Name():            scanOne(t, all, dbmeta.UserMappings, m, db),
		dbmeta.ForeignTables.Name():           scanOne(t, all, dbmeta.ForeignTables, m, db),
		dbmeta.Publications.Name():            scanOne(t, all, dbmeta.Publications, m, db),
		dbmeta.PublicationTables.Name():       scanOne(t, all, dbmeta.PublicationTables, m, db),
		dbmeta.Subscriptions.Name():           scanOne(t, all, dbmeta.Subscriptions, m, db),
		dbmeta.TextSearchParsers.Name():       scanOne(t, all, dbmeta.TextSearchParsers, m, db),
		dbmeta.TextSearchDictionaries.Name():  scanOne(t, all, dbmeta.TextSearchDictionaries, m, db),
		dbmeta.TextSearchTemplates.Name():     scanOne(t, all, dbmeta.TextSearchTemplates, m, db),
		dbmeta.TextSearchConfigs.Name():       scanOne(t, all, dbmeta.TextSearchConfigs, m, db),
		dbmeta.TextSearchConfigMaps.Name():    scanOne(t, all, dbmeta.TextSearchConfigMaps, m, db),
		dbmeta.OperatorClasses.Name():         scanOne(t, all, dbmeta.OperatorClasses, m, db),
		dbmeta.OperatorFamilies.Name():        scanOne(t, all, dbmeta.OperatorFamilies, m, db),
		dbmeta.OperatorFamilyOperators.Name(): scanOne(t, all, dbmeta.OperatorFamilyOperators, m, db),
		dbmeta.OperatorFamilyFunctions.Name(): scanOne(t, all, dbmeta.OperatorFamilyFunctions, m, db),
		dbmeta.Extensions.Name():              scanOne(t, all, dbmeta.Extensions, m, db),
		dbmeta.ExtensionObjects.Name():        scanOne(t, all, dbmeta.ExtensionObjects, m, db),
		dbmeta.ExtendedStats.Name():           scanOne(t, all, dbmeta.ExtendedStats, m, db),
		dbmeta.Comments.Name():                scanOne(t, all, dbmeta.Comments, m, db),
		dbmeta.IndexColumns.Name():            scanOne(t, all, dbmeta.IndexColumns, m, db),
		dbmeta.Constraints.Name():             scanOne(t, all, dbmeta.Constraints, m, db),
		dbmeta.Triggers.Name():                scanOne(t, all, dbmeta.Triggers, m, db),
		dbmeta.Sequences.Name():               scanOne(t, all, dbmeta.Sequences, m, db),
		dbmeta.PartitionedTables.Name():       scanOne(t, all, dbmeta.PartitionedTables, m, db),
		dbmeta.ConstraintColumns.Name():       scanOne(t, all, dbmeta.ConstraintColumns, m, db),
		dbmeta.RoutineParameters.Name():       scanOne(t, all, dbmeta.RoutineParameters, m, db),
		dbmeta.EnumValues.Name():              scanOne(t, all, dbmeta.EnumValues, m, db),
		dbmeta.Views.Name():                   scanOne(t, all, dbmeta.Views, m, db),
		dbmeta.ColumnStats.Name():             scanOne(t, all, dbmeta.ColumnStats, m, db),
		dbmeta.CurrentSchema.Name():           scanOne(t, all, dbmeta.CurrentSchema, m, db),
		dbmeta.CurrentUser.Name():             scanOne(t, all, dbmeta.CurrentUser, m, db),
	}
	var read int
	for _, q := range dbmeta.Queries() {
		if q.Support(m) != dbmeta.Supported {
			continue
		}
		drain, ok := drains[q.Name()]
		if !ok {
			t.Errorf("%s: no scan for it here. Add it to scanEveryQuery", q.Name())
			continue
		}
		n, err := drain()
		if err != nil {
			t.Errorf("%s: %v", q.Name(), err)
			continue
		}
		read++
		t.Logf("%-26s %d rows", q.Name(), n)
	}
	t.Logf("%d queries read through their Scan", read)
	if dbmeta.Tables.Support(m) == dbmeta.Supported {
		checkTypes(t, m, db, system)
	}
	checkFold(t, m, db)
}

// foldQueries select one row with the unquoted alias DbMeta_Fold, for a
// product that needs more than SELECT 1 AS DbMeta_Fold.
var foldQueries = map[dbmeta.Dialect]string{
	dbmeta.Oracle:    `SELECT 1 AS DbMeta_Fold FROM dual`,
	dbmeta.Firebird:  `SELECT 1 AS DbMeta_Fold FROM rdb$database`,
	dbmeta.HANA:      `SELECT 1 AS DbMeta_Fold FROM DUMMY`,
	dbmeta.Cassandra: `SELECT key AS DbMeta_Fold FROM system.local`,
	dbmeta.Neo4j:     `RETURN 1 AS DbMeta_Fold`,
	// AQL has no SELECT, and an object's keys are the columns.
	dbmeta.ArangoDB: `RETURN {DbMeta_Fold: 1}`,
	// InfluxQL selects from a measurement, and the fixture makes author.
	dbmeta.InfluxQL:  `SELECT rating AS DbMeta_Fold FROM author`,
	dbmeta.SurrealDB: `SELECT 1 AS DbMeta_Fold FROM ONLY {}`,
}

// foldColumns is the position of the alias in the answer, for a product
// whose answer has more columns than the one the query names. dbimp's
// influxdb driver puts measurement first, and InfluxQL puts time before the
// rest (dbimp D81).
var foldColumns = map[dbmeta.Dialect]int{
	dbmeta.InfluxQL: 2,
}

// foldTables are the products whose alias does not say how a name is
// stored. Trino and Presto return an alias as it is written and store a
// table's name in lower case, measured on 476, 483 and 0.299. For each, the
// fold is measured by making a table, in the memory catalog their fixtures
// use, and reading back the name the catalog holds.
var foldTables = map[dbmeta.Dialect]string{
	dbmeta.Trino:  "memory.dbmeta_fixture",
	dbmeta.Presto: "memory.dbmeta_fixture",
}

// checkFoldByTable makes the table DbMeta_Fold in schema and reads back the
// name the catalog holds.
func checkFoldByTable(t *testing.T, m *dbmeta.Meta, db *sql.DB, schema string) {
	t.Helper()
	ctx := t.Context()
	cleanup(t, db, `DROP TABLE IF EXISTS `+schema+`.DbMeta_Fold`)
	exec(t, db, `CREATE TABLE `+schema+`.DbMeta_Fold (a integer)`)
	t.Cleanup(func() { cleanup(t, db, `DROP TABLE IF EXISTS `+schema+`.DbMeta_Fold`) })
	catalog, name, _ := strings.Cut(schema, ".")
	var got string
	//nolint:gosec // the catalog and the schema are the test's own
	err := db.QueryRowContext(ctx, `SELECT table_name FROM `+catalog+`.information_schema.tables`+
		` WHERE table_schema = '`+name+`' AND lower(table_name) = 'dbmeta_fold'`).Scan(&got)
	if err != nil {
		t.Errorf("fold: reading the table back: %v", err)
		return
	}
	if want := m.Dialect().FoldIdentifier("DbMeta_Fold"); got != want {
		t.Errorf("fold: the product stores DbMeta_Fold as %q, and FoldIdentifier says %q", got, want)
	}
	t.Logf("fold: DbMeta_Fold is %s", got)
}

// checkFold measures what the product does to the case of a name that is
// not quoted, and checks that Dialect.FoldIdentifier says the same. Most
// products report an alias as they store a name, so the name of the column
// the product returns is the fold. See D143.
func checkFold(t *testing.T, m *dbmeta.Meta, db *sql.DB) {
	t.Helper()
	if schema, ok := foldTables[m.Dialect()]; ok {
		checkFoldByTable(t, m, db, schema)
		return
	}
	q, ok := foldQueries[m.Dialect()]
	if !ok {
		q = `SELECT 1 AS DbMeta_Fold`
	}
	rows, err := db.QueryContext(t.Context(), q)
	if err != nil {
		t.Errorf("fold: %v", err)
		return
	}
	defer rows.Close()
	cols, err := rows.Columns()
	at := foldColumns[m.Dialect()]
	if err != nil || len(cols) != at+1 {
		t.Errorf("fold: expected %d columns, got %v, %v", at+1, cols, err)
		return
	}
	for rows.Next() {
	}
	if err := rows.Err(); err != nil {
		t.Errorf("fold: reading the row: %v", err)
		return
	}
	if want := m.Dialect().FoldIdentifier("DbMeta_Fold"); cols[at] != want {
		t.Errorf("fold: the product stores DbMeta_Fold as %q, and FoldIdentifier says %q", cols[at], want)
	}
	t.Logf("fold: DbMeta_Fold is %s", cols[at])
}

// checkTypes checks that the types parameter of Tables returns exactly the
// rows of the types it names. It reads every table once, and then each type
// on its own and the first two together. See D138.
//
// A model that cannot filter declares the parameter and narrows nothing, as
// Cassandra does, and this skips it rather than failing.
func checkTypes(t *testing.T, m *dbmeta.Meta, db *sql.DB, system bool) {
	t.Helper()
	ctx := t.Context()
	params, err := dbmeta.Tables.Params(m)
	if err != nil {
		t.Errorf("tables: reading the parameters: %v", err)
		return
	}
	read := func(types []string) (map[string]int, error) {
		// Only the parameters Tables takes, because a query refuses one it
		// does not know.
		all := dbmeta.Args{WithSystem: system, Types: types}.Map()
		args := map[string]any{}
		for _, p := range params {
			if v, ok := all[p.Name]; ok {
				args[p.Name] = v
			}
		}
		counts := map[string]int{}
		for v, err := range dbmeta.Tables.All(ctx, m, db, args) {
			if err != nil {
				return nil, err
			}
			counts[v.Type]++
		}
		return counts, nil
	}
	every, err := read(nil)
	if err != nil {
		t.Errorf("tables: %v", err)
		return
	}
	if m.Dialect() == dbmeta.Cassandra {
		t.Log("types: Cassandra narrows nothing, so it is not checked")
		return
	}
	// matches says whether a read narrowed to types holds exactly those
	// types, as many of each as every read.
	matches := func(got, every map[string]int, types []string) bool {
		if len(got) != len(types) {
			return false
		}
		for _, name := range types {
			if got[name] != every[name] {
				return false
			}
		}
		return true
	}
	// narrowed reads the tables of types and compares them with every read.
	// A catalog can change between two reads: HANA's statistics service
	// makes its views for a minute after the server first answers, and a
	// server shared with another session gains that session's tables. So a
	// read that disagrees is taken again once with a fresh every read, and
	// only a disagreement that stays is a fault.
	narrowed := func(types []string) {
		got, err := read(types)
		if err != nil {
			t.Errorf("tables of types %v: %v", types, err)
			return
		}
		if matches(got, every, types) {
			return
		}
		again, err := read(nil)
		if err != nil {
			t.Errorf("tables: %v", err)
			return
		}
		if got, err = read(types); err != nil {
			t.Errorf("tables of types %v: %v", types, err)
			return
		}
		if !matches(got, again, types) {
			t.Errorf("tables of types %v: expected only those, as many as %v, got %v", types, again, got)
		}
	}
	names := slices.Sorted(maps.Keys(every))
	for _, name := range names {
		narrowed([]string{name})
	}
	if len(names) >= 2 {
		narrowed(names[:2])
	}
	// A type no model reports matches nothing.
	if got, err := read([]string{"no such type"}); err != nil || len(got) != 0 {
		t.Errorf("tables of a type that does not exist: expected none, got %v, %v", got, err)
	}
	t.Logf("types: %v", every)
}

// scanOne returns a function that reads every row of q under the filter all,
// and counts them. The filter carries only the parameters the query takes,
// because a query refuses one it does not know.
func scanOne[T any](t *testing.T, all map[string]any, q *dbmeta.Query[T], m *dbmeta.Meta, db *sql.DB) func() (int, error) {
	t.Helper()
	return func() (int, error) {
		params, err := q.Params(m)
		if err != nil {
			return 0, err
		}
		args := map[string]any{}
		for _, p := range params {
			if v, ok := all[p.Name]; ok {
				args[p.Name] = v
			}
		}
		n := 0
		for _, err := range q.All(t.Context(), m, db, args) {
			if err != nil {
				return n, err
			}
			n++
		}
		return n, nil
	}
}

// TestEveryModelScansEveryQuery runs scanEveryQuery against each server that
// is running. The PostgreSQL family, SQLite, CrateDB, Cassandra and Couchbase
// have tests of their own, because each opens in a way of its own.
func TestEveryModelScansEveryQuery(t *testing.T) {
	for _, c := range []struct {
		name  string
		open  func(*testing.T) *sql.DB
		setup func(*testing.T, *sql.DB) *dbmeta.Meta
	}{
		{"mysql", openMySQL, setupMySQL},
		{"duckdb", openDuckDB, setupDuckDB},
		{"sqlserver", openSQLServer, setupSQLServer},
		{"oracle", openOracle, setupOracle},
		{"clickhouse", openClickHouse, setupClickHouse},
		{"trino", openTrino, setupTrino},
		{"presto", openPresto, setupPresto},
		{"firebird", openFirebird, setupFirebird},
		{"hana", openHANA, setupHANA},
		{"hive", openHive, setupHive},
		{"exasol", openExasol, setupExasol},
		{"vertica", openVertica, setupVertica},
		{"ydb", openYDB, setupYDB},
	} {
		t.Run(c.name, func(t *testing.T) {
			db := c.open(t)
			m := c.setup(t, db)
			// Oracle 11g reads all_objects and all_types slowly, 196 and
			// 126 seconds each over the whole catalog, measured on
			// 2026-09-30, so tables and types with the system objects pass
			// the test timeout. 18c reads them in seconds. So 11g is scanned
			// without them. docs/COVERAGE.md has the timings (D150).
			if c.name == "oracle" && m.Version().Main().Compare(dbmeta.V(12)) < 0 {
				t.Log("scanned without the system objects, which Oracle 11g reads too slowly")
				scanEveryQueryWith(t, m, db, false)
				return
			}
			scanEveryQuery(t, m, db)
		})
	}
}

// TestSQLiteScansEveryQuery runs scanEveryQuery on each SQLite driver.
func TestSQLiteScansEveryQuery(t *testing.T) {
	eachSQLite(t, func(t *testing.T, db *sql.DB, m *dbmeta.Meta) {
		scanEveryQuery(t, m, db)
	})
}
