# dbmeta Plan and Decisions

This document records the plan for `github.com/xo/dbmeta` and the decisions
made so far. Other coding agents must read this file before they change code
in this repository.

Status of this repository at the time of writing: `go.mod` and `LICENSE` only.
There are no commits and no Go source files.

Two other documents sit beside this one. `CLAUDE.md` holds the rules for
writing code here. `EVALUATION.md` holds the method for deciding which versions
of a database to support, and the evidence behind each decision already made.

## Purpose

`dbmeta` holds database metadata queries and models in one Go module. Other
projects consume it. The first two consumers are `usql`, an interactive SQL
command line client, and `dbtpl`, a code generator that reads a database
schema. The goal is to cover as many databases as `usql` supports.

The work is a move of the existing `usql/drivers/metadata` package into this
module, not a new design from nothing. The `usql` backlog records the move as
item 7.

## Architecture

The module has two layers.

The root package `dbmeta` gives consumers one driver agnostic API. Driver
agnostic means the caller asks for tables without knowing which database
answers. This layer holds the common types, the reader interfaces, and the
error values.

The package `dbmeta/models/<driver>` holds the generated code for one driver.
For example, `dbmeta/models/sqlite3` holds the SQLite3 queries and the structs
that receive their rows. `dbtpl` generates these packages from SQL that this
repository stores. Each driver gets its own Go package, so the types in
`dbmeta/models/postgres` and the types in `dbmeta/models/sqlite3` are different
types.

There is no version directory below the driver. A database changes its metadata
between releases, and D8 holds those differences as generated data inside the
one driver package rather than as a package per release.

A third layer joins the two. Something must convert the per driver model rows
into the common types of the root package. It also merges the version fragments
for the connected server. The location of that code is an open question. See
question 2 below.

## Decisions

Each decision below carries a status. "Decided" means Ken chose it. "Proposed"
means an agent or a peer session suggested it and Ken has not confirmed it.
"Open" means nobody has chosen yet.

### D1. The module centralizes database metadata. Decided.

`dbmeta` is the single home for database metadata queries. `usql` and `dbtpl`
consume it instead of each keeping their own copy.

### D2. Models are generated per driver under `models/<driver>`. Decided.

`dbtpl` generates the model code. Each driver gets one package, named after the
driver, under `models/`. One package covers every supported version of that
database, because D8 holds the version differences as data inside it.

### D3. The root package is the driver agnostic API. Decided.

External projects use the root package. They do not import
`dbmeta/models/<driver>` to do ordinary work.

### D4. Keep the object coverage, drop the Reader naming. Decided in part.

The 15 one method interfaces in `usql/drivers/metadata/metadata.go` are useful
for what they cover, which is one interface per kind of object. They are not
useful as a naming scheme. D5 drops the `Reader` and `Writer` concept, so
`TableReader` and `ColumnReader` do not carry over under those names.

Take the object coverage. Leave the names and the composition.

The name `Reader` carried information in `usql` because a `Writer` sat beside
it. In `dbmeta` everything reads, so `Reader` says nothing and repeats the
package. `dbmeta.TableReader` with a `Tables` method is three words for one
idea.

There is a further question about whether `dbmeta` exports these as interfaces
at all. `CLAUDE.md` already carries the Go rule that an interface belongs where
it is consumed, not where it is implemented. Under that rule `dbmeta` returns a
concrete type per driver with methods on it, and `usql` and `dbtpl` each
declare the narrow interface each one needs. That removes the naming problem
rather than solving it, and it removes the type assertion composition that D18
rejects.

Do not settle this before D13 delivers the models. The right shape will be
clearer when several databases have answered the same questions. See question 5.

### D5. dbmeta only reads. Decided.

`dbmeta` reads metadata. It does not render it and it does not write it.

The `tblfmt` based writer stays in `usql`. That file,
`usql/drivers/metadata/writer.go`, is 838 lines and it is the only file that
uses `tblfmt` and `usql/env`. Leaving it behind keeps both out of this module
and is part of how D7 is met. It also uses `dburl`, across the eight `Writer`
methods at lines 148 to 162, but that is no longer a reason either way, because
D19 makes `dburl` a direct dependency.

The `Reader` and `Writer` pair is a `usql` concept and it does not come here.
Ken will change those names in `usql` itself. Do not design `dbmeta` around
them and do not preserve them for the sake of an easier phase 5.

One consequence belongs to phase 5. The `usql` writer consumes the cursor types
that D18 removes, so adapting it is part of integrating, not a reason to keep
the old shape.

### D6. Fix the NULL scan defect once, at generation time. Decided.

See "Known defects" below for the evidence. The fix belongs in the generator
flags, not in hand written patches.

### D7. Use the standard library. Third party packages are a last resort. Decided.

`dbmeta` uses the Go standard library for almost everything. An agent must not
add a third party package without asking Ken first.

There are exactly two dependencies. `database/sql` is in the standard library.
`github.com/xo/dburl` is the one approved outside package, under D19, and it
handles everything to do with a connection string.

The survey below shows that the reader half of the code already meets this rule
after two small changes. A third case needs a decision.

The `Writer` interface is the only user of `github.com/xo/dburl` in
`metadata.go`. All eight of its methods take a `*dburl.URL`, at lines 148 to 162. This no longer matters for the dependency list, because D19 makes `dburl`
a direct dependency. The writer still leaves under D5, for the `tblfmt` and
`usql/env` reasons rather than the `dburl` one.

The postgres reader uses `github.com/lib/pq` in one place. It calls `pq.Array`
to scan the `TopN` and `TopNFreqs` columns of the column statistics query, at
`postgres/metadata.go:257`. Replace it. Either return the array from SQL as one
delimited string and split it in Go, or write a small scanner for the postgres
array format.

The impala reader is the hard case. It does not run SQL. It calls
`driver.NewMetadata` from `github.com/sclgo/impala-go` and reads the metadata
through the driver. An agent must not port it as it stands. Either rewrite it
as SQL queries, or leave impala out until Ken decides.

The mysql reader imports `github.com/gohxs/readline` and
`usql/drivers/completer`. Both belong to the completion feature, not to
metadata. Leave them in `usql`.

Check the generated code too. `dbtpl` has a `--go-uuid` flag that defaults to
`github.com/google/uuid`. No metadata query needs a UUID column, so the default
must never take effect. Make sure that the generated packages import nothing
outside the standard library.

### D8. Version differences are generated data, not packages. Decided.

A metadata query must work against more than one release of a database. There
is one package per driver, and the version differences live inside it as
generated data.

This replaces an earlier form of D8 that gave each release its own package.
That form is abandoned. Two independent reviews called it a combinatorial
explosion and both were right. Do not reintroduce it.

#### The shape

Each query is a list of fragments. A fragment carries the lowest server version
that it applies to and a piece of SQL:

```go
[]Fragment{
    {MinVersion: 120000, SQL: ...},
    {MinVersion: 150000, SQL: ...},
}
```

A selector reads the server version once per connection, merges the fragments
that apply, and runs the result. `SHOW server_version_num` returns the integer
that `psql` itself compares against, so 15.0 is 150000.

Version by feature range, not by release. Add a fragment when a query must
change. Do not add one per release.

#### The rule that makes this work: pad the column list

A fragment must never change the set of columns that a query returns. When a
column has no source on an older server, the fragment still selects it, as a
literal NULL under the same name:

```sql
NULL AS "access_privileges"
```

This rule exists because Go scans into a fixed struct. Without it, one query
returns 12 columns on one server and 13 on another, no single generated struct
fits both, and positional scanning breaks.

`psql` does not need this rule and mostly does not follow it. It renders a
table whose shape it discovers at run time, so it lets the column set vary. It
uses the padding technique once, at `describe.c:4506`, where it emits
`NULL AS "Access privileges"` for a server below 15 rather than the real
column. That one line is the technique. `dbmeta` applies it everywhere.

Verify this before you dismiss it. The version gates in `describe.c` add
columns rather than only rewriting expressions. Release 11 adds `pubtruncate`
as "Truncates". Release 13 adds `pubviaroot` as "Via root". Another gate adds
`am.amname` as "Access method". Each one changes the column count.

#### What dbtpl generates

`dbtpl` introspects one concrete statement against one live server. With the
padding rule, the newest supported version is that statement. It produces the
row struct and the scan code once per query, and both fit every version,
because every version returns the same columns in the same order.

The fragments are data beside the generated code, not something `dbtpl`
introspects.

#### The subtlety to resolve

Padding makes two different facts look the same. A NULL now means either that
the value is genuinely null on this server, or that the column does not exist
at this version. A caller that cannot tell them apart will report a missing
feature as missing data.

Do not solve this with a sentinel value. Record, next to each query, which
fields are valid at the detected version, and let a caller ask. This is the
same mechanism that question 6 raises for a database that cannot answer at all,
and the two must be one mechanism rather than two.

#### What this decision settles

Questions 3 and 4 are closed. There is no version directory to name, so the Go
rule about a trailing `/vN` import path element no longer applies. There is no
inheritance to arrange, because data needs none.

D2 still holds: one package per driver under `models/<driver>`. The version
segment that an earlier draft put below it is gone.

### D9. There are two platonic models. PostgreSQL is the primary one. Decided.

A platonic model is an idealized shape that a real database only approximates.
`dbmeta` implements two, and they rank.

The primary model is PostgreSQL itself, and more exactly the metadata commands
of `psql`, the PostgreSQL command line client. These are the `\d` family, such
as `\dt` for tables and `\df` for functions. `psql` implements them in one C
file named `describe.c`. That file is the reference for what a metadata query
must return and for how it must behave.

The second model is the generic `information_schema`. Standards bodies define
it, many databases provide it, and every database varies it.

Read the ranking narrowly. It decides the shape and the naming of an object
that two databases both have. When PostgreSQL and `information_schema`
describe the same thing differently, follow PostgreSQL. Use
`information_schema` for the databases that offer nothing better.

It is not a requirement that every database answer all 48 PostgreSQL objects.
Both external reviews read it that way and warned that most drivers would then
return "not supported" for most calls. That reading is wrong, and the wording
above is narrowed to prevent it. A database answers for the objects it has, and
the capability mechanism in question 6 reports the rest. PostgreSQL sets the
shape of the answer, not the list of questions every database must answer.

