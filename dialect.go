package dbmeta

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
)

// Dialect names one database family. Its value is the dburl driver name of
// the scheme that is canonical for the product, and a consumer with a dburl
// URL reads it from URL.Dialect, which dburl sets from v0.32.0.
//
// Read URL.Dialect and never URL.Driver. A product with two Go drivers has two
// schemes, and URL.Driver names the driver: pgx and postgres, moderncsqlite
// and sqlite3, godror and oracle. Each pair has one Dialect. dbmeta holds no
// such list, because hard rule 1 keeps that taxonomy in dburl. See D99.
//
// A product that speaks another product's wire protocol has a Dialect of its
// own, as dburl gives it from v0.36.0. CockroachDB, CrateDB, QuestDB and
// Redshift speak PostgreSQL's protocol, and SingleStore, TiDB and Vitess
// speak MySQL's, and each has its own dialect, because its catalog is its
// own. A model for one of them can share the statements of the product it
// imitates, as the CockroachDB model does (dburl D30 and D37, D123, D125).
//
// A dialect names the family and never the release. A release arrives as a
// [VersionSet] value, because D8 resolves version differences at run time. See
// [New].
type Dialect string

// Dialect values. One per database family.
//
// The name is what the database calls itself. The value is the `dburl` driver
// name, which is not always the same word. PostgreSQL calls itself PostgreSQL
// and its driver is postgres. Cassandra calls itself Cassandra and its driver
// is cassandra, since the driver moved from github.com/xo/cql to
// github.com/xo/cassandra and dburl v0.47.0 renamed its scheme (D196).
// Couchbase was n1ql until dburl v0.33.0 renamed its scheme couchbase, when the
// driver moved to github.com/xo/dbimp
// (D101). Read these as constants and never as literals.
const (
	// ArangoDB is queried in AQL rather than SQL, over HTTP.
	ArangoDB Dialect = "arangodb"
	Athena   Dialect = "awsathena"
	// Avatica is the standalone Avatica server, which stands in front of
	// HSQLDB and which dbimp's avatica driver reaches. Phoenix speaks the same
	// protocol and has no model. See D186.
	Avatica    Dialect = "avatica"
	BigQuery   Dialect = "bigquery"
	Cassandra  Dialect = "cassandra"
	Chai       Dialect = "chai"
	ClickHouse Dialect = "clickhouse"
	// CockroachDB speaks PostgreSQL's protocol, and pgx reaches it.
	CockroachDB Dialect = "cockroachdb"
	Couchbase   Dialect = "couchbase"
	Cosmos      Dialect = "cosmos"
	// CrateDB speaks PostgreSQL's protocol on its port 5432, and pgx reaches
	// it.
	CrateDB    Dialect = "cratedb"
	CSVQ       Dialect = "csvq"
	Databricks Dialect = "databricks"
	Databend   Dialect = "databend"
	// Drill is Apache Drill, which dbimp's drill driver reaches on the REST
	// interface of a Drillbit. See D178.
	Drill Dialect = "drill"
	// Druid is Apache Druid, which dbimp's druid driver reaches on the SQL
	// API of the Router. See D171.
	Druid    Dialect = "druid"
	DuckDB   Dialect = "duckdb"
	DynamoDB Dialect = "dynamodb"

	// Elasticsearch reaches its SQL on /_sql, through dbimp's elasticsearch
	// driver. See D177.
	Elasticsearch Dialect = "elasticsearch"

	Exasol   Dialect = "exasol"
	Firebird Dialect = "firebirdsql"
	// GizmoSQL serves Arrow Flight SQL, and the flightsql driver reaches it.
	GizmoSQL Dialect = "gizmosql"
	Hive     Dialect = "hive"
	HANA     Dialect = "hdb"
	// Impala is Apache Impala, which serves HiveServer2's protocol and has
	// a catalog of its own. See D145.
	Impala   Dialect = "impala"
	InfluxDB Dialect = "influxdb"
	InfluxQL Dialect = "influxql"
	// LibSQL is libSQL, the fork of SQLite by Turso, which sqld serves
	// over HTTP.
	LibSQL     Dialect = "libsql"
	MaxCompute Dialect = "maxcompute"
	// MemSQL is SingleStore, which speaks MySQL's protocol. dburl names
	// its scheme memsql, the name SingleStore had until 2020.
	MemSQL Dialect = "memsql"
	MySQL  Dialect = "mysql"
	Neo4j  Dialect = "neo4j"

	// OpenSearch reaches its SQL on /_plugins/_sql, through dbimp's opensearch
	// driver. See D181.
	OpenSearch Dialect = "opensearch"

	Oracle     Dialect = "oracle"
	Presto     Dialect = "presto"
	PostgreSQL Dialect = "postgres"
	// QuestDB speaks PostgreSQL's protocol on its port 8812, and pgx
	// reaches it.
	QuestDB Dialect = "questdb"
	// Redshift speaks PostgreSQL's protocol, and pgx reaches it.
	Redshift Dialect = "redshift"
	// Rqlite is rqlite, which runs SQLite behind an HTTP API.
	Rqlite    Dialect = "rqlite"
	Snowflake Dialect = "snowflake"
	// Solr is Apache Solr, which dbimp's solr driver reaches on the SQL handler
	// of a collection. See D179.
	Solr       Dialect = "solr"
	Spanner    Dialect = "spanner"
	SQLite3    Dialect = "sqlite3"
	SQLServer  Dialect = "sqlserver"
	SurrealDB  Dialect = "surrealdb"
	Tablestore Dialect = "ots"
	// TiDB speaks MySQL's protocol, and the mysql driver reaches it.
	TiDB    Dialect = "tidb"
	Trino   Dialect = "trino"
	Vertica Dialect = "vertica"
	// Vitess speaks MySQL's protocol, and the mysql driver reaches it.
	Vitess Dialect = "vitess"
	// YDB is Yandex's distributed database, which ydb-go-sdk reaches over
	// gRPC. See D161.
	YDB Dialect = "ydb"
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
	// Hard rule 1 normally sends a fact about a scheme to dburl, and
	// dburl carries this one from v0.29.0: it marks file, sqlite3,
	// moderncsqlite, csvq, duckdb and chai DeploymentEmbedded, which is the
	// same statement about the same six schemes. It is still declared
	// here, because dbmeta has no dependencies and never opens a connection,
	// so a caller holding a Dialect has no URL to hand to dburl.
	//
	// The two do not even count the same objects. dburl counts schemes and
	// marks three for the two products here, because SQLite has two drivers.
	// dbmeta counts models and marks two. See D80.
	Embedded bool

	// Placeholder writes the bind parameter for position n, counting from 1.
	// PostgreSQL writes $1, MySQL writes ?, Oracle writes :1.
	Placeholder func(n int) string
	// Named says that the drivers take a named argument only, so that a
	// value is bound by its name and never by its position. Under it, the
	// parameter at position n is written $p1, $p2 and so on, and its value
	// is passed as sql.Named("p1", v), so [Query.Build] still returns the
	// values in order and database/sql carries the names. Placeholder is not
	// called. False for every product but SurrealDB.
	//
	// SurrealQL has named parameters only, and $1 is a parse error. dbimp's
	// surrealdb driver refuses a positional argument for that reason
	// (dbimp D50). See D164.
	Named bool
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
	// BindValue converts one parameter value before it is bound, for a
	// product whose drivers cannot bind every type a parameter here takes.
	// Nil for every product but Oracle, and a nil BindValue binds the value
	// as it is.
	//
	// Oracle is the case because it has no boolean before 23ai, and
	// go-ora/v3 refuses a Go bool with "no parameter coder registered for
	// go type bool". The Oracle model compares with_system with 1, so it
	// binds a bool as 1 or 0, which every go-ora release carries. usql found
	// it on 2026-09-30. See D136.
	BindValue func(v any) any

	// VersionQuery reads the server version. An empty string means the database
	// reports no version, and a caller uses an unknown version.
	VersionQuery string
	// VersionColumns is how many columns VersionQuery returns. Most return
	// one. SQL Server returns five, and CockroachDB, CrateDB, SingleStore and
	// Vitess return two.
	VersionColumns int
	// ParseVersion turns the columns of the first row into a version set and a
	// display line.
	ParseVersion func(cols []string) (VersionSet, error)
	// FollowUpQuery returns a second version statement, and how many columns
	// it returns, when what the first one found calls for it. It returns an
	// empty statement when none is needed. Nil for every dialect but cassandra.
	//
	// It exists because two products share the cassandra dialect and only one
	// table both have. ScyllaDB reports the Cassandra release it keeps
	// compatible with in system.local, and keeps its own release in
	// system.versions, which Cassandra does not have. The first statement
	// finds ScyllaDB and this one reads its release. See D92.
	FollowUpQuery func(s VersionSet) (query string, cols int)
	// ParseFollowUp adds the columns of the follow-up row to the set the
	// first statement produced.
	ParseFollowUp func(s VersionSet, cols []string) (VersionSet, error)

	// QuotingQuery reads the session state that decides how a string literal is
	// escaped. Empty when the product has no such state. See D56.
	QuotingQuery string
	// QuotingColumns is how many columns QuotingQuery returns.
	QuotingColumns int
	// ParseQuoting turns those columns into the state.
	ParseQuoting func(cols []string) (Quoting, error)
	// ChangePassword builds the statement that sets a password. It returns
	// text and never runs anything. Nil when the product has no such
	// statement or the model builds none, as for every embedded database
	// here.
	ChangePassword func(c PasswordChange, q Quoting) (string, error)
	// OldPassword says ChangePassword needs PasswordChange.Old: the product
	// refuses a user's own password change without the current one. SQL
	// Server is the case. A caller asks for the old password when it is
	// true. See D143.
	OldPassword bool

	// Syntax is the lexical forms the product's SQL has, for a client that
	// splits text into statements. See D143.
	Syntax Syntax
	// Terminator says what the product does with a semicolon that ends a
	// statement. The zero value keeps it. See D143.
	Terminator Terminator
	// Batches are the statements that open and close a batch, which a client
	// sends as one statement with everything between them. See D143.
	Batches []Batch
	// Fold is what the product does to the case of a name that is not
	// quoted. See [Dialect.FoldIdentifier] and D143.
	Fold Fold
}

