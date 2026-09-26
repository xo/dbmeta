<div align="center">
  <a href="#about" title="About">About</a> |
  <a href="#installing" title="Installing">Installing</a> |
  <a href="#using" title="Using">Using</a> |
  <a href="#database-support" title="Database Support">Database Support</a> |
  <a href="#version-support" title="Version Support">Version Support</a> |
  <a href="#design" title="Design">Design</a> |
  <a href="#contributing" title="Contributing">Contributing</a>
</div>

<br/>

[![Unit Tests][dbmeta-ci-status]][dbmeta-ci]
[![Go Reference][goref-dbmeta-status]][goref-dbmeta]

[dbmeta-ci]: https://github.com/xo/dbmeta/actions/workflows/test.yml "Test CI"
[dbmeta-ci-status]: https://github.com/xo/dbmeta/actions/workflows/test.yml/badge.svg "Test CI"
[goref-dbmeta]: https://pkg.go.dev/github.com/xo/dbmeta "Go Reference"
[goref-dbmeta-status]: https://pkg.go.dev/badge/github.com/xo/dbmeta.svg "Go Reference"

# About

`dbmeta` reads database metadata. Metadata means the description of what a
database contains: its schemas, tables, columns, indexes, constraints,
functions, types, roles, privileges and the rest.

One API answers for every database. A caller asks for tables without knowing
which database answers, and receives the same types whichever one does.

`dbmeta` reads. It does not change a database and it does not render results.
It is used by [`usql`][usql], a command line client for many databases, and by
[`dbtpl`][dbtpl], a code generator.

PostgreSQL is the model. The shape of every answer follows the metadata
commands of [`psql`][psql], because bringing that experience to other databases
is the purpose of this package.

# Installing

```sh
go get github.com/xo/dbmeta
```

`dbmeta` depends on the standard library and on nothing else. It imports no
database driver and no URL parser. The caller opens the connection with the
driver of its choice and passes it in.

Nothing here parses a connection string, because nothing here opens a
connection. If you want a URL to open a database, use [`dburl`][dburl], which
is what the examples below do. `dbmeta` never sees it.

# Using

Import the package and the models for the databases you need:

```go
import (
    "github.com/xo/dbmeta"
    _ "github.com/xo/dbmeta/all"
)
```

`dbmeta` asks a database to do one thing, and says so in the type system:

```go
type Querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}
```

`sql.DB`, `sql.Tx` and `sql.Conn` all satisfy it. Pass a `sql.Tx` to read
several catalogs in one snapshot.

Open a connection, read the version, then ask:

```go
db, err := dburl.Open("postgres://localhost/example")
if err != nil {
    return err
}

// dbmeta supplies the version statement, runs it against the connection you
// passed, and parses the answer.
versions, err := dbmeta.PostgreSQL.Version(ctx, db)
if err != nil {
    return err
}

m, err := dbmeta.New(dbmeta.PostgreSQL, versions)
if err != nil {
    return err
}

for t, err := range dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Schema: "public"}.Map()) {
    if err != nil {
        return err
    }
    fmt.Println(t.Schema, t.Name, t.Type)
}
```

There is one query value per kind of object, such as `dbmeta.Tables` and
`dbmeta.Columns`. Name the value and the result type follows from it.

## Running the statement yourself

A caller that prints statements, or that runs them its own way, asks for the
statement instead of the rows:

```go
sqlstr, args, err := dbmeta.Tables.SQL(m, dbmeta.Args{Schema: "public"}.Map())
```

`Query.Fields` and `Query.Params` describe what a query returns and what it
takes. `dbmeta.Queries` lists every query, and `Query.Support` says whether
this database answers it.

## Overriding the version

The caller chooses the version. `dbmeta` never detects one behind your back.
Build a version set by hand and pass it to `New` when a proxy hides the server,
when a compatible product reports a version it does not behave like, or when
there is no server at all.

## Iterators hold a connection

An iterator holds a database connection until it ends. Stopping early, with
`break` or by cancelling the context, releases it.

Do not open a second iterator inside the body of the first. That needs a second
connection and deadlocks on a pool of one. Ask for every row you want in one
call and filter in the loop.

# Database Support

| Database   | Model              | Queries | Status      |
| ---------- | ------------------ | ------- | ----------- |
| PostgreSQL | native             | 55      | Complete    |
| any with an information_schema | shared | 12 | Ready to build on |
| MariaDB    | native             | 29      | Complete    |
| MySQL      | native             | 26      | Complete    |
| SQLite3    | native             | 14      | Complete    |
| DuckDB     | native             | 20      | Complete    |
| SQL Server | native             | 32      | Complete    |
| Oracle     | native             | 25      | In progress |
| Cassandra  | native             | 17      | In progress |
| ClickHouse | native             | 23      | In progress |
| Trino      | native             | 13      | In progress |
| Presto     | native             | 9       | In progress |
| Firebird   | native             | 24      | In progress |
| SAP HANA   | native             | 32      | In progress |
| Apache Hive | native            | 16      | In progress |

