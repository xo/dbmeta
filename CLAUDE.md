# dbmeta

`dbmeta` holds database metadata queries and models in one Go module. `usql`
and `dbtpl` consume it.

`dbmeta` only reads. It does not render metadata and it does not write it. The
`Reader` and `Writer` pair from `usql` is a `usql` concept and it does not
come here.

One statement is the exception and it is a narrow one. `Dialect.ChangePassword`
builds the statement that sets a password and returns it as text, because the
statement and its escaping are per product knowledge and a password cannot be
bound as a parameter. It takes no database and runs nothing, so everything
`dbmeta` executes is still a read. Do not widen that. See D56.

## Which document to read

Two before anything else. `docs/NULLS.md` is the shortest and the one that cost
the most to learn. `docs/PLAN.md` holds every decision, with a table of all 70
at the top. Read the status, because 10 of them amend or replace an earlier
one. Do not decide an open question on your own. They are at the end of
`docs/PLAN.md`. Ask Ken.

Then by what you are doing:

| If you are | Read |
| --- | --- |
| writing or changing any query | `docs/NULLS.md`, then `docs/COVERAGE.md` |
| asking why something is the way it is | the table at the top of `docs/PLAN.md` |
| adding an object kind | D46 and D47 in `docs/PLAN.md`, then `docs/COMMANDS.md` |
| adding a database | D66 for which one is next, `docs/EVALUATION.md` for the version range, D43 for the rule about asking other models, D52 for which driver to test with, D61 for the principals to measure it against |
| wondering which database comes next | D66, which holds the order and why a product with no container is last |
| a parity failure | D61. Decide whether the query began depending on who is asking, or whether one release genuinely differs, and record it |
| changing what a database answers | `docs/COVERAGE.md`, which is the record of what each one can do |
| changing a fixture | D53, then run the fixture test in the root module. A core object belongs in every fixture |
| a conformance failure | D53. Decide whether it is a fault or a product difference, then fix it or rewrite the expectation and say why |
| deciding whether a field belongs here | D47 in `docs/PLAN.md`, which holds the cost test |
| wiring up a client | `docs/COMMANDS.md`, then `docs/USQL.md` or `docs/DBTPL.md` |
| designing the object set | `docs/QUERIES.md`, the survey of psql against information_schema |
| adding a release to CI | `container/container.go`, which is the only copy of that list |
| adding an old SQL Server that needs a Windows VM | `container/windows.go`, then `docs/WINDOWS.md` and D57 |
| starting a database for any reason | `dbrun`, and nothing else. `cd test && go run ./cmd/dbrun help`. See D68 and `docs/RUNNER.md` |
| changing how a database is started | `docs/RUNNER.md`, the design of that command |
| provisioning a Windows machine | `docs/WINDOWS.md`, then `container/windows.go` and D57 |
| testing against Cassandra | `test/cmd/dbrun/image/cassandra.Containerfile`. `dbrun` embeds it and builds it when the image is missing |
| changing the CI workflow | D69. It reads the release list from `dbrun list --json` and names no image of its own |
| answering a lint finding | the rule below, under Linting |
| ignoring a build artifact | the root `.gitignore`, which is the only one. See D58 |

`CONTRIBUTING.md` is the same thing for a person, and shorter.

A document that is not in that table does not exist. If you cannot find where
something is written down, it is not written down, and it is an open question.

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
   SQLite driver, and `duckdb/duckdb-go`.
   Never `godror`, which needs Oracle client libraries rather than only a C
   compiler. See D48.
   Every driver in the `test` module is the one `usql` uses for that database.
   The version may differ and the package may not. Oracle is the one exception
   and D59 says why: the `go-ora/v3` that `usql` pins panics rather than
   connecting on 11g and 18c. It is fixed upstream and untagged, so Oracle uses
   v2 until v3 tags the fix, and then goes back. `usql` marks them, so
   `grep -rn "// DRIVER" usql` is the list, and it is the first thing to check
   before adding a driver or a dialect. A query that works here and fails on
   the driver `usql` ships is a query that does not work. See D52.
11. Never write a `//go:build` constraint on an operating system or an
   architecture, and never branch on either. Testing is `linux/amd64` only.
   The same database version is assumed to answer the same way everywhere.
   A driver may carry its own platform builds, which is the driver's business.
12. Write idiomatic Go. This code is a move of an older package, so a pattern
   being present in the source is not a reason to keep it. See D18 in
   `docs/PLAN.md` for the two patterns that must not carry over.
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
   stretch unsupported, and record it in `docs/COVERAGE.md` with the reason. See
   D43.
15. Never start a container by hand. `test/cmd/dbrun` starts every database
   this project uses, and every container is named `<product>-<release>`, such
   as `postgres-18` or `clickhouse-26.9`. Run
   `cd test && go run ./cmd/dbrun help`: it has `start`, `stop`, `remove`,
   `status`, `version`, `dsn`, `usql` and `test`, and it takes `all` or a tier
   name where it takes a server.
   This is not tidiness. A container started by hand gets a port somebody
   typed rather than the one `container.All` assigns, so `dbrun version`
   cannot reach a server that is plainly running, and two copies of the same
   release end up on the machine with nothing to tell them apart. See D68.
