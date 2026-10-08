// Package influxdb holds the metadata queries for InfluxDB 3.
//
// Import it for its effect. It registers what InfluxDB 3 provides, and the
// root package answers for it afterwards:
//
//	import _ "github.com/xo/dbmeta/models/influxdb"
//
// InfluxDB 3 answers SQL with Apache DataFusion, and DataFusion keeps an
// information_schema: tables, columns, schemata, routines, parameters and
// df_settings. The model reads it through dbimp's influxdb driver, which is
// what dburl's influxdb scheme opens and what usql uses. InfluxQL is another
// dialect, influxql, and this model does not answer it. See D152.
//
// It answers 9 of the 65 questions, on 3.10.6, 3.11.6 and 3.12.0. A
// measurement is a table in the schema iox, its tags and fields are columns,
// and the functions are DataFusion's own. InfluxDB 3 has no view a user can
// make, no index, no constraint, no user a statement can list, and no
// comment. A processing engine trigger is a trigger (D170), and docs/COVERAGE.md says why each of the rest is not answered.
//
// The version is the release of DataFusion, which version() returns and every
// statement depends on. No SQL statement names the InfluxDB release, which
// only GET /ping reports.
package influxdb

import (
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
)

func init() {
	dbmeta.RegisterDialect(dbmeta.InfluxDB, &dbmeta.Info{
		// usql's influxdb driver sets no lexer flags, and the fold is
		// measured by scanEveryQuery (D143). DataFusion folds a name that is
		// not quoted to lower case, as PostgreSQL does.
		Fold:           dbmeta.FoldLower,
		Placeholder:    func(n int) string { return "$" + strconv.Itoa(n) },
		VersionQuery:   `SELECT version()`,
		VersionColumns: 1,
		ParseVersion:   parseVersion,
	})
	register()
}

// parseVersion reads what version() returns, such as "Apache DataFusion
// 51.0.0, x86_64 on linux".
func parseVersion(cols []string) (dbmeta.VersionSet, error) {
	if len(cols) < 1 {
		return dbmeta.VersionSet{}, dbmeta.ErrInvalidVersion
	}
	_, rest, ok := strings.Cut(cols[0], "DataFusion ")
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
	set.Display = "InfluxDB 3, which runs Apache DataFusion " + release
	return set, nil
}
