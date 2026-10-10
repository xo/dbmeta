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

Every quoting rule lives here, and none in a consumer. A rule two products
share goes in the root package, as `QuoteLiteral` and `QuoteIdentifier` do,
and a product's own goes beside its model. See D127.

## Standing rules

These hold in every `xo` repository, for every coding agent (D110).

1. Stage changes for review. Commit and push only when Ken says so.
2. Load the `simple-english` skill before you write any text that a person
   reads: a document, a code comment, an error message or a commit message.
   Follow it for that text.
3. Load the `go-pedantry` skill before you write or review Go code. Follow it
   where it does not conflict with a rule in this file. A rule here wins.

`CLAUDE.md` holds one line that imports this file, so that Claude Code and
every other agent read the same rules. Edit this file, not that one.

## Which document to read

Two before anything else. `docs/NULLS.md` is the shortest and the one that cost
the most to learn. `docs/decisions/` holds every decision, one file each, and
`docs/decisions/README.md` is a table of all 211. Read the status, because 76
of them amend or replace an earlier one. Do not decide an open question on
your own. They are at the end of `docs/PLAN.md`. Ask Ken.

Then by what you are doing:

| If you are | Read |
| --- | --- |
| writing or changing any query | `docs/NULLS.md`, then `docs/COVERAGE.md` |
| asking why something is the way it is | the index in `docs/decisions/README.md` |
| adding an object kind | D46 and D47 in `docs/decisions/`, then `docs/COMMANDS.md` |
| adding a database | `docs/DIALECT.md`, which is every step in order. It points at D66, `docs/EVALUATION.md`, D43, D154, D38 and D61 |
| wondering which database comes next | D66, which holds the order and why a product with no container is last |
| a parity failure | D61. Decide whether the query began depending on who is asking, or whether one release genuinely differs, and record it |
| changing what a database answers | `docs/COVERAGE.md`, which is the record of what each one can do |
| changing a fixture | D53, then run the fixture test in the root module. A core object belongs in every fixture |
| a conformance failure | D53. Decide whether it is a fault or a product difference, then fix it or rewrite the expectation and say why |
| deciding whether a field belongs here | D47 in `docs/decisions/`, which holds the cost test |
| wiring up a client | `docs/COMMANDS.md`, then `docs/USQL.md` or `docs/DBTPL.md` |
| designing the object set | `docs/QUERIES.md`, the survey of psql against information_schema |
| adding a release to CI | the product's file in `container/`, which is the only copy of its list |
| adding an old SQL Server that needs a Windows VM | `docs/DIALECT.md` for where the steps differ, then `docs/WINDOWS.md`, `container/windows.go` and D57 |
| starting a database for any reason | `dbrun`, and nothing else. `docs/DBRUN.md` holds its use and the rules for a shared machine. Read those rules first |
| adding a container or a machine that dbrun starts | `docs/CONTAINERS.md`, every step in order |
| reaching a hosted service, such as Snowflake | `docs/DBRUN.md`, under Hosted services, and D117. Never enter a credential yourself |
| running a product that needs a license file, such as Stardog | `docs/DBRUN.md`, under License files, and D118. Never sign up for one or download one yourself |
| adding an entry for a product dbimp or usql reaches | D118, which holds the rules the entries follow and what each one cost to measure |
| changing how a database is started | D68, D70, D75, D86, D97, D98, D105, D108 and D115 in `docs/decisions/`, then `docs/DBRUN.md` |
| changing how dbrun calls podman or docker | D122. Read JSON, name many containers in one call, and do not ask twice |
| provisioning a Windows machine | `docs/WINDOWS.md`, then `container/windows.go` and D57 |
| testing against Cassandra | `test/cmd/dbrun/image/cassandra.Containerfile`. `dbrun` embeds it and builds it when the image is missing |
| changing the CI workflow | D69. It reads the release list from `dbrun list --json` and names no image of its own |
| answering a lint finding | the rule below, under Linting |
| ignoring a build artifact | the root `.gitignore`, which is the only one. See D58 |
| writing a document, a code comment, an error message or a commit message | the `simple-english` skill. Load it first. See below, under Writing documentation |
| writing or reviewing Go code | the `go-pedantry` skill. Load it first. See Standing rules |
| looking for work that is known and not done | `docs/BACKLOG.md` |
| resuming a session that ended or crashed | `docs/PROGRESS.md`, which says where the work stands |
| adding or updating an agent skill | `CONTRIBUTING.md`, under Agent skills, D89 and D110 |

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
   `URL.Dialect`, from dburl v0.32.0, is the dbmeta `Dialect` and selects the
   model package, and `URL.OriginalScheme` holds an alias such as `mariadb`.
   A product that speaks another product's protocol, such as `cockroachdb`
   or `tidb`, has a `Dialect` of its own from dburl v0.36.0, so nothing else
   names it. `URL.Driver` is the name `sql.Open` takes and never the dialect:
   `pgx://` and `postgres://` both have the dialect `postgres`. See D99 and
   D125.
