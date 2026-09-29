package test

import (
	"database/sql"
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
	} {
		t.Run(c.name, func(t *testing.T) {
			db := c.open(t)
			m := c.setup(t, db)
			// Oracle 11g reads its whole catalog slowly: tables, types,
			// privileges and column_stats each took more than a minute with
			// the system objects included, measured on 2026-09-29, and the
			// scan passed the test timeout. 18c reads it in seconds. So 11g
			// is scanned without them. docs/COVERAGE.md has the timings.
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
