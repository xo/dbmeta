# dbmeta

`dbmeta` holds database metadata queries and models in one Go module. `usql`
and `dbtpl` consume it.

`dbmeta` only reads. It does not render metadata and it does not write it. The
`Reader` and `Writer` pair from `usql` is a `usql` concept and it does not
come here.

Read `NULLS.md` and `PLAN.md` before you change anything. `PLAN.md` records the
architecture, the decisions, the known defects, and the questions that nobody
has answered yet. Do not decide an open question on your own. Ask Ken.

`REVIEW.md` holds the argument behind two decisions that changed exported API,
both taken in D49. Read it before undoing either: the reasoning is the part
that is easy to lose.

The other documents: `COMMANDS.md` maps every `psql` metadata command to the Go
value that answers it. `COVERAGE.md` says what each database can and cannot
answer, and which analogues were rejected. `USQL.md` and `DBTPL.md` measure
what the two consumers read today and what `dbmeta` would have to add before
either can move onto it. D46 holds that list: five object kinds, three of which
both consumers need.

## Hard rules

1. The root module depends on the standard library and on nothing else. Its
   `go.mod` has no `require` block and must keep it that way. Never add a
   database driver, not even for a test. A consumer brings its own driver and
   picks its own version of it, and `dbmeta` must not constrain that. Anything
   needing a driver goes in the `test` module. Do not add any third party
   package without asking Ken first.
   D19 once made `github.com/xo/dburl` a direct dependency. It never became
   one, because nothing here opens a connection: the caller passes a `DB` and
   names a `Dialect`, so there is no URL to parse. That is the better answer
   and it stands.
   The rule that outlived the dependency is this one. Never write a scheme
   list, an alias list, a flavor table, or a connection string parser in this
   module. That taxonomy is `dburl`'s and repeating it is how two copies start
   disagreeing. A consumer that has a URL reads `dburl` itself:
   `URL.Driver` names the family and selects the model package,
   `URL.UnaliasedDriver` names a wire compatible product such as `cockroachdb`,
   and `URL.OriginalScheme` holds an alias such as `mariadb`. A flavor arrives
   on one of the last two, never on both, so a consumer reads both.
2. PostgreSQL is the primary model, and `psql` defines it. When two databases
   describe the same object differently, follow `psql`. This decides the shape
   of an answer. It does not require every database to answer every question.
   `psql` sets the object model. It does not set the column set, and it never
   did: `psql` prints what a person reads at a terminal, and a consumer needs
   the parts. Return a fact `psql` does not print when one statement can
   produce it, and never return prose as the only form of anything. D47 holds
   the cost test, and rule 13 is the short version.
3. A query that must differ between releases is one query with version
   fragments, held as generated data in the one model package. There is no
   package per release. A fragment must never change the set of columns a
   query returns. Whichever side lacks a source pads, old or new: select the
   column as `NULL AS name` on the server that has no source for it. A type
   that differs between versions is cast to a common one, never left to `any`.
   Where two products share a dialect, a fragment for one of them gates on
   that product's version key and never on the number alone. MariaDB 11.8 and
   MySQL 9 have no numeric relation, so a gate at `V(10, 2)` silently means
   "MariaDB only" and shipped a wrong answer once already. Write
   `Gate{Key: MariaDB, Min: V(10, 2)}`. Set a key only for a product you
   detected. See D44.
4. Stay backward compatible within reason. An old database keeps working when
   support for a new one arrives. Every version sits in one of three tiers:
   Tested in CI, Verified on a development machine before a release, or
   Archived with no tests at all. Never call a version supported without naming
   its tier. See D40.
5. A query translated from a source tree that upstream no longer ships records
   the release and commit of that tree beside the query. PostgreSQL 9.6 is the
   case: `psql` dropped it in release 20, so there is nothing current to check
   a translation against.
6. Configuration is a value that the caller owns. Do not put a configured
   value in a package level variable. One driver borrowed another driver's
   configuration in `usql` and shipped a fault to users.
