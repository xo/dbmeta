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
	ClickHouse Dialect = "clickhouse"
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
	// VersionQuery reads the server version. An empty string means the database
	// reports no version, and a caller uses an unknown version.
	VersionQuery string
	// VersionColumns is how many columns VersionQuery returns. SQL Server and
	// Cassandra return three.
	VersionColumns int
	// ParseVersion turns the columns of the first row into a version set and a
	// display line.
	ParseVersion func(cols []string) (VersionSet, error)

	// QuotingQuery reads the session state that decides how a string literal is
	// escaped. Empty when the product has no such state. See D56.
	QuotingQuery string
	// QuotingColumns is how many columns QuotingQuery returns.
	QuotingColumns int
	// ParseQuoting turns those columns into the state.
	ParseQuoting func(cols []string) (Quoting, error)
	// ChangePassword builds the statement that sets a password. It returns
	// text and never runs anything. Nil when the product has no such
	// statement, which is every embedded database here.
	ChangePassword func(c PasswordChange, q Quoting) (string, error)
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
func (d Dialect) VersionQuery() (query string, cols int, ok bool) {
	info, found := d.Info()
	if !found || info.VersionQuery == "" {
		return "", 0, false
	}
	return info.VersionQuery, info.VersionColumns, true
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
	query, n, ok := d.VersionQuery()
	if !ok {
		if _, built := d.Info(); !built {
			return VersionSet{}, ErrModelNotBuilt
		}
		return VersionSet{Display: "unknown"}, nil
	}
	cols, err := readRow(ctx, db, d, query, n)
	if err != nil {
		return VersionSet{}, err
	}
	return d.ParseVersion(cols)
}

// readRow runs a statement that returns one row of n columns and hands back
// the values as text.
//
// Every column is scanned as nullable and flattened to empty. A property the
// server does not have comes back NULL, and scanning that straight into a
// string is a hard error naming neither the column nor the dialect. SQL Server
// has one: productupdatelevel arrived after the oldest release supported here.
//
// Flattening a NULL is right for these two callers and nowhere else.
// docs/NULLS.md forbids it for a catalog column, where NULL and empty are two
// facts a caller has to tell apart. Here the result is a version line shown to
// a person or a session setting read as a word, and absent and empty mean the
// same thing to both.
func readRow(ctx context.Context, db Querier, d Dialect, query string, n int) ([]string, error) {
	// QueryContext rather than QueryRowContext, so that Querier needs one
	// method. The cost is closing the rows by hand, which is four lines.
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("reading from %s: %w", d, err)
	}
	defer rows.Close()
	scanned := make([]sql.Null[string], n)
	dest := make([]any, n)
	for i := range scanned {
		dest[i] = &scanned[i]
	}
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("reading from %s: %w", d, err)
		}
		return nil, fmt.Errorf("reading from %s: %w", d, sql.ErrNoRows)
	}
	if err := rows.Scan(dest...); err != nil {
		return nil, fmt.Errorf("reading from %s: %w", d, err)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading from %s: %w", d, err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("reading from %s: %w", d, err)
	}
	cols := make([]string, n)
	for i, v := range scanned {
		cols[i] = v.V
	}
	return cols, nil
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
