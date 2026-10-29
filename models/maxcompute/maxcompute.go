// Package maxcompute holds the metadata queries for Alibaba Cloud MaxCompute,
// in the SQL of MaxCompute.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/maxcompute"
//
// # What it reads
//
// It reads the hosted service through the project level INFORMATION_SCHEMA. The
// driver runs each statement as an SQL task and reads the result through the
// instance tunnel, so only a SELECT returns rows. SHOW, DESC and LIST FUNCTIONS
// fail with "it is not select statement", and no model reads them. The D146 walk
// does not help, because a walk is one statement after another. See D237.
//
// The views exist only after a person runs `install package
// Information_Schema.systables;` as the project owner or as a RAM user with the
// role super_administrator. The model reads TABLES, COLUMNS, PARTITIONS, UDFS,
// USERS, ROLES, USER_ROLES, TABLE_PRIVILEGES, UDF_PRIVILEGES and
// COLUMN_PRIVILEGES. The views show a new object after about an hour, and the
// documentation says up to three hours. See D240.
//
// A project that is not schema enabled has two levels. TABLES reports the catalog
// odps and puts the project in the column for the schema, so the model calls the
// project the schema and odps the database.
//
// # What it answers
//
// 15 of the 65. Databases, schemas, tables, columns, views, partitioned tables,
// partitions, comments, roles, role grants, privileges, column privileges,
// functions, the current schema and the current user.
//
// A view and a materialized view are both in Views. A function is a user defined
// function, and UDFS holds its name, its owner and its time and nothing else.
// TABLE_PRIVILEGES names users and never roles. The primary key of a transactional
// table is in DESC and in no view, so Column.PrimaryKey is always false. The
// statements order by a column name and never by a number, because MaxCompute
// reads `ORDER BY 1` as a constant and sorts nothing. docs/COVERAGE.md says what
// is not answered and why.
//
// # The driver writes values as they are
//
// The driver writes a parameter into the statement with no quote, so the dialect
// sets [dbmeta.Info.Literal], as Hive does, and the statements bind nothing. A
// statement must end with a semicolon, and the model ends each one with it. See
// D238.
//
// # The version
//
// `SELECT version();` answers a text of nine lines. The model reads the line
// ODPS_BUILD_NAME, such as V59.group_cn.SQL_RC60_hf9, and the number after the V,
// the sprint of the service, is the main version. See D237.
package maxcompute

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
)

// versionQuery reads the version text of the SQL engine. It is one string of
// many lines, and the statement must end with a semicolon. usql declares no
// version query for MaxCompute, so it runs the generic `SELECT version();`, which
// is this statement. See docs/USQL.md.
const versionQuery = `SELECT version();`

// buildName finds the name of the build in the text that version() returns.
var buildName = regexp.MustCompile(`(?m)^\s*ODPS_BUILD_NAME:\s*(\S+)`)

// sprint reads the number that follows the V in a build name, such as the 59 in
// V59.group_cn.SQL_RC60_hf9.
var sprint = regexp.MustCompile(`^V(\d+)\.`)

// parseVersion reads the one column that versionQuery returns.
//
// MaxCompute is a service, and version() reports the build of the SQL engine as
// a text of many lines. The line ODPS_BUILD_NAME names the build, such as
// V59.group_cn.SQL_RC60_hf9. The number after the V is the sprint of the
// service. The model reads it as the main version and gates no fragment on it,
// because no difference between two sprints is known. See D237.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	if len(cols) != 1 {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	m := buildName.FindStringSubmatch(cols[0])
	if m == nil {
		return dbmeta.VersionSet{}, fmt.Errorf("maxcompute: %w: the text has no ODPS_BUILD_NAME", dbmeta.ErrInvalidVersion)
	}
	build := m[1]
	var set dbmeta.VersionSet
	if s := sprint.FindStringSubmatch(build); s != nil {
		v := dbmeta.ParseVersion(s[1])
		v.Raw = build
		set.Set("", v)
	}
	set.Display = "MaxCompute " + build
	return set, nil
}

func init() {
	dbmeta.RegisterDialect(dbmeta.MaxCompute, &dbmeta.Info{
		// The language has block comments, hash comments are not measured, and a
		// name is quoted with backticks.
		Syntax: dbmeta.Syntax{BlockComments: true, Backticks: true},
		// The service refuses a statement that does not end with a semicolon.
		Terminator: dbmeta.TerminatorRequired,
		// An unquoted name is stored in lower case.
		Fold: dbmeta.FoldLower,
		// The driver binds by writing a value into the statement as it is, with no
		// quote, so the dialect writes each value as a literal itself and a query
		// here never calls Placeholder. A client that writes its own statement can
		// ask for one, and the form is a question mark.
		Placeholder:    func(int) string { return "?" },
		Literal:        literal,
		VersionQuery:   versionQuery,
		VersionColumns: 1,
		ParseVersion:   parseVersion,
	})
	registerRelations()
	registerExtra()
}

// literal renders one filter value as a MaxCompute literal.
//
// MaxCompute reads two quotes in a row as two literals that are joined, as Hive
// does, so 'a”b' is ab. It escapes with a backslash, so a backslash is escaped
// first and the quote second. The driver writes a value with no quote at all.
//
// It refuses a type it does not know and a string that holds a NUL.
func literal(v any) (string, error) {
	switch t := v.(type) {
	case string:
		if strings.ContainsRune(t, 0) {
			return "", fmt.Errorf("maxcompute: %w: a filter cannot hold a NUL", dbmeta.ErrInvalidParam)
		}
		t = strings.ReplaceAll(t, `\`, `\\`)
		t = strings.ReplaceAll(t, `'`, `\'`)
		return "'" + t + "'", nil
	case bool:
		if t {
			return "TRUE", nil
		}
		return "FALSE", nil
	case int:
		return strconv.Itoa(t), nil
	case int64:
		return strconv.FormatInt(t, 10), nil
	}
	return "", fmt.Errorf("maxcompute: %w: cannot render %T as a literal", dbmeta.ErrInvalidParam, v)
}

// always is a fragment every release takes.
func always(query string) dbmeta.Choice { return dbmeta.Choice{{Query: query}} }

// last is the last fragment of a statement. The service refuses a statement
// that does not end with a semicolon, so the semicolon is part of the text.
func last(query string) dbmeta.Choice { return always(query + ";") }

// like is the filter that matches col to the pattern in param, and passes when
// the pattern is empty.
func like(col, param string) string {
	return `(` + param + ` = '' OR ` + col + ` LIKE ` + param + `)`
}

func schemaName(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "project name pattern, empty for every project", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
	}
}

func schemaParentName(parent, kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "project name pattern, empty for every project", Default: ""},
		{Name: "parent", Desc: parent + " name pattern, empty for every " + parent, Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
	}
}

func nameOnly(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
	}
}
