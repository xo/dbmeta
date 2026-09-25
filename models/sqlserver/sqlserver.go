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
// Thirty two of the 55, which is second only to PostgreSQL. SQL Server is the
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
		VersionQuery:   versionQuery,
		VersionColumns: 5,
		ParseVersion:   parseVersion,

		// No QuotingQuery: T-SQL has no setting that changes how a literal is
		// escaped, and a backslash is an ordinary character. Verified on
		// 2022, where LEN('a\b') is 3.
		ChangePassword: changePassword,
	})
	registerRelations()
	registerServer()
}

// parseVersion reads what SERVERPROPERTY('ProductVersion') returns, such as
// "16.0.4295.3". @@VERSION is not used: it is a sentence rather than a
// version, and it differs by language setting.
// versionQuery reads the five things a person wants to see about a SQL Server,
// in one statement.
//
// Four of them are server properties and the fifth is not. No SERVERPROPERTY
// returns the name the product is sold under: ProductMajorVersion says 16 and
// nothing says 2022. Only @@VERSION carries it, in its first words, so the
// name is cut from there rather than kept in a table of major numbers to
// years. A table would need an edit for every release Microsoft ships, and
// this does not.
//
// The cut is guarded. @@VERSION reads "Microsoft SQL Server 2022 (RTM-CU27)
// ..." and the name is everything before the parenthesis, so a banner without
// one would ask LEFT for -1 characters and raise an error. NULLIF turns that
// into a NULL instead, which arrives as empty and is left out of the line.
//
// productupdatelevel is the CU number. SERVERPROPERTY returns NULL for a
// property it does not know rather than failing, so this is safe on a release
// older than the one that added it.
const versionQuery = `SELECT LEFT(@@VERSION, NULLIF(CHARINDEX('(', @@VERSION), 0) - 1)
, CAST(SERVERPROPERTY('productversion') AS nvarchar(128))
, CAST(SERVERPROPERTY('productlevel') AS nvarchar(128))
, CAST(SERVERPROPERTY('productupdatelevel') AS nvarchar(128))
, CAST(SERVERPROPERTY('edition') AS nvarchar(128))`

// productName cuts the name the product is sold under out of the @@VERSION
// banner, which versionQuery has already trimmed at the first parenthesis.
//
// That trim is enough when the banner names a service pack, because the first
// parenthesis is then right after the year:
//
//	Microsoft SQL Server 2016 (SP2) (KB4052908) - 13.0.5026.0 (X64)
//
// It is not enough on a release with no service pack, where the first
// parenthesis is (X64) and comes after the build:
//
//	Microsoft SQL Server 2014 - 12.0.2000.8 (X64)
//
// so the trim keeps the build and the display then prints it twice. Stopping
// at " - " as well fixes that and changes nothing for the other form. A real
// 2014 found this, and no container could have: every SQL Server with a Linux
// container ships a cumulative update and names it.
func productName(banner string) string {
	name := strings.TrimSpace(banner)
	if i := strings.Index(name, " - "); i >= 0 {
		name = name[:i]
	}
	return strings.TrimSpace(name)
}

// parseVersion builds the version to gate on and the line to show a person.
//
// Only the second column gates anything. The rest are for the display line,
// which reads
//
//	Microsoft SQL Server 2022 16.0.4295.3, RTM-CU27, Developer Edition (64-bit)
//
// Every part after the version is left out when the server did not report it,
// so an older release that has no update level reads "RTM" rather than
// "RTM-".
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	if len(cols) < 5 {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	var (
		name    = productName(cols[0])
		raw     = strings.TrimSpace(cols[1])
		level   = strings.TrimSpace(cols[2])
		update  = strings.TrimSpace(cols[3])
		edition = strings.TrimSpace(cols[4])
	)
	if raw == "" {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	ver := dbmeta.ParseVersion(raw)
	ver.Raw = raw
	var set dbmeta.VersionSet
	set.Set("", ver)

	// The product name already begins "Microsoft SQL Server", so it replaces
	// the prefix rather than following it.
	if name == "" {
		name = "Microsoft SQL Server"
	}
	if update != "" {
		level += "-" + update
	}
	parts := []string{name + " " + raw}
	for _, p := range []string{level, edition} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	set.Display = strings.Join(parts, ", ")
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
func always(query string) dbmeta.Choice { return dbmeta.Choice{{Query: query}} }

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

// changePassword builds ALTER LOGIN ... WITH PASSWORD.
//
// A login is an identifier in brackets and the password is a string literal,
// prefixed N so that a password outside the code page survives. T-SQL doubles
// the quote and has no backslash escape, so this needs no session state and
// takes a zero Quoting.
//
// OLD_PASSWORD is added only when the caller supplies one. A login changing
// its own password needs it and a member of the server role does not, and
// which of those applies is the caller's to know.
func changePassword(c dbmeta.PasswordChange, q dbmeta.Quoting) (string, error) {
	stmt := "ALTER LOGIN " + dbmeta.QuoteIdentifier(c.User, "[", "]") +
		" WITH PASSWORD = N" + dbmeta.QuoteLiteral(c.Password, q)
	if c.Old != "" {
		stmt += " OLD_PASSWORD = N" + dbmeta.QuoteLiteral(c.Old, q)
	}
	return stmt, nil
}
