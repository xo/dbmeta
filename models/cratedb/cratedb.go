// Package cratedb holds the metadata queries for CrateDB.
//
// Import it for its effect. It registers what CrateDB provides, and the root
// package answers for it afterwards:
//
//	import _ "github.com/xo/dbmeta/models/cratedb"
//
// CrateDB speaks PostgreSQL's protocol on its port 5432, and pgx reaches it.
// Its catalog is its own: information_schema, sys and a pg_catalog that holds
// part of PostgreSQL's. Where one of the postgres model's statements answers
// correctly, this model shares it with [dbmeta.Query.Share], and elsewhere it
// reads CrateDB's own catalog. It imports the postgres model, which registers
// first. See D123.
//
// It answers 26 of the 56 questions on 6.4.5 and 25 on 6.3.7, which has no
// collation view. 3 are the postgres model's statements and 23 are its own.
// The other 30 are objects CrateDB does not have, or has in a form that is
// not the one the question asks about. docs/COVERAGE.md says which, and what
// each answer lacks.
package cratedb

import (
	"strings"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/postgres"
)

// Release is the version key of CrateDB's own release, such as 6.4.5. A
// fragment of this model's own gates on it.
//
// The main version is the PostgreSQL release that CrateDB claims to be, which
// is what server_version answers, 14.0 on 6.3 and 6.4. The statements shared
// from the postgres model gate on it.
const Release = "cratedb"

func init() {
	pg, ok := dbmeta.PostgreSQL.Info()
	if !ok {
		panic("dbmeta: the cratedb model needs the postgres model registered first")
	}
	info := *pg
	info.VersionQuery = `SELECT pg_catalog.version(), pg_catalog.current_setting('server_version')`
	info.VersionColumns = 2
	info.ParseVersion = parseVersion
	info.ChangePassword = changePassword
	dbmeta.RegisterDialect(dbmeta.CrateDB, &info)
	register()
}

// parseVersion reads CrateDB's own release from version(), such as "CrateDB
// 6.4.5 (built 1b58099/NA, Linux ...)", and the PostgreSQL release it claims
// from server_version.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	if len(cols) < 2 {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	banner, claimed := strings.TrimSpace(cols[0]), strings.TrimSpace(cols[1])
	rest, ok := strings.CutPrefix(banner, "CrateDB ")
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
	set.Display = "CrateDB " + release + ", which claims PostgreSQL " + claimed
	return set, nil
}

// changePassword builds ALTER USER ... SET (password = ...), which is how
// CrateDB sets a password. It has no old password clause and ignores
// PasswordChange.Old.
func changePassword(c dbmeta.PasswordChange, q dbmeta.Quoting) (string, error) {
	if !q.BackslashEscapes.Valid {
		return "", dbmeta.ErrQuotingUnknown
	}
	return "ALTER USER " + dbmeta.QuoteIdentifier(c.User, `"`, `"`) +
		" SET (password = " + dbmeta.QuoteLiteral(c.Password, q) + ")", nil
}

// share registers the postgres model's binding of q for CrateDB.
func share[T any](q *dbmeta.Query[T]) {
	if !q.Share(dbmeta.PostgreSQL, dbmeta.CrateDB) {
		panic("dbmeta: the postgres model registers no " + q.Name())
	}
}

// register shares each statement of the postgres model that answers on
// CrateDB, and registers CrateDB's own statement for the rest.
func register() {
	registerOwn()
	share(dbmeta.Settings)
	share(dbmeta.RoleGrants)
	share(dbmeta.CurrentUser)
}
