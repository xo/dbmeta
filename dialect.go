package dbmeta

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
)

// Dialect names one database family. It is the `dburl` driver name, so
// `dburl.URL.Driver` selects a dialect directly.
//
// A dialect names the family and never the release. A release arrives as a
// [VersionSet] value, because D8 resolves version differences at run time. See
// [New].
type Dialect string

// Dialect values. One per database family.
//
// The name is what the database calls itself. The value is the `dburl` driver
// name, which is not always the same word. PostgreSQL calls itself PostgreSQL
// and its driver is postgres.
const (
	Cassandra  Dialect = "cassandra"
	DuckDB     Dialect = "duckdb"
	MySQL      Dialect = "mysql"
	Oracle     Dialect = "oracle"
	PostgreSQL Dialect = "postgres"
	SQLite3    Dialect = "sqlite3"
	SQLServer  Dialect = "sqlserver"
)

// Info is what a model declares about its database.
type Info struct {
	// Placeholder writes the bind parameter for position n, counting from 1.
	// PostgreSQL writes $1, MySQL writes ?, Oracle writes :1.
	Placeholder func(n int) string
	// VersionSQL reads the server version. An empty string means the database
	// reports no version, and a caller uses an unknown version.
	VersionSQL string
	// VersionColumns is how many columns VersionSQL returns. SQL Server and
	// Cassandra return three.
	VersionColumns int
	// ParseVersion turns the columns of the first row into a version set and a
	// display line.
	ParseVersion func(cols []string) (VersionSet, error)
}

var (
	infoMu sync.RWMutex
	infos  = make(map[Dialect]*Info)
)

// RegisterDialect records what a model declares about its database. A model
// file in internal calls this from its init. Registering twice panics, the way
// [database/sql.Register] does, because two models for one dialect is a build
// mistake rather than a run time condition.
func RegisterDialect(d Dialect, info *Info) {
	infoMu.Lock()
	defer infoMu.Unlock()
	if _, ok := infos[d]; ok {
		panic("dbmeta: dialect registered twice: " + string(d))
	}
	infos[d] = info
}

// Dialects returns every dialect built into this binary, which depends on the
// build tags. See D31.
func Dialects() []Dialect {
	infoMu.RLock()
	defer infoMu.RUnlock()
	out := make([]Dialect, 0, len(infos))
	for d := range infos {
		out = append(out, d)
	}
	return out
}

// Info returns what d declares. The second result is false when no model for d
// was built into this binary, which is not the same as the database being
// unable to answer. See [ErrModelNotBuilt].
func (d Dialect) Info() (*Info, bool) {
	infoMu.RLock()
	defer infoMu.RUnlock()
	info, ok := infos[d]
	return info, ok
}

// VersionQuery returns the statement that reads the server version, and how
// many columns it returns.
//
// dbmeta cannot run it. The caller runs it as its first statement, scans the
// columns as strings, and passes them to [Dialect.ParseVersion]. See D38.
//
// The second result is false when the model was not built or the database
// reports no version.
func (d Dialect) VersionQuery() (sql string, cols int, ok bool) {
	info, found := d.Info()
	if !found || info.VersionSQL == "" {
		return "", 0, false
	}
	return info.VersionSQL, info.VersionColumns, true
}

// ParseVersion turns the columns returned by [Dialect.VersionQuery] into a
// version set carrying both the comparable version and the line to show a
// person.
func (d Dialect) ParseVersion(cols []string) (VersionSet, error) {
	info, ok := d.Info()
	if !ok {
		return VersionSet{}, ErrModelNotBuilt
	}
	if info.ParseVersion == nil {
		return VersionSet{Display: "unknown"}, nil
	}
	return info.ParseVersion(cols)
}

// Version runs the version statement against db and returns the parsed
// result.
//
// It is the two steps above done together, because every caller needs both and
// doing them by hand means building a slice of pointers into a slice of
// strings. A caller that wants the statement without running it, to print it
// or to run it its own way, uses [Dialect.VersionQuery] and
// [Dialect.ParseVersion] instead.
//
// This does not detect anything on the caller's behalf. The caller decides
// whether to call it, and is free to build a [VersionSet] by hand and pass
// that to [New] instead. Overriding the version is the point of D36 and this
// method does not take it away.
//
// A database that reports no version returns an unknown version and no error.
// An unknown version selects the newest fragment of every piece.
func (d Dialect) Version(ctx context.Context, db Querier) (VersionSet, error) {
	sqlstr, n, ok := d.VersionQuery()
	if !ok {
		if _, built := d.Info(); !built {
			return VersionSet{}, ErrModelNotBuilt
		}
		return VersionSet{Display: "unknown"}, nil
	}
	// QueryContext rather than QueryRowContext, so that Querier needs one
	// method. The cost is closing the rows by hand, which is four lines.
	rows, err := db.QueryContext(ctx, sqlstr)
	if err != nil {
		return VersionSet{}, fmt.Errorf("reading the version of %s: %w", d, err)
	}
	defer rows.Close()
	cols := make([]string, n)
	dest := make([]any, n)
	for i := range cols {
		dest[i] = &cols[i]
	}
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return VersionSet{}, fmt.Errorf("reading the version of %s: %w", d, err)
		}
		return VersionSet{}, fmt.Errorf("reading the version of %s: %w", d, sql.ErrNoRows)
	}
	if err := rows.Scan(dest...); err != nil {
		return VersionSet{}, fmt.Errorf("reading the version of %s: %w", d, err)
	}
	if err := rows.Err(); err != nil {
		return VersionSet{}, fmt.Errorf("reading the version of %s: %w", d, err)
	}
	if err := rows.Close(); err != nil {
		return VersionSet{}, fmt.Errorf("reading the version of %s: %w", d, err)
	}
	return d.ParseVersion(cols)
}

// Meta is what a caller holds. It names the dialect and the server version,
// and it decides which fragment of a query applies.
//
// The caller builds it. dbmeta never detects a dialect and never detects a
// version. See D36.
type Meta struct {
	dialect  Dialect
	versions VersionSet
}

// New returns the Meta for a dialect at a version.
//
// Pass the version the caller read with [Dialect.VersionQuery], or any version
// the caller wants to force. Overriding matters for a proxy that hides the
// server, for a compatible product that reports a version it does not behave
// like, for a generator with no server at all, and for finding out whether a
// fault is a version gate.
//
// An unknown version selects the newest fragment of every piece, which is what
// D21 requires above the ceiling.
func New(d Dialect, versions VersionSet) (*Meta, error) {
	if _, ok := d.Info(); !ok {
		return nil, ErrModelNotBuilt
	}
	return &Meta{dialect: d, versions: versions}, nil
}

// Dialect returns the dialect m was built for.
func (m *Meta) Dialect() Dialect { return m.dialect }

// String satisfies the [fmt.Stringer] interface. It returns the line a person
// should see, such as "PostgreSQL 16.2", which is what usql prints for
// \conninfo and in its prompt.
func (m *Meta) String() string {
	if s := m.versions.String(); s != "" && s != "unknown" {
		return s
	}
	return string(m.dialect)
}

// Version returns the version set m was built with. Call [VersionSet.Main] for
// the one number most callers want.
//
// The two accessors mirror the arguments of [New], so there is nothing to
// remember about their names.
func (m *Meta) Version() VersionSet { return m.versions }
