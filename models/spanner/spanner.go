// Package spanner holds the metadata queries for Google Cloud Spanner, in the
// GoogleSQL dialect.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/spanner"
//
// # The dialect
//
// A Spanner database speaks GoogleSQL or the PostgreSQL dialect, and
// DATABASE_OPTIONS says which one (database_dialect). This model reads the
// GoogleSQL one only. A database in the PostgreSQL dialect has the catalog of
// PostgreSQL, which models/postgres reads.
//
// # What it reads
//
// Spanner answers INFORMATION_SCHEMA, and that is the whole catalog. It was
// written against Spanner Omni 2026.r4-lts, the engine of the hosted service as
// a container, and measured again on Cloud Spanner, which answers the same 47
// views. See D216 and D219.
//
// The default schema is named by the empty string. A named schema, made with
// CREATE SCHEMA, has a name. INFORMATION_SCHEMA and SPANNER_SYS are the two
// schemas that Spanner keeps for itself, and a query hides them unless the
// caller sets with_system. An empty schema pattern means every schema, so no
// pattern selects the default schema alone.
//
// # What it answers
//
// 22 of the 65. Schemas, tables, columns, indexes, index columns, constraints,
// constraint columns, not null constraints, views, sequences, functions,
// routine parameters, roles, role grants, privileges, column privileges,
// settings, the current schema, the current user, publications, publication tables and
// tablespaces.
//
// Three of those are analogues, and docs/COVERAGE.md says why each is one. A
// change stream is a publication, with its tables and its columns. A locality
// group is a tablespace, because it says where the data of a table lives. A
// named NOT NULL check constraint that Spanner makes for every NOT NULL column
// is a not null constraint, and Constraints leaves those out the way psql 18
// does.
//
// The rest is absent from Spanner. It has no trigger, no rule, no policy, no
// partition, no extension, no foreign data wrapper, no comment and no type
// catalog. Databases has no answer, because no function returns the name of the
// database. CurrentUser is answered by Cloud Spanner and refused by Spanner
// Omni, which fails SESSION_USER with an unknown user name when it has no
// authentication.
//
// # The version
//
// Spanner has no version function, and Spanner Omni reports its release only to
// its own program. The one number that SQL reads and that moves with the
// service is the highest optimizer version in
// SPANNER_SYS.SUPPORTED_OPTIMIZER_VERSIONS, so the model uses that as the
// version. It gates nothing today. See D216.
//
// # Roles
//
// A database role is a name that grants attach to. It cannot log in, so
// Roles reports can_login false for each of them. A session names the role it
// uses with the property database_role, and Cloud Spanner and Spanner Omni both
// enforce it, so what a session reads depends on the role it names. The parity
// test records that.
package spanner

import (
	"strings"

	"github.com/xo/dbmeta"
)

// Reference is the Spanner Omni release these queries were written against.
const Reference = "2026.r4-lts"

// versionQuery reads the highest optimizer version the server supports.
const versionQuery = `SELECT CAST(MAX(version) AS STRING) AS version FROM spanner_sys.supported_optimizer_versions`

// parseVersion reads the one column versionQuery returns, such as 9.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	var s dbmeta.VersionSet
	if len(cols) != 1 {
		return s, dbmeta.ErrInvalidVersion
	}
	raw := strings.TrimSpace(cols[0])
	ver := dbmeta.ParseVersion(raw)
	ver.Raw = raw
	s.Set("", ver)
	s.Display = "Spanner optimizer " + raw
	return s, nil
}

func init() {
	dbmeta.RegisterDialect(dbmeta.Spanner, &dbmeta.Info{
		// GoogleSQL has block comments and hash comments, and quotes a name
		// with backticks. A name keeps its case and is compared without it.
		Syntax: dbmeta.Syntax{BlockComments: true, HashComments: true, Backticks: true},
		// go-sql-spanner turns a ? into @p1, @p2 and so on.
		Placeholder:    func(int) string { return "?" },
		VersionQuery:   versionQuery,
		VersionColumns: 1,
		ParseVersion:   parseVersion,
	})
	registerRelations()
	registerDetail()
	registerRoles()
	registerExtra()
}

// always is a fragment every release takes.
func always(query string) dbmeta.Choice { return dbmeta.Choice{{Query: query}} }

// like is the filter that matches col to the pattern in param, and passes when
// the pattern is empty.
func like(col, param string) string {
	return `(` + param + ` = '' OR ` + col + ` LIKE ` + param + `)`
}

// notSystem hides the two schemas Spanner keeps for itself, unless the caller
// asks for them. col is the schema column.
func notSystem(col string) string {
	return `(@with_system OR ` + col + ` NOT IN ('INFORMATION_SCHEMA', 'SPANNER_SYS'))`
}

// notNullPrefix starts the name of every check constraint that Spanner makes for
// a NOT NULL column. Spanner refuses a name of that form from a person, so the
// prefix tells a made constraint from a written one.
const notNullPrefix = `'CK_IS_NOT_NULL_'`

func schemaNameSystem(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{
			Name:    "schema",
			Desc:    "schema name pattern, empty for every schema. The default schema is named empty, so no pattern selects it alone",
			Default: "",
		},
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
		{Name: "with_system", Desc: "include INFORMATION_SCHEMA and SPANNER_SYS", Default: false},
	}
}

func schemaParentName(parent, kind string) []dbmeta.Param {
	return append([]dbmeta.Param{
		{Name: "parent", Desc: parent + " name pattern, empty for every " + parent, Default: ""},
	}, schemaNameSystem(kind)...)
}

func nameOnly(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
	}
}