A native model reads the catalog the database keeps for itself. A shared model
reads `information_schema`, which is a smaller answer that many databases have.
It answers 12 object kinds where the native PostgreSQL model answers 55, and
answers none of them completely: no size, owner or access method for a table,
no storage or index detail for a column, no exclusion constraint, no aggregate.

ClickHouse ships an `information_schema` and the model does not read it. It is
an emulation that reports what the standard names and drops what makes a
ClickHouse table what it is: the engine, the partition key, the sorting key,
the codec per column and the skipping indices. `system` has all of it.

Cassandra has no `information_schema` at all and has a native model for the
same reason SQLite, DuckDB and Oracle do. It is the only one here that is not
SQL, and CQL is narrower than the name suggests: it cannot compute, it cannot
express an optional filter, and it cannot order across partitions. D62 holds
what follows from that. `usql` builds on the shared reader today for
Snowflake, Trino, Databend and Netezza, which is the evidence for who the
shared model serves.

SQL Server was on that list too. `usql` reads it through the shared reader with
sequences and constraints switched off and a small plugin for catalogs and
indexes, and the native model here answers 32 kinds instead, including the
sequences and constraints that reader turns off.

[`COVERAGE.md`](docs/COVERAGE.md) says what each database answers, what it cannot,
and which analogues were found and rejected. MariaDB answers 29 of the 55 and
MySQL answers 26, because a native model beats the shared one by seventeen.

MariaDB and MySQL share one model. A query written for one of them gates on the
product rather than on the release number, because MariaDB is at 13.0 and MySQL
at 26.7 and neither number says anything about the other. CI runs both products
and a third job compares them against the same schema.

SQLite and DuckDB have no server. Both are libraries, so the release under test
is whichever one the Go driver was built with, neither needs a container, and
neither model carries a version gate.

Every driver the tests use is the one `usql` uses for that database. The version
may differ and the package may not, because a query that works here and fails
on the driver `usql` ships is a query that does not work. See D52.

A model ships its queries and a fixture together. The fixture is a known good
schema containing one of every object the queries read, exported so that other
projects can generate against it.

# Version Support

Every version sits in one of four tiers. Read the tier before you rely on a
version.

| Tier     | What it means                                                      |
| -------- | ------------------------------------------------------------------ |
| Tested   | Tests run on every change, in CI                                     |
| Nightly  | Tests run once a night, in CI                                        |
| Verified | Tests run on a development machine before a release, and not in CI   |
| Archived | The queries exist and were checked once, and nothing runs them now   |

D40 set three of these and D42 added Nightly. The first three are values of
`container.Tier`. Archived is not, because an archived release is one the
list does not name at all.

## PostgreSQL

Supported from release 9.6 to release 18, which is ten major versions: 9.6, 10,
11, 12, 13, 14, 15, 16, 17 and 18.

| Releases          | Tier     |
| ----------------- | -------- |
| 9.6, 12, 15, 18   | Tested   |
| 10, 11, 13, 14, 16, 17 | Nightly  |

Four releases are Tested: CI starts a real server for each and runs the
integration tests on every change. They are the floor, the ceiling and one on
each side of the middle, which is the smallest set that catches every fault
found so far. Testing only the newest would have caught two of six.

The other six are Nightly. All ten run nightly in CI, and `dbrun` runs any of
them on a development machine.

Every query is executed against a real server at all ten releases by `dbrun`,
which also checks that the columns returned match the fields
declared and that a field the server is too old for arrives as NULL. The
objects PostgreSQL did not have before release 10 are refused there rather than
returning an empty result:
publications, publication tables, subscriptions, extended statistics and
partitioned tables.

Note that `psql` itself dropped support for servers below release 10 in
PostgreSQL 20. `dbmeta` supports 9.6 deliberately, and its queries for that
release are translated from an older checkout.

## SQL Server

Supported on every major release that runs on Linux: 2017, 2019, 2022 and 2025.

| Releases                     | Tier     |
| ---------------------------- | -------- |
| 2017, 2019, 2022, 2025       | Tested   |
| 2016 and older               | Archived |

All four are Tested, and CI starts a real server for each on every change.
There are only four, and every version gate the model carries sits below all of
them, so there is no older branch that a smaller matrix would leave uncovered.

2017 is a hard floor rather than a choice. Microsoft shipped SQL Server on
Linux from 2017, so no container exists for 2016 or earlier and no test can run
against one. The queries carry gates for `sys.sequences` and
`sys.dm_db_stats_properties` at 2012 and for `sys.external_tables` at 2016, and
those gates resolve correctly without a server, which
`models/sqlserver/version_test.go` checks. That is all that is claimed for an
older release. It is not a claim that the query ran. See D54.

# Design

`dbmeta` supplies the data and the consumer decides what to show. `psql` sets
the object model and it does not set the column set, so a query here returns
facts `psql` does not print where the database can produce them in the same
statement. D47 holds the rule and the cost test.

Read [`NULLS.md`](docs/NULLS.md) before writing a query for any database. It is
the shortest document here and the one that cost the most to learn.

Everything else is in [`docs/`](docs/):

