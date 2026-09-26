# Adding a Dialect

Every step needed to add a database to `dbmeta`, in the order to do them.

Read this before you start. The rules themselves live in `CLAUDE.md` and the
reasoning lives in `PLAN.md`, which is large. This file is the checklist, and
it points at both.

Read [`NULLS.md`](NULLS.md) before you write the first query. It is 139 lines,
it is the shortest document here, and it cost three rounds of real bugs to
learn. Step 10 says where it applies. It is a separate document because its
audience is anyone writing any query, not only somebody adding a dialect.

A dialect is one deliverable. The queries, the fixture, the tests, the parity
targets and the documentation ship together. A dialect with queries and none of
the rest is not nearly finished. It is one whose answers have been measured
once, by you, on one server, as one user.

## Before you write anything

### 1. Check it is the one to do next

`PLAN.md` D66 holds the order and D67 amends it. Do not start a database out of
order without asking Ken.

### 2. Choose the version range

Follow the procedure in [`EVALUATION.md`](EVALUATION.md). Work through its four
steps and stop at the first that gives a clear answer. Record the floor, the
ceiling and which step decided, in the doc comment of the container file you
write next.

Step 2 does not always answer. Trino publishes a tag per release and rebuilds
none of them, so "the oldest release still rebuilt" means nothing there, and
step 3 decided instead. Say so when it happens.

### 3. Find the driver `usql` uses

```bash
grep -rn "// DRIVER" ~/src/go/src/github.com/xo/usql
```

D52 requires the same package `usql` uses. The version may differ and the
package may not. A query that works here and fails on the driver `usql` ships
is a query that does not work.

If `usql` ships two drivers for the product, test both, as subtests named for
the driver. SQLite and PostgreSQL both do this.

## Standing the server up

### 4. Write the container entry

Add `container/<product>.go`. One file per database, holding the product, its
release list and any helper only it needs. Give the product var a doc comment.

The list names every release and its tier. Read D40 for the tiers and D42 for
what runs on a push against what runs nightly.

Add the product to `All()` in `container/container.go`. Do this before anything
else that needs a server, because D68 means nothing else may start one.

Use `container.Password` for the password. It is one value for every product
and it is shaped to clear the strictest policy any of them enforces.

Check before this that the published image can do what the queries read. Two
here cannot and are built instead. The Apache Cassandra image refuses a user
defined function, a materialized view and a role, and its entrypoint maps only
eight yaml keys to environment variables, none of them those. Oracle publishes
no free 19c image at all. `dbrun build` makes both, the Containerfile is
embedded with `//go:embed` so the command carries its own input, and
`buildFor` in `test/cmd/dbrun/build.go` is where a third would go.

Look for a configuration switch before reaching for a built image. Trino
refuses `CREATE CATALOG` under its default static catalog management and its
image reads that setting from an environment variable, so a catalog can be
added at run time with no build at all.

### 5. Add the dialect constant

Add it to the `Dialect` block in `dialect.go`, in alphabetical order. The value
is the `dburl` driver name, which is not always the word the database calls
itself.

If the database is a library rather than a server, set `Embedded: true` in the
`Info` the model registers, at step 10. Nothing else has to be told: `dbrun`
builds its target list from that flag, and a model that does not declare it
will not appear at all. `TestEveryEmbeddedModelSaysSo` checks both directions,
including that an embedded model is exempt from parity.

### 5a. If two products share the dialect

A driver is a family rather than a product, which is D14. MariaDB and MySQL
share one dialect and one model, and so would Trino and Presto.

When that happens the model must tell them apart by a product key and never by
the number alone. MariaDB 11.8 and MySQL 9 have no numeric relation, so a gate
at `V(10, 2)` silently means "MariaDB only" and shipped a wrong answer once
already. Write `Gate{Key: MariaDB, Min: V(10, 2)}` and set a key only for a
product you detected. Hard rule 3 and D44 hold this.

Each product gets its own container file and its own release list. The
reference product is the one the queries are written against and the other is
the flavor. Say which is which in both files.