2. PostgreSQL is the primary model, and `psql` defines it. When two databases
   describe the same object differently, follow `psql`. This decides the shape
   of an answer. It does not require every database to answer every question.
   `psql` sets the object model. It does not set the column set, and it never
   did: `psql` prints what a person reads at a terminal, and a consumer needs
   the parts. Return a fact `psql` does not print when one statement can
   produce it, and never return prose as the only form of anything. D47 holds
   the cost test, and rule 13 is the short version.
3. A query that must differ between releases is one query with version
   fragments, held as data in the one model package. There is no
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
   support for a new one arrives. Every version sits in one of five tiers:
   Tested in CI on every push, Nightly in CI once a night, Verified on a
   development machine before a release and never in CI, Staged when no
   model reads it yet, or Archived with no tests at all. Never call a version
   supported without naming its tier, and never call a Staged one supported
   at all. D40 set three, D42 added Nightly and D119 added Staged. The first
   four are values of `container.Tier` and Archived is not, because an
   archived release is one `container.All` does not name. A release is Staged
   exactly when no model reads it. A Staged release records the cadence it
   will have, Tested, Nightly or Verified, and takes it as its tier in the
   change that adds its model. See D40, D42, D119 and D120.
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
   The `test` module can use cgo, because its own `go.mod` keeps it out of
   everything a consumer builds. Where a database has a canonical cgo driver
   and a pure Go one, test both: the canonical driver is what users run, and
   the pure Go one is what a consumer who cannot use cgo runs, and the two
   ship different library versions. Two drivers need cgo today, and only two:
   `mattn/go-sqlite3`, which builds the real SQLite source and is the primary
   SQLite driver, and `duckdb/duckdb-go`.
   Never `godror`, which needs Oracle client libraries rather than only a C
   compiler. See D48.
   Compare the version query too. Adding or changing a dialect means reading
   what `usql` runs for the same product, in the `Version` field of its
   `drivers.Driver`, and recording the comparison in the statements table in
   `docs/USQL.md`. A driver declaring no `Version` falls through to the generic
   `SELECT version();`, and that counts as its statement. Read the field and
   not the `drivers.Register` call: Oracle and godror both register through
   `orshared.Register`, so a grep for the call says neither has a version
   query and both do. The Oracle defect is real and it is a different one.
   `usql` reads `v$instance`, which an ordinary user cannot see, so it prints
   a version for an administrator and none for anybody else. Comparing
   printed output never found that. `docs/USQL.md` holds the measurement.
   `TestEveryModelIsInTheVersionTable` fails when a model has no row. See D38.
   Every driver in the `test` module is the one the `dburl` registry names for
   that dialect: `Scheme.GoPackage` of each scheme whose `Dialect` it is, from
   dburl v0.29.0, with `Scheme.RequiresCGO` saying whether it needs a C
   compiler. dburl is upstream of `dbmeta` and of `usql` both, and `usql` is
   downstream, so its imports do not decide this. The version is the test
   module's own, because the registry carries none. Where two schemes of one
   dialect name two packages, test both as subtests named for the driver:
   SQLite runs on `mattn/go-sqlite3` and `modernc.org/sqlite`, and PostgreSQL
   on `jackc/pgx/v5/stdlib` and `lib/pq`. Parity is the one exception, and
   D52 says why. The `go-ora/v3` release that dburl names panics rather than
   connecting on 11g and 18c, and the fix is upstream and untagged. So the
   Oracle tests pin v3 at the commit that fixes it, and move to the tag when
   one holds the fix (D59, D157). If what `usql` imports differs from
   what dburl names, the fault is in one of those two, and the fix goes
   there. A query that works here and fails on the driver dburl names is a
   query that does not work. See D52, D80 and D154.
