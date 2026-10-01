// Package cockroachdb holds the metadata queries for CockroachDB.
//
// Import it for its effect. It registers what CockroachDB provides, and the
// root package answers for it afterwards:
//
//	import _ "github.com/xo/dbmeta/models/cockroachdb"
//
// CockroachDB speaks PostgreSQL's protocol and imitates PostgreSQL's catalog,
// pg_catalog and information_schema. So most of its answers are the postgres
// model's statements, shared with [dbmeta.Query.Share], and the model writes
// a statement of its own only where CockroachDB's catalog differs. It imports
// the postgres model, which registers first. See D123.
//
// It answers 54 of the 56 questions, on every release measured: 24.3.36,
// 26.2.7 and 26.3.2. 47 are the postgres model's statements and 7 are its
// own. It does not answer column_stats, because CockroachDB keeps pg_stats
// empty, or text_search_config_maps, because it has no text search
// configuration. docs/COVERAGE.md says what each answer lacks.
package cockroachdb

import (
	"strings"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/postgres"
)

// Release is the version key of CockroachDB's own release, such as 26.3.2. A
// fragment written for one CockroachDB release names it.
//
// The main version is the PostgreSQL release that CockroachDB claims to be,
// which is what server_version answers: 13.0.0 on 24.3 and 26.2, and 18.0.0 on
// 26.3. The statements shared from the postgres model gate on the main
// version, so each release of CockroachDB takes the fragments of the
// PostgreSQL release whose catalog it says it imitates. That is what was
// measured to run, and a fragment of this model's own gates on Release.
const Release = "cockroachdb"

func init() {
	pg, ok := dbmeta.PostgreSQL.Info()
	if !ok {
		panic("dbmeta: the cockroachdb model needs the postgres model registered first")
	}
	info := *pg
	info.VersionQuery = `SELECT pg_catalog.version(), pg_catalog.current_setting('server_version')`
	info.VersionColumns = 2
	info.ParseVersion = parseVersion
	dbmeta.RegisterDialect(dbmeta.CockroachDB, &info)
	register()
}

// parseVersion reads CockroachDB's own release from version(), such as
// "CockroachDB CCL v26.3.2 (x86_64-pc-linux-gnu, built 2026/09/16 12:26:01,
// go1.26.6)", and the PostgreSQL release it claims from server_version.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	if len(cols) < 2 {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	banner, claimed := strings.TrimSpace(cols[0]), strings.TrimSpace(cols[1])
	if !strings.HasPrefix(banner, "CockroachDB") {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	_, rest, ok := strings.Cut(banner, " v")
	if !ok {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	release, _, _ := strings.Cut(rest, " ")
	own := dbmeta.ParseVersion(release)
	own.Raw = release
	pg := dbmeta.ParseVersion(claimed)
	pg.Raw = claimed
	var set dbmeta.VersionSet
	set.Set("", pg)
	set.Set(Release, own)
	set.Display = "CockroachDB " + release + ", which claims PostgreSQL " + claimed
	return set, nil
}

// share registers the postgres model's binding of q for CockroachDB.
func share[T any](q *dbmeta.Query[T]) {
	if !q.Share(dbmeta.PostgreSQL, dbmeta.CockroachDB) {
		panic("dbmeta: the postgres model registers no " + q.Name())
	}
}

// register shares every statement of the postgres model that CockroachDB
// answers, and registers CockroachDB's own statement for the rest.
func register() {
	registerOwn()
	share(dbmeta.Tables)
	share(dbmeta.Schemas)
	share(dbmeta.Columns)
	share(dbmeta.Indexes)
	share(dbmeta.AccessMethods)
	share(dbmeta.Languages)
	share(dbmeta.Conversions)
	share(dbmeta.Casts)
	share(dbmeta.Collations)
	share(dbmeta.LargeObjects)
	share(dbmeta.EventTriggers)
	share(dbmeta.Settings)
	share(dbmeta.Functions)
	share(dbmeta.Aggregates)
	share(dbmeta.Types)
	share(dbmeta.Domains)
	share(dbmeta.Operators)
	share(dbmeta.Roles)
	share(dbmeta.RoleSettings)
	share(dbmeta.RoleGrants)
	share(dbmeta.Privileges)
	share(dbmeta.DefaultACLs)
	share(dbmeta.ForeignDataWrappers)
	share(dbmeta.ForeignServers)
	share(dbmeta.UserMappings)
	share(dbmeta.ForeignTables)
	share(dbmeta.Publications)
	share(dbmeta.PublicationTables)
	share(dbmeta.Subscriptions)
	share(dbmeta.TextSearchParsers)
	share(dbmeta.TextSearchDictionaries)
	share(dbmeta.TextSearchTemplates)
	share(dbmeta.TextSearchConfigs)
	// TextSearchConfigMaps is not shared, so it is not supported.
	// CockroachDB has no ts_token_type, which names a token, and keeps
	// pg_ts_config empty, so there is no configuration to map. See D147.
	share(dbmeta.OperatorClasses)
	share(dbmeta.OperatorFamilies)
	share(dbmeta.OperatorFamilyFunctions)
	share(dbmeta.Extensions)
	share(dbmeta.Comments)
	share(dbmeta.Constraints)
	share(dbmeta.Sequences)
	share(dbmeta.PartitionedTables)
	share(dbmeta.ConstraintColumns)
	share(dbmeta.RoutineParameters)
	share(dbmeta.EnumValues)
	share(dbmeta.Views)
	// ColumnStats is not shared, so it is not supported. CockroachDB keeps
	// pg_stats empty even after ANALYZE, so the postgres statement says
	// that no column has statistics, which is false. Its statistics are in
	// SHOW STATISTICS FOR TABLE, which takes the table in the statement and
	// not as a bind parameter, so no one statement reads a schema's worth.
	// system.table_statistics is refused from 26.3. See D123.
	share(dbmeta.CurrentSchema)
	share(dbmeta.CurrentUser)
}