7. Do not import `usql` or `dbtpl`. The dependency runs the other way.
8. Every function that reads metadata takes `ctx context.Context` as its first
   parameter. There are no exceptions. Call `QueryContext` and its relatives,
   never `Query`, `Exec`, `QueryRow`, or `Prepare`. Never call
   `context.Background()` or `context.TODO()` inside this library.
9. Every model ships both its queries and its fixtures. The fixture creates one
   of every object the queries read, so a test gets rows worth checking. It is
   versioned like the queries, because syntax and objects arrive in different
   releases, and a step the server is too old for is skipped rather than
   refused. A query that has never run against a real server is not finished.
10. The root module is pure Go and has no cgo, ever. It has no driver at all,
   so there is nothing to argue about: a consumer builds it with
   `CGO_ENABLED=0` and cross compiles it.
   The `test` module may use cgo, because its own `go.mod` keeps it out of
   everything a consumer builds. Where a database has a canonical cgo driver
   and a pure Go one, test both: the canonical driver is what users run, and
   the pure Go one is what a consumer who cannot use cgo runs, and the two
   ship different library versions. Two drivers need cgo today, and only two:
   `mattn/go-sqlite3`, which builds the real SQLite source and is the primary
   SQLite driver, and the coming DuckDB driver.
   Use the pure Go driver where one is enough: `jackc/pgx` or `lib/pq`,
   `go-sql-driver/mysql`, `modernc.org/sqlite`, `microsoft/go-mssqldb`,
   `sijms/go-ora`, `gocql/gocql`. Never `godror`, which needs Oracle client
   libraries rather than only a C compiler. See D48.
11. Never write a `//go:build` constraint on an operating system or an
   architecture, and never branch on either. Testing is `linux/amd64` only.
   The same database version is assumed to answer the same way everywhere.
   A driver may carry its own platform builds, which is the driver's business.
12. Write idiomatic Go. This code is a move of an older package, so a pattern
   being present in the source is not a reason to keep it. See D18 in
   `PLAN.md` for the two patterns that must not carry over.
13. `dbmeta` supplies the data and the consumer decides what to show. Never
   withhold a fact, reorder one, or shape a result so that somebody's output
   looks right. `usql` filters to match `psql` and `dbtpl` shows none of it,
   and one set of queries serves both because no presentation is baked in.
   A field may be added when one statement can produce it: a column already in
   the row, a column reached by a join, or a correlated subquery whose plan
   stays bounded as the catalog grows. It may not when it needs a second
   statement, a per row round trip, or a scan that grows with the whole
   catalog. Check a new one with `EXPLAIN` against a catalog with thousands of
   tables, not against a fixture with five.
   A child of an object is its own kind with flat rows, never a slice on the
   parent, because filling a slice needs a second statement or a dialect
   specific aggregate. See D47.
14. A new dialect is not finished until several AI models have been asked
   about the queries it cannot answer. Consult at least two of Gemini,
   DeepSeek and Astra, and ask each one to sort the unanswered queries into
   three groups: absent from the product, present under another name, and
   derivable from several catalog reads or one complex statement. A first pass
   only finds the objects that the product names the way PostgreSQL names
   them, and MariaDB proved the cost: 29 queries looked unanswerable until a
   second opinion named the tables that hold four of them. Treat every answer
   as a lead and run it against a real server. Leave an analogue that is a
   stretch unsupported, and record it in `COVERAGE.md` with the reason. See
   D43.

## Layout

- `/` is the root package `dbmeta`. It holds the driver agnostic API: the
  object types, the `Query` values, the one method `Querier` interface, the
  `Args` filter, and the error values. External projects use this package.
- `models/<driver>` holds the code that `dbtpl` generates for one driver. One
  package covers every supported version of that database. For example,
  `models/sqlite3`. Do not edit generated files. Change the SQL and generate
  again.
- `internal/` holds one file per model, such as `internal/postgres.go`, gated
  by the build tags `none`, `base`, `most` and `all`, following `usql`. A
  `gen.go` generates those files and the model table in `README.md`. Say model,
  not driver: `dbmeta` never opens a connection.
- `container/` names every database release the tests run against, as Go data.
  It starts no container and imports no container client. `test/run.sh` and the
  CI workflow both read it, and a test fails when they drift.