| Document | What it holds |
| --- | --- |
| [`PLAN.md`](docs/PLAN.md) | Every decision, 83 of them, with the reasoning and what was rejected. A table at the top lists them with their status, because 12 amend or replace an earlier one. |
| [`NULLS.md`](docs/NULLS.md) | One rule: never collapse a NULL. |
| [`COVERAGE.md`](docs/COVERAGE.md) | What each database can and cannot answer, per object kind, and which analogues were rejected and why. |
| [`COMMANDS.md`](docs/COMMANDS.md) | Every `psql` metadata command mapped to the Go value that answers it, which is what wiring up a client needs. |
| [`QUERIES.md`](docs/QUERIES.md) | What `psql` describes, what `information_schema` describes, and where the two meet. |
| [`DIALECT.md`](docs/DIALECT.md) | Every step needed to add a database, in order, with the test that catches each one you skip. |
| [`EVALUATION.md`](docs/EVALUATION.md) | How the supported version range is chosen, and how to choose one for a database not covered yet. |
| [`USQL.md`](docs/USQL.md) | What `usql` answers today for each of its 47 drivers, and what changes if it reads `dbmeta`. |
| [`DBTPL.md`](docs/DBTPL.md) | The same measurement for `dbtpl`. |
| [`RUNNER.md`](docs/RUNNER.md) | The design of `dbrun`, the command that starts the databases the tests run against. |
| [`WINDOWS.md`](docs/WINDOWS.md) | The Windows machines that host the SQL Server releases with no Linux container, and why each of them is awkward. |

[`CLAUDE.md`](CLAUDE.md) holds the rules for writing code here, with a table
saying which document to read for which task.
[`CONTRIBUTING.md`](CONTRIBUTING.md) is the same for a person, and shorter.

# Testing

`container/container.go` names every database release the tests run against,
as Go data. It holds the image, the tag, the environment, the readiness
command and the connection string, and it starts nothing: a caller brings its
own podman, docker or Go client, and `dbmeta` depends on none of them.

```go
for _, s := range container.All() {
	fmt.Println(s.Name(), s.Tier, s.Ref())
}
```

Run the tests against a release, or a product, or a tier:

```bash
cd test && go run ./cmd/dbrun test mariadb-13.0
```

`dbrun test all` runs every release. It reads the list from `container`, starts
each server, waits for it to accept a connection, runs the tests and removes
the container. CI builds its matrix from the same list, and
`container/workflow_test.go` fails when the two disagree. Nothing else starts a
container, which is D68, and `docs/RUNNER.md` is the design.

Every database also answers one checked in expectation. `TestConformance`
builds the same core schema on PostgreSQL, MariaDB, MySQL, SQLite and DuckDB
and compares the portable facts, so a difference between two families is either
fixed or written down. 23 of the canonical lines are identical across all five,
and [`COVERAGE.md`](docs/COVERAGE.md) records every difference that is not.

# Related Projects

`dbmeta` is one of a set of packages that each do one part of the job, so that
a client can take the parts it needs.

- [`usql`][usql] is a command line client for many databases. It is the reason
  the object model follows `psql`. [`USQL.md`](docs/USQL.md) measures what it
  answers today and what `dbmeta` would change.
- [`dbtpl`][dbtpl] generates Go code from a database schema. It reads the same
  metadata and it can build against the fixtures here. [`DBTPL.md`](docs/DBTPL.md)
  measures the same thing for it.
- [`dburl`][dburl] parses a database URL and opens a connection. `dbmeta` never
  parses one, and it never repeats the scheme and flavor taxonomy that `dburl`
  holds. From v0.29.0 `dburl.Scheme` describes each scheme as well as parsing
  it, with the Go driver package, whether that driver needs cgo, and whether
  the product is embedded, a server or hosted. That is where to look up a
  driver, and D80 says why `dbmeta` reads it as a document rather than
  importing it.
- [`tblfmt`][tblfmt] renders a result set the way `psql` does. `dbmeta` reads
  metadata and does not render it, so a client that wants a table passes the
  rows to `tblfmt`.

# Contributing

Read [`CLAUDE.md`](CLAUDE.md) first. It holds the rules, including the ones
that are not obvious: the standard library only, no cgo in anything a consumer
builds, no build constraint on an operating system or an architecture, and a
context on every function that reads from a database.

Run the checks before you send a change, with the same flags CI uses:

```sh
gofmt -l . && go vet ./... && go build ./... && go test -race -count=2 ./...
```

To run the integration tests, start a server and point the test module at it:

```sh
cd test && go run ./cmd/dbrun test postgres-18
```

`dbrun test all` runs every supported release of every product, which is what
has to pass before a release.

Tests in this module never open a database connection. They render statements,
resolve versions, and read rows from a fake driver replaying recorded data, so
they need nothing installed and they cover releases whose container images no
longer start.

[usql]: https://github.com/xo/usql "usql"
[dbtpl]: https://github.com/xo/dbtpl "dbtpl"
[dburl]: https://github.com/xo/dburl "dburl"
[tblfmt]: https://github.com/xo/tblfmt "tblfmt"
[psql]: https://www.postgresql.org/docs/current/app-psql.html "psql"