16. A dialect is not finished until every query has been asked as the
   administrator and as every lesser kind of principal the product has, and
   the differences are written down. Add the principals to `parityTargets` in
   `test/parity_test.go`, run `go test -run TestPrivilegeParity -update`, and
   read the diff.
   A principal is not one thing. SQL Server has three: a sysadmin, a server
   login mapped to a database user, and a contained database user whose
   password is in the database and which has no login at the server. Oracle
   has the same three from 12c, where a common user is the login and a local
   user in a pluggable database is the contained user. PostgreSQL and MySQL
   have no containment, so each has a superuser or root, an owner, and a
   grantee. SQLite and DuckDB have no user at all and the rule cannot reach
   them.
   This is not a formality. It found six MariaDB queries that are refused
   outright for a user holding ALL PRIVILEGES on its own database, because
   they read tables in the `mysql` database rather than views that filter
   themselves. Nothing had recorded that. See D61.

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
  It starts no container and imports no container client. `test/cmd/dbrun` and
  the CI workflow both read it, and a test fails when they drift.
- `test/cmd/dbrun` is the only thing that starts a database, container or
  Windows machine alike. `container/windows.go` holds the machine list, and
  `dbrun provision` builds one from the payload it embeds in
  `test/cmd/dbrun/oem/`. Read `docs/WINDOWS.md`. See D57 and D68.
- `test/cmd/dbrun/image/` holds the Containerfiles this repository builds.
  The Apache Cassandra image cannot be configured from the outside for what
  the queries read, so the settings are baked in. `dbrun` embeds the file and
  builds the image when it is missing.
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

Start the database with `dbrun`, which is the only thing that starts one, and
ask it for the URL to pass to `dbtpl`:

```bash
cd test && go run ./cmd/dbrun start postgres && go run ./cmd/dbrun dsn postgres
```

`dbrun start` leaves the server running, which is what generating a model
needs. Remove it with `dbrun remove postgres` when you are done. See D68.

Read `dbtpl/gen.sh` for the full pattern. It runs 60 such commands and shows
how to hold each SQL statement in a shell heredoc.

Two flags prevent the recurring NULL scan fault. Pass `-U` to make a field
nullable when the column allows NULL. Pass `-Z` to force the field type by hand
when introspection reports NOT NULL and the database still returns NULL.

## Go conventions

These apply to hand written code. Generated code follows the `dbtpl`
templates instead. See question 4 in `docs/PLAN.md`.

Wrap every error with `%w`, never `%s` or `%v`:

```go
if err != nil {
    return nil, fmt.Errorf("reading columns for %s: %w", table, err)
}
```

Write error messages in lower case, starting with a gerund. Do not write
"failed to" or "error". Name the object that failed.

Never hide a NULL. Read `docs/NULLS.md` before writing a query for any database. It
is the shortest document here and it is the one that has cost the most to
learn.

In brief: never wrap a nullable catalog column in `COALESCE`, and never pad an
absent column with a literal. Give the field the type `sql.Null[string]`, or
`sql.Null[T]` for whatever T it is, and select `NULL AS "name"`. There is no
alias for a nullable type and there must not be one: a reader of
`go doc dbmeta.Sequence` learns from the field itself that it can be absent,
and would not from a name like `Text`. See D51. Before padding, ask whether the value on the old release is
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

An embedded database is not in there and must not be. SQLite and DuckDB have no
server and no container, and the release is whichever one the pinned Go driver
ships, so both are tested in a CI job that starts nothing. See D42.

That list is the only copy. `test/cmd/dbrun` reads it, the CI workflow builds
its matrix from `dbrun list --json --names`, and
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

CI runs a release matrix, which D42 decided and which replaced the single
latest version D24 first called for. Every push runs PostgreSQL 9.6, 12, 15 and
18, MariaDB 10.6 and 13.0, MySQL 8.4 and 26.7, SQL Server 2017, 2019, 2022 and
2025, Oracle 21c and 26ai, Cassandra 3.11 and 5.0, and SQLite3 and DuckDB in a
job that starts no container. The remaining releases run nightly.

The Cassandra job builds its image first rather than naming a service, because
the published one refuses what the queries read. See D62 and
`test/cmd/dbrun/image/cassandra.Containerfile`.

Do not write that list in the workflow from memory. It lives in
`container/container.go`, `container.AtTier` selects a tier, and
`TestWorkflowMatchesTheList` fails when the YAML and the Go disagree. Change the
Go first and then the YAML. See D42 and D54.

Two facts about the runner. The preinstalled PostgreSQL is 16, not the latest
release, so testing the newest PostgreSQL needs a service container. The
preinstalled MySQL server is MySQL, not MariaDB, which is intentional: MariaDB
is the reference product and MySQL is the flavor, so CI covers the flavor for
free. Do not replace it with MariaDB.

## Writing documentation

A new document goes in `docs/`. Only `README.md`, `CLAUDE.md` and
`CONTRIBUTING.md` belong in the repository root, and a test enforces that. Add
it to the table at the top of this file and to the one in `README.md`, because
a document nobody can find is a document nobody reads. See D50.

A decision goes in `docs/PLAN.md` and nowhere else. Put its status in the
heading, after the title: `Decided`, or `Amends D26`, or `Superseded by D24`.
The index at the top is generated from those headings and a test checks it. If
your decision changes an earlier one, say so in both headings, because a reader
who finds the older one has to be told.

Write plain English. Use short sentences and the active voice. Use `can`,
`will`, and `must`, and do not use `should`, `may`, or `might`. Do not use
semicolons or em dashes. Put the condition before the command: "If the query
returns no rows, return the empty set."
