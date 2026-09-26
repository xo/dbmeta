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
	Firebird   Dialect = "firebirdsql"
	Hive       Dialect = "hive"
	HANA       Dialect = "hdb"
	MySQL      Dialect = "mysql"
	Oracle     Dialect = "oracle"
	Presto     Dialect = "presto"
	PostgreSQL Dialect = "postgres"
	SQLite3    Dialect = "sqlite3"
	SQLServer  Dialect = "sqlserver"
	Trino      Dialect = "trino"
)

// Info is what a model declares about its database.
type Info struct {
	// Embedded says the database is a library rather than a server.
	//
	// There is nothing to connect to over a network, nothing to start, no
	// release to pin beyond whatever the driver links, and no user to be. Two
	// are: SQLite and DuckDB.
	//
	// A consumer reads it to decide what is worth offering. There is no host
	// to report, no server version separate from the driver's, and no second
	// principal, so a command that asks about any of those has nothing to
	// say. dbmeta branches on it nowhere itself: every query runs the same way
	// against a library as against a server.
	//
	// Hard rule 1 would normally send a fact about a scheme to dburl, and
	// dburl does carry something adjacent: it marks file, sqlite3,
	// moderncsqlite, csvq, duckdb, chai and ql as Opaque. That is a statement
	// about how a URL parses rather than about the product, and dbmeta cannot
	// read it in any case, because dbmeta has no dependencies and never opens
	// a connection. A caller holding a Dialect has no URL to hand to dburl.
	//
	// So the fact is declared here, by the model, and it is the model that
	// knows it.
	Embedded bool

	// Placeholder writes the bind parameter for position n, counting from 1.
	// PostgreSQL writes $1, MySQL writes ?, Oracle writes :1.
	Placeholder func(n int) string
	// Literal renders one parameter value as a SQL literal, for a product
	// whose protocol cannot carry a parameter at all. Nil for every
	// product that can bind, which is every one but Apache Hive, and a
	// nil Literal is what says the dialect binds.
	//
	// This exists because HiveServer2 has no parameter channel. Its
	// Thrift request carries a session handle, a statement, a
	// configuration overlay, an asynchronous flag and a timeout, and
	// nothing else, so no driver can bind and Hive's own JDBC
	// PreparedStatement substitutes on the client. A dialect in that
	// position either renders its values into the statement or answers
	// nothing at all. See D78.
	//
	// It is deliberately not a general literal mode and it is not a
	// switch. The dialect supplies the function, because escaping is per
	// product knowledge and one shared rule is not enough: Hive does not
	// accept a doubled quote the way [QuoteLiteral] writes one. It reads
	// 'a''b' as two literals joined and returns ab, silently, so a
	// doubled quote there is a wrong answer rather than an error.
	//
	// This is the same reasoning D56 used to let ChangePassword build a
	// statement: the escaping is the product's and the value cannot be
	// bound.
	//
	// What it renders is a parameter this project declared, never a
	// statement. The statement is written here and no caller supplies
	// one. A value still reaches it from outside, so an implementation
	// refuses what it cannot render rather than guessing, and returns an
	// error for a type it does not know and for a string it cannot
	// escape safely.
	Literal func(v any) (string, error)

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

// Embedded reports whether the database is a library rather than a server.
//
// It is false for a dialect no model was built for, the same as for a server,
// because the question cannot be answered without the model. See
// [Info.Embedded].
func (d Dialect) Embedded() bool {
	info, found := d.Info()
	return found && info.Embedded
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