`parityFlavors` and `parityName` in `test/parity_test.go` then file each
product's parity answers under its own name, because two products that share a
dialect do not share the tables their queries are refused on.

### 6. Start it

```bash
cd test && go run ./cmd/dbrun start <product>-<release>
```

Never start a container any other way. D68 says why, and
[`RUNNER.md`](RUNNER.md) is the design of the command. `dbrun usql <name>` opens
a shell on it and `dbrun dsn <name>` prints the URL.

### Three kinds of database, and which steps change

Most of this file assumes a server in a Linux container. Two kinds are not
that, and each changes a handful of steps rather than all of them.

**An embedded database is a library.** SQLite and DuckDB have no server, no
port, no password and no release to pin, because the release is whichever one
the driver links. D42 keeps them out of `container/container.go` and they must
stay out. Declare them in the `embedded` list in `test/cmd/dbrun/target.go`
instead, so that `dbrun test sqlite3` works and `dbrun status` says what they
are rather than leaving somebody wondering why the name is missing.

What changes for an embedded database:

- Set `Embedded: true` in the `Info` the model registers. That is the whole
  declaration, and `dbrun` reads it rather than keeping a list of its own. A
  consumer reads it too: `usql` and `dbtpl` have the same question, which is
  why the fact is in `dbmeta` and not in the test code. `dburl` marks the
  same schemes `Opaque`, but that says how a URL parses rather than what the
  product is, and `dbmeta` cannot read it anyway: it has no dependencies, and
  a caller holding a `Dialect` has no URL to hand to `dburl`.
- Skip steps 4 and 6 entirely. There is nothing to start, and the database is
  a file under `$XDG_DATA_HOME/dbmeta/embedded` that `dbrun` names and keeps.
- The test opens a file in `t.TempDir()` rather than reading a DSN from the
  environment, and never skips for a missing server.
- CI runs them in the job that starts no container. Nothing to add: the
  workflow already has it.
- They go in `parityExempt` with the reason, not in `parityTargets`. A file on
  disk has no user, so there is no second principal to be, and hard rule 16
  cannot reach them. That is an exemption for the only reason an exemption is
  allowed.
- Where the product has two drivers, run every test twice as subtests named
  for the driver. SQLite does, because `mattn/go-sqlite3` compiles the
  upstream source and `modernc.org/sqlite` is a translation of it, and they
  ship different library versions.

**An old release with no Linux container needs a Windows machine.** SQL Server
on Linux begins at 2017, so 2008 R2 through 2016 have no container and no way
to run in CI. D57 is the decision and [`WINDOWS.md`](WINDOWS.md) is the whole
procedure. Read that rather than this section if you are provisioning one.

What changes for a machine:

- It is declared in `container/windows.go` as a `WindowsVM`, not as a
  `product` in the product's own file. The fields carry what the era needs:
  the Windows release that hosts it, the installer, and the registry key for
  the instance, which is named for the SQL Server release and writes the port
  where nothing reads it if you get it wrong.
- Its tier is always Verified and never Tested. A machine needs KVM and an
  hour, so CI cannot run one. D40 has the tiers and D64 fails when a Verified
  release is not documented as such.
- `dbrun provision <name>` builds it. `--render` writes the payload and stops,
  which is how the templating is checked in a second rather than an hour.
  Every other `dbrun` verb then treats it as an ordinary target.
- A parity scene the release is too old for carries a `min` and is skipped
  with the reason. A contained database arrived in SQL Server 2012, so 2008 R2
  skips that scene rather than failing.
- The machine is kept after a test, not removed, because rebuilding it is an
  hour where a container is a minute.
- The evaluation licence rearms itself at every boot. D65 explains it and
  nothing here activates Windows.

A frozen baseline is the point of both. The machine exists so that a release
nobody can run in CI is still measured before a release, rather than being
claimed without evidence. Say which tier a release sits in and never call one
supported without naming it.

## Finding out what it can answer

### 7. Survey the catalog yourself first

List every metadata source the product has and map it against the 55 object
kinds. Run the statements. Read the columns back. A source that looks right in
the documentation and returns nothing on a real server is not a source.

Two things are worth checking early, because they shape every query:

Does a source span the whole server, or only the thing you are connected to?
Trino has both, and the per catalog one silently returns nothing for a filter
naming another catalog, which is a wrong answer rather than an empty one.

Which source carries the comment? It is often not the obvious one.

### 8. Ask at least two models, then verify every answer

D43 and hard rule 14 require this, and it is not a formality. Ask two of
Gemini, DeepSeek and Astra to sort the kinds you could not answer into three
groups: absent from the product, present under another name, and derivable from
one statement.

Then run every lead against a real server. Every one.

Both outcomes happen. For MariaDB a second opinion named the tables holding
four queries that looked unanswerable. For Trino one model invented eight
sources that do not exist, and three of its answers rested on the first of
them. The rule pays for itself either way, and only if you check.

Leave an analogue that is a stretch unsupported. Record it in
[`COVERAGE.md`](COVERAGE.md) with the reason.

### 9. Compare the version query against `usql`

D38 requires this. Find `usql`'s side in the `Version` field of its registered
`drivers.Driver`. A driver that declares none falls through to the generic
`SELECT version();`, and that fallback is its statement rather than an absence
of one.

Record a row in the statements table in [`USQL.md`](USQL.md). Say what each
side runs and whether the answers agree. `TestEveryModelIsInTheVersionTable`
fails when a model has no row.

## Writing it

### 10. The model

Add `models/<driver>/`. Nothing generates it, so write it and edit it in place.
See D71.

The package file holds the doc comment, the version query, `parseVersion`, the
`init` that registers the dialect, and the small helpers. Split the bindings
across files by what they describe, the way `relation.go`, `role.go` and
`extra.go` do elsewhere.

The package doc states how many of the 55 the model answers.
`TestEveryPackageCommentStatesItsCount` checks the number.

Each object kind is a `Binding` registered from `init`, carrying the statement
as `Stmt`, the `Fields` it returns, the `Params` it takes and a `Scan`.

Four rules decide most of the detail, and the first one is the one that has
cost this project the most.

**Never hide a NULL.** Read [`NULLS.md`](NULLS.md) in full before writing a
query, and keep it open while you write them. It is the whole rule and it is
139 lines.

The short version, which is not a substitute for reading it. A database
returns NULL to mean something, and turning it into an empty string, a zero or
a false throws that meaning away with nothing downstream able to recover it.
There are two ways to lose one and both have shipped a bug here. Do not wrap a
nullable catalog column in `COALESCE` so that a Go field can stay a plain
`string`. Do not pad a column the old release has no source for with a
literal, because a caller cannot tell a padded value from a real one.

Give the field `sql.Null[T]` and select `NULL AS "name"`. There is no alias
for a nullable type and there must not be one, which is D51. Before padding,
ask whether the value on the old release is unknown or genuinely that value,
and set `Field.Min` only for the first.

A column the catalog declares NOT NULL can still arrive NULL through an outer
join, so the declaration is not the answer. Only a real server is.

A query that must differ between releases is one statement with version
fragments, and a fragment must never change the column set. See hard rule 3.
Where two products share a dialect, gate on the product key and never on the
number alone. See D44.

Return a fact the product has, even where `psql` does not print it, as long as
one statement can produce it. See D47 and hard rule 13.

Describe every field that is always empty, always false or always absent, and
say why in the `Desc`. A reader of `go doc` should never have to test a column
to find out that it is never filled.

Three answers are not "supported" and each is different. A product that has no
such object reports `NotSupported`, and an empty result must never be used to
mean it, which is D34. A server older than the release that added the object
reports `TooOld`, which an upgrade fixes, and D63 added that state for it. A
model left out of the build reports `NotBuilt`.

A query may answer part of a question, once, and must say so. D45 allows it
under four conditions and SQLite constraints are the only case: the part
returned is exact, the part missing is missing structurally, the field
description names what is missing, and a test asserts the absence so that it
stays a decision rather than becoming a bug.

### 11. Register it

Add `all/<driver>.go`, a blank import behind the same build tag shape the
others use.

### 12. Add the driver to the test module