11. Never write a `//go:build` constraint on an operating system or an
   architecture, and never branch on either. Testing is `linux/amd64` only.
   The same database version is assumed to answer the same way everywhere.
   A driver can carry its own platform builds, which is the driver's business.
12. Write idiomatic Go. This code is a move of an older package, so a pattern
   being present in the source is not a reason to keep it. See D18 in
   `docs/decisions/` for the two patterns that must not carry over.
13. `dbmeta` supplies the data and the consumer decides what to show. Never
   withhold a fact, reorder one, or shape a result so that somebody's output
   looks right. `usql` filters to match `psql` and `dbtpl` shows none of it,
   and one set of queries serves both because no presentation is baked in.
   A field can be added when one statement can produce it: a column already in
   the row, a column reached by a join, or a correlated subquery whose plan
   stays bounded as the catalog grows. It cannot when it needs a second
   statement, a per row round trip, or a scan that grows with the whole
   catalog. Check a new one with `EXPLAIN` against a catalog with thousands of
   tables, not against a fixture with five.
   A child of an object is its own kind with flat rows, never a slice on the
   parent, because filling a slice needs a second statement or a dialect
   specific aggregate. See D47.
14. A new dialect is not finished until you ask several AI models
   about the queries it cannot answer. `docs/DIALECT.md` holds every step of
   adding one, and this is one of them. Consult at least two of Gemini,
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
   `cd test && go run ./cmd/dbrun help`, which lists every command, and
   `docs/DBRUN.md` holds the same table, and the rules for sharing this
   machine with other sessions. It takes `all` or a tier name
   where it takes a server. The commands most often wanted are `start`,
   `stop`, `remove`, `status`, `version`, `dsn`, `usql` and `test`. The rest
   are in that table and this file does not repeat them.
   This is not tidiness. A container started by hand gets a port somebody
   typed rather than the one `container.All` assigns, so `dbrun version`
   cannot reach a server that is plainly running, and two copies of the same
   release end up on the machine with nothing to tell them apart. See D68.
   If you are a coding agent, set `DBMETA_OWNER_NAME` to the name of your
   session on every `dbrun` command, so that `status` shows whose each server
   is. See D115.
16. A dialect is not finished until you ask every query as the
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
   Parity ships with the queries, the fixture and the documentation. They are
   one deliverable. `TestEveryDialectIsMeasuredForParity` fails when a dialect
   has neither a target nor an entry in `parityExempt` saying why it has none,
   because `TestPrivilegeParity` says nothing about a target nobody wrote.
   A scene the server is too old for carries a `min` and is skipped with the
   reason, the way a fixture step is.
   This is not a formality. It found six MariaDB queries that are refused
   outright for a user holding ALL PRIVILEGES on its own database, because
   they read tables in the `mysql` database rather than views that filter
   themselves. Nothing had recorded that. See D61.

## Layout

- `/` is the root package `dbmeta`. It holds the driver agnostic API: the
  object types, the `Query` values, the one method `Queryer` interface, the
  `Args` filter, and the error values. External projects use this package.
