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

`dbmeta` depends on the standard library and on [`dburl`][dburl]. It imports no
database driver. The caller opens the connection with the driver of its choice.

# Using

Import the package and the models for the databases you need:

```go
import (
    "github.com/xo/dbmeta"
    _ "github.com/xo/dbmeta/all"
)
```

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
| PostgreSQL | native             | 48      | Complete    |
| any with an information_schema | shared | 7 | Ready to build on |
| MariaDB    | native             | 23      | Complete    |
| MySQL      | native, untested   | 23      | Shares the MariaDB model |
| SQL Server | shared, planned    | 0       | Not started |
| DuckDB     | shared, planned    | 0       | Not started |
| SQLite3    | native, planned    | 0       | Not started |
| Oracle     | native, planned    | 0       | Not started |
| Cassandra  | native, planned    | 0       | Not started |

A native model reads the catalog the database keeps for itself. A shared model
reads `information_schema`, which is a smaller answer that many databases have.
It answers 7 object kinds where the native PostgreSQL model answers 48, and
answers none of them completely: no size, owner or access method for a table,
no storage or index detail for a column, no exclusion constraint, no aggregate.

SQLite3, Oracle and Cassandra have no `information_schema` at all and need a
native model. `usql` builds on the same shared reader today for DuckDB, SQL
Server, Snowflake, Trino, Databend and Netezza, which is the evidence for who
the shared model serves.

[`COVERAGE.md`](COVERAGE.md) says what each database answers, what it cannot,
and which analogues were found and rejected. MariaDB answers 23 of the 48
because a native model beats the shared one by sixteen.

A model ships its queries and a fixture together. The fixture is a known good
schema containing one of every object the queries read, exported so that other
projects can generate against it.

# Version Support

Every version sits in one of three tiers. Read the tier before you rely on a
version.

| Tier     | What it means                                                      |
| -------- | ------------------------------------------------------------------ |
| Tested   | Tests run on every change, in CI                                     |
| Verified | Tests run on a development machine before a release                  |
| Archived | The queries exist and were checked once, and nothing runs them now   |

## PostgreSQL

Supported from release 9.6 to release 18, which is ten major versions: 9.6, 10,
11, 12, 13, 14, 15, 16, 17 and 18.

| Releases          | Tier     |
| ----------------- | -------- |
| 9.6, 12, 15, 18   | Tested   |
| 10, 11, 13, 14, 16, 17 | Verified |

Four releases are Tested: CI starts a real server for each and runs the
integration tests on every change. They are the floor, the ceiling and one on
each side of the middle, which is the smallest set that catches every fault
found so far. Testing only the newest would have caught two of six.

The other six are Verified. All ten run nightly in CI, and `test/run.sh` runs
them on a development machine before a release.

Every query is executed against a real server at all ten releases by
`test/run.sh`, which also checks that the columns returned match the fields
declared and that a field the server is too old for arrives as NULL. The
objects PostgreSQL did not have before release 10 are refused there rather than
returning an empty result:
publications, publication tables, subscriptions, extended statistics and
partitioned tables.

Note that `psql` itself dropped support for servers below release 10 in
PostgreSQL 20. `dbmeta` supports 9.6 deliberately, and its queries for that
release are translated from an older checkout.

# Design

[`COMMANDS.md`](COMMANDS.md) maps every `psql` metadata command to the Go value
that answers it, which is what wiring up a client needs.

Read [`NULLS.md`](NULLS.md) before writing a query for any database. It is the
shortest document here and the one that cost the most to learn.

[`COVERAGE.md`](COVERAGE.md) records what each database answers and why, and
names the analogues that looked right and were rejected.

The full record is in [`PLAN.md`](PLAN.md), which holds every decision and the
evidence behind it. [`QUERIES.md`](QUERIES.md) surveys what `psql` and
`information_schema` each describe. [`EVALUATION.md`](EVALUATION.md) records
how the supported versions were chosen. [`CLAUDE.md`](CLAUDE.md) holds the
rules for writing code here.

Four points are worth knowing before reading the source.

**The caller drives.** `dbmeta` does not open a connection, import a driver,
detect a dialect or detect a version. The caller supplies all four.

**Version differences are data, not packages.** Each piece of a statement
carries alternatives with a minimum server version, and the applicable ones are
merged at run time. There is one package per database, not one per release.

**A query returns the same columns on every version.** Where a server has no
source for a column, the statement selects a literal NULL under the same name.
That rule is what lets one Go type read the result of every supported version.
Each column declares the version it arrived in, so a caller can tell a value
that is null from a field the server is too old to have.

**Results stream.** A query returns an iterator rather than a slice, so
`dbmeta` holds no state, caches nothing and never loads a catalog into memory.

# Contributing

Read [`CLAUDE.md`](CLAUDE.md) first. It holds the rules, including the ones
that are not obvious: the standard library and `dburl` only, pure Go with no
cgo, no build constraint on an operating system or an architecture, and a
context on every function that reads from a database.

Run the checks before you send a change, with the same flags CI uses:

```sh
gofmt -l . && go vet ./... && go build ./... && go test -race -count=2 ./...
```

To run the integration tests, start a server and point the test module at it:

```sh
cd test && ./run.sh 18
```

`./run.sh` with no argument runs every supported release, which is what has to
pass before a release.

Tests in this module never open a database connection. They render statements,
resolve versions, and read rows from a fake driver replaying recorded data, so
they need nothing installed and they cover releases whose container images no
longer start.

[usql]: https://github.com/xo/usql "usql"
[dbtpl]: https://github.com/xo/dbtpl "dbtpl"
[dburl]: https://github.com/xo/dburl "dburl"
[psql]: https://www.postgresql.org/docs/current/app-psql.html "psql"