Add it to `test/go.mod` and to the `depguard` allow list in
`test/.golangci.yml`. The lint fails until you do, which is the reminder.

### 13. The fixture

Add `models/<driver>/fixture/`. Hard rule 9 requires it: the fixture creates
one of every object the queries read, so a test has rows worth checking.

D53 requires the core objects to match every other fixture, so the cross family
comparison reads the same schema everywhere. Copy the shape from an existing
one.

The package doc says what the product cannot build, and why. An object the
fixture cannot create is not a failure, and it must be written down: ClickHouse
cannot make a named collection and Trino cannot make a role, so those queries
are verified to run and return nothing.

## Proving it

### 14. The integration tests

Add `test/<driver>_test.go`. At a minimum:

- a version test, checking what the model makes of what the server reports
- one that runs every supported query and checks the columns match the fields
- one that reads the fixture back through the typed API
- one per thing that is peculiar to this product

Write a test for every "always empty" claim that could rot, and for every
object the fixture cannot build. `TestSQLiteConstraints` asserts that the two
check constraints the fixture creates do not appear, because D45 says SQLite
answers three quarters of that question and never the fourth. Without the test
a deliberate absence is indistinguishable from a query that forgot them.

### 15. Conformance

Add the database to `conformTargets` in `test/conform_test.go`, then record it:

```bash
cd test && go test -run TestConformance -update ./...
```

Read the diff. `TestConformanceAgreementHolds` is a ratchet and it will fail if
the new database makes the others agree on less. Decide whether that is a fault
or a fact. If it is a fact, add the database to `agreementExcluded` with the
reason rather than lowering the floor.

### 16. Parity

Hard rule 16 and D61. Add the principals to `parityTargets` in
`test/parity_test.go`, then record:

```bash
cd test && go test -run TestPrivilegeParity -update ./...
```

Read the diff. A query that answers differently for a lesser principal has
begun depending on who is asking.

A principal is not one thing. SQL Server has three and Oracle has the same
three from 12c. A product with no second principal at all goes in
`parityExempt` with the reason, and only for that reason.
`TestEveryDialectIsMeasuredForParity` fails when a dialect has neither.

A scene the server is too old for carries a `min` and is skipped with the
reason, the way a fixture step is.

Where one release genuinely answers differently from the rest, write a section
named `product@major` rather than lowering the shared one. Two exist.
PostgreSQL 12 grants public SELECT on six columns of `pg_subscription` and not
on the seventh, so an ordinary role is refused the whole query where 13 serves
it. MariaDB 10 reports a routine definition as NULL to a grantee, because
`SHOW CREATE ROUTINE` became a grantable privilege in 11.3 and there is nothing
to grant before it.

Both were found by CI rather than by the cross-release check meant to catch
them, so expect to find yours the same way.

## Writing it down

### 17. The counts

Three places hold a count and a test checks each:

- the table at the top of [`COVERAGE.md`](COVERAGE.md)
- the support table in `README.md`
- the model's own package doc

Add the database to the list in `all/coverage_test.go`, in both `answers` and
the name map in `TestTheReadmeTableIsRight`. The tests then tell you which
numbers are wrong.

### 18. The coverage section

Add a section to [`COVERAGE.md`](COVERAGE.md). It is the record of what the
database can and cannot do, and it is what a consumer reads. Cover:

- which catalog the model reads, and why that one
- what it answers
- what it cannot answer, and whether the thing is absent or merely unreachable
- what a second opinion found, including the leads that turned out to be wrong
- what the fixture cannot build
- what the conformance test says
- which answers depend on who is asking

### 19. The two consumer documents

Both are updated for every dialect, not only when something changes.

[`USQL.md`](USQL.md) needs the version query row from step 9, and a line in the
command coverage if the new model changes what `usql` could answer.

[`DBTPL.md`](DBTPL.md) needs two rows. One in the table counting how many of
`dbtpl`'s nine reads the model answers. One in the table saying whether `dbtpl`
could generate from the database at all, which is a different question and is
not implied by the count. `TestEveryModelSaysWhetherDbtplCanUseIt` fails when a
model has no verdict. A database can answer most of the nine and still be
useless to a generator: Trino answers four and the answer is no, because it has
no foreign key, so there are no relationships for `dbtpl` to follow.

