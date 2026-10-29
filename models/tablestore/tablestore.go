// Package tablestore holds the metadata queries for Alibaba Cloud Tablestore,
// through its SQL feature.
//
// Import it for its effect:
//
//	import _ "github.com/xo/dbmeta/models/tablestore"
//
// # What it reads
//
// It reads the hosted service. Tablestore is a wide column store, and its SQL
// feature is a front end that speaks the MySQL 5.7 dialect over tables that a
// person maps with CREATE TABLE. It has no INFORMATION_SCHEMA and no system
// table. SHOW DATABASES, SHOW VARIABLES, SHOW STATUS, SHOW GRANTS, SHOW
// FUNCTIONS and the other SHOW statements of MySQL answer an error, and so do
// version(), user() and database().
//
// Three statements answer, and each names one table. DESCRIBE, SHOW CREATE TABLE
// and SHOW INDEX read the columns, the definition and the indexes of a table that
// the caller names. A SELECT cannot filter or join them, so a read of every
// column needs one statement for each table. That is a walk. D146 allowed a walk
// for Impala, InfluxQL, Elasticsearch and OpenSearch, and D242 allows it for
// Tablestore, because its SQL has no catalog that a SELECT reads. The model walks
// DESCRIBE and SHOW INDEX. It does not read SHOW CREATE TABLE, which holds no
// fact that those two lack except the collation of the table.
//
// One statement names no table. SHOW FULL TABLES lists the mapped tables of the
// instance and says BASE TABLE for each. The model reads it for Tables, and the
// instance is the only namespace, so a table has no catalog and no schema.
//
// # What it answers
//
// 4 of the 65. Tables, Columns, Indexes and IndexColumns.
//
// Tables is one statement. SHOW FULL TABLES takes a LIKE pattern, and it takes no
// other filter that a statement can bind, so the statement has none and the Keep
// function of the binding matches the rows in Go, as the Cassandra model does.
// See D200 and D241. The tables of an instance are as many as a person made, and
// the statement returns one row for each.
//
// Columns walks SHOW FULL TABLES and then one DESCRIBE for each table that the
// parent pattern keeps. Indexes and IndexColumns walk SHOW FULL TABLES and then
// one SHOW INDEX for each such table, which lists the key, each secondary index
// and each search index. The patterns are matched in Go with dbmeta.Like, because
// a SHOW statement binds none. A table that is dropped between the list and its
// statement is skipped, and any other error reaches the caller. The cost was
// measured with the four tables of the fixture only. No instance with thousands
// of tables was available, so the check that D47 asks for has not been run, and
// the cost grows with the number of tables that the parent pattern keeps. See
// D242.
//
// A secondary index and a search index are not tables, and SHOW FULL TABLES does
// not list them. A table that exists in Tablestore and has no mapping is not
// listed either, because SQL cannot see it. A person maps it with CREATE TABLE,
// and the mapping is what the model reads.
//
// # The version
//
// Tablestore has no release number. The SQL front end answers SELECT @@version
// with a text such as 5.7.25-SQLFrontend-b30fece-dirty. The number is the level
// of MySQL that the front end claims and the rest is its build. The model reads
// the text and keeps it whole, and it gates no statement on the number. usql runs
// SELECT version(), which the service refuses. See D241.
//
// # The driver
//
// The driver that dburl v0.53.0 names is
// github.com/aliyun/aliyun-tablestore-go-sql-driver, and it registers ots. It
// signs each request with the key pair in the DSN, which dburl writes with the
// scheme https. It binds a parameter on the client, writing a backslash before a
// quote, and it ignores a context. See D241.
package tablestore

import (
	"fmt"
	"strings"

	"github.com/xo/dbmeta"
)

// versionQuery reads the text that the SQL front end reports. It is the only
// place the service says what it is.
const versionQuery = "SELECT @@version"

func init() {
	dbmeta.RegisterDialect(dbmeta.Tablestore, &dbmeta.Info{
		// The front end reads block comments and quotes a name with backticks.
		Syntax: dbmeta.Syntax{BlockComments: true, Backticks: true},
		// A table name is case sensitive, so nothing folds one.
		Fold: dbmeta.FoldNone,
		// The driver binds on the client, with a question mark.
		Placeholder:    func(int) string { return "?" },
		VersionQuery:   versionQuery,
		VersionColumns: 1,
		ParseVersion:   parseVersion,
	})
	registerRelations()
	registerWalks()
}

// parseVersion reads the text of @@version, such as
// 5.7.25-SQLFrontend-b30fece-dirty.
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	if len(cols) < 1 {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	text := strings.TrimSpace(cols[0])
	v := dbmeta.ParseVersion(text)
	if v.Unknown {
		return dbmeta.VersionSet{}, fmt.Errorf("tablestore: %w: %q", dbmeta.ErrInvalidVersion, text)
	}
	v.Raw = text
	var set dbmeta.VersionSet
	set.Set("", v)
	set.Display = "Tablestore SQL " + text
	return set, nil
}
