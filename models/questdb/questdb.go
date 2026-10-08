// Package questdb holds the metadata queries for QuestDB.
//
// Import it for its effect. It registers what QuestDB provides, and the root
// package answers for it afterwards:
//
//	import _ "github.com/xo/dbmeta/models/questdb"
//
// QuestDB speaks PostgreSQL's protocol on its port 8812, and pgx reaches it,
// as dburl's questdb scheme opens it from v0.36.0. Its catalog is its own. The
// functions tables(), views(), materialized_views() and functions() describe
// what it holds, and a small information_schema and pg_catalog imitate
// PostgreSQL's for the clients that read them. The model reads whichever of
// the two answers each question in one statement, and shares nothing with the
// postgres model, because that model's statements read catalogs and functions
// QuestDB does not have.
//
// It answers 11 of the 61 questions, on 9.4.3 and 10.0.1. QuestDB has no
// constraint, no index a statement can list, no role in the open source
// edition, no comment, no trigger and no sequence, and docs/COVERAGE.md says
// why each of the rest is not answered.
package questdb

import (
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
)

// versionQuery reads QuestDB's own release. SHOW server_version and version()
// both answer PostgreSQL 12.3 on every release, and only build() names
// QuestDB's, as "Build Information: QuestDB 10.0.1, JDK 25.0.2, Commit Hash
// ...".
const versionQuery = `SELECT build()`

func init() {
	dbmeta.RegisterDialect(dbmeta.QuestDB, &dbmeta.Info{
		// The syntax is usql's lexer flags for this product, and the fold
		// is measured by scanEveryQuery (D143).
		Syntax:         dbmeta.Syntax{DollarQuotes: true, BlockComments: true},
		Placeholder:    func(n int) string { return "$" + strconv.Itoa(n) },
		VersionQuery:   versionQuery,
		VersionColumns: 1,
		ParseVersion:   parseVersion,
	})
	register()
}

// parseVersion reads the release from the text build() returns.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	if len(cols) < 1 {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	_, rest, ok := strings.Cut(cols[0], "QuestDB ")
	if !ok {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	release, _, _ := strings.Cut(rest, ",")
	release = strings.TrimSpace(release)
	v := dbmeta.ParseVersion(release)
	if v.Unknown {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	v.Raw = release
	var set dbmeta.VersionSet
	set.Set("", v)
	set.Display = "QuestDB " + release
	return set, nil
}