- `models/<driver>` holds one model, written by hand. One package covers every
  supported version of that database. For example, `models/sqlite3`. A model
  registers its queries from `init` and nothing generates it, so edit it
  directly. See D71.
- `all/` holds one file per model, such as `all/postgres.go`, each a blank
  import gated by a build tag. Importing `all` registers the base set, and
  `no_base`, a model's own name and `no_<model>` change which. A model can be
  imported from `models/<driver>` instead. Say model, not driver: `dbmeta`
  never opens a connection. See D31.
- `container/` names every database release the tests run against, as Go data.
  It starts no container and imports no container client. `test/cmd/dbrun` and
  the CI workflow both read it, and a test fails when they drift.
- `hosted/` names the hosted services, such as Snowflake and BigQuery, as Go
  data. It holds no secret. `dbrun` resolves each one's connection string at
  run time, and a service exists in `dbrun` only while it resolves. See D117.
- `test/cmd/dbrun` is the only thing that starts a database, container or
  virtual machine alike. `container/machine.go` holds the machine list, and
  `container/windows.go` holds the Windows machines in it. `dbrun provision`
  builds a Windows machine from the payload it embeds in
  `test/cmd/dbrun/oem/`, and imports an appliance from the file a person
  downloaded. Read `docs/WINDOWS.md`. See D57, D68 and D86.
- `test/cmd/dbrun/image/` holds the Containerfiles this repository builds,
  one for each product whose image dbrun builds. `dbrun` embeds them all and
  builds `<product>.Containerfile` as `localhost/dbmeta/<product>`. See D118.
  The Apache Cassandra image cannot be configured from the outside for what
  the queries read, so the settings are baked in. `dbrun` embeds the file and
  builds the image when it is missing.
- `test/` is a separate module with its own `go.mod`. It holds the integration
  tests and the database drivers. Neither can appear in the root module. It is
  the only place cgo is allowed, and the separate `go.mod` is what makes that
  safe.

Read an existing model before you write one. `models/postgres` is the primary
one and `models/sqlite3` is a small one.

## Writing a model

Nothing generates the code here. Every model is written, and no tool produces
or reproduces it, so a file under `models/` is edited in place. See D71.

A model is one package that registers a `Binding` per object kind from `init`.
The `Binding` carries the statement as `Stmt`, the `Fields` it returns, the
`Params` it takes and a `Scan` function. A query that must differ between
releases is one statement with version fragments, which is rule 3.

Write the statement against a running server rather than from the
documentation. `dbrun` is the only thing that starts one:

```bash
cd test && go run ./cmd/dbrun start postgres && go run ./cmd/dbrun usql postgres
```

`dbrun start` leaves the server up, which is what writing a query needs.
Remove it with `dbrun remove postgres` when you are done. See D68.

Two rules catch the recurring NULL scan fault. Give a field `sql.Null[T]`
whenever the catalog column can be NULL, and select `NULL AS "name"` rather
than a literal where a release has no source for a column. A column that the
catalog declares NOT NULL can still arrive NULL through an outer join, so read
`docs/NULLS.md` before deciding.

A model is not finished until it has run against every release in its tier,
its fixture builds one of every object its queries read, and its parity
targets exist. Those are one deliverable. See rule 9, rule 16 and D61.

## Go conventions

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
and does not learn it from a name like `Text`. See D51. Before padding, ask whether the value on the old release is
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
connection, and so must canceling the context. A caller that nests one
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
guard first written for it tested the wrong quantity and was never able to
fire. That warning is excluded with its reason.

Read `golangci-lint run` output as a list of questions, not a list of tasks.

## Container images

`container/` names every server release that `dbrun` starts, as Go data, one
file per product, and `container.All` in `container/container.go` joins them.
Most are what dbmeta is tested against. The rest are Staged, because no model
reads them: they are there for dbimp's drivers, for the flavors usql reaches,
and for the emulators of hosted services, and CI does not run them (D118,
D119). The package starts
nothing and imports no container client, and it must not: a consumer brings
its own podman, docker or Go client and picks its own version of it, the same
way it brings its own driver.