- `test/` is a separate module with its own `go.mod`. It holds the integration
  tests, the database drivers, and the `tool` directive pinning `dbtpl`. None
  of that may appear in the root module. It is the only place cgo is allowed,
  and the separate `go.mod` is what makes that safe.

Read `dbtpl/models` before you design anything. It shows what a working
generated metadata model looks like.

## Generating a model

`dbtpl` generates every file under `models/`. `dbmeta` pins `dbtpl` with the
`tool` directive in `go.mod`, so run it as a tool and not from the path:

```bash
go tool dbtpl query <url> -T <Type> -F <Func> --go-pkg <driver> -o models/<driver>
```

`dbtpl query` introspects a live connection. It creates a temporary view from
your statement, reads the column types, and drops the view. There is no offline
mode, so the database must be running before you generate.

Start the database from the container configuration in `usql/contrib`:

```bash
/home/ken/src/go/src/github.com/xo/usql/contrib/podman-run.sh postgres
```

`usql/contrib/config.yaml` holds the connection URL for each container. Phase 2
uses MariaDB, and its directory is named `mysql`, not `mariadb`.

Read `dbtpl/gen.sh` for the full pattern. It runs 60 such commands and shows
how to hold each SQL statement in a shell heredoc.

Two flags prevent the recurring NULL scan fault. Pass `-U` to make a field
nullable when the column allows NULL. Pass `-Z` to force the field type by hand
when introspection reports NOT NULL and the database still returns NULL.

## Go conventions

These apply to hand written code. Generated code follows the `dbtpl`
templates instead. See question 4 in `PLAN.md`.

Wrap every error with `%w`, never `%s` or `%v`:

```go
if err != nil {
    return nil, fmt.Errorf("reading columns for %s: %w", table, err)
}
```

Write error messages in lower case, starting with a gerund. Do not write
"failed to" or "error". Name the object that failed.

Never hide a NULL. Read `NULLS.md` before writing a query for any database. It
is the shortest document here and it is the one that has cost the most to
learn.

In brief: never wrap a nullable catalog column in `COALESCE`, and never pad an
absent column with a literal. Give the field the type `Text` and select
`NULL AS "name"`. Before padding, ask whether the value on the old release is
unknown or genuinely that value, and set `Field.Min` only for the first.

Declare sentinel errors as constants of a defined string type, never as
variables. A `var` declared with `errors.New` can be reassigned by any
importer. A constant cannot.

```go
// Error is an error.
type Error string

// Error satisfies the error interface.
func (err Error) Error() string {
	return string(err)
}

// Error values.
const (
	// ErrNotSupported is the not supported error.
	ErrNotSupported Error = "not supported"
)
```

Keep the `Err` prefix and the PascalCase name. Compare with `errors.Is`.

Accept interfaces and return concrete structs. Keep an interface to three
methods or fewer. Define an interface where it is consumed, not where it is
implemented.

Do not name a type `Reader`. Everything in this module reads, so the word adds
nothing and repeats the package.

Return an iterator, `iter.Seq2[Table, error]`. Read the result one record at a
time and never materialize it into a slice. Do not write a cursor type. An
error can arrive part way through a result, so a caller must be able to tell
the end of a result from a failure in the middle of one.

An iterator holds a database connection until it ends. Stopping early, by
`break` or by `yield` returning false, must close the rows and release the
connection, and so must cancelling the context. A caller that nests one
iterator inside another uses a second connection, which deadlocks on a pool of
one. Document the filter form that avoids nesting.

Use generics rather than `interface{}` for a container of one type. Compare
errors with `errors.Is` and `errors.As`, never by string.

Put `context.Context` first in every parameter list and name it `ctx`. Never
store it in a struct.

Name a package with one short lower case word. Do not stutter: write
`postgres.Reader`, not `postgres.PostgresReader`.

Name a receiver with one or two lower case letters, and use the same name on
every method of the type. Never write `this` or `self`.

Group struct fields by purpose, with exported fields first and a blank line
between groups.

## Linting