#### The PostgreSQL model and pgdesc

The repository `github.com/xo/pgdesc` already holds a machine translation of
`describe.c` into Go. Do not start this model from nothing. Read `pgdesc`
first.

Facts about `pgdesc` that an agent needs:

- `pgdesc.go` is 3786 lines and holds about 30 describe functions. `gen.go`,
  at 804 lines, is the translator that produced it.
- It covers far more objects than the 15 that `usql` models. It has access
  methods, aggregates, casts, collations, conversions, domains, event triggers,
  extensions, foreign data wrappers, foreign servers, foreign tables,
  languages, operators, publications, subscriptions, roles, default access
  control lists, and text search parsers and configurations.
- The last commit is from January 2019, so it reflects a PostgreSQL release
  around 11. Regenerate it against a current release before you trust it.
- Its `TODO` file lists seven known faults in the translation, including a
  broken ternary operator and queries that need splitting into separate
  functions.

The object list above sets the target for the root package. The 15 object types
in `usql` are a subset of it, not the goal.

#### How psql handles versions, and how dbmeta differs

`psql` does not keep one query per release. It builds one query and switches
fragments on an integer server version. The generated Go shows the pattern as
`if d.version < 90600`, where 90600 means release 9.6.0. When the server is too
old for a feature, `psql` returns an error that names the version.

`dbmeta` takes the other path. D8 sets discrete models per version, because
`dbtpl` generates each model from a concrete SQL statement run against a
concrete server. An inline conditional has no single statement to introspect.

Two consequences follow. An agent that ports a query from `describe.c` or from
`pgdesc` must read the version conditionals and split them into one statement
per supported version. The integer form of the version, such as 90600 and
180000, is the natural key for selection, and `SHOW server_version_num` returns
it directly.

#### The information_schema model and its variations

The variations are the point. A driver does not get its own copy of the
`information_schema` queries. It gets the shared queries plus a description of
how it differs. `usql` already works this way, and its
`informationschema.InformationSchema` type is the model to carry over. It
describes a database in four ways.

1. Eight feature flags say whether the database has a thing at all:
   `hasFunctions`, `hasSequences`, `hasIndexes`, `hasConstraints`,
   `hasCheckConstraints`, `hasTablePrivileges`, `hasColumnPrivileges`, and
   `hasUsagePrivileges`.
2. Fifteen named clauses replace one fragment of SQL each. The names are values
   of the `ClauseName` type, such as `columns.data_type` and
   `privileges.grantor`. MySQL, for example, replaces `data_type` with
   `column_type` and replaces the deferrable clauses with an empty string.
3. A placeholder function writes the bind parameter for the database, `$1` for
   postgres and `?` for MySQL.
4. Two lists name the system schemas to hide and the expression that gives the
   current schema.

Carry this mechanism into `dbmeta`, with one change that D8 forces. Today the
description is fixed per driver. It must become fixed per driver and version,
because a database gains and loses `information_schema` features between
releases.

Carry it with a second change that the shared instance hazard forces. See the
section on known defects. The description must be a value the caller owns, not
a package level variable that another driver can borrow.

### D10. Take the initial design from dbtpl and its models directory. Decided.

Do not design the model shape from nothing. Read `dbtpl` first, and read
`dbtpl/models` most closely. It shows what a generated metadata model looks
like when it works: the struct per object, the query function per lookup, and
the shared package file that holds the `DB` interface and the logging hooks.

`dbtpl/gen.sh` shows the other half, which is how those files are produced. It
runs 60 `dbtpl query` commands across five databases, each with the SQL in a
shell heredoc.

Skim, then adapt. `dbtpl/models` is one flat package with the driver in the
function name, as in `PostgresTables` and `MysqlTables`. D2 sets a package per
driver instead, so the names lose the prefix and become `postgres.Tables`.

### D11. dbtpl is pinned as a tool, in the generation module. Decided, amended by D26.

`dbtpl` is pinned with the `tool` directive. Go 1.24 added the directive and
these modules target Go 1.27.1, so it is available.

The pin does not go in the root `go.mod`. D26 keeps database drivers out of the
root module, and `dbtpl` depends on four of them, so the pin lives in the
separate generation and testing module.

Pinning fixes the generator version. Two agents on two machines then generate
the same Go from the same SQL. Run the generator through `go tool dbtpl`, not
through whatever `dbtpl` sits on the path.

This pin is a build dependency, not a runtime dependency. It does not weaken
D7. The generated code and the root package still import the standard library
and `dburl` only.

### D12. Generate against live databases running in containers. Decided.

`dbtpl query` introspects a real connection. It creates a temporary view from
the statement, reads the column types, and drops the view. There is no offline
mode. Every driver therefore needs a running database at generation time.

`usql/contrib` already solves this and `dbmeta` must reuse the pattern rather
than invent one. It holds a directory per database, each with a `podman-config`
file of four lines, and the scripts `podman-run.sh` and `podman-stop.sh` that
start and stop them. The postgres config reads:

```
NAME=postgres
IMAGE=docker.io/usql/postgres
PUBLISH=5432:5432
ENV="POSTGRES_PASSWORD=P4ssw0rd"
```

`usql/contrib/config.yaml` records the connection URL for each running
container, such as `postgres://postgres:P4ssw0rd@localhost`.

`contrib` covers every database in the three phases, and 27 databases in total.
Note one naming detail before you look for something that is not missing. Phase
2 uses MariaDB, and there is no `mariadb` directory. The `mysql` directory is
the MariaDB one. Its config reads `IMAGE=docker.io/library/mariadb`.

The directories are adodb, cassandra, charts, clickhouse, cockroach, couchbase,
db2, duckdb, exasol, firebird, flightsql, godror, h2, hive, ignite, mymysql,
mysql, oracle, oracle-enterprise, pgx, postgres, presto, sqlite3, sqlserver,
trino, vertica, and ydb.

D8 adds one requirement that `contrib` does not yet meet. Most configs name an
image without a tag, as in `IMAGE=docker.io/usql/postgres`, so each database
runs at one version. `dbmeta` generates one model per database version, so the
harness must start a named version, such as postgres 15 and postgres 18, and
must record which version produced which model. That is an extension of the
existing configs, not a replacement for them.

### D13. Build the models before the root package. Decided.

Build the queries for the primary databases first. PostgreSQL comes first, then
MariaDB, then the other primary databases. Write the root `dbmeta` package
after them.

The order matters. A root API designed before any query exists is a guess. An
API written after several databases have answered the same questions is a
description of what they can actually do.

This inverts the order that an earlier draft of this file gave. Ignore that
draft.

### D14. A driver is a family, not a product. Decided.

The `mysql` driver targets MariaDB, and its models must also work against
MySQL. The driver name is the family name. The reference database is the
product that the models are generated against.

This adds a second axis to D8. A model varies by version, and it varies by
flavor. Flavor means a product that speaks the same dialect and claims the same
driver, such as MariaDB and MySQL, or such as PostgreSQL and the databases that
copy its wire protocol.

The two axes are not the same and one does not contain the other. MariaDB 11
and MySQL 8 are two flavors at a similar age. MariaDB 10 and MariaDB 11 are one
flavor at two ages. Testing must cover both axes. See the testing plan.

Rule for agents: generate against the reference product, then test against
every flavor in the family. A query that only works on the reference product is
not finished.

### D15. CI runs on GitHub Actions, on ubuntu-latest only. Decided.

The other `xo` projects test on several runners because their code is platform
dependent. `dbmeta` is not. The queries are SQL and the code is pure Go, so one
runner is enough.

Keep the workflow small. Build, vet, and test. Do not copy the matrix
workflows from the other `xo` projects.

The repository already exists on GitHub. Use the GitHub MCP tools to read and
write workflows rather than guessing at the repository state.

### D16. User facing text follows the simple English rules. Decided.

This covers `README.md`, every document in the repository, code comments, and
error messages. Write short sentences in the active voice. Use `can`, `will`,
and `must`. Do not use `should`, `may`, or `might`. Do not use semicolons or em
dashes. Put the condition before the command.

The rules come from ASD-STE100, the controlled English that aerospace uses so
that a reader cannot misread an instruction.

### D17. Every metadata read takes a context. Decided.

There is no API surface in `dbmeta` that reads metadata without a
`context.Context`. This is not a preference and it has no exceptions.

Follow idiomatic Go. The context is the first parameter and it is named `ctx`.
It is never stored in a struct. It is never `context.TODO()` in shipped code.
A function that does not reach a database does not take one.

```go
Tables(ctx context.Context, f Filter) (*TableSet, error)
```

Four rules follow from this.

1. Use only the context methods of `database/sql`. Call `QueryContext`,
   `ExecContext`, `QueryRowContext`, and `PrepareContext`. Never call `Query`,
   `Exec`, `QueryRow`, or `Prepare`.
2. Define the `DB` interface with those four methods only. The `usql` version
   has eight, because it carries both forms. Four is enough, `database/sql.DB`
   and `database/sql.Tx` both satisfy it, and a smaller interface is easier to
   fake in a test.
3. Do not create a root context inside the library. `context.Background()` and
   `context.TODO()` belong to the caller.
4. Do not take a timeout as an option. A caller that wants one wraps the
   context with `context.WithTimeout` before the call.

Rules 3 and 4 exist because the code being moved breaks both. In
`usql/drivers/metadata/reader.go`, `LoggingReader.Query` accepts no context. It
manufactures one at line 264 with
`context.WithTimeout(context.Background(), r.timeout)` when a timeout option is
set, and at line 268 it calls `r.db.Query(q, v...)` with no context at all when
one is not. A caller therefore cannot cancel a metadata query. `WithTimeout` at
line 218 is the option that must not carry over.

This matters for more than tidiness. A metadata query can be slow, and `usql`
runs them while a person waits. Completion in `usql` sets a three second
timeout for exactly that reason. With a context the caller sets that deadline,
and pressing an interrupt key can stop the query.

### D18. The whole package is idiomatic Go. Decided.

Write Go that a Go reviewer recognizes. `dbmeta` is a move of older code, and
the older code predates generics, iterators, and the context rules that D17
sets. Do not carry a pattern over because it is already written.

The conventions are in `CLAUDE.md`. This decision records the two judgments
that need explaining, because both reverse something in the source.

#### The Set cursor types do not carry over