An embedded database is not in there and must not be. SQLite3, DuckDB,
moderncsqlite, chai and csvq have no server and no container, and the
release is whichever one the pinned Go driver ships. `dbrun` knows them
itself (D116), and CI tests the ones a model reads in the same matrix as the
servers, where `dbrun` starts nothing for them. chai and csvq have no
model yet, so they are Staged (D119). See D42.

That list is the only copy. `test/cmd/dbrun` reads it, the CI workflow builds
its matrix from `dbrun list --json --names`, and
`container/workflow_test.go` fails when the workflow and the Go list disagree.
Change the product's file in `container/` first and the workflow second.

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
latest version D24 first called for. Every push runs the Tested tier, and
`container.AtTier(container.Tested)` is the list. This file does not repeat
it, because the copy written here went stale the moment a product was added,
and it named six products for as long as ClickHouse, Trino, Presto, Firebird,
SAP HANA and Apache Hive went on being added to the tier without it. Read the
list with:

```bash
cd test && go run ./cmd/dbrun list --json --names tested
```

That command adds the embedded databases that a model reads, which are not
in `container.AtTier` at all, and `dbrun` starts nothing for them. D42 keeps an
embedded database out of the release list, and `dbrun` puts them back because
a person asking for the tier wants to run them too. The Nightly tier runs
once a night.

`dbrun` builds the Cassandra image before it tests a Cassandra release, from
the Containerfile it embeds, because the published image refuses what the
queries read. See D62 and `test/cmd/dbrun/image/cassandra.Containerfile`.

Do not write that list in the workflow from memory. It lives in `container/`,
`container.AtTier` selects a tier, and
`TestWorkflowReadsTheList` fails when the YAML and the Go disagree. Change the
Go first and then the YAML. See D42 and D54.

CI uses no database that is preinstalled on the runner. `dbrun` starts every
release, and the compare job runs MariaDB 13.0 and MySQL 26.7 as service
containers to check that the two products answer the same way (D44).

## Writing documentation

A new document goes in `docs/`. Only `README.md`, `AGENTS.md`, `CLAUDE.md` and
`CONTRIBUTING.md` belong in the repository root, and a test enforces that. Add
it to the table at the top of this file and to the one in `README.md`, because
a document nobody can find is a document nobody reads. See D50.

A decision goes in a file of its own in `docs/decisions/` and nowhere else
(D111). Name it `D<nnn>-<title>.md` with the next number. It opens with
`# D<n>. <title>`, a blank line, and `Status: <status>.`, where the status is
`Decided`, or `Amends D26`, or `Superseded by D24`. Add its row to
`docs/decisions/README.md`. `TestTheDecisionIndexIsComplete` prints the row
when it is missing. If your decision changes an earlier one, say so in both
statuses, because a reader who finds the older one has to be told.

Write plain English. Use short sentences and the active voice. Use `can`,
`will`, and `must`, and do not use `should`, `may`, or `might`. Do not use
semicolons or em dashes. Put the condition before the command: "If the query
returns no rows, return the empty set."

Load the `simple-english` skill before you write any text that a person reads:
a document, a code comment, an error message or a commit message. Follow it
for that text. Its rules include the ones above and add more, such as no
contractions and one word for one meaning. Ken asked for this. See D89 and
D110.

`TestProseIsSimpleEnglish` checks the rules of the skill that a machine can
check: the banned modals, semicolons, dashes, contractions, bold, "has been",
British spellings, filler words and Latin abbreviations. It reads every
document, every comment in Go, YAML, a Containerfile, a batch file or an INI
file, and every error and test message. It skips text
in backticks or double quotes. There is no marker that turns a rule off. If
it reports a sentence, rewrite the sentence. Sentence length and the voice
are still yours to check. See D156.

Work that is known and not done goes in `docs/BACKLOG.md`, with the decision
or the measurement that found it. When an item is done, delete it, and record
anything decided in `docs/decisions/`.
