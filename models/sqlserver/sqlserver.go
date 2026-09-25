// Package sqlserver holds the metadata queries for Microsoft SQL Server.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/sqlserver"
//
// # The sys schema, not information_schema
//
// SQL Server ships both. The sys views carry what information_schema cannot:
// whether an index is unique, what a foreign key points at, where a table
// lives, and the identity of every object. Microsoft's own documentation says
// to prefer them, and D9 says to prefer a native catalog anyway.
//
// # Visibility rather than refusal
//
// A SQL Server catalog view shows the caller what the caller may see, and
// returns fewer rows otherwise. It does not refuse.
//
// That matters for three queries here. ColumnStats reads
// sys.dm_db_stats_properties, which needs VIEW STATISTICS; a caller without it
// gets no rows rather than an error. UserMappings reads sys.linked_logins,
// which needs a server level permission, and behaves the same way.
// ForeignTables reads sys.external_tables, which exists in every install and
// holds nothing until PolyBase is configured.
//
// All three were run as a user with VIEW DEFINITION alone and all three
// returned an empty result rather than failing. That is worth knowing, because
// PostgreSQL shows a caller everything and MariaDB refuses outright, so a
// consumer sees three different behaviours for the same lack of privilege.
//
// # What it answers
//
// Thirty one of the 54, which is second only to PostgreSQL. SQL Server is the
// only database here besides PostgreSQL with roles, privileges, tablespaces
// and DDL triggers, and the only one with a catalog of comments rather than a
// comment on each object.
//
// See docs/COVERAGE.md for what it cannot answer and for the analogues that
// were rejected.
package sqlserver

import (
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
)

// Reference is the SQL Server release these queries were written against.
const Reference = "16.0.4295.3"

// Releases a fragment gates on. SQL Server reports four numbers and the major
// moves with the product year: 11 is 2012, 13 is 2016, 14 is 2017, 15 is 2019,
// 16 is 2022 and 17 is 2025.
//
// Both gates sit below the oldest release that can be tested, because
// Microsoft shipped SQL Server on Linux from 2017 and there is no container
// for anything earlier. They are written from the documentation and reviewed,
// not run, and D54 says so rather than letting the tier table imply otherwise.
var (
	// sys.sequences and sys.dm_db_stats_properties arrived in 2012.
	v11 = dbmeta.V(11)
	// sys.external_tables, sys.tables.is_external and
	// sys.tables.temporal_type arrived in 2016.
	v13 = dbmeta.V(13)
)

func init() {
	dbmeta.RegisterDialect(dbmeta.SQLServer, &dbmeta.Info{
		Placeholder:    func(n int) string { return "@p" + strconv.Itoa(n) },
		VersionSQL:     `SELECT CAST(SERVERPROPERTY('ProductVersion') AS nvarchar(128))`,
		VersionColumns: 1,
		ParseVersion:   parseVersion,
	})
	registerRelations()
	registerServer()
}

// parseVersion reads what SERVERPROPERTY('ProductVersion') returns, such as
// "16.0.4295.3". @@VERSION is not used: it is a sentence rather than a
// version, and it differs by language setting.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	if len(cols) == 0 {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	raw := strings.TrimSpace(cols[0])
	ver := dbmeta.ParseVersion(raw)
	ver.Raw = raw
	var set dbmeta.VersionSet
	set.Set("", ver)
	set.Display = "Microsoft SQL Server " + raw
	return set, nil
}

// systemSchemas are the schemas SQL Server keeps for itself. sys and
// INFORMATION_SCHEMA hold the catalog, and the rest are the fixed database
// roles, which exist as schemas in every database.
const systemSchemas = `'sys', 'INFORMATION_SCHEMA', 'guest', 'db_owner', ` +
	`'db_accessadmin', 'db_securityadmin', 'db_ddladmin', 'db_backupoperator', ` +
	`'db_datareader', 'db_datawriter', 'db_denydatareader', 'db_denydatawriter'`

// notSystem excludes them unless the caller asked. Every query aliases the
// schema view s, so the alias is fixed rather than a parameter.
//
// The comparison against 1 is not decoration. T-SQL has no boolean type, so a
// bit parameter cannot stand alone as a condition: "WHERE @with_system OR ..."
// is rejected with "an expression of non-boolean type specified in a context
// where a condition is expected". Every other model here writes the bare
// parameter and every one of those would fail on SQL Server.
const notSystem = `(@with_system = 1 OR s.name NOT IN (` + systemSchemas + `))`

func schemaNameSystem(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
		{Name: "with_system", Desc: "include the objects SQL Server keeps for itself", Default: false},
	}
}

func schemaParentName(kind string) []dbmeta.Param {
	return append([]dbmeta.Param{
		{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
	}, schemaNameSystem(kind)...)
}

// always wraps SQL that is the same on every release.
func always(sqlstr string) dbmeta.Choice { return dbmeta.Choice{{SQL: sqlstr}} }

// comment reaches the MS_Description extended property of an object, which is
// where SQL Server keeps what every other database calls a comment.
//
// class 1 is an object or a column, and minor_id 0 means the object itself.
// SQL Server has no COMMENT ON, so this is set with sp_addextendedproperty and
// a caller that never ran it sees nothing, which is a fact rather than a gap.
const objectComment = `(SELECT CAST(p.value AS nvarchar(max)) FROM sys.extended_properties p` +
	` WHERE p.class = 1 AND p.major_id = %s AND p.minor_id = 0 AND p.name = 'MS_Description')`

// commentOn returns that subquery for one object id expression.
func commentOn(id string) string {
	return strings.Replace(objectComment, "%s", id, 1)
}

// columnComment is the same for a column, where minor_id is the column id.
func columnComment(objectID, columnID string) string {
	return `(SELECT CAST(p.value AS nvarchar(max)) FROM sys.extended_properties p` +
		` WHERE p.class = 1 AND p.major_id = ` + objectID +
		` AND p.minor_id = ` + columnID + ` AND p.name = 'MS_Description')`
}