`usql` returns each list as a `*CatalogSet`, a `*TableSet`, and 13 more like
them. Read `metadata.go` around line 1048 before you port one. The type is a
cursor with `Next`, `Get`, `Reset`, `Len`, `SetColumns`, and `SetScanValues`
over a slice that is already in memory.

Three things are wrong with it for a new package.

1. The cursor buys nothing. `NewCatalogSet` takes a `[]Catalog` and the rows
   are already materialized. A cursor over a full slice is a cursor over
   nothing.
2. It throws the type away and takes it back by assertion. Every row is stored
   as `Result`, an interface holding `Values() []interface{}`, and `Get`
   recovers the value with `r.(CatalogProvider)`. That assertion is unchecked,
   so a wrong row type is a panic at run time rather than an error at compile
   time.
3. It exists to feed the writer. `SetColumns` and `SetScanValues` serve the
   `tblfmt` renderer. D5 leaves that writer in `usql`, so the reason for the
   shape leaves with it.

Return an iterator, `iter.Seq2[Catalog, error]`. D33 supersedes an earlier
version of this paragraph that preferred a typed slice. The package streams and
does not materialize a result.

Note the consequence for `usql`. Its writer consumes the cursor, so phase 5
must adapt the writer to a slice or an iterator. That work belongs to phase 5
and it is a reason to keep the writer in `usql` rather than an argument against
doing this.

#### Compose readers explicitly, not by type assertion

`usql` builds a reader with `PluginReader`, which takes a list of readers and
type asserts each one against 15 interfaces, keeping the last that matches.
Read `reader.go` from line 31.

This is how the DuckDB fault happened. Nothing in the type system says which
reader answers which call, so a reader from the wrong driver satisfies the
interface and wins silently.

Name the parts instead. A driver states which reader answers each object, and a
missing one is a value that reads as missing rather than a failed assertion.
D4 keeps the interfaces, which are useful. It does not require this way of
combining them.

#### The rest

Use generics instead of `interface{}` for a container of one type. Use
`errors.Is` and `errors.As` rather than comparing strings. Give every error a
wrapped cause with `%w`. Make the zero value useful where you can. Keep an
interface small and define it where it is consumed.

`gofmt` and `go vet` must both be clean. See `CLAUDE.md` for the command.

### D19. dburl is a direct dependency. Do not repeat it. Decided.

`github.com/xo/dburl` is a direct dependency of `dbmeta`. It is the named
exception to D7.

Every package that uses `dbmeta` is expected to use `dburl` as well, so the
dependency costs a consumer nothing that it does not already carry. Use it for
connection information in tests and anywhere else that `dbmeta` handles a
connection string.

Do not repeat any part of it. `dbmeta` must not carry its own list of schemes,
its own aliases, its own flavor table, or its own connection string parser. If
`dbmeta` needs to know something about a database URL, `dburl` answers it.

An earlier draft of the testing plan said to copy the flavor taxonomy out of
`dburl` as data in order to protect D7. That instruction is withdrawn. Import
`dburl` and read it.

#### dburl already draws the distinction that D14 needs

D14 separates a family from a flavor. `dburl` already separates them and it
exposes both on a parsed URL. Read `dburl.go:172` to see it:

```go
u.Driver, u.UnaliasedDriver = scheme.Driver, scheme.Driver
if scheme.Override != "" {
    u.Driver = scheme.Override
}
```

Three fields matter to `dbmeta`.

`URL.Driver` names the family, and it selects the model package. For
`cockroach://` it is `postgres`, because the `cockroachdb` scheme sets
`Override` to `postgres`.

`URL.UnaliasedDriver` names the wire compatible product. For `cockroach://` it
is `cockroachdb`. This is the flavor for a product that has its own scheme, and
`dburl` records five of them: `cockroachdb` and `redshift` over `postgres`, and
`memsql`, `tidb` and `vitess` over `mysql`.

`URL.OriginalScheme` holds what the caller typed. This is the flavor for a
product that `dburl` treats as an alias rather than a scheme. The `mysql`
scheme carries the aliases `mariadb`, `maria`, `percona` and `aurora`, so
`mariadb://` parses with `Driver` and `UnaliasedDriver` both set to `mysql`,
and only `OriginalScheme` records that MariaDB was asked for.

Note the consequence. The two kinds of flavor arrive on different fields. A
wire compatible shows up in `UnaliasedDriver`. An alias shows up only in
`OriginalScheme`. Code that reports the flavor must read both, and a test that
covers only one will miss half the matrix that the testing plan describes.

`SchemeDriverAndAliases` at `scheme.go:512` resolves a scheme name to its driver
and its aliases, and applies `Override` the same way. Use it rather than
reading the scheme table.

### D20. PostgreSQL goes back to 9.6. Every other database starts at the maintained floor. Decided.

There are two rules here, not one, because PostgreSQL is not an ordinary
database in this project.

**PostgreSQL: support 9.6 and newer.** The ceiling is the newest stable
release, 18 today. That is ten major versions: 9.6, 10, 11, 12, 13, 14, 15, 16,
17 and 18.

The unit is the major version, not the point release. `dbmeta` does not treat
14.1 and 14.2 separately. This is safe rather than convenient: every one of the
11 version gates in `describe.c` sits on a major boundary, and PostgreSQL does
not change a catalog in a patch release, because that would change the on disk
format. `EVALUATION.md` records the check and the command to repeat it.

Note that "major" means two different things across this range. Before release
10 a major version is the first two numbers, so 9.5 and 9.6 are different
majors. From 10 onward it is a single number. `EVALUATION.md` explains what
that does to the integer the server reports.

**Every other database: the floor is the oldest release with a maintained
container image**, which usually matches the oldest release its vendor still
supports. See `EVALUATION.md` for the method and for the evidence behind each
choice.

#### Why PostgreSQL is special

D9 makes `psql` the primary model. The goal of `usql` and of this project is to
bring the `psql` command line experience to every other database. Compatibility
with `psql` is the product, so PostgreSQL is not one supported database among
many. It is the specification.

The whole of PostgreSQL is also open. Every release is public, the catalog
history is readable, and `describe.c` states its own version rules. Nothing has
to be guessed. That is not true of Oracle or SQL Server, where old behavior can
only be learned by running an old server.

So for PostgreSQL the ordinary cost argument does not apply. Go as far back as
the source allows, and 9.6 is the line.

#### What the 9.6 floor costs, measured

Counted from `describe.c` in the local PostgreSQL checkout. The file holds 76
version gates. How many stay live depends entirely on the floor:

| Floor | Live gates | Gates that collapse |
| ----- | ---------- | ------------------- |
| 9.6   | 63         | 13                  |
| 14    | 13         | 63                  |

The numbers invert. A 9.6 floor is close to five times the version work of a 14
floor. That is the price of `psql` compatibility and it is accepted knowingly.

The 13 gates that collapse at a 9.6 floor are the ones at 9.3, 9.4, 9.5 and 9.6
itself. A gate reading `>= 90400` is always true once the floor is 9.6, and a
gate reading `< 90300` is always false, so both stop being conditional.

Gates live at a 9.6 floor, by version: 15 at release 10, 13 at 11, 11 at 12, 4
at 13, 7 at 14, 12 at 15, and 1 at 16.

#### Testing the old releases

Container images exist for every release back to 9.6, and all of them publish
`linux/amd64` and `linux/arm64`. They are frozen rather than gone:

| Release       | Image last updated |
| ------------- | ------------------ |
| 18 through 14 | 2026-09-19         |
| 13            | 2025-11-14         |
| 12            | 2025-01-14         |
| 11            | 2022-06-23         |
| 10            | 2022-06-23         |
| 9.6           | 2022-02-12         |

One thing is unverified and someone must check it before phase 1 relies on it.
The 9.6, 10 and 11 images were last built in 2022 on a Debian base of that era,
and a container that old can fail on a current host over blocked syscalls or a
C library mismatch. The image being listed is not proof that it runs. Pull each
one and start it before planning work around it.

D24 already puts these releases outside CI. They are tested on a development
machine, which is also where an old image that needs a workaround is least
disruptive.

#### The floor moves for other databases, not for PostgreSQL

D21 drops a version when its upstream support ends. That rule governs the other
databases. It does not govern PostgreSQL, whose floor is fixed at 9.6 by this
decision and moves only if Ken says so.

### D21. Drop a server version on a rule, not on a judgment. Decided.

The support matrix grows every year unless something removes from it. This
decision makes removal mechanical, so no release needs an argument about it.

Both external reviews were asked and both gave the same primary trigger.

#### The rule

> A server major version is supported while its upstream vendor end of life
> date is in the future. Support is dropped in the first minor release of
> `dbmeta` published after that date passes.

Use upstream end of life, not the other two candidates. A container image
disappearing is an operational accident rather than a policy. Cloud provider
support is commercial, differs per cloud, and often runs longer than upstream
for customers who pay for it. Upstream dates are published years ahead and
anyone can check them.

DeepSeek proposed adding a six month margin, so that a version leaves before
its end of life rather than after. Prefer the plain rule. A margin means
dropping a version that upstream still supports, which item three below turns
into a breaking change.

#### Notice

Give one minor release of notice, and at least three months.

Use three mechanisms and not a fourth.

1. A support table in `README.md` with a column for the planned removal
   release.
2. An entry in the release notes and the changelog when a version is deprecated
   and again when it is removed.
3. A function that a caller can ask, so an application can warn in its own
   voice.

Do not log a warning from the library. Go has no single logging convention, so
a library that writes to standard output corrupts the output of a command line
tool. `usql` is exactly such a tool. Gemini made this point and it is right for
this project in particular.

A compile time signal is impossible. The server version is discovered over a
network connection at run time.

#### Semantic versioning

Dropping a version that is already past upstream end of life is a minor
release. Go module compatibility covers the exported Go API, and the exported
API does not change when a query set is removed.

Dropping a version that upstream still supports is a breaking change and needs
a major version. This is the reason to avoid the six month margin above.

Both reviews agreed on this and both cited `jackc/pgx` and
`go-sql-driver/mysql` as precedent. The specific claims about those projects
were not verified.

#### Delete rather than freeze

Delete the queries for a dropped version. Tag the last release that supported
it, and say so in the support table, so that a user on an old server can pin
that tag.

