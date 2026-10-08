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
[![Releases][release-status]][releases]
[![Discord Discussion][discord-status]][discord]

[dbmeta-ci]: https://github.com/xo/dbmeta/actions/workflows/test.yml "Test CI"
[dbmeta-ci-status]: https://github.com/xo/dbmeta/actions/workflows/test.yml/badge.svg "Test CI"
[goref-dbmeta]: https://pkg.go.dev/github.com/xo/dbmeta "Go Reference"
[goref-dbmeta-status]: https://pkg.go.dev/badge/github.com/xo/dbmeta.svg "Go Reference"
[release-status]: https://img.shields.io/github/v/release/xo/dbmeta?display_name=tag "Latest Release"
[releases]: https://github.com/xo/dbmeta/releases "Releases"
[discord]: https://discord.gg/WDWAgXwJqN "Discord Discussion"
[discord-status]: https://img.shields.io/discord/829150509658013727.svg?label=Discord&logo=Discord&colorB=7289da&style=flat-square "Discord Discussion"

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
type Queryer interface {
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

`dbmeta.Open` does the version and `New` in one step, and returns the error
when the version cannot be read. `Query.Each` takes an `Args` rather than a
map and passes only the arguments the query takes, so one `Args` serves every
kind of object. `All` refuses an argument the query does not take, which
catches a misspelled name. `Dialect.Pattern` turns a psql pattern, such as
`public.film*`, into a schema pattern and a name pattern, folded as the
product folds a name. See D169.

```go
m, err := dbmeta.Open(ctx, dbmeta.PostgreSQL, db)
if err != nil {
    return err
}
schema, name := dbmeta.PostgreSQL.Pattern("public.film*")
for t, err := range dbmeta.Tables.Each(ctx, m, db, dbmeta.Args{Schema: schema, Name: name}) {
    if err != nil {
        return err
    }
    fmt.Println(t.Schema, t.Name, t.Type)
}
```

## Running the statement yourself

A caller that prints statements, or that runs them its own way, asks for the
statement instead of the rows:

```go
sqlstr, args, err := dbmeta.Tables.Build(m, dbmeta.Args{Schema: "public"}.Map())
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
`break` or by canceling the context, releases it.

Do not open a second iterator inside the body of the first. That needs a second
connection and deadlocks on a pool of one. Ask for every row you want in one
call and filter in the loop.

# Database Support

| Database   | Model              | Queries | Status      |
| ---------- | ------------------ | ------- | ----------- |
| PostgreSQL | native             | 56      | Complete    |
| any with an information_schema | shared | 12 | Ready to build on |
| MariaDB    | native             | 29      | Complete    |
| MySQL      | native             | 26      | Complete    |
| SQLite3    | native             | 14      | Complete    |
| DuckDB     | native             | 20      | Complete    |
| SQL Server | native             | 32      | Complete    |
| Oracle     | native             | 26      | In progress |
| Cassandra  | native             | 17      | In progress |
| ScyllaDB   | native             | 18      | In progress |
| ClickHouse | native             | 23      | In progress |
| Trino      | native             | 13      | In progress |
| Presto     | native             | 9       | In progress |
| Firebird   | native             | 24      | In progress |
| SAP HANA   | native             | 32      | In progress |
| Apache Hive | native            | 16      | In progress |
| Exasol     | native             | 25      | In progress |
| Vertica    | native             | 26      | In progress |
| Couchbase  | native             | 12      | In progress |
| CockroachDB | native            | 54      | In progress |
| CrateDB    | native             | 26      | In progress |
| QuestDB    | native             | 11      | In progress |
| TiDB       | native             | 19      | In progress |
| Vitess     | native             | 20      | In progress |
| Databend   | native             | 20      | In progress |
| SingleStore | native            | 23      | In progress |
| Snowflake  | native             | 13      | In progress |
| Amazon Redshift | native        | 11      | In progress |
| Apache Impala | native          | 11      | In progress |
| rqlite     | native             | 14      | In progress |
| libSQL     | native             | 14      | In progress |
| InfluxDB 3 | native             | 9       | In progress |
| Neo4j      | native             | 17      | In progress |
| YDB        | native             | 7       | In progress |
| ArangoDB   | native             | 7       | In progress |
| Apache Avatica | native         | 24      | In progress |
| Apache Druid | native           | 7       | In progress |
| Apache Drill | native           | 10      | In progress |
| Elasticsearch | native          | 8       | In progress |
| OpenSearch | native             | 3       | In progress |
| Apache Solr | native            | 4       | In progress |
| GizmoSQL   | native             | 20      | In progress |
| InfluxQL   | native             | 7       | In progress |
| SurrealDB  | native             | 18      | In progress |

A native model reads the catalog the database keeps for itself. A shared model
reads `information_schema`, which is a smaller answer that many databases have.
It answers 12 object kinds where the native PostgreSQL model answers 56, and
answers none of them completely: no size, owner or access method for a table,
no storage or index detail for a column, no exclusion constraint, no aggregate.

ClickHouse ships an `information_schema` and the model does not read it. It is
an emulation that reports what the standard names and drops what makes a
ClickHouse table what it is: the engine, the partition key, the sorting key,
the codec per column and the skipping indices. `system` has all of it.

Cassandra has no `information_schema` at all and has a native model for the
same reason SQLite and Oracle do. It is the only one here that is not
SQL, and CQL is narrower than the name suggests: it cannot compute, it cannot
express an optional filter, and it cannot order across partitions. D62 holds
what follows from that. `usql` builds on the shared reader today for
Snowflake, Trino, Databend and DuckDB, which is the evidence for who
the shared model serves.

SQL Server was on that list too. `usql` reads it through the shared reader with
sequences and constraints switched off and a small plugin for catalogs and
indexes, and the native model here answers 32 kinds instead, including the
sequences and constraints that reader turns off.

[`COVERAGE.md`](docs/COVERAGE.md) says what each database answers, what it cannot,
and which analogues were found and rejected. MariaDB answers 29 of the 56 and
MySQL answers 26, because a native model beats the shared one by seventeen.

MariaDB and MySQL share one model. A query written for one of them gates on the
product rather than on the release number, because MariaDB is at 13.0 and MySQL
at 26.7 and neither number says anything about the other. CI runs both products
and a third job compares them against the same schema.

Cassandra and ScyllaDB share one model in the same way. ScyllaDB answers one
kind more, `RoleSettings`, from the service level attached to a role. D91
records how the model tells the two products apart.

SQLite and DuckDB have no server. Both are libraries, so the release under test
is whichever one the Go driver was built with, neither needs a container, and
neither model carries a version gate.

Every driver the tests use is the one the `dburl` registry names for that
dialect, because dburl is upstream of `dbmeta` and of every consumer. The
version can differ and the package must not, because a query that works here
and fails on the driver a consumer opens is a query that does not work. See D52
and D154.

A model ships its queries and a fixture together. The fixture is a known good
schema containing one of every object the queries read, exported so that other
projects can generate against it.

# Version Support

Every version sits in one of five tiers. Read the tier before you rely on a
version.

| Tier     | What it means                                                      |
| -------- | ------------------------------------------------------------------ |
| Tested   | Tests run on every change, in CI                                     |
| Nightly  | Tests run once a night, in CI                                        |
| Verified | Tests run on a development machine before a release, and not in CI   |
| Staged   | dbrun starts it, and it was measured when it was added. No model reads it yet, so nothing runs it in CI |
| Archived | The queries exist and were checked once, and nothing runs them now   |

D40 set three of these, D42 added Nightly and D119 added Staged. The first four
are values of `container.Tier`. Archived is not, because an archived release is
one the list does not name at all.

A Staged release is not supported by dbmeta. It is there so that dbrun can
start a server for a sister project, such as dbimp's drivers, or for a flavor
that a model does not detect yet. It moves to Tested, Nightly or Verified in
the change that adds its model, and takes as its tier the cadence it already
records: Tested, Nightly or Verified. `dbrun list --json` prints the cadence,
and dbimp runs the tested ones on each push and the nightly ones at night
(D120).

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
found so far. Testing only the newest catches two of the six.

The other six are Nightly, and CI runs them once a night. `dbrun` runs any of
the ten on a development machine.

Every query is executed against a real server at all ten releases by `dbrun`,
which also checks that the columns returned match the fields
declared and that a field the server is too old for arrives as NULL. The
objects PostgreSQL did not have before release 10 are refused there rather than
returning an empty result:
publications, publication tables, subscriptions, extended statistics and
partitioned tables.

`psql` itself dropped support for servers below release 10 in PostgreSQL 20.
`dbmeta` supports 9.6 deliberately, and its queries for that release are
translated from an older checkout.

## SQL Server

Supported on 2017, 2019, 2022 and 2025 in containers, and on 2008R2, 2012,
2014 and 2016 on Windows machines.

| Releases                     | Tier     |
| ---------------------------- | -------- |
| 2017, 2019, 2022, 2025       | Tested   |
| 2008R2, 2012, 2014, 2016     | Verified |

All four are Tested, and CI starts a real server for each on every change.
There are only four, and every version gate the model carries sits below all of
them, so there is no older branch that a smaller matrix leaves uncovered.

2017 is the first release with a Linux container. Microsoft shipped SQL Server
on Linux from 2017, so the older releases run on Windows machines that `dbrun`
builds. They are Verified: a person runs them on a development machine before
a release, and CI never does. The queries carry gates for `sys.sequences` and
`sys.dm_db_stats_properties` at 2012 and for `sys.external_tables` at 2016,
and `models/sqlserver/version_test.go` checks that those gates resolve. See
D54 and D57.

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
| [`PLAN.md`](docs/PLAN.md) | The plan: the purpose, the architecture, what exists, the testing plan and the open questions for Ken. |
| [`decisions/`](docs/decisions/README.md) | Every decision, 199 of them, one file each, with the reasoning and what was rejected. The index lists them with their status, because 71 amend or replace an earlier one. |
| [`NULLS.md`](docs/NULLS.md) | One rule: never collapse a NULL. |
| [`COVERAGE.md`](docs/COVERAGE.md) | What each database can and cannot answer, per object kind, and which analogues were rejected and why. |
| [`COMMANDS.md`](docs/COMMANDS.md) | Every `psql` metadata command mapped to the Go value that answers it, which is what wiring up a client needs. |
| [`QUERIES.md`](docs/QUERIES.md) | What `psql` describes, what `information_schema` describes, and where the two meet. |
| [`DIALECT.md`](docs/DIALECT.md) | Every step needed to add a database, in order, with the test that catches each one you skip. |
| [`EVALUATION.md`](docs/EVALUATION.md) | How the supported version range is chosen, and how to choose one for a database not covered yet. |
| [`USQL.md`](docs/USQL.md) | What `usql` answered for each of the 47 drivers it built at the commit measured, and what changes if it reads `dbmeta`. |
| [`DBTPL.md`](docs/DBTPL.md) | The same measurement for `dbtpl`. |
| [`DBRUN.md`](docs/DBRUN.md) | How to use `dbrun`, the command that starts the databases the tests run against, and the rules for sharing one machine. |
| [`CONTAINERS.md`](docs/CONTAINERS.md) | How to add a container or a virtual machine that `dbrun` can start. |
| [`BACKLOG.md`](docs/BACKLOG.md) | Work that is known and not done, with the decision or the measurement that found each item. |
| [`PROGRESS.md`](docs/PROGRESS.md) | Where the work stands, so that a session that ends or crashes can resume. |
| [`WINDOWS.md`](docs/WINDOWS.md) | The Windows machines that host the SQL Server releases with no Linux container, and why each of them is awkward. |

[`AGENTS.md`](AGENTS.md) holds the rules for writing code here, for a coding
agent, with a table saying which document to read for which task.
[`CLAUDE.md`](CLAUDE.md) imports it, so that Claude Code reads the same rules.
[`CONTRIBUTING.md`](CONTRIBUTING.md) is the same for a person, and shorter.

# Testing

`container/` names every server release that `dbrun` starts, as Go data, one
file per product, and `container.All` joins them. Many are there only for
dbimp's drivers, for the flavors usql reaches, or as emulators of hosted
services, and no model reads them. Those are Staged, and CI does not run them
(D119). Each entry holds the image, the tag, the environment, the readiness
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
container, which is D68. `docs/DBRUN.md` says how to use it.

Every database also answers one checked in expectation. `TestConformance`
builds the same core schema on every database with a model and compares the
portable facts, so a difference between two families is either fixed or written
down. The relational databases agree on at least 23 of the canonical lines, and
every database agrees on at least 4,
and [`COVERAGE.md`](docs/COVERAGE.md) records every difference that is not.

# Related Projects

`dbmeta` is one of a set of packages that each do one part of the job, so that
a client can take the parts it needs.

- [`usql`][usql] is a command line client for many databases. It is the reason
  the object model follows `psql`. [`USQL.md`](docs/USQL.md) measures what it
  answers today and what `dbmeta` changes.
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

Read [`AGENTS.md`](AGENTS.md) first. It holds the rules, including the ones
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

`dbrun test tested nightly verified` runs every supported release of every
product, which is what has to pass before a release. `dbrun test all` adds the
Staged releases, which no model reads.

Tests in this module never open a database connection. They render statements,
resolve versions, and read rows from a fake driver replaying recorded data, so
they need nothing installed and they cover releases whose container images no
longer start.

<br/>

<div align="center">
  <a href="https://github.com/xo/usql" title="A command line client for many databases">usql</a> |
  <a href="https://github.com/xo/dburl" title="Database connection URLs">dburl</a> |
  <a href="https://github.com/xo/dbmeta" title="Database metadata, this project">dbmeta</a> |
  <a href="https://github.com/xo/dbimp" title="Database drivers in pure Go">dbimp</a> |
  <a href="https://github.com/xo/cassandra" title="A database/sql driver for Cassandra">cassandra</a> |
  <a href="https://github.com/xo/dbtpl" title="Go code generated from a database">dbtpl</a> |
  <a href="https://github.com/xo/tblfmt" title="Tables of database results">tblfmt</a> |
  <a href="https://github.com/xo/rline" title="The line editor of usql">rline</a> |
  <a href="https://github.com/xo/transit" title="tree-sitter in pure Go">transit</a>
</div>

[usql]: https://github.com/xo/usql "usql"
[dbtpl]: https://github.com/xo/dbtpl "dbtpl"
[dburl]: https://github.com/xo/dburl "dburl"
[tblfmt]: https://github.com/xo/tblfmt "tblfmt"
[psql]: https://www.postgresql.org/docs/current/app-psql.html "psql"
