// Package oracle holds the metadata queries for Oracle Database.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/oracle"
//
// # ALL_ rather than DBA_ or USER_
//
// Every query here reads the ALL_ views. Both reviews said the same thing and
// the reasoning is the one this project already knows from information_schema:
// ALL_ shows the connected user what it may see and needs no special role,
// DBA_ needs SELECT_CATALOG_ROLE that an ordinary application user does not
// have, and USER_ shows only the caller's own schema and has no OWNER column
// at all.
//
// The cost is the same one information_schema has. ALL_ silently drops a row
// the caller cannot see, so an unprivileged connection gets a smaller answer
// rather than an error. A consumer that needs the whole catalog connects as
// somebody who can see it.
//
// # A schema is a user
//
// Oracle has no schema separate from the user that owns it, so Schemas reads
// ALL_USERS and the OWNER column everywhere else is that same name.
//
// # What it answers
//
// Eleven of the 55, which is a start rather than a finish. Schemas, tables,
// columns, indexes, index columns, constraints, constraint columns,
// sequences, views, the current schema and the current user.
//
// See docs/COVERAGE.md for what is not written yet and why.
//
// # Versions
//
// Eleven years of releases, from 11g Release 2 to 26ai. The gates are written
// against what the dictionary has rather than what the marketing says, and the
// two do not line up: 26ai is version 23.26, so a release named for 2026
// reports 23 as its major.
package oracle

import (
	"strings"

	"github.com/xo/dbmeta"
)

// Reference is the Oracle release these queries were written against.
const Reference = "23.26.3.0.0"

// The releases a fragment will gate on, recorded here because the work of
// finding them is done and the queries that use them are not written yet.
//
// They are read off the banner, which is the one place every release from 11g
// up reports a version at all. product_component_version.version_full is more
// precise, does not exist before 18c, and buys nothing: no gate needs a patch
// level.
//
//	v12    12c brought the identity column, the CDB_ views and CON_ID, and
//	       widened an identifier from 30 bytes to 128
//	v18    18c added product_component_version.version_full
//	v21    21c reports a native JSON column type rather than a BLOB
//	v23    23ai brought domains, and the vector and boolean types
//	v26ai  26ai is version 23.26, which is why it is not V(26). It adds a
//	       dozen ALL_ views over 23ai and removes none
//
// They are written as constants rather than left in prose when the first query
// needs one. Declaring them now would be five unused variables.

func init() {
	dbmeta.RegisterDialect(dbmeta.Oracle, &dbmeta.Info{
		Placeholder:    func(n int) string { return ":" + itoa(n) },
		VersionSQL:     versionSQL,
		VersionColumns: 1,
		ParseVersion:   parseVersion,
	})
	registerRelations()
	registerColumns()
	registerExtra()
}

// versionSQL reads the banner.
//
// One statement has to serve every release, because Info.VersionSQL is a
// string rather than a gated statement, and this is the only thing that works
// on all of them. product_component_version has no version_full before 18c and
// v$version has no banner_full before 18c, so both of those are a parse error
// on 11g.
//
// The banner carries two things worth having. It names the release the way
// Oracle sells it, which no other source does, and its number separates 23ai
// from 26ai: they report 23.0.0.0.0 and 23.26.3.0.0.
//
// It reads a V$ view, which needs a privilege an ordinary user does not have.
// usql already requires that for Oracle, reading v$instance for the same
// purpose, so this asks for nothing new.
const versionSQL = `SELECT banner FROM v$version WHERE ROWNUM = 1`

// parseVersion pulls the version and the product name out of the banner.
//
//	Oracle Database 11g Express Edition Release 11.2.0.2.0 - 64bit Production
//	Oracle AI Database 26ai Free Release 23.26.3.0.0 - Develop, Learn, and Run for Free
//
// The number after "Release" is the version. Everything before it is the
// product, and it is kept whole rather than cut down, because it carries the
// edition as well as the name and a person reading a connection line wants
// both.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	if len(cols) == 0 {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	banner := strings.TrimSpace(cols[0])
	if banner == "" {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	// Everything after the last "Release " and before the next space.
	raw, name := "", banner
	if before, after, found := strings.CutLast(banner, "Release "); found {
		raw, _, _ = strings.Cut(strings.TrimSpace(after), " ")
		name = strings.TrimSpace(before)
	}
	if raw == "" {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	ver := dbmeta.ParseVersion(raw)
	ver.Raw = raw
	var set dbmeta.VersionSet
	set.Set("", ver)
	set.Display = name + " " + raw
	return set, nil
}

// itoa is strconv.Itoa for the small numbers a placeholder uses. It is here so
// that the package imports nothing it does not need.
func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}