Frozen queries rot. They stay in the tree, they stop being tested to save CI
time, and a version selection fault can silently fall back to one and return a
wrong answer rather than an error.

#### Below the floor: refuse, with a way through

When the server is older than the floor, return an error and name the versions:

```
postgres 12 is not supported, the oldest supported version is 14
```

Offer one option that proceeds anyway with the oldest known query set, for a
caller who accepts the risk. Do not make that the default. A best effort
attempt against an old catalog fails with a confusing SQL error, or worse
returns a partial answer that looks complete.

#### Above the ceiling: proceed

When the server is newer than anything `dbmeta` knows, use the newest known
query set and continue.

This is the common case, because people upgrade a server faster than they
upgrade a library. A library that refuses an unknown newer server breaks every
user on the day a new release ships.

It can fail, and the failure is acceptable because it is loud and searchable.
PostgreSQL 12 removed `pg_attrdef.adsrc`, so a query written for 11 failed on
12 with `column "adsrc" does not exist`. That is a clear error that produces a
bug report. Silently returning an empty result would not.

Do not catch a catalog error and return an empty set. An empty set means the
database has no such object.

### D22. Test every supported major, not a sample of them. Superseded by D24.

D24 overrides where this runs. CI tests the latest version only, and the full
matrix runs on a development machine. The technical argument below is why the
matrix must exist at all, and it is unchanged. Only the venue changed.

Run one job per supported major version of each primary database. Under D20
that is PostgreSQL 14, 15, 16, 17 and 18, which is five runs.

The two reviews disagreed here, and the disagreement was settled against the
PostgreSQL source rather than by preferring one model.

Gemini argued for testing the floor, the middle and the ceiling only, on the
grounds that catalog changes are monotonic: a column added in one release stays
in the next. DeepSeek said that is false and that a sampled matrix misses a
change in an untested middle version.

DeepSeek is right, and the source proves it. PostgreSQL removes catalog
columns. Commit `fe5038236c` in the PostgreSQL tree is titled "Remove obsolete
pg_attrdef.adsrc column". A query written for release 11 fails on release 12
with `column "adsrc" does not exist`. Testing 11 and 13 would not find it.

`describe.c` shows the same thing directly. It carries 11 gates of the form
`pset.sversion < N`, at 9.3, 9.6, 10 six times, 11, 12 and 15. A gate that asks
whether the server is below a version exists because something present in the
older release is absent in the newer one. Catalog change is not monotonic.

The deprecation rule in D21 is what keeps this affordable. It removes a version
every year, so the matrix stays near five rather than growing without bound.

DeepSeek also suggested generating the CI matrix from the version gates in the
code, so that every distinct minimum version becomes a job. Take this once the
fragments exist. It makes the matrix follow the queries automatically, and it
cannot drift from them.

Add one job beyond the supported set: the current beta of the next release,
allowed to fail without failing the build. It gives warning of a catalog change
before the release lands.

### D23. Do not build dbtest first. Let dbmeta pull it into existence. Decided.

Both reviews were asked whether the container package `dbtest` must come first,
and both said no. Build `dbmeta`, and grow `dbtest` only when a real `dbmeta`
test cannot be written without it.

The reason both gave is the same. Infrastructure written before it has a
consumer gets the wrong shape. The existing shell scripts in `usql/contrib`
already work and already cover 27 databases, so nothing is blocked today.

#### The split

1. `dbtest` starts as a thin wrapper over the `podman` command, a few hundred
   lines at most. It starts a container, waits for the database to accept a
   connection, returns a connection string, and stops the container.
2. `dbmeta` generates and tests one driver at one version against it.
3. `dbtest` then gains the one thing the shell scripts cannot do, which is two
   versions of the same database at once. That forces unique container names
   and dynamic host ports.
4. `dbmeta` adds the version matrix for that driver.
5. Repeat per driver. Extract a stable `dbtest` after three drivers, not
   before.

Wait on everything else: volumes, networks, log streaming, a Podman REST
client, cross platform support, and the 20 databases that `dbmeta` does not
cover yet.

#### How to tell the abstraction went wrong

Watch for these. Any one of them means stop and simplify.

1. `dbtest` is larger than `dbmeta`.
2. `dbtest` exposes a general `RunContainer(image, env, ports)` API. The right
   shape is `dbtest.StartPostgres(ctx, version)` returning a connection.
3. `dbmeta` maps a container port to build its own connection string.
4. Adding a database means changing the core of `dbtest`.
5. An interface in `dbtest` has exactly one implementation.

#### What this harness needs that a general one does not

Both reviews produced nearly the same list, and every item maps onto a decision
already in this file.

1. Two or more versions of one database at once. This is the version axis of
   D8 and the shell scripts cannot do it.
2. A pinned image digest, not a tag. A tag moves, and generated code must be
   reproducible. Record which digest produced which model. This is the gap D12
   already names.
3. A readiness check. A container reports itself started well before the
   database accepts a connection, and a generator that connects too early
   crashes.
4. A seeded schema that exercises the metadata surface: types, arrays, enums,
   domains, constraints, comments, indexes, partitions, generated columns and
   extensions. A general harness gives an empty database, and an empty database
   tests nothing.
5. Named users with fixed grants. Introspection returns different answers to
   different users, which the testing plan already requires and which two of
   the five open `usql` pull requests exist because of.

#### The third party question is still Ken's

The two reviews split on whether to grant D7 an exception for a test only
container library.

Gemini said grant it and use `testcontainers-go`, on the grounds that D7
protects consumers of the library and a test harness is never compiled into a
consumer's binary.

DeepSeek said keep the shell scripts and add a thin wrapper over the `podman`
command, and grant an exception only if the ban is meant for production
dependencies alone.

The thin wrapper is the smaller commitment and it matches D23 above, because it
is what step 1 describes either way. Note one fact in favor of it: the reviews
also observed that `testcontainers-go` reaches Podman through a Docker
compatibility socket, which is an extra moving part for a project that has
already chosen Podman.

### D24. CI tests the latest version only. The matrix runs locally. Decided.

CI tests the major databases at their latest version and nothing else. Every
other version, and every flavor, is tested on a development machine.

This overrides D22, which proposed one CI job per supported major. Read D22 for
why a sampled matrix misses catalog changes. That reasoning still holds. The
matrix is not cancelled, it moves off CI, because running every version of
every database on every push costs more than the project will pay.

The CI databases are PostgreSQL, MySQL and SQLite3. D35 removed DuckDB, because
D29 makes the project pure Go and DuckDB has no pure Go driver. More can be
added later.

#### What the runner actually provides

Checked against `actions/runner-images` for Ubuntu 24.04 on 2026-09-24. The
four databases fall into two groups, and the difference decides how each one is
started.

SQLite3 and DuckDB are embedded. There is no server and no container. SQLite3
is preinstalled at 3.45.1, and DuckDB arrives as a Go driver. Both are free to
test.

PostgreSQL and MySQL are servers. Both are preinstalled and both have their
service disabled, so a job starts them with `sudo systemctl start
postgresql.service` or `sudo systemctl start mysql.service`.

Two facts about the preinstalled versions matter.

The preinstalled PostgreSQL is 16.15, not 18. The latest release is not what
the runner gives you. A job that must test the newest PostgreSQL needs a
service container or the upstream apt repository. Starting the preinstalled
service tests release 16.

The preinstalled MySQL is MySQL 8.0.46, not MariaDB. There is no MariaDB on the
runner. D14 makes MariaDB the reference product for the `mysql` driver, so the
preinstalled server is the flavor rather than the reference.

That second point is useful rather than a problem. CI exercises MySQL while
local testing exercises MariaDB, so the flavor axis of D14 gets covered on
every push at no extra cost. Write it down as intentional, because someone will
otherwise "fix" it by installing MariaDB in CI and lose the coverage.

#### The risk this accepts, and what reduces it

A version regression now reaches the main branch unless somebody runs the local
matrix. The `pg_attrdef.adsrc` class of fault, where a newer server removes a
catalog column, is exactly the kind that the CI job on the latest version can
miss for an older supported version.

Three things keep that risk small. None is optional.

1. One command runs the full local matrix. If running it takes research, it
   will not be run.
2. The local matrix runs before a release, and the result is recorded in the
   release notes. A release that has not passed it does not go out.
3. The support table in `README.md` states which versions CI covers and which
   are covered only locally. Do not claim in public that a version is tested
   when only a person's machine tests it.

### D25. Test on amd64 only. No build tags and no platform gates. Decided.

`dbmeta` tests on `linux/amd64` and on nothing else, both in CI and on a
development machine. This applies to every database, PostgreSQL included.

`dbmeta` contains no `//go:build` constraint on an operating system or an
architecture, and no code path that branches on either. `dburl` works this way
and `dbmeta` follows it.

#### The assumption, stated plainly

The same version of the same database, given the same schema, answers a query
the same way on every platform.

This is an assumption, not a fact, and it is adopted on purpose. The database
vendor is responsible for its product behaving the same across the platforms it
ships on. `dbmeta` takes the vendor at its word rather than multiplying the
test matrix by the number of platforms.

Do not add a platform test because you suspect a difference. Report the
difference to the vendor. If a real one is found that `dbmeta` must work
around, bring it to Ken and this decision gets revisited. Do not quietly add a
build tag.

#### Where the assumption is weakest

Recorded so that a future reader knows what was accepted, not as a reason to
act now.

Collations are the clearest case. PostgreSQL builds `pg_collation` from the
locale data of the host, so `\dO` can list different collations on two
machines. This is not even a platform difference in the usual sense, because
two amd64 Linux hosts with different C library versions can disagree.

D12 already neutralizes most of this without any extra work. Generation and
tests run against a pinned container image, and the container carries its own C
library and its own locale data. The result depends on the image, not on the
host, which is one more reason to pin a digest rather than a tag.

Other places where a platform difference is plausible: default character set
and encoding, values in `pg_settings` that contain a file path, and system
views that differ between SQL Server on Linux and SQL Server on Windows. None
is a reason to test more platforms today.

#### Platform specific dependencies are allowed, inside the driver

The rule bans platform gates in `dbmeta`. It does not ban depending on a driver
that has them.

The DuckDB driver is the live example. `github.com/duckdb/duckdb-go/v2` pulls a
separate binding module per platform, and `usql` carries all five as indirect
dependencies: `lib/darwin-amd64`, `lib/darwin-arm64`, `lib/linux-amd64`,
`lib/linux-arm64` and `lib/windows-amd64`. The driver selects one with its own
build tags.