Say which of the two it is and why, in the product's own terms.

Add a decision to `PLAN.md` for anything that had to be decided rather than
discovered. Put the status in the heading and say so in both headings when it
changes an earlier decision. See D50.

### 20. CI

Nothing to do. D69 means the workflow builds its matrix from
`container/container.go`, so a release added there is a job. Do not add it to
the YAML.

## Before you call it done

```bash
gofmt -l . && go vet ./... && go build ./... && go test -race -count=2 ./...
golangci-lint run ./... && (cd test && golangci-lint run ./...)
cd test && go run ./cmd/dbrun test <product>
```

`gofmt -l .` must print nothing. Run the tests with `-count=2`, because that is
what CI runs.

## When the answer is that it cannot be a model

Some products cannot be one, and finding that out is a result rather than a
failure. Say so, record it, and move to the next.

Impala is the case. D66 put it first because `usql` had a hand written reader
for it, and D67 struck it off: it has no queryable catalog, and its `usql`
reader is not SQL but a set of `DESCRIBE` calls parsed in Go. A model here is
one statement per object kind against a catalog, so there was nothing to move.
Four containers were started to prove it before the decision was written.

The test is whether the product has a catalog a statement can read. A product
that answers only through `SHOW` or `DESCRIBE` cannot be a model, because those
are statements rather than relations and cannot be filtered, joined or aliased.
Trino is the same shape in miniature: its function list exists only behind
`SHOW FUNCTIONS`, so Functions is unanswered while everything with a table
behind it is answered.

Write the decision in `PLAN.md`, amend D66 if the order changes, and put the
evidence in it. A later reader will ask why the product is missing.

## The tests that tell you what you forgot

These exist so that an unfinished dialect fails rather than passing quietly.
Reading the list is faster than rediscovering them one at a time:

| Test | Fails when |
| --- | --- |
| `TestEveryDialectIsMeasuredForParity` | a dialect has no parity target and no recorded reason |
| `TestEveryModelIsInTheVersionTable` | a model's version query was never compared against `usql`'s |
| `TestEveryModelSaysWhetherDbtplCanUseIt` | a model has no verdict in `DBTPL.md` on whether `dbtpl` could generate from it |
| `TestEveryEmbeddedModelSaysSo` | a library does not declare `Embedded`, or declares it and is not parity exempt |
| `TestTheCoverageTableIsRight` | the count in `COVERAGE.md` is not what the model answers |
| `TestTheReadmeTableIsRight` | the count in `README.md` is not what the model answers |
| `TestEveryPackageCommentStatesItsCount` | the count in the package doc is wrong |
| `TestConformanceAgreementHolds` | the new database makes the others agree on less |
| `TestWorkflowReadsTheList` | the workflow stopped reading the release list |
| `TestTheDecisionIndexIsComplete` | a decision is written and not indexed |
| `TestEveryDecisionReferenceExists` | a document points at a decision that does not exist |
| `TestTheCountsInProseAreRight` | a number written in prose went stale |

Three of those check a table that could be generated instead. The `usql`
session made the argument and it is right: `usql` does not test its README
driver table, it builds it from the `dburl` registry, so there is nothing for
the prose to drift from. Generation is stronger than a test, because a test
tells you the prose is stale and generation means it never was.

Where a table is derived from something the code already knows, generate it.
Keep a test for the residue that cannot be, which here is the counts written
as words in running prose.

Assert the unit, not only the number. The `usql` session made this point after
a day in which every wrong figure between the two projects was a right count of
the wrong thing: four open pull requests that were two, eight commands that
were eleven, eight gating readers that were seven, 47 drivers that were 51 under
a build nobody had stated. A test that checks a number without checking what is
being counted catches none of those. Write the unit into the sentence the test
greps, so that changing the unit breaks the test.

And the honest limit on generation, which is theirs too: it beats a test only
where something structured already knows the answer. A number measured by a
program that is then deleted is prose, and it rots like prose.