var (
	infoMu sync.RWMutex
	infos  = make(map[Dialect]*Info)
)

// RegisterDialect records what a model declares about its database. A model
// package calls this from its init. Registering twice panics, the way
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

// FollowUpQuery returns the second version statement for s, the set that
// [Dialect.ParseVersion] returned, and how many columns it returns.
//
// The caller runs it after the first statement and passes its columns to
// [Dialect.ParseFollowUp]. The third result is false when the dialect needs no
// second statement for this server, which is every dialect but cassandra, and cassandra
// on Cassandra. See D92.
func (d Dialect) FollowUpQuery(s VersionSet) (query string, cols int, ok bool) {
	info, found := d.Info()
	if !found || info.FollowUpQuery == nil {
		return "", 0, false
	}
	query, cols = info.FollowUpQuery(s)
	return query, cols, query != ""
}

// ParseFollowUp adds the columns returned by [Dialect.FollowUpQuery] to s.
func (d Dialect) ParseFollowUp(s VersionSet, cols []string) (VersionSet, error) {
	info, ok := d.Info()
	if !ok {
		return VersionSet{}, ErrModelNotBuilt
	}
	if info.ParseFollowUp == nil {
		return s, nil
	}
	return info.ParseFollowUp(s, cols)
}

// Version runs the version statement against db and returns the parsed
// result.
//
// It is the steps above done together, because every caller needs both and
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
func (d Dialect) Version(ctx context.Context, db Queryer) (VersionSet, error) {
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
	s, err := d.ParseVersion(cols)
	if err != nil {
		return VersionSet{}, err
	}
	query, n, ok = d.FollowUpQuery(s)
	if !ok {
		return s, nil
	}
	// The follow-up refines what the first statement found, and the server
	// can refuse it to a principal that the first statement served.
	// ScyllaDB refuses system.versions to a role granted nothing on it and
	// serves system.local to the same role. So a refusal leaves the set as
	// the first statement read it, which still names the product. Only a
	// canceled context is an error, because then nothing that follows can
	// run either.
	cols, err = readRow(ctx, db, d, query, n)
	switch {
	case err == nil:
		return d.ParseFollowUp(s, cols)
	case ctx.Err() != nil:
		return VersionSet{}, fmt.Errorf("reading from %s: %w", d, ctx.Err())
	}
	return s, nil
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
func readRow(ctx context.Context, db Queryer, d Dialect, query string, n int) ([]string, error) {
	// QueryContext rather than QueryRowContext, so that Queryer needs one
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

// Open reads the version of the server behind db and returns the metadata
// for it, which is Version followed by New. If the version cannot be read,
// Open returns the error and no Meta. A caller that wants to go on without a
// version calls New with an empty VersionSet, which takes the newest
// fragments, and decides that for itself, because on an old server those
// statements can be wrong.
func Open(ctx context.Context, d Dialect, db Queryer) (*Meta, error) {
	versions, err := d.Version(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("reading the version of %s: %w", d, err)
	}
	return New(d, versions)
}

// Dialect returns the dialect m was built for.
func (m *Meta) Dialect() Dialect { return m.dialect }

// String satisfies the [fmt.Stringer] interface. It returns the line to show
// a person, such as "PostgreSQL 16.2", which is what usql prints for
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