That is the driver's business. `dbmeta` writes none of those tags. Do not be
surprised when `go mod tidy` adds five platform modules, and do not try to trim
them.

### D26. No database driver in the dbmeta module. Decided.

The `dbmeta` module depends on the standard library and on `dburl`. It does not
depend on a database driver, and its `go.mod` does not name one.

A package that uses `dbmeta` brings its own driver and chooses its own version
of it. `dbmeta` must not constrain that choice, and must not force an upgrade
on a consumer who is holding a driver back.

This is why `dbmeta` takes a connection through an interface rather than
opening one. D17 already defines that interface with four context methods, and
`database/sql.DB` and `database/sql.Tx` both satisfy it. The caller opens the
connection with whatever driver it likes and hands it over.

#### This conflicts with D11, and D11 gives way

D11 pins `dbtpl` as a tool in `go.mod`. That cannot stay in the root module.

`dbtpl` depends on four database drivers, which its own `go.mod` lists:
`github.com/go-sql-driver/mysql`, `github.com/lib/pq`,
`github.com/mattn/go-sqlite3` and `github.com/microsoft/go-mssqldb`. Pinning
`dbtpl` in the root module puts all four into the root `go.mod` and `go.sum`,
which is exactly what this decision forbids.

State the risk accurately, because it is smaller than it first looks and the
decision does not rest on exaggerating it. Go prunes the module graph for
modules declaring go 1.17 or later, so a dependency that provides no imported
package is dropped from a consumer's build list. A consumer of `dbmeta` would
almost certainly not be forced onto `lib/pq v1.12.3` by this.

It is still the wrong shape. The root `go.mod` and `go.sum` would list four
drivers that the library never imports. Vulnerability scanners would report
them, `go mod graph` would show them, and building `dbmeta` itself would
download them. The stated intent is that the module carries no driver, and a
separate module delivers that without relying on a pruning rule to hide it.

#### Where the drivers and the generator go

Everything that needs a driver lives outside the root module, in a module of
its own with its own `go.mod`. That module holds:

1. The `tool` directive pinning `dbtpl`, moved out of the root.
2. Every database driver used for generation or for testing.
3. The container harness from D23.
4. The integration tests that connect to a real server.

This gives `dbtest` a second and stronger reason to exist. D23 justified it as
a container harness. It is also the boundary that keeps drivers out of
`dbmeta`. Without it, either this decision or D11 has to break.

D27 proposes how that splits between a nested module here and the sibling
repository `xo/dbtest`.

#### The root module still needs testing

Most of `dbmeta` can be tested without a driver at all, and that work belongs
in the root module.

Fragment merging is pure logic. Given a set of fragments and a server version,
the selector produces one SQL string. Test that against golden files, with no
database present. This covers the part of D8 most likely to break, and it
covers every version, including the ones no container can reach.

Version parsing and selection are the same. Feed integers, assert which model
is chosen, assert the fallback below the floor and the behavior above the
ceiling that D21 defines.

The rule for the root module is that a test there never opens a connection. A
test that needs a server goes in the other module.

### D27. Split the work in two: a nested test module here, a shared harness in dbtest. Decided.

Both reviews were asked and both gave the same answer, which was neither of the
two options as posed. Split by what is reusable.

**A nested module in this repository** holds what belongs to `dbmeta` alone:
the integration tests, the database drivers they need, and the `tool` directive
pinning `dbtpl` that D11 moved out of the root.

**The sibling repository `xo/dbtest`** holds what three projects share: the
podman container harness. `usql` backlog item 6 already asks for it, and
`usql` and `dbtpl` will use it too.

Neither review liked the alternatives. Putting `dbmeta`'s own integration tests
in a separate repository means a change to a query and the test that covers it
land in two repositories, and CI must check out both and rewrite a module path
to test an unreleased `dbmeta`. Putting the shared harness in a nested module
here means `usql` has to depend on a module inside `dbmeta` to start a
container.

#### What actually reaches a consumer

The two reviews disagreed here and the accurate answer is between them.

A `tool` directive adds a `require` line to the module that declares it. That
much is certain, and it is why D26 moves the pin out of the root.

What a consumer of `dbmeta` gets is less than it looks. Module graph pruning
drops a dependency that provides no imported package, so the drivers reach
neither the consumer's build list nor its `go.sum`. DeepSeek said nothing
reaches a consumer at all. Gemini added the qualification that matters: a
scanner that reads `go.mod` as text rather than resolving the build list will
report those drivers, and a consumer then sees advisories for code it never
compiles.

So the honest statement is that the root module having no driver is about the
dependency surface people read and scan, not about what the compiler links.
That is still worth having, and with D27 it costs nothing.

#### Verified: a nested module needs no underscore

Gemini suggested naming the directory `_test`, because the Go tool ignores a
directory whose name begins with an underscore. DeepSeek said an underscore is
unwise. DeepSeek is right, and this was checked rather than argued.

A throwaway module was built with a nested `test/go.mod` and a separate
`_hidden/` directory. Running `go list ./...` in the parent listed neither. The
nested module is already excluded from the parent's `./...` because it has its
own `go.mod`. The underscore adds nothing and it hides the directory from
Dependabot, Renovate and editors as well.

Use a plain name. `test` is the obvious one.

Do not put it under `internal/`. It does not need to be, because a nested
module is already a boundary, and `internal/` would block reuse if any part of
it is later shared.

#### What this costs, and what to do about each

A nested module is not free. Four things need handling and none is hard.

1. `go test ./...` from the root does not descend into it. CI runs it a second
   time from inside the directory. Verified above.
2. Editors and a local `go build` across both modules want a `go.work` file.
   Do not commit it.
3. Dependabot needs a second entry pointing at the nested directory, or it will
   never see those drivers.
4. Tagging a nested module requires a path prefixed tag, such as
   `test/v0.1.0`. This only matters if something outside ever imports it, which
   nothing should.

#### Precedent

Both reviews named `golang.org/x/tools/gopls`, which is a nested module
specifically so that the heavy dependencies of `gopls` stay out of the
lightweight `x/tools` library. Gemini also named the OpenTelemetry Go
repository, which uses nested modules per instrumentation to keep third party
drivers out of the core SDK. Other examples they gave were not verified.

### D28. Root tests use a fake driver replaying captured data. Decided.

Tests in the root module use a fake `database/sql/driver` implementation
written in a `_test.go` file, replaying captured responses from `testdata`.

This is the right shape and both reviews agreed. `database/sql/driver` is in
the standard library, so the fake costs no dependency and D26 holds without
strain. The tests are fast, they need no container, and they can cover a server
version whose image no longer starts, which matters because D20 supports
PostgreSQL back to 9.6 and those images were last built in 2022.

Note that this is a replay driver, not an expectation mock. It answers with
recorded bytes from a real server. Gemini suggested `DATA-DOG/go-sqlmock`
instead, which is a different tool that asserts which queries were issued. It
is also a third party dependency, which D26 forbids. Do not use it.

#### What a replay test cannot catch

Both reviews produced overlapping lists. This matters, because a green test
suite that proves less than it appears to is worse than a smaller one.

A replay test cannot catch:

1. Invalid SQL. The server never parses the query, so a syntax error or a
   missing catalog column passes. This is the largest gap and it is the exact
   class of fault that the `pg_attrdef.adsrc` removal created.
2. Driver level type translation. A driver converts wire types into
   `driver.Value`, which permits only a few Go types. A capture freezes the
   translation that one driver version performed. If a consumer uses a
   different driver, or a later version changes how it surfaces a type, the
   test still passes.
3. Permissions and visibility. `information_schema` hides what the connected
   user cannot see, so the answer depends on the user, and a capture records
   one user.
4. Session state. Real drivers set things on connect, and results depend on
   `search_path`, timezone, character set, collation, `sql_mode` and the Oracle
   and SQL Server equivalents.
5. Driver specific error types and codes, connection pooling, retries on a bad
   connection, and any ordering that the server does not guarantee.

The conclusion is not to abandon the fake. It is that the fake proves the Go
side and proves nothing about the SQL. Live tests in the other module prove the
SQL. Neither replaces the other, and the plan must not let the fast one create
the impression that the slow one is optional.

#### Captures must carry their provenance

A capture with no provenance becomes folklore. Every capture records:

1. The database engine, and the exact server version, build and edition.
2. The driver module and its exact version.
3. The connected user and the grants it held.
4. The session settings in effect.
5. The query, its parameters, and the resulting columns, rows or error.
6. The container image digest that produced it, which D12 already requires.
7. The commit of the capture tool, and a checksum.

Detect a stale capture mechanically rather than by review. Key each capture to
a hash of the query that produced it. When the SQL changes and the capture does
not, fail. Re-capture against the live matrix on a schedule and fail on a
normalized difference.

Never hand edit a capture. A capture is a generated artifact. A corpus that
someone has corrected by hand records a server that does not exist.

### D29. Pure Go only. No single package imports every driver. Decided.

The proposal put all driver imports in one sub-package. Both reviews rejected
that, and they were right, though the reason is narrower than Gemini stated.

Importing every driver into one package forces cgo on everyone who builds it,
breaks `CGO_ENABLED=0` and cross compilation, and invites a conflict between
two drivers over a shared transitive dependency.

#### Verified: only DuckDB actually forces cgo

Gemini called this a fatal flaw and named SQLite3, Oracle and DuckDB as cgo
drivers. That is true of the drivers it picked and false as a constraint,
because a pure Go driver exists for all but one. Checked in the local module
cache on 2026-09-24 by looking for `import "C"`:

| Database          | Pure Go driver         | cgo driver to avoid |
| ----------------- | ---------------------- | ------------------- |
| PostgreSQL        | `jackc/pgx`, `lib/pq`  | none needed         |
| MySQL and MariaDB | `go-sql-driver/mysql`  | none needed         |
| SQLite3           | `modernc.org/sqlite`   | `mattn/go-sqlite3`  |
| SQL Server        | `microsoft/go-mssqldb` | none needed         |
| Oracle            | `sijms/go-ora`         | `godror`            |
| Cassandra         | `gocql/gocql`          | none needed         |
| DuckDB            | none                   | `duckdb/duckdb-go`  |