`.golangci.yml` holds the rules, and `test/.golangci.yml` holds them for the
test module. golangci-lint reads the `go.mod` of the directory it runs in, so
both are linted separately and CI runs it twice.

```bash
golangci-lint run ./... && (cd test && golangci-lint run ./...)
```

The posture is `default: all` with a disable list. A new linter gets a look
rather than silence, and that has paid: turning it on found a missing
`rows.Err` check, a parameter shadowing the `min` builtin, and a copy loop that
`maps.Copy` now does.

### The rule for a lint finding

A linter that makes idiomatic Go worse is disabled, with the reason written
beside it in the configuration. Only a real defect gets a code change. Ask
which one it is before you touch the code, and if the remedy reads worse than
what it replaces, that is the answer.

A change made to quiet a linter rather than to fix something is itself a
defect. It leaves code that no reader can explain and that the next person
copies. Two were written here and reverted, and they are named so that they do
not come back:

Never write `defer func() { _ = rows.Close() }()`. Write `defer rows.Close()`.
A deferred `Close` returns an error nothing can act on, and the error that
matters was already reported by `Err`. `errcheck` is configured to allow it.

Never add an empty `case X:` to satisfy `exhaustive`. A switch that handles two
of three values and answers the third after the switch says more than a case
with no body. `exhaustive` is disabled.

A guard that can never fire is not a fix. `gosec` wanted a bound on a
conversion of a PostgreSQL `server_version_num`, which is six digits, and the
guard first written for it tested the wrong quantity and could never have
fired. That warning is excluded with its reason.

Read `golangci-lint run` output as a list of questions, not a list of tasks.

## Container images

`container/container.go` names every database release dbmeta is tested
against, as Go data. It starts nothing and imports no container client, and it
must not: a consumer brings its own podman, docker or Go client and picks its
own version of it, the same way it brings its own driver.

An embedded database is not in there and must not be. SQLite has no server and
no container, and its release is whichever one the pinned Go driver ships, so
it is tested in a CI job that starts nothing. The same will be true of DuckDB.
See D42.

That list is the only copy. `test/run.sh` reads it through
`test/tool/servers`, the CI workflow repeats it in YAML, and
`container/workflow_test.go` fails when the workflow and the Go list disagree.
Change `container/container.go` first and the workflow second.

Add a release there before you add it anywhere else.

## Before you commit

Run these:

```bash
gofmt -l . && go vet ./... && go build ./... && go test -race -count=2 ./...
golangci-lint run ./... && (cd test && golangci-lint run ./...)
```

`gofmt -l .` must print nothing.

Run it with `-count=2`, because that is what CI runs and a weaker local command
has already let two failures through. Both were the same fault: a test body
that registers a dialect or a query, which a registry rejects the second time.
Registration belongs in `init`.

That does not cover the `test` module. `./...` does not descend into a
directory that has its own `go.mod`, so run the integration tests separately:

```bash
cd test && go test ./...
```

A test in the root module never opens a database connection. Test fragment
merging, version parsing and model selection there, against golden files. A
test that needs a server belongs in the `test` module.

CI runs on GitHub Actions, on `ubuntu-latest` only. The queries are SQL and the
code is pure Go, so `dbmeta` does not need the runner matrix that the other
`xo` projects use. Keep the workflow small.

CI tests four databases at their latest version: PostgreSQL, MySQL, SQLite3 and
DuckDB. It does not test older versions and it does not test flavors. Those run
on a development machine, and they must run before a release. Do not add a
version matrix to the workflow. See D24 in `PLAN.md`.

Two facts about the runner. The preinstalled PostgreSQL is 16, not the latest
release, so testing the newest PostgreSQL needs a service container. The
preinstalled MySQL server is MySQL, not MariaDB, which is intentional: MariaDB
is the reference product and MySQL is the flavor, so CI covers the flavor for
free. Do not replace it with MariaDB.

## Writing documentation

Write plain English. Use short sentences and the active voice. Use `can`,
`will`, and `must`, and do not use `should`, `may`, or `might`. Do not use
semicolons or em dashes. Put the condition before the command: "If the query
returns no rows, return the empty set."
