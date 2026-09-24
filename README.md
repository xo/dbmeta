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

| Database   | Model        | Queries | Status      |
| ---------- | ------------ | ------- | ----------- |
| PostgreSQL | native       | 48      | Complete    |
| MariaDB    | planned      | 0       | Not started |
| MySQL      | planned      | 0       | Not started |
| SQLite3    | planned      | 0       | Not started |
| SQL Server | planned      | 0       | Not started |
| Oracle     | planned      | 0       | Not started |
| Cassandra  | planned      | 0       | Not started |

A native model reads the catalog the database keeps for itself. A generic model
reads `information_schema`, which is a smaller answer that many databases share.

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

| Releases   | Tier     |
| ---------- | -------- |
| 18         | Tested   |
| 9.6 to 17  | Verified |

All 48 queries were executed against a live PostgreSQL 18 server, and the 43
that apply were executed against a live PostgreSQL 9.6 server. The five that do
not apply are the objects PostgreSQL did not have before release 10:
publications, publication tables, subscriptions, extended statistics and
partitioned tables. Asking for one of those on an older server reports that the
version is too old rather than returning an empty result.

Note that `psql` itself dropped support for servers below release 10 in
PostgreSQL 20. `dbmeta` supports 9.6 deliberately, and its queries for that
release are translated from an older checkout.

# Design

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

Run the checks before you send a change:

```sh
gofmt -l . && go vet ./... && go build ./... && go test ./...
```

Tests in this module never open a database connection. They render statements,
resolve versions, and read rows from a fake driver replaying recorded data, so
they need nothing installed and they cover releases whose container images no
longer start.

[usql]: https://github.com/xo/usql "usql"
[dbtpl]: https://github.com/xo/dbtpl "dbtpl"
[dburl]: https://github.com/xo/dburl "dburl"
[psql]: https://www.postgresql.org/docs/current/app-psql.html "psql"