Six of the seven primary databases have a pure Go driver. Only DuckDB has no
alternative, and its cgo arrives with prebuilt platform libraries rather than
requiring a C toolchain.

So the rule is: prefer the pure Go driver everywhere one exists, and isolate
DuckDB. That removes most of the objection without splitting anything.

#### The rule: pure Go only, everywhere

Ken has settled this beyond preference. `dbmeta` is pure Go, and so is the
generation sub-package. Neither uses cgo. There is no build tag, no
`CGO_ENABLED` requirement, and no C toolchain anywhere in this project.

Use the pure Go driver in the table above for every database that has one.
Never import `mattn/go-sqlite3` or `godror`.

Do not put every driver import in one package even though all of them are pure
Go. A conflict over a shared transitive dependency does not care about cgo.

#### DuckDB has no pure Go driver, and D24 puts it in CI

This is an unresolved conflict between two decisions and it needs Ken.

D24 names DuckDB as one of the four databases CI tests. The table above shows
DuckDB is the one primary database with no pure Go driver. `duckdb/duckdb-go`
uses cgo, and nothing else exists. Pure Go only and DuckDB through a Go driver
cannot both hold.

Three ways out, and one of them is better than it first sounds.

1. Reach DuckDB through its own command line client rather than a Go driver.
   The container already carries the client, so the generation step runs the
   query through it and reads the result. No driver, no cgo.
2. Cover DuckDB only through captured data. D28 already replays captures in the
   root module, so DuckDB tests would run there like any other. Something still
   has to produce the capture, which returns to option 1.
3. Drop DuckDB from the tested set and support it without testing.

Option 1 generalizes further than DuckDB, which is why it is worth weighing
properly rather than treating as a workaround. Every database in the set ships
a command line client, and every container image already contains it. A
generation step built on the client needs no Go driver for any database, which
makes the pure Go rule trivially true and shrinks the sub-package's dependency
list to nothing.

The cost of option 1 is real and must not be waved away. Parsing client output
is weaker than reading typed values from a driver. Type fidelity, NULL against
empty string, and encoding all become parsing problems. A client also formats
for people and changes that formatting between releases, which is a new source
of version drift in a project already managing one.

See question 7.

### D30. dbtpl is not used to generate dbmeta. Decided, with the cost recorded.

Generation is driven by ordinary Go code in the sub-package. `dbtpl` is not
used, and `dbmeta` does not pin it. The sub-package is pure Go, like everything
else here. See D29.

Ken decided this. Both reviews questioned the stated reason and their objection
is recorded here, because a later reader will ask.

Neither review accepted that the bootstrap was a cycle. Go modules pin
versions, so `dbtpl` at one version can generate `dbmeta` at the next, the same
way a compiler bootstraps from an older build of itself. Both called it an
ordering problem rather than a module cycle. My earlier note in this file said
the same thing.

Both also warned about the cost. A hand written generator has to reimplement
what `dbtpl query` already does: connect per driver, introspect the result
columns of a statement, map database types to Go types, decide nullability,
handle arrays and enums, and emit structs and scan code. DeepSeek put it as
"most of `dbtpl`".

Two things reduce that cost and they are worth stating, because they are the
reason the decision is reasonable despite the warnings.

`dbmeta` needs far less than `dbtpl` does. `dbtpl` generates models for an
arbitrary user schema it has never seen. `dbmeta` generates models for a fixed,
known set of metadata queries that the project itself writes. The column types
are known in advance because the author wrote the query, so the hardest part of
`dbtpl`, which is inferring a type from an arbitrary result, is mostly not
needed.

D8 also removes the reason `dbtpl` had to introspect at all. Under the padding
rule every fragment returns the same columns for every version, so the result
shape of a query is fixed and can be declared rather than discovered.

If the generator starts growing type inference, stop and reconsider. That is
the signal that it is turning into `dbtpl`.

### D31. Models register from internal, one file each, gated by build tags. Decided.

The wiring layer copies `usql` and sits in `internal/`, with one file per
model. `internal/postgres.go` registers the PostgreSQL model,
`internal/mysql.go` the MySQL one, and so on.

Say models, not drivers. `usql` calls them drivers because it opens
connections. `dbmeta` never opens one, so the word here is model. Crib the
`usql` implementation and rename as you go.

#### The tags

Copy the `usql` tiers exactly: `none`, `base`, `most` and `all`. A build picks
how many models it compiles in, and a consumer that wants three databases does
not carry thirty.

`usql/internal` shows the three shapes in use:

```go
//go:build (!no_base || postgres) && !no_postgres
//go:build (all || most || couchbase) && !no_couchbase
//go:build (all || odbc) && !no_odbc
```

The first is the base tier, built by default and turned off with `no_base`. The
second is the `most` tier. The third is the `all` tier, which `usql` uses for
the three it would rather not build by default: `charts`, `odbc` and `godror`.

`dbmeta` has no equivalent of that third tier. There is no bad model here,
because a model is SQL rather than a driver with a C dependency, and D29 keeps
cgo out entirely. Use `none`, `base`, `most` and `all` only.

The `usql` base tier is these eight: `csvq`, `clickhouse`, `oracle`, `duckdb`,
`sqlserver`, `postgres`, `mysql`, `sqlite3`. Note that this is not the same set
as the primary databases in the phase plan. It has `clickhouse` and `csvq`,
which the phases do not mention, and it does not have Cassandra, which phase 3
does.

#### gen.go

A `gen.go` generates the `internal/<model>.go` files. It reads the metadata
each model declares about itself, and it produces the wiring plus the
documentation that has to agree with it, including the model links and the
support table in `README.md`.

The point is that the list of models exists once. A table in `README.md` that
someone edits by hand drifts from the code within two releases, and D21 and D24
both require that table to be accurate.

### D32. Errors are constants of a string type. Decided.

Use the `xo` house pattern. Errors are untyped constants of a defined string
type, not package level variables:

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

`dburl` does this at `dburl.go:352`, and so do `tblfmt` and `usql`.

The reason is immutability. A sentinel declared with `errors.New` is a package
level variable, and any importer can assign to it. A constant cannot be
reassigned, so no consumer can change what `dbmeta.ErrNotSupported` means for
every other consumer in the process.

Comparison still works with `errors.Is`, and wrapping still works with `%w`.

This amends the naming guidance in `CLAUDE.md`, which said to declare sentinels
with `errors.New`. The `Err` prefix and the PascalCase name are unchanged. Only
the declaration changes.

The taxonomy needs at least these four, because they are four different
answers and `usql` returns one undifferentiated error for all of them today:
not supported, permission denied, version too old, and no such object.

### D33. Results stream. The package does not materialize them. Decided.

`dbmeta` executes a model against the database and reads the result one record
at a time. It does not load a result set into memory and hand back a slice.

Alongside that, provide standard Go iterators, so a model can be consumed from
outside the package in the ordinary way.

This reverses part of D18, which said to return a typed slice such as
`[]Table` and to reach for an iterator only when a result is large enough to
matter. That guidance is withdrawn. The iterator is the shape.

Two consequences follow and both are improvements.

Question 8 answers itself. A caller that asks for the columns of 200 tables
issues one query with a schema filter and walks the rows. There is nothing to
batch and nothing to cache, so `dbmeta` holds no state, has no staleness rules
and no invalidation to get wrong.

It also removes the last reason the `usql` cursor types existed. D18 rejected
them for being a cursor over a slice already in memory. A real iterator over
live rows is the thing that cursor was pretending to be.

Note what this requires of the error handling. An error can arrive part way
through a result, so the iterator must be able to report one. Use
`iter.Seq2[T, error]`, and make the caller able to tell the end of a result
from a failure in the middle of one.

### D34. Report capabilities, and return a typed error when asked anyway. Decided.

A caller can ask what a database supports before querying it. If it asks for
something unsupported regardless, it gets `ErrNotSupported` and not an empty
result.

Both external reviews raised this independently and both stressed the same
rule: an empty result means the database has no such object. It never means
that `dbmeta` cannot ask. Returning an empty slice for an unsupported object is
the failure mode to design against, because a caller cannot tell it from a real
answer.

The same mechanism answers a second question. D8 pads a missing column with
`NULL AS name` when the server is too old to have it, which makes "this version
has no such field" look exactly like "this value is null". The capability
report is where that is resolved, by recording which fields are valid at the
detected version. These are one mechanism, not two.

### D35. DuckDB is out of the initial testing set. Decided.

DuckDB leaves the four databases that D24 puts in CI. It is not tested at the
start.

The reason is D29. `dbmeta` is pure Go, and DuckDB is the one primary database
with no pure Go driver, so it cannot be reached the way the others are.

It is not dropped. DuckDB is one of the eight base models in `usql`, so it has
to work. Its design is decided before any work starts on the models outside the
base tier, and that decision reopens the choice in D29: reach it through its
command line client, cover it by captured data alone, or something else.

Do not treat this as permission to skip it quietly. A base model that no test
touches is a gap, and the support table that `gen.go` writes must say so.


## What exists today

An agent that starts work must read these sources first.

### `usql/drivers/metadata`

This is the code to move. It contains:

- `metadata.go`, 1175 lines. It defines 15 object types, a `*Set` cursor type
  for each, a one method reader interface for each, one `Filter` type, and the
  `Writer` interface.
- `reader.go`, 273 lines. It defines `PluginReader`, which composes readers by
  type assertion, and `LoggingReader`, which adds logging, dry run, and a
  timeout.
- `writer.go`, 838 lines. It defines `DefaultWriter`, which renders the output
  of the `\d` family of commands through `tblfmt`.

The 15 object types are Catalog, Schema, Table, Column, ColumnStat, Index,
IndexColumn, Trigger, Constraint, ConstraintColumn, Function, FunctionColumn,
Sequence, PrivilegeSummary, and the shared `Bool`.

### `dbtpl/loader`

`dbtpl` reads a schema through a `Loader` struct of functions. It covers five
databases: postgres, mysql, sqlserver, sqlite3, and oracle. It needs four
things that `usql` does not model:

1. Enums and enum values.
2. Procedures and procedure parameters.
3. View DDL operations: create, schema, truncate, and drop. DDL means data
   definition language, the SQL that creates and drops objects.
4. A column order query for postgres.

The union of the `usql` object set and the `dbtpl` object set is the target
object set for `dbmeta`.

### Coupling from the metadata package back into `usql`

The reader half of the package needs six symbols from `usql`. All six are small
and `dbmeta` can define its own:

- `text.ErrNotSupported`
- `text.NotSupportedByDriver`
- `text.RelationNotFound`
- `text.ErrWrongNumberOfArguments`
- `drivers.DB`, an interface of eight methods that `database/sql.DB` and
  `database/sql.Tx` both satisfy
- `env.Vars()`, used by `writer.go` only

### Driver coverage today

14 of the 44 `usql` drivers have metadata support:

clickhouse, databend, duckdb, impala, mymysql, mysql, netezza, oracle, pgx,
postgres, snowflake, sqlite3, sqlserver, trino.

30 drivers have none:

adodb, athena, avatica, bigquery, cassandra, chai, cosmos, couchbase, csvq,
databricks, dynamodb, exasol, firebird, flightsql, godror, h2, hive, ignite,
maxcompute, moderncsqlite, odbc, ots, presto, ql, sapase, saphana, spanner,
vertica, voltdb, ydb.

The 14 supported drivers are also split across two places. Five live under
`usql/drivers/metadata/`. The rest live next to their driver, for example
`usql/drivers/sqlite3/sqshared/reader.go` at 331 lines and
`usql/drivers/sqlserver/reader.go` at 239 lines. The move must collect both.

## How dbtpl generates a model

`dbtpl query` takes a database URL and a SQL statement on standard input. It
creates a temporary view from the statement, reads the column types of that
view, drops the view, and writes a Go struct and a query function. The file
`dbtpl/gen.sh` shows the pattern. It runs 60 such commands across five
databases.

Generation connects to a live database. You cannot generate a model for a
driver without a running instance of that database.

These flags matter for this project:

- `--go-pkg` sets the package name. `dbmeta/models/<driver>` needs it.
- `-2`, or `--go-not-first`, suppresses the shared package file. The first
  command for a package writes that file. Every later command for the same
  package must pass `-2`.
- `-U`, or `--allow-nulls`, makes a result field nullable when the introspected
  column allows NULL.
- `-Z`, or `--fields`, overrides the field names and Go types by hand. This is
  the escape hatch when introspection reports the wrong type.
- `-T` sets the struct name and `-F` sets the function name.
- `-X`, or `--exec`, turns off introspection for a statement that returns no
  rows.

## Known defects to fix once

### The NULL scan defect

Four open pull requests against `usql` fix the same fault in four places: 526
for information schema functions, 570 and 524 for oracle, and 583 for postgres.
Commit `200c0c8` fixed a fifth case by wrapping `routine_definition` in
`COALESCE`.

The cause is plain. The readers scan into struct fields of type `string`, as in
`rows.Scan(&rec.Catalog, &rec.Schema, &rec.Name, &rec.Type)`. When the database
returns SQL NULL for one of those columns, the scan fails.

Note for agents: this is not the same fault as `usql` backlog item 14. That
item describes the display path, where `usql` builds a scan destination with
`reflect.New(ct.ScanType())`. The helper `usql/drivers/columns.go:53`,
`NullSafeColumnType`, fixes the display path. It does not fix the readers,
because the readers never call it. There is no `reflect.New` anywhere under
`drivers/metadata`.

The fix for `dbmeta` has two parts. Generate nullable fields with `-U`. Force
the field type with `-Z` where introspection reports a column as NOT NULL and
the database still returns NULL. Do not port the four patches.

### The shared reader instance hazard

`informationschema.New(opts...)` builds one `*InformationSchema` value and
returns a closure over it. Every connection for that driver shares the one
configured value. `mysql.NewReader` is a package level variable built that way.

This caused a real fault. The duckdb driver registered MySQL's completer, which
carried MySQL's reader, which was configured with
`WithCurrentSchema("COALESCE(DATABASE(), '%'))")`. Every completion ran a MySQL
function against duckdb. Commit `3cd55cc` fixed it. No test caught it.

Two rules follow for `dbmeta`. Configuration must not live in a package level
variable that one driver can borrow from another. Composition of readers must
be explicit, not implicit through a shared default.

### The Bool type needs its String method

`metadata.Bool` is a named string type. A Go type switch does not match a named
type to its underlying type, so `tblfmt` printed `"YES"` with quotation marks in
`\d` output for every driver. Commit `cd8edc9` added `func (b Bool) String()
string` at `usql/drivers/metadata/metadata.go:362`. If `dbmeta` keeps `Bool`,
keep the method.

## Open pull requests against the source package

Five pull requests against `usql` touch `drivers/metadata`. The `usql` peer
session will report when any of them lands.

- 452, add column comment, touches `informationschema/metadata.go` and
  `metadata.go`
- 524, oracle driver version from a view available to all users
- 526, NULL scan in the list functions query
- 570, lower the privilege the oracle catalogs query needs
- 583, NULL scan for catalog privileges and table description

An agent that ports a reader must check whether one of these changed it first.

## External review

Two models reviewed this plan on 2026-09-24: Gemini 3.1 Pro and DeepSeek V4.
They read the same brief and did not see each other's answers. This section
records what they said, and what this project does about it.

Treat their output as an opinion, not as an instruction. Two of their claims
were wrong and are marked below.

### Both reviews agreed on four points

Add `context.Context` to every method. Verified, correct, and now decided. See
D17.

Hold no global state. Bind a reader to one connection. Both reviews pointed at
the DuckDB fault as evidence. This confirms the rule already written under the
shared instance hazard.

Distinguish "not supported" from "found nothing". An empty list must mean that
the database has no such object. It must not mean that the driver cannot ask.
This answers question 6 in favor of a capability report.

Use nullable field types for every optional column. This confirms D6.

### Both reviews attacked D8, and their alternative is worth taking

Both called discrete packages per database version a combinatorial explosion.
Both said to follow what `psql` does and resolve the version at run time.

DeepSeek gave a concrete form that fits this project better than either
extreme. Generate the version differences as data rather than as packages:

```go
[]Fragment{
    {MinVersion: 120000, SQL: ...},
    {MinVersion: 150000, SQL: ...},
}
```

One package per driver holds the fragments. A selector merges them against the
detected server version. This keeps what D8 asks for, which is a model that
varies by version, and it drops what D8 costs, which is a package per release.

It also answers three open questions at once. Question 3 disappears, because
there is no version directory to name. Question 4 disappears, because data
needs no inheritance. Question 1 gets simpler, because the adapter has one
package per driver to live in.

Ken must decide this. D8 stands until he does.

### Both reviews attacked D9, and both partly misread it

Both said that ranking PostgreSQL first over fits the API to one database.
DeepSeek wrote that non PostgreSQL users would get "PG-shaped wrong answers".

The concern is real but the reading is too strong. D9 makes PostgreSQL the
reference for the shape and the behavior of an object that two databases both
have. It does not require every database to answer all 48 objects. The
capability mechanism covers the gap.

One phrase invited the misreading. "When they disagree, follow PostgreSQL" read
as a rule about every field. Ken agreed with this assessment and D9 now carries
the narrower wording. The ranking decides the shape of an answer. It does not
decide which questions a database must be able to answer.

### Both reviews attacked D7 on tests, and the container half is already answered

Gemini said to allow `testcontainers-go` and `google/go-cmp` in tests. DeepSeek
called stdlib only testing dogmatic.

Half of this objection is already answered. D12 starts containers with the
`podman-run.sh` script from `usql/contrib`, so no Go container library is
needed. The `usql` tests use `ory/dockertest` only because they predate that
choice.

The other half is real but small. Comparing a large metadata struct without
`go-cmp` means writing the comparison and the diff by hand. That is a cost, not
a blocker.

### Two claims from the reviews are wrong

Gemini called `dbtpl` generating `dbmeta` a cyclic dependency. It is not.
`dbtpl` is a build time tool, pinned under D11. The runtime dependency runs one
way, from `dbtpl` to `dbmeta`. This is bootstrapping, which is ordinary.

Gemini said to abandon `dbtpl` and hand write the queries with `go:embed`. That
contradicts D2, D10 and D11, which Ken decided. The review did not know that.

### Points raised that the plan did not cover

DeepSeek raised these and they are not yet decided anywhere:

1. An error taxonomy. Separate "not supported" from "permission denied", from
   "connection failed", and from "version too old". The `usql` readers return
   one undifferentiated error today, and the oracle privilege pull requests
   exist because of it.
2. Reproducible generation. D12 generates against a live container. Most
   `usql/contrib` configs name an untagged image, so the same command run twice
   can read two different servers. Pin the image digest and record which digest
   produced which model.
3. Identifier handling. Quoting, case folding, reserved words, and the maximum
   identifier length differ per database and the plan says nothing about them.
4. Visibility. `information_schema` hides objects that the connected user
   cannot see, so two users get two answers from the same query. Metadata tests
   must fix the user, and the API must say which user it reflects.
5. Repeated queries. A caller that asks for the columns of 200 tables must not
   send 200 queries. Decide whether the API batches, caches, or leaves this to
   the caller.

## Plan

Ken set these phases. They replace any earlier ordering in this file.

D13 sets the order within them. Build the models for the primary databases
first, starting with PostgreSQL and then MariaDB. Write the root `dbmeta`
package after those models exist, so that the API describes what the databases
can answer instead of guessing at it.

The API surface is the contract that binds all three phases. Phase 1 defines
it. Phase 2 and phase 3 implement the same surface for other databases. Where a
database cannot answer, it reports that it cannot, and it does not change the
shape of the answer.

Read one phrase carefully. "Information schema compatible" describes the API,
not the source of the data. Three of the phase 3 databases have no
`information_schema` at all. They still present the same surface.

### Phase 1. Translate the PostgreSQL queries from the PostgreSQL source

This phase produces the primary platonic model. Every later phase copies its
API.

Translate from the PostgreSQL source, not from `xo/pgdesc`. The file is
`src/bin/psql/describe.c`. Use `pgdesc` as a second opinion when a translation
is unclear, and remember that it is old and that its `TODO` file lists seven
known faults.

Size of the work, measured on the local checkout:

- `describe.c` is 7009 lines.
- It holds 48 entry points. They are the `describe*`, `list*`,
  `permissionsList` and `objectDescription` functions.
- It holds 76 version gates of the form `pset.sversion >= 110000`. They span
  release 9.3 to release 16.

The version gates are the work, not a detail. `psql` writes one query and
switches fragments on the integer server version. D8 requires one model per
version instead. Each gate you meet becomes a decision about which versions get
their own model.

Two source versions need a decision before this phase starts. The local
checkout at `/home/ken/src/postgres` sits at `REL_15_BETA2-740-ge59a67fb8f`,
which is release 15 under development. The installed client is 18.6. Ask Ken
which releases `dbmeta` supports, then check out each one in turn and translate
from it.

Read `SHOW server_version_num` to select a model at run time. It returns the
same integer that `describe.c` compares against.

### Phase 2. Build the information schema reader, with MariaDB as the reference

This phase produces the second platonic model. It must present the same API
surface as phase 1.

Use MariaDB as the reference database. Its `information_schema` is the one to
read against while the code takes shape.

Carry over the variation mechanism from
`usql/drivers/metadata/informationschema`. See D9 for its four parts: the
feature flags, the named clauses, the placeholder function, and the schema
lists. Add the version dimension that D8 requires, and make the description a
value the caller owns rather than a package level variable.

Expect gaps. `information_schema` does not describe most of the 48 objects that
`psql` describes. Report each gap through the capability mechanism. Do not
invent a query that returns a partly filled object.

### Phase 3. Extend to the remaining reference databases

Add these, each presenting the same API surface: SQLite3, DuckDB, Microsoft SQL
Server, Oracle, and Cassandra.

They split into two groups, and the split decides how much of phase 2 each one
reuses.

DuckDB and Microsoft SQL Server have an `information_schema`. Both already use
the shared reader in `usql` today. They extend phase 2 with their own clause
overrides.

SQLite3, Oracle and Cassandra have no `information_schema`. Each needs its own
queries against its own catalog. `usql` shows what SQLite3 and Oracle use.
SQLite3 reads `sqlite_master`, `sqlite_temp_master`, and the `pragma_*` table
functions such as `pragma_table_info` and `pragma_index_xinfo`. Oracle reads
the `all_*` and `dba_*` views, such as `all_tab_columns` and `all_indexes`.

Cassandra is the hardest of the five and has no reader in `usql` at all. It is
not a relational database, it speaks CQL rather than SQL, and it keeps its
metadata in the `system_schema` keyspace. Treat it as the test of whether the
API surface holds for a database that does not fit the relational model. If it
does not hold, that is a finding to report, not a reason to bend the surface
for the other five.

### Phase 4. Test, measure, and fuzz

This phase is new work. `usql` has no benchmark and no fuzz target anywhere in
the repository. Its metadata tests total about 3300 lines across seven files,
and the largest are `clickhouse_test.go` at 1660 lines and
`informationschema/metadata_test.go` at 1096 lines. Read them for the cases
they cover, not for the harness they use.

Four kinds of work belong here.

Correctness tests run the same root API calls against every supported database
and every supported version. The output shape must match across all of them,
because the API surface is the contract. Compare against golden files, so that
a change in one query shows up as a diff and not as a silent difference.

Version tests protect D8. For each database with more than one model, run the
same call against each version and assert that the caller sees the same types.
Assert also that selection picks the right model, and that a version older than
every model falls back rather than fails.

Benchmarks measure the cost per call and the cost per row. Metadata queries run
inside an interactive client, where a slow response is visible to a person.
Measure the query, the scan, and the conversion into the root types as three
separate figures, so that a regression points at one of the three.

Fuzz targets take the inputs that come from a person rather than from the
database. The `Filter` values come first, because a caller passes names and
patterns straight into a query. Fuzz the pattern handling, the clause
substitution that D9 describes, and the version parser. A fuzz target must
assert that no input causes a panic and that no input reaches the SQL unquoted.

Split the work by where it runs. D24 puts the latest version of PostgreSQL,
MySQL, SQLite3 and DuckDB in CI, and everything else on a development machine.
Write the local matrix so that one command runs it, or it will not be run.

One warning about dependencies. The `usql` metadata tests import
`github.com/ory/dockertest/v4` and `github.com/google/go-cmp/cmp`. D7 forbids
both here. Start the databases with the `podman-run.sh` script from
`usql/contrib`, and compare with `reflect.DeepEqual` or with a written
comparison. Do not copy the `usql` test harness.

### Phase 5. Integrate into usql

Change `usql` to import `dbmeta`, then delete the moved code from `usql`.

D5 keeps `writer.go` in `usql`. That file is 838 lines and it is the only one
that uses `tblfmt` and `usql/env`. Leaving the writer behind is what keeps
those two out of this module. It also uses `dburl` across the eight `Writer`
methods at lines 148 to 162, which no longer matters, because D19 makes `dburl`
a direct dependency.

Coordinate through the `usql` session. It owns that repository and has agreed
to report before any of the five open pull requests against `drivers/metadata`
lands. Check that list again before you start, because a merge during this
phase changes code that you are deleting.

Settle the completion path in this phase. `usql/drivers/completer` holds a
reader, and it drove the fault described under the shared instance hazard.
Decide whether completion reads through the `dbmeta` API or keeps its own path.

### Phase 6. Expand to the other databases

Add the databases that `usql` supports and that the earlier phases did not
cover. 14 of the 44 drivers have metadata support today and 30 do not. The
uncovered list is in the section on driver coverage.

Order the work by two facts. Prefer a driver that `usql/contrib` can already
start in a container. Prefer a driver whose database has an
`information_schema`, because phase 2 already did most of that work.

Leave the hard cases until the API has settled. Several of the 30 are not
relational at all, such as dynamodb and couchbase. The finding from the
Cassandra work in phase 3 tells you whether the surface holds for them.

## Testing plan for versions and flavors

D8 adds a version axis. D14 adds a flavor axis. A model must be tested on both,
and the two axes multiply, so the plan must keep the matrix small on purpose.

### The two axes

A version is the same product at a different age, such as PostgreSQL 13 and
PostgreSQL 18.

A flavor is a different product that claims the same driver, such as MariaDB
and MySQL under the `mysql` driver.

Neither axis contains the other. A query can work on every MariaDB release and
fail on MySQL 8. A query can work on both products at release 8 and fail on
both at release 5.

### dburl holds the flavor taxonomy. Import it.

Do not invent a list of flavors and do not copy one. D19 makes
`github.com/xo/dburl` a direct dependency, so read the taxonomy from it.
`dburl` separates two kinds of flavor and puts them on different fields of a
parsed URL. See D19 for which field carries which.

An alias is a product that `dburl` treats as the same driver. The `mysql`
scheme carries the aliases `mariadb`, `maria`, `percona`, and `aurora`. The
`sqlserver` scheme carries `azuresql`. The `oracle` scheme carries `ora`,
`oci`, `oci8`, `odpi`, and `odpi-c`.

A wire compatible is a separate product that speaks another product's protocol.
`dburl` records a parent for each one:

- `cockroachdb` and `redshift` have the parent `postgres`.
- `memsql`, `tidb` and `vitess` have the parent `mysql`.
- `oleodbc` has the parent `adodb`.

The taxonomy sets an expectation to test, not a promise that holds. CockroachDB
claims the PostgreSQL protocol and does not implement the whole PostgreSQL
catalog. Redshift forked from PostgreSQL 8.0. Each one is a test case that can
fail, and a failure is a finding to record rather than a fault to hide.

### Which combinations to run

Run three tiers. The tier decides where the test runs, not whether it matters.
D24 sets the split between CI and a development machine.

Tier 1 runs in CI on every change. It holds three databases at their latest
version: PostgreSQL, MySQL and SQLite3. SQLite3 is embedded, so it costs
nothing. PostgreSQL and MySQL are preinstalled on the runner. DuckDB was in
this tier until D35 removed it. More databases can join later.

Tier 2 runs on a development machine, not in CI. It holds every supported major
release of each primary driver. This is the version axis and it is what proves
that model selection works. D22 explains why a sample of versions is not
enough, and D24 explains why it still moved off CI.

Tier 3 also runs on a development machine. It holds the flavors: MariaDB
against the `mysql` models, and CockroachDB, Redshift and TiDB against their
parent models. Note that tier 1 already covers one flavor by accident, because
the runner provides MySQL while D14 makes MariaDB the reference.

Tier 2 and tier 3 must run before a release. See D24.

### What a test asserts

Every tier asserts the same three things.

1. The call returns without an error, or returns a "not supported" error that
   names the object. It never returns an empty list to mean "cannot ask".
2. The shape of the result matches the golden file for that object. The shape
   is the contract, and it must not change with the database.
3. No field that the database can return as NULL reaches a caller as a zero
   value that is indistinguishable from a real value.

Tier 2 asserts one more thing. Given a server version, selection picks the
newest model that is not newer than the server, and a server older than every
model falls back rather than fails.

### Requirements on the container harness

D12 reuses the podman configuration in `usql/contrib`. Three changes are
needed before the matrix above can run.

1. Most configs name an image without a tag, as in
   `IMAGE=docker.io/usql/postgres`. A version axis needs a tag per version.
2. Pin the image digest, not only the tag. A tag moves. Record which digest
   produced which model, so that a generated model can be traced to the server
   that produced it.
3. Add configs for the flavors in tier 3. `contrib` has `cockroach` already. It
   has no MySQL config, because its `mysql` directory runs the MariaDB image.

### Fixing the user, and why it matters

`information_schema` hides the objects that the connected user cannot see. Two
users therefore get two answers from the same query against the same database.
Every test must connect as a named user with a fixed set of grants, and the
golden files must record which user produced them.

This is not a detail. Two of the five open pull requests against
`usql/drivers/metadata` exist because an Oracle query needed a privilege that
an ordinary user does not have.

## Open questions for Ken

None. Every question raised in this document has been answered, and every
decision is marked Decided or Superseded.

Two things are deferred rather than open, and both have an owner and a trigger.

D4 and question 4 as it was: whether `dbmeta` exports interfaces at all, and
under what names. Deferred until D13 delivers the PostgreSQL and MariaDB
models, when the real shape is visible. Nothing depends on it until the root
package is written.

D35: how DuckDB is reached, given pure Go only. Deferred until before work
starts on the models outside the base tier. DuckDB is a base model in `usql`,
so this cannot be dropped, only scheduled.

Raise a new question here rather than deciding one alone.
