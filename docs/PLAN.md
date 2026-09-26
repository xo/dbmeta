# dbmeta Plan and Decisions

This document records the plan for `github.com/xo/dbmeta` and the decisions
made so far. Other coding agents must read this file before they change code
in this repository.

Status of this repository at the time of writing: `go.mod` and `LICENSE` only.
There are no commits and no Go source files.

Three other documents sit beside this one. `CLAUDE.md` holds the rules for
writing code here. `EVALUATION.md` holds the method for deciding which versions
of a database to support. `QUERIES.md` surveys what `psql` and
`information_schema` each describe, and compares them side by side. Read
`QUERIES.md` before designing the object set, and `NULLS.md` before writing a
query for any database. `COMMANDS.md` maps every `psql` metadata command to the
Go value that answers it, for wiring up a client.

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

The package `dbmeta/models/<driver>` holds the code for one driver. For
example, `dbmeta/models/sqlite3` holds the SQLite3 queries and the structs that
receive their rows. Each driver gets its own Go package, so the types in
`dbmeta/models/postgres` and the types in `dbmeta/models/sqlite3` are different
types.

There is no version directory below the driver. A database changes its metadata
between releases, and D8 holds those differences as generated data inside the
one driver package rather than as a package per release.

A third layer joins the two. Something must convert the per driver model rows
into the common types of the root package. It also merges the version fragments
for the connected server. That code is in the root package, and it is written:
D3 puts the driver agnostic API there, D8 holds the fragments as data, and
`Stmt.Build` merges them for a detected version. Each model converts its own
rows in the `Scan` function it registers.

## The decisions, in one table

Every decision is in this file and this file is append only. The index is
here so that reading one decision does not mean loading all of them: find the
number, then jump to it.

Read the status before the decision. 12 of them amend or replace an earlier
one, and a decision read without its amendment is worse than no decision. That
is the reason this is one file rather than one file per decision, and D50
records the argument.

| # | Decision | Status |
| --- | --- | --- |
| [D1](#d1-the-module-centralizes-database-metadata-decided) | The module centralizes database metadata | Decided |
| [D2](#d2-models-are-one-package-per-driver-under-modelsdriver-amended-by-d71) | Models are one package per driver under `models/<driver>` | Amended by D71 |
| [D3](#d3-the-root-package-is-the-driver-agnostic-api-decided) | The root package is the driver agnostic API | Decided |
| [D4](#d4-keep-the-object-coverage-drop-the-reader-naming-decided) | Keep the object coverage, drop the Reader naming | Decided |
| [D5](#d5-dbmeta-only-reads-amended-by-d56) | dbmeta only reads | Amended by D56 |
| [D6](#d6-fix-the-null-scan-defect-once-and-never-hide-a-null-decided-amended-in-place) | Fix the NULL scan defect once, and never hide a NULL | Decided, amended in place |
| [D7](#d7-use-the-standard-library-third-party-packages-are-a-last-resort-decided) | Use the standard library. Third party packages are a last resort | Decided |
| [D8](#d8-version-differences-are-generated-data-not-packages-decided) | Version differences are generated data, not packages | Decided |
| [D9](#d9-there-are-two-platonic-models-postgresql-is-the-primary-one-decided) | There are two platonic models. PostgreSQL is the primary one | Decided |
| [D10](#d10-take-the-initial-design-from-dbtpl-and-its-models-directory-decided) | Take the initial design from dbtpl and its models directory | Decided |
| [D11](#d11-dbtpl-is-pinned-as-a-tool-in-the-generation-module-amended-by-d26-superseded-by-d71) | dbtpl is pinned as a tool, in the generation module | Amended by D26, superseded by D71 |
| [D12](#d12-work-against-live-databases-running-in-containers-amended-by-d70-and-d71) | Work against live databases running in containers | Amended by D70 and D71 |
| [D13](#d13-build-the-models-before-the-root-package-decided) | Build the models before the root package | Decided |
| [D14](#d14-a-driver-is-a-family-not-a-product-decided) | A driver is a family, not a product | Decided |
| [D15](#d15-ci-runs-on-github-actions-on-ubuntu-latest-only-decided) | CI runs on GitHub Actions, on ubuntu-latest only | Decided |
| [D16](#d16-user-facing-text-follows-the-simple-english-rules-decided) | User facing text follows the simple English rules | Decided |
| [D17](#d17-every-metadata-read-takes-a-context-decided) | Every metadata read takes a context | Decided |
| [D18](#d18-the-whole-package-is-idiomatic-go-decided) | The whole package is idiomatic Go | Decided |
| [D19](#d19-do-not-repeat-dburl-half-decided-half-overtaken-by-the-code) | Do not repeat dburl | Half decided, half overtaken by the code |
| [D20](#d20-postgresql-goes-back-to-96-every-other-database-starts-at-the-maintained-floor-decided) | PostgreSQL goes back to 9.6. Every other database starts at the maintained floor | Decided |
| [D21](#d21-drop-a-server-version-on-a-rule-not-on-a-judgment-decided) | Drop a server version on a rule, not on a judgment | Decided |
| [D22](#d22-test-every-supported-major-not-a-sample-of-them-superseded-by-d24) | Test every supported major, not a sample of them | Superseded by D24 |
| [D23](#d23-do-not-build-dbtest-first-let-dbmeta-pull-it-into-existence-decided) | Do not build dbtest first. Let dbmeta pull it into existence | Decided |
| [D24](#d24-ci-tests-the-latest-version-only-the-matrix-runs-locally-supersedes-d22-superseded-by-d42) | CI tests the latest version only. The matrix runs locally | Supersedes D22, superseded by D42 |
| [D25](#d25-test-on-amd64-only-no-build-tags-and-no-platform-gates-decided) | Test on amd64 only. No build tags and no platform gates | Decided |
| [D26](#d26-no-database-driver-in-the-dbmeta-module-amends-d11-amended-by-d48) | No database driver in the dbmeta module | Amends D11, amended by D48 |
| [D27](#d27-split-the-work-in-two-a-nested-test-module-here-a-shared-harness-in-dbtest-decided) | Split the work in two: a nested test module here, a shared harness in dbtest | Decided |
| [D28](#d28-root-tests-use-a-fake-driver-replaying-captured-data-decided) | Root tests use a fake driver replaying captured data | Decided |
| [D29](#d29-pure-go-only-no-single-package-imports-every-driver-amended-by-d48) | Pure Go only. No single package imports every driver | Amended by D48 |
| [D30](#d30-dbtpl-is-not-used-to-generate-dbmeta-amended-by-d71) | dbtpl is not used to generate dbmeta | Amended by D71 |
| [D31](#d31-models-register-from-internal-one-file-each-gated-by-build-tags-decided) | Models register from internal, one file each, gated by build tags | Decided |
| [D32](#d32-errors-are-constants-of-a-string-type-decided) | Errors are constants of a string type | Decided |
| [D33](#d33-results-stream-the-package-does-not-materialize-them-decided) | Results stream. The package does not materialize them | Decided |
| [D34](#d34-report-capabilities-and-return-a-typed-error-when-asked-anyway-decided) | Report capabilities, and return a typed error when asked anyway | Decided |
| [D35](#d35-duckdb-is-out-of-the-initial-testing-set-superseded-by-d48) | DuckDB is out of the initial testing set | Superseded by D48 |
| [D36](#d36-the-client-drives-dbmeta-decides-nothing-about-the-connection-decided) | The client drives. dbmeta decides nothing about the connection | Decided |
| [D37](#d37-a-version-is-a-list-of-numbers-with-a-name-and-there-can-be-several-decided) | A version is a list of numbers with a name, and there can be several | Decided |
| [D38](#d38-dbmeta-holds-the-version-query-and-will-run-it-on-request-amended-in-place) | dbmeta holds the version query, and will run it on request | Amended in place |
| [D39](#d39-queries-are-listed-described-and-rendered-for-the-client-to-run-decided) | Queries are listed, described, and rendered for the client to run | Decided |
| [D40](#d40-three-support-tiers-and-a-trigger-that-can-remove-a-version-decided) | Three support tiers, and a trigger that can remove a version | Decided |
| [D41](#d41-every-model-ships-its-fixtures-beside-its-queries-decided) | Every model ships its fixtures beside its queries | Decided |
| [D42](#d42-four-releases-per-push-every-release-nightly-supersedes-d24-amended-by-d69) | Four releases per push, every release nightly | Supersedes D24, amended by D69 |
| [D43](#d43-ask-several-models-before-a-dialect-is-declared-finished-decided) | Ask several models before a dialect is declared finished | Decided |
| [D44](#d44-a-version-key-names-the-product-a-number-alone-never-does-decided) | A version key names the product. A number alone never does | Decided |
| [D45](#d45-a-query-may-answer-partially-once-and-must-say-so-decided) | A query may answer partially, once, and must say so | Decided |
| [D46](#d46-five-object-kinds-are-missing-and-two-consumers-say-which-decided) | Five object kinds are missing, and two consumers say which | Decided |
| [D47](#d47-dbmeta-supplies-the-data-the-consumer-decides-what-to-show-decided) | dbmeta supplies the data. The consumer decides what to show | Decided |
| [D48](#d48-cgo-is-allowed-in-the-test-module-and-nowhere-else-amends-d26-d29-and-d35) | cgo is allowed in the test module, and nowhere else | Amends D26, D29 and D35 |
| [D49](#d49-one-method-on-the-interface-and-a-not-null-is-not-a-constraint-row-decided) | One method on the interface, and a NOT NULL is not a constraint row | Decided |
| [D50](#d50-documentation-lives-in-docs-and-the-decision-log-stays-one-file-decided) | Documentation lives in docs, and the decision log stays one file | Decided |
| [D51](#d51-there-is-no-alias-for-a-nullable-type-decided) | There is no alias for a nullable type | Decided |
| [D52](#d52-a-test-driver-is-the-one-usql-uses-or-it-is-the-wrong-driver-amended-by-d59) | A test driver is the one usql uses, or it is the wrong driver | Amended by D59 |
| [D53](#d53-one-canonical-expectation-checked-in-that-every-database-must-meet-decided) | One canonical expectation, checked in, that every database must meet | Decided |
| [D54](#d54-sql-server-covers-every-release-that-ships-a-linux-container-amended-by-d63) | SQL Server covers every release that ships a Linux container | Amended by D63 |
| [D55](#d55-the-current-user-moves-here-changing-a-password-does-not-decided) | The current user moves here. Changing a password does not | Decided |
| [D56](#d56-the-password-statement-is-built-here-and-run-by-the-caller-amends-d5) | The password statement is built here and run by the caller | Amends D5 |
| [D57](#d57-a-windows-machine-is-how-a-pre-2017-sql-server-gets-tested-and-it-is-verified-decided) | A Windows machine is how a pre 2017 SQL Server gets tested, and it is Verified | Decided |
| [D58](#d58-one-gitignore-in-the-repository-root-decided) | One .gitignore, in the repository root | Decided |
| [D59](#d59-oracle-is-tested-with-go-ora-v2-until-v3-tags-its-fix-amends-d52) | Oracle is tested with go-ora v2 until v3 tags its fix | Amends D52 |
| [D60](#d60-the-oracle-model-reads-all_-views-and-there-is-no-dba_-variant-decided) | The Oracle model reads ALL_ views, and there is no DBA_ variant | Decided |
| [D61](#d61-every-dialect-is-measured-against-every-principal-the-product-has-amended-in-place) | Every dialect is measured against every principal the product has | Amended in place |
| [D62](#d62-cql-cannot-compute-so-the-cassandra-model-computes-in-scan-decided) | CQL cannot compute, so the Cassandra model computes in Scan | Decided |
| [D63](#d63-support-says-when-a-release-is-too-old-amends-d54) | Support says when a release is too old | Amends D54 |
| [D64](#d64-the-verified-tier-is-checked-against-the-document-decided) | The Verified tier is checked against the document | Decided |
| [D65](#d65-a-windows-machine-rearms-its-evaluation-before-it-expires-decided) | A Windows machine rearms its evaluation before it expires | Decided |
| [D66](#d66-the-order-the-remaining-dialects-are-written-in-amended-by-d67-and-d77) | The order the remaining dialects are written in | Amended by D67 and D77 |
| [D67](#d67-impala-cannot-be-a-dbmeta-model-and-clickhouse-goes-first-amends-d66) | Impala cannot be a dbmeta model, and ClickHouse goes first | Amends D66 |
| [D68](#d68-every-container-is-started-by-the-runner-and-named-product-release-amended-by-d70) | Every container is started by the runner and named product-release | Amended by D70 |
| [D69](#d69-the-workflow-builds-its-matrix-from-the-go-list-amends-d42) | The workflow builds its matrix from the Go list | Amends D42 |
| [D70](#d70-the-runner-is-a-go-command-called-dbrun-amends-d68-and-d12) | The runner is a Go command called dbrun | Amends D68 and D12 |
| [D71](#d71-nothing-here-is-generated-the-models-are-written-amends-d2-d12-and-d30-supersedes-d11) | Nothing here is generated. The models are written | Amends D2, D12 and D30, supersedes D11 |
| [D72](#d72-trino-reads-system-jdbc-and-a-catalog-is-a-real-level-decided) | Trino reads system.jdbc, and a catalog is a real level | Decided |
| [D73](#d73-presto-is-its-own-dialect-and-not-a-flavor-of-trino-decided) | Presto is its own dialect, and not a flavor of Trino | Decided |
| [D74](#d74-firebird-has-no-schemas-and-none-is-invented-decided) | Firebird has no schemas, and none is invented | Decided |
| [D75](#d75-every-container-is-bounded-and-four-run-at-once-decided) | Every container is bounded, and four run at once | Decided |
| [D76](#d76-sap-hana-reads-sys-and-answers-more-than-anything-but-postgresql-decided) | SAP HANA reads SYS, and answers more than anything but PostgreSQL | Decided |
| [D77](#d77-exasol-will-not-run-here-and-hive-goes-ahead-of-it-amends-d66) | Exasol will not run here, and Hive goes ahead of it | Amends D66 |
| [D78](#d78-hive-reads-sys-and-is-a-model-decided) | Hive reads sys, and is a model | Decided |
| [D79](#d79-a-dialect-that-cannot-bind-renders-its-values-decided) | A dialect that cannot bind renders its values | Decided |
| [D80](#d80-the-driver-registry-is-dburls-and-reading-it-is-not-importing-it-decided) | The driver registry is dburl's, and reading it is not importing it | Decided |
| [D81](#d81-the-cassandra-dialect-is-cql-decided) | The Cassandra dialect is cql | Decided |
| [D82](#d82-ci-compiles-once-and-every-job-runs-the-binary-decided) | CI compiles once and every job runs the binary | Decided |
| [D83](#d83-a-server-is-ready-when-it-can-run-a-query-not-when-it-answers-one-decided) | A server is ready when it can run a query, not when it answers one | Decided |

## Decisions

Each decision below carries a status. "Decided" means Ken chose it. "Proposed"
means an agent or a peer session suggested it and Ken has not confirmed it.
"Open" means nobody has chosen yet.

### D1. The module centralizes database metadata. Decided.

`dbmeta` is the single home for database metadata queries. `usql` and `dbtpl`
consume it instead of each keeping their own copy.

### D2. Models are one package per driver under `models/<driver>`. Amended by D71.

Each driver gets one package, named after the driver, under `models/`. One
package covers every supported version of that database, because D8 holds the
version differences as data inside it.

D71 amends the part that said a generator produces the code. Nothing does. The
package layout this decision chose is what stands.

### D3. The root package is the driver agnostic API. Decided.

External projects use the root package. They do not import
`dbmeta/models/<driver>` to do ordinary work.

### D4. Keep the object coverage, drop the Reader naming. Decided.

The 15 one method interfaces in `usql/drivers/metadata/metadata.go` are useful
for what they cover, which is one interface per kind of object. They are not
useful as a naming scheme. D5 drops the `Reader` and `Writer` concept, so
`TableReader` and `ColumnReader` do not carry over under those names.

Take the object coverage. Leave the names and the composition.

The name `Reader` carried information in `usql` because a `Writer` sat beside
it. In `dbmeta` everything reads, so `Reader` says nothing and repeats the
package. `dbmeta.TableReader` with a `Tables` method is three words for one
idea.

#### The further question is answered: there are no per object interfaces

This decision once asked whether `dbmeta` should export those fifteen as
interfaces at all, and said not to settle it before D13 delivered the models.
D13 delivered eight. The shape they produced is the one this decision hoped
for, and the code is now the record of it.

The root package exports two interfaces and neither is per object.
`Querier` declares `QueryContext` and nothing else, which is D49.
`AnyQuery` is what lets a caller hold queries of different row types in one
list. There is no `TableReader`, no `ColumnReader` and no composition by type
assertion, which is what D18 rejected.

An object kind is a `Query` value rather than an interface. A caller writes
`dbmeta.Tables.All(ctx, m, db, args)` and gets an iterator, and the same four
arguments read every other kind. The interface a consumer wants is the one it
declares for itself, which is the rule in `CLAUDE.md` and the reason nothing
here declares it for them.

#### Read on rather than designing from here

The design is settled and it is recorded further down, not here. This decision
is the earliest one and it predates every model. A reader who takes it as the
current shape of the API will be several years of decisions out of date.

D39 has how a query is listed, described and rendered. D33 has why a result
streams as an iterator rather than arriving as a slice. D34 has what happens
when a database cannot answer. D47 has the cost test for whether a field
belongs here at all, and why a child of an object is its own kind with flat
rows rather than a slice on the parent. D49 has `Querier` and why the
interface is documentation rather than a seam for a mock.

### D5. dbmeta only reads. Amended by D56.

`dbmeta` reads metadata. It does not render it and it does not write it.

D56 amends the second half of that by a hair and is worth reading with this.
`dbmeta` builds one statement that writes, the one that sets a password, and
does not run it. Everything `dbmeta` executes is still a read.

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

### D6. Fix the NULL scan defect once, and never hide a NULL. Decided, amended in place.

#### The amendment, and the bug that forced it

The first PostgreSQL model met this decision by writing `COALESCE(x, '')`
around every nullable column, so that every Go field could stay a plain string.
That was wrong and it shipped a real fault. It is corrected here rather than
quietly rewritten.

PostgreSQL reports three states for an access control list. A NULL means the
default privileges apply, so the owner has everything. An empty list means
every privilege was revoked, so nobody has anything. A list means explicit
grants. `psql` handles all three, and `printACLColumn` in `describe.c` prints
the empty case as `(none)`.

Coalescing the first two into an empty string makes "the owner has full access"
read exactly like "nobody has any access". This was demonstrated on a live
PostgreSQL 18 server:

```
    relname    | acl_is_null | acl_len | psql_shows | dbmeta_showed
---------------+-------------+---------+------------+---------------
 default_privs | t           |      -1 | <null>     |
 revoked_privs | f           |       0 | (none)     |
```

#### The rule

Never wrap a nullable catalog column in `COALESCE`. Let the NULL through and
give the field the type `sql.Null[string]`. There is no alias. See D51.

`COALESCE` is still correct over an aggregate that matched no rows, because
there NULL and empty mean the same thing. "No members" and "an empty member
list" are one answer. Sixteen such wrappers remain and they are right.

Reading `.V` prints empty for an absent value, so a command line client behaves
as it did. Reading `.Valid` recovers the difference for a caller that needs it,
and a code generator does: a column with no default is not a column whose
default is the empty string.

`TestNoCoalesceOnCatalogColumns` in the model package guards the rule.

### D6a. The original decision: fix the defect at generation time. Superseded by D71.

It said the NULL scan fix belonged in a generator's flags rather than in hand
written patches. There is no generator, which D71 records, so there is nowhere
for it to go but the code. The rule that replaced it is in D6 above and in
`docs/NULLS.md`: a field that can be absent is declared `sql.Null[T]`, and a
`TestNoCoalesceOnCatalogColumns` in the model package guards it.

See "Known defects" below for the evidence that produced it.

### D7. Use the standard library. Third party packages are a last resort. Decided.

`dbmeta` uses the Go standard library for almost everything. An agent must not
add a third party package without asking Ken first.

There are exactly two dependencies. `database/sql` is in the standard library.
`github.com/xo/dburl` was the one approved outside package under D19, and it
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

Check the models too. No metadata query needs a UUID column or anything else
that would pull in a package, so make sure that every model imports nothing
outside the standard library and the root package.

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
column has no source on a given server, the fragment still selects it, as a
literal NULL under the same name:

```sql
NULL AS "access_privileges"
```

The rule is symmetric, and an earlier version of this section got that wrong.
Both external reviews caught it. Padding is not only for a column that arrives
in a newer release. A column can exist on an older server and be removed from a
newer one, and then the newer fragment is the one that pads.

`pg_attrdef.adsrc` is the case. Commit `fe5038236c` in the PostgreSQL tree
removed it in 2018, so it is present up to release 11 and gone from 12. If the
canonical shape includes a field with no replacement, releases 12 and later pad
it. Most removals do have a replacement, as `adsrc` does in
`pg_get_expr(adbin, adrelid)`, and then both fragments select a real value
under the same name. The padding case is the one where nothing replaces it.

State it as: whichever side lacks a source pads, old or new.

#### Padding does not cover a type change

A NULL pad fixes the presence of a column. It does not fix its type. Both
reviews raised this and it is a real limit of the mechanism.

If a column exists on both versions but its type differs, one Go field cannot
scan both. The fix is a cast in the fragment, so that every version returns the
same type under the same name, chosen to lose nothing:

```sql
CAST(col AS text) AS "col"
```

Do not reach for `any` or an empty interface to paper over this. That discards
the typed scanning the whole design rests on.

A NULL pad also needs a type where the database cannot infer one. PostgreSQL
accepts a bare `NULL AS name`, but a stricter database can require
`CAST(NULL AS text) AS "name"`. Write the cast when the server asks for it.

Gemini offered `pg_class.reltuples` as an example of a type change, saying it
went from `real` to `bigint` in release 14. That is wrong and it was checked:
it is still `float4` in the PostgreSQL tree. Release 14 changed its default and
its meaning, not its type. The category of risk is real even though that
example is not.

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

#### What a model holds

A model is written against one concrete statement run on one live server. With
the padding rule, the newest supported version is that statement. It gives the
row struct and the scan code once per query, and both fit every version,
because every version returns the same columns in the same order.

The fragments are data beside the query, not something a server is asked
about.

#### The subtlety to resolve

Padding makes two different facts look the same. A NULL now means either that
the value is genuinely null on this server, or that the column does not exist
at this version. A caller that cannot tell them apart will report a missing
feature as missing data.

Do not solve this with a sentinel value. Record, next to each query, which
fields are valid at the detected version, and let a caller ask. This is the
same mechanism D34 settles for a database that cannot answer at all, and the
two must be one mechanism rather than two.

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
the capability mechanism D34 describes reports the rest. PostgreSQL sets the
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

`dbmeta` takes the other path. D8 sets discrete fragments per version, because
each is a concrete SQL statement that has been run against a concrete server.
An inline conditional has no single statement to check.

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

Do not design the model shape from nothing. Read `dbtpl/models` first, as a
worked example of the shape: the struct per object, the query function per
lookup, and the shared package file. It is another project's code and it is
read for its shape rather than run.

D71 records that nothing here is produced by a tool. The shape above is worth
reading. The way that project builds its own files is not, because this one
does not build files.

Skim, then adapt. `dbtpl/models` is one flat package with the driver in the
function name, as in `PostgresTables` and `MysqlTables`. D2 sets a package per
driver instead, so the names lose the prefix and become `postgres.Tables`.

### D11. dbtpl is pinned as a tool, in the generation module. Amended by D26, superseded by D71.

Nothing in this decision is in force. No `go.mod` here has a `tool` directive,
no generator is run, and D71 records why. It is kept because the reasoning
about where a build dependency belongs is still the reasoning that keeps
drivers out of the root module, which is D26.

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
only.

### D12. Work against live databases running in containers. Amended by D70 and D71.

What this decision got right is that the work needs a running server, and that
is still true for a different reason than it gave. D71 has it: nothing is
generated, so there is no generation step needing a connection. A query is
written against a live server and checked there, and a fixture is built there,
so every model still needs one.

D70 replaced the harness this decision chose. Read it first: the databases are
started by `dbrun` and by nothing else, and the rest of this section is the
record of what was decided in 2025 and why. What does not stand is
`usql/contrib`, and neither does the paragraph below about generation time.

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

### D19. Do not repeat dburl. Half decided, half overtaken by the code.

The half that stands: `dbmeta` must not carry its own list of schemes, its own
aliases, its own flavor table, or its own connection string parser. That
taxonomy belongs to `github.com/xo/dburl`, and a second copy of it is how two
copies start disagreeing. Everything below about what `dburl` records is still
how a consumer reads a URL.

The half that was wrong: this decision made `dburl` a direct dependency, and it
never became one. `dbmeta` imports the standard library and nothing else, and
its `go.mod` has no `require` block at all.

#### Why the dependency never happened

The API took the shape that made it unnecessary. A caller opens its own
connection, passes a `DB`, and names a `Dialect`. No URL ever reaches this
module, so there is nothing here to parse and nothing to look up. D36 decided
that the client drives, and this is a consequence of D36 that nobody noticed
until the code was written.

A zero dependency library is the better answer, and it is strictly stronger
than the rule it replaces: a `go.mod` with no `require` block cannot acquire a
transitive dependency, and `depguard` now refuses one at lint time. A consumer
that has a URL imports `dburl` itself, which it was going to do anyway.

Do not add `dburl` back to reach the taxonomy below. Read it from a consumer.

#### What dburl is still for

Every package that uses `dbmeta` is expected to use `dburl` as well, and that
is where a connection string is handled.

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

#### Upstream moved, and the floor was reviewed

The checkout was updated on 2026-09-24 from a release 15 tree to `master`,
which is release 20 under development. One upstream change bears on this
decision and it was reviewed rather than absorbed quietly.

Commit `831bec45924`, on 2026-07-02, removed every `psql` code path for a
server older than release 10. Its message gives the reason:

> Our current policy is to support at least 10 previous major versions, so this
> bumps the minimum to v10 for the v20 release.

#### What upstream actually gave as its reason

The commit messages were read, because the reason matters more than the fact.
There is no metadata specific rationale anywhere. The removal was one commit
covering all of `psql`, and metadata was simply the largest part of it: of 288
deleted lines, 244 were in `describe.c`.

The 2026 reason is policy. `831bec45924` says in full:

> Per discussion, it seems like a good time to bump the minimum supported
> version for various applications. Our current policy is to support at least
> 10 previous major versions, so this bumps the minimum to v10 for the v20
> release.

It does not claim that the old servers cannot be tested, or that the old code
was wrong, or that maintaining it was costly. It cites a policy about how far
back to support.

The 2021 reason was technical, and it is the one that would transfer. Commit
`cf0cab868a`, which set the previous cutoff at 9.2, says:

> Per discussion, we'll limit support for old servers to those branches that
> can still be built easily on modern platforms, which as of now is 9.2 and up.

That is a real constraint: a server you cannot build is a server you cannot
test against. It would apply to `dbmeta` too, and it is exactly the condition
D40's trigger watches for. It is not the reason given in 2026, and 9.6 was
verified to run on this host on 2026-09-24.

PostgreSQL also sets a precedent for keeping a capability it cannot easily
test. The matching pg_dump commit from 2021, `30e7c175b8`, notes in passing:

> (As in previous changes of this sort, we aren't removing pg_restore's ability
> to read older archive files ... though it's fair to wonder how that might be
> tested nowadays.)

So upstream removes the code that reads from an old server, and keeps the code
that reads an old format, on the grounds that the second costs little once
written. That is close to the argument for keeping 9.6 here.

The conclusion is narrow and worth stating exactly. Upstream's 2026 reason is a
scope policy for a C project with a ten version commitment. It is not evidence
that these queries cannot be maintained or cannot be tested, and `dbmeta`
adopting it would be copying a conclusion without its premise.

#### The gap is one release, not four

State the size of the problem before arguing about it, because it is smaller
than it first appears.

`psql` in release 20 supports servers from release 10, which
`command.c:4489` confirms with `pset.sversion < 100000`. Releases 10 and 11 are
inside that window and the current tree still describes both. Only 9.6 falls
outside it.

So the question is whether `dbmeta` supports one release that upstream `psql`
no longer does, and the answer costs one extra source tree, not four.

#### Verified: 9.6 runs today

This was flagged as unverified twice in earlier drafts. It is now checked.

`docker.io/library/postgres:9.6` was pulled and started with podman 6.1.2 on a
current Linux host on 2026-09-24. It became ready in four seconds, reported
`server_version_num` 90624, and both `\d` and `\dt` returned correct output
against a real table. The image is frozen, not broken.

#### The review disagreed, and the useful part is where it agreed

Two models were asked. Gemini said drop the old releases and follow `psql`.
DeepSeek said keep them as a named legacy tier with scheduled tests, and drop
them otherwise. They agreed on the parts that matter.

Both rejected the argument that Go data is cheaper to maintain than C. Their
correction is the same and it is right: cheaper to edit, not cheaper to verify.
The SQL text of a frozen query does not rot. The environment that proves it
does. The cost is the container image, the runner, the driver, the auth method
and the person who triages an issue, and none of those becomes cheaper because
the difference is data.

Both insisted that a version must not be called supported unless something
tests it, and both asked for a falsifiable trigger to remove one. D40 sets
both.

#### One argument against was checked and does not apply

Gemini's strongest objection was a refactoring tax. It argued that the padding
rule makes every new column expensive, because adding a column for a new
release means going back and patching every old version with
`NULL AS "new_column"`.

That is true of the design D8 abandoned, which gave each release its own query.
It is not true of the design D8 chose. A new column is one `Choice` with two
alternatives:

```go
Choice{
	{Query: `NULL AS "x"`},
	{Min: v20, Query: `real_expression AS "x"`},
}
```

The zero minimum alternative covers every release below 20, whether the floor
is 9.6 or 14. Adding a column costs the same number of alternatives regardless
of how many versions are supported. The tax Gemini describes is real for a
package per release and absent here.

This is worth recording because it was the main argument for dropping 9.6, and
it was aimed at the wrong design.

#### The floor stays at 9.6

Ken reviewed this and kept 9.6. The reasoning holds up against the review.

The extraction is genuinely one time for a frozen release. 9.6 has been out of
upstream support since 2021 and its catalog will never change again, so the
query set has a final form rather than a moving one.

The image demonstrably runs, so this is not a claim of support that nothing can
check.

The gap is one release, and the cost of it is one additional source checkout at
release 15 or older.

What the review adds, and what D40 now requires, is that keeping a version is
not the same as claiming it is tested, and that something must eventually be
able to remove it.


#### What a floor costs, measured

Counted from `describe.c` on the current tree, which holds 68 version gates.
These counts cover release 11 to 19 only, because the current tree has nothing
older. Measuring a floor below 11 needs an older checkout.

| Floor | Live gates | Gates that collapse |
| --- | --- | --- |
| 10 | 68 | 0 |
| 11 | 55 | 13 |
| 12 | 44 | 24 |
| 13 | 40 | 28 |
| 14 | 34 | 34 |
| 15 | 20 | 48 |
| 16 | 15 | 53 |
| 17 | 12 | 56 |
| 18 | 9 | 59 |

The gap between a floor of 10 and a floor of 14 is 68 gates against 34, so
exactly double. On the release 15 tree the same comparison ran 63 against 13,
close to five times. The ratio narrowed because the oldest gates went away with
the code that used them, not because the work got easier.


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

`describe.c` shows the same thing directly. It carries 3 gates of the form
`pset.sversion < N`, at 11, 12 and 15. A gate that asks whether the server is
below a version exists because something present in the older release is absent
in the newer one. Catalog change is not monotonic.

The release 15 tree carried 11 such gates. The drop is not evidence that the
catalog settled down. Release 20 removed every code path for a server below 10,
so the gates went with the code rather than with the problem.

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

### D24. CI tests the latest version only. The matrix runs locally. Supersedes D22, superseded by D42.

CI tests the major databases at their latest version and nothing else. Every
other version, and every flavor, is tested on a development machine.

This overrides D22, which proposed one CI job per supported major. Read D22 for
why a sampled matrix misses catalog changes. That reasoning still holds. The
matrix is not cancelled, it moves off CI, because running every version of
every database on every push costs more than the project will pay.

The CI databases are PostgreSQL, MySQL and SQLite3. D35 removed DuckDB, because
D29 makes the project pure Go and DuckDB has no pure Go driver. More can be
added later.

CI runs two jobs, not one. The first opens no connection and covers every
release through a fake driver replaying recorded data. The second starts a real
PostgreSQL 18 as a service container and runs the integration tests against it.
Only the newest release runs there, which is what this decision requires. The
other nine run with `dbrun` before a release.

One detail the container needs. Its health check must force TCP, with
`pg_isready -U postgres -h 127.0.0.1`. Checking the socket reports ready during
the bootstrap phase, before the server restarts to accept connections, and a
job that starts then fails with a connection reset.

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

Two clarifications, because both external reviews misread this decision in
different ways.

This is a testing policy, not a restriction on where the code runs. `dbmeta` is
pure Go under D29, so it builds and runs anywhere Go does. Nothing enforces
amd64 and nothing should. Gemini read the decision as needing a `//go:build
amd64` constraint to enforce itself, which would contradict the rule. It does
not, because nothing is being enforced. Only testing is limited.

The ban is on operating system and architecture constraints. It is not a ban on
build tags of every kind. D31 gates model registration with the feature tags
`none`, `base`, `most` and `all`, which say nothing about a platform. DeepSeek
read the two as contradictory. They are not, and the difference is the subject
of the tag.

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

### D26. No database driver in the dbmeta module. Amends D11, amended by D48.

The `dbmeta` module depends on the standard library alone. It does not
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

1. Every database driver used for testing.
2. The container harness from D23.
3. The integration tests that connect to a real server.

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
the integration tests and the database drivers they need. It once also held the
`tool` directive that D11 moved out of the root, and D71 removed the last of
that.

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

### D29. Pure Go only. No single package imports every driver. Amended by D48.

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
generation sub-package. The root module uses no cgo and never will. The test
module may, and does for SQLite. D48 amends this: the separate `go.mod` is what
makes a C toolchain safe there and absent everywhere else.

Use the pure Go driver in the table above for every database that has one.
Never import `mattn/go-sqlite3` or `godror`.

Do not put every driver import in one package even though all of them are pure
Go. A conflict over a shared transitive dependency does not care about cgo.

#### DuckDB has no pure Go driver, and D24 puts it in CI. D48 answered this

Read D48 before anything below this heading. The conflict was real and it is
closed: cgo is allowed in the `test` module and nowhere else, so DuckDB is
reached through `duckdb/duckdb-go` like any other driver, and `models/duckdb`
answers 20 of the 55. None of the three ways out below was taken and none is a
live option.

The rest of this subsection is the reasoning at the time, kept because the rule
that survived it is narrower than it looks. The root module has no driver at
all, so pure Go is not a constraint it has to be careful about. It is a
property of holding nothing but the standard library. The `test` module is
where the drivers live, and its own `go.mod` is what keeps them out of
everything a consumer builds.

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

That cost is why option 1 was not taken. D48 settled it the other way.

### D30. dbtpl is not used to generate dbmeta. Amended by D71.

`dbtpl` is not used and `dbmeta` does not pin it. That half is right and it
holds.

The other half said generation would be driven by ordinary Go code in a
sub-package. There is no such sub-package and there is no generation at all.
D71 records what actually happened, which is that the models were written.

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

#### The risk this carries

Both external reviews objected to build tags in a library, and the objection is
recorded because Ken chose this deliberately to match `usql`.

Their point: a build tag is global to a build, not per dependency. A consumer
cannot ask for `dbmeta` with the `most` tier while another dependency asks for
`base`. The tier is chosen by whoever builds the final binary. That works for
`usql`, which already sets these tags and is a binary, and it is surprising for
a library consumed by something else.

DeepSeek added the sharper version, which is an interaction with D34. A model
excluded by a build tag is not present at all, so it cannot report a
capability and it cannot return `ErrNotSupported`. That is a third state beyond
the two D34 names, and the API must distinguish all three:

1. The database does not have that object.
2. The model exists and the database cannot answer.
3. The model was not compiled into this binary.

The third needs its own error. Call it what it is, and do not let it surface as
either of the others. A user whose build is missing a model must be told that,
not told their database lacks a feature.

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

#### The iterator holds a connection, and that is a hazard

Both external reviews raised the same failure and it is the sharpest finding
against this decision. A live iterator holds an open `*sql.Rows`, and an open
`*sql.Rows` holds a connection from the pool.

So this deadlocks:

```go
for table, err := range meta.Tables(ctx, f) {
    for col, err := range meta.Columns(ctx, filterFor(table)) { // second connection
        ...
    }
}
```

The outer loop holds one connection while the inner one asks for another. With
a pool of N, N nested iterations exhaust it. With `MaxOpenConns(1)`, which is
ordinary for SQLite3 and common in tests, the first nested call deadlocks
immediately.

Three things address it and all three are required.

1. Document it. The package documentation must say that an iterator holds a
   connection until it is exhausted or stopped, and must show the filter form
   that avoids nesting. Asking for every column in a schema in one call is
   almost always what the caller wanted.
2. Define the lifecycle. Stopping early, by `break` or by a `yield` returning
   false, must close the rows and release the connection. An iterator that
   leaks a connection on `break` is worse than a slice. Cancelling the context
   must do the same.
3. Make the escape hatch available. A caller that genuinely needs the whole
   result in memory collects the iterator into a slice with `slices.Collect`,
   which releases the connection at once. That is the caller's choice and it
   needs no API of its own.

This is the cost of D33 and it is accepted. Note that materializing by default
would trade this hazard for unbounded memory on a large catalog, which is the
worse failure.

#### Reading more than one catalog needs one snapshot

Gemini raised this and the plan did not cover it. Reading tables and then
reading their columns are two queries, and a table can be dropped between them.

`dbmeta` does not solve this and must not pretend to. The `DB` interface from
D17 is satisfied by `*sql.Tx` as well as `*sql.DB`, so a caller that needs a
consistent view opens a transaction and passes that in. Say so in the package
documentation, because a caller who does not know cannot guess.

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

### D35. DuckDB is out of the initial testing set. Superseded by D48.

Nothing in this decision is still in force. DuckDB is tested, it is reached
through `duckdb/duckdb-go` in the `test` module, and `models/duckdb` answers 20
of the 55. It runs in CI in the job that starts no container, beside SQLite,
because neither has a server. The design question this decision deferred was
answered by allowing cgo in the `test` module, not by finding a way around a
driver.

The rest is the record of what was decided before that.

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


### D36. The client drives. dbmeta decides nothing about the connection. Decided.

The client opens the connection, chooses the dialect, and chooses the version
used to resolve queries. `dbmeta` never detects any of the three and never
guesses.

```go
func New(dialect Dialect, ver VersionSet) (*Meta, error)
```

`dbmeta` reads a version from a server only in the sense that it hands the
client a query to run. It does not run it. See D38.

#### Why an override is not a luxury

Both reviews gave the same reasons, and each is a real deployment.

A proxy hides the server. PgBouncer and ProxySQL report themselves rather than
the database behind them.

A compatible product lies on purpose. CockroachDB answers a PostgreSQL version
query, and the answer describes neither its real catalog nor a PostgreSQL
release that behaves like it. This is the D14 flavor axis arriving at run time.

A code generator has no server. `dbtpl` generates against a target release the
developer names, with nothing to connect to.

A person is debugging. Forcing an older query set is how you find out whether a
fault is a version gate.

#### Out of range

Follow D21, which already settled the behavior and now gets an API.

Above the newest version `dbmeta` knows, use the newest query set and proceed.
Do not fail. People upgrade a server faster than a library.

Below the oldest, return `ErrVersionTooOld`. The client can pass an explicit
override to try anyway, which is the opt-in escape D21 requires.

An unknown version is treated as newest, not as oldest. A serverless database
is continuously released, so newest is the truthful reading, and it agrees with
the rule above the ceiling.

### D37. A version is a list of numbers with a name, and there can be several. Decided.

The current `Version` type in this repository has `Major`, `Minor` and `Patch`.
That is wrong and it must be replaced. Oracle reports five components and SQL
Server reports four.

The real shapes, taken from the version queries `usql` runs today:

| Database | Reported | Note |
| --- | --- | --- |
| Trino | `443` | one component |
| Presto | `0.287` | two |
| PostgreSQL | `16.2` | two, plus an integer form |
| SQLite3 | `3.45.1` | three |
| MariaDB | `11.4.2-MariaDB` | three, with a suffix |
| DuckDB | `v1.1.3` | three, with a leading letter |
| ClickHouse | `24.3.1.2672` | four |
| SQL Server | `16.0.4295.3` | four, in one of five columns |
| Oracle | `19.3.0.0.0` | five |
| Cassandra | `4.1.3`, `3.4.6`, `5` | three independent versions |
| YDB | `<unknown>` | none |
| Snowflake, BigQuery, Athena | none | serverless |

Two types, because a reported version and a minimum version are not the same
thing. A minimum is numbers only. A reported version also carries the text it
came from, a suffix, and whether it is known at all.

```go
// Version is one version.
type Version struct {
	Raw     string
	Parts   []uint32
	Suffix  string
	Unknown bool
}

// VersionSet is every version one server reports, keyed by name, with the
// empty name for the main one.
type VersionSet struct {
	Versions map[string]Version
	Display  string
}
```

Compare by padding the shorter list with zeros, so `16.2` equals `16.2.0` and
equals `16.2.0.0.0`. Compare left to right. Never compare version strings.

Ignore the suffix when comparing. `11.4.2-MariaDB` and `11.4.2` compare equal,
and D14 already says the flavor is a separate axis. The suffix is recorded, not
ranked.

Cassandra needs the set rather than one version, because its release version,
its CQL version and its protocol version move independently. A fragment names
which one it gates on. When a reported set lacks the name a minimum asks for,
the gate fails rather than passing by accident.

### D38. dbmeta holds the version query, and will run it on request. Amended in place.

`dbmeta` holds the version query for every database, because that knowledge
belongs with the metadata queries.

#### The original reasoning was wrong

The first version of this decision said `dbmeta` "cannot run one, because D26
gives it no driver and D36 gives it no connection", and exposed only the two
step form. Writing the examples showed that premise is false, and it is
corrected here rather than quietly dropped.

Running a query needs no driver, only the `DB` interface, and the caller hands
one over for every `Query.All` call. `dbmeta` already runs queries against a
caller supplied connection. There was never a reason the version query was
different.

What D36 actually requires is that `dbmeta` decides nothing. It must not detect
a version behind the caller's back and must let the caller override. A method
the caller chooses to call satisfies that. A constructor that silently probes
the server would not.

#### Compare it against usql, every time

Adding or changing a dialect means comparing its version query against the one
`usql` runs for the same product, and recording the comparison in
`docs/USQL.md`. This sits alongside the other requirements for a dialect: the
fixture in rule 9, the second opinion in D43, the driver in D52 and the
principals in D61.

Find `usql`'s side in `usql/drivers/<driver>/<driver>.go`, in the `Version`
field of the registered `drivers.Driver`. A driver that declares none falls
through to `drivers.Version`, which runs the generic `SELECT version();`, and
that fallback is part of the comparison rather than an absence of one.

Record one row per model in the statements table in `docs/USQL.md`, saying
what each side runs and whether the answers agree. Three outcomes are normal
and each is written differently. The same statement is the common case. A
different statement with the same answer is fine and is left alone, with the
reason the two differ. A different answer is a decision: say which side reads
more and why this one does what it does.

The reason this is a rule is that it was not done, and the gap was invisible
from the other end. `docs/USQL.md` already compared the printed version lines
across 26 servers and called the coverage complete. It compared output rather
than statements, and it predated three models, so nobody noticed that `usql`
declares no `Version` function for Oracle at all. It falls through to
`SELECT version();`, Oracle answers `ORA-00904: "VERSION": invalid
identifier`, `drivers.Version` discards the error, and `usql` prints
`<unknown>`. Comparing the two statements finds that in a minute. Comparing
the two printed lines finds it only if somebody notices Oracle is missing from
the table.

`TestEveryModelIsInTheVersionTable` holds it: a model with no row in that
table fails.

#### Both forms exist

The one step form is what nearly every caller wants:

```go
// Version runs the version statement against db and parses the result.
func (d Dialect) Version(ctx context.Context, db DB) (VersionSet, error)
```

A method named `Version` coexists with the `Version` type. A method name lives
in the method set of its receiver, not in the package scope, so there is no
collision. `Dialect.Version`, `Dialect.VersionQuery` and `Dialect.ParseVersion`
then read as one group.

The two step form stays, for a caller that wants the statement without running
it. `usql` prints statements in its trace output and needs this:

```go
// VersionQuery returns the statement and how many columns it returns.
func (d Dialect) VersionQuery() (sql string, cols int, ok bool)

// ParseVersion parses the columns of the first row.
func (d Dialect) ParseVersion(cols []string) (VersionSet, error)
```

The one step form exists because the two step form made every caller build a
slice of pointers into a slice of strings to scan into. That is the same nine
lines in every consumer, which is a sign the package drew the line in the wrong
place.

Neither form takes override away. A caller that wants to force a version builds
a `VersionSet` and passes it to `New`, and never calls either.

```go
// VersionQuery returns the query that reads the server version. The second
// result is false when the database reports no version.
func (d Dialect) VersionQuery() (VersionQuery, bool)

// ParseVersion parses the columns of the first row.
func (d Dialect) ParseVersion(cols []any) (VersionSet, error)

type VersionQuery struct {
	SQL     string
	Columns []Column
}
```

The column count is part of the two step form and the client must be told it,
because two databases return more than one column. SQL Server returns five: the
product name, the version, the level, the update level and the edition.
Cassandra returns three independent versions.

Parsing produces both things the client needs from one call. `VersionSet` gates
the queries. `Display` is the line `usql` prints, and it is built where the
shape is known rather than by the client guessing. For SQL Server that is

	Microsoft SQL Server 2022 16.0.4295.3, RTM-CU27, Developer Edition (64-bit)

Only the version gates anything. The rest is for the person reading it, and
each part is left out when the server did not report it, so a release with no
update level reads `RTM` rather than `RTM-`.

Four of the five are server properties and the product name is not. No
`SERVERPROPERTY` returns the name the product is sold under: `ProductMajorVersion`
says 16 and nothing says 2022. Only `@@VERSION` carries it, in its first words,
so the name is cut from there. The alternative is a table mapping a major
number to a year, which needs an edit for every release Microsoft ships, and
reading it from the server does not.

A column can arrive NULL, and `productupdatelevel` does on a release older than
the one that added it. `Dialect.Version` scans every version column as nullable
for that reason, and hands the parser empty text. That is the one place
flattening a NULL is right, because a version line is a single string shown to
a person, and a property that is absent and a property that is empty both mean
there is nothing to print. `docs/NULLS.md` governs a catalog column, where the
two are different facts.

A database with no version returns false, and the client uses an unknown
version, which D36 treats as newest.

### D39. Queries are listed, described, and rendered for the client to run. Decided.

The client can ask what queries exist, what each one takes, and what it
returns, then run one itself.

```go
func (m *Meta) Queries() []QueryDef
func (m *Meta) QueryDef(name string) (QueryDef, error)

// Render resolves the query for the version held by m and writes its
// placeholders in the dialect's syntax. It returns SQL ready to execute and
// the arguments in order.
func (m *Meta) Render(name string, args map[string]any) (string, []any, error)

type QueryDef struct {
	Name        string
	Description string
	Params      []Param
	Columns     []Column
}
```

#### Placeholders

Write every query once, with named parameters such as `@schema`. `Render`
translates them into what the dialect wants and returns the arguments in the
matching order: `$1` for PostgreSQL, `?` for MySQL, `:1` for Oracle, `@p1` for
SQL Server.

Named parameters are worth the small cost. A query with four parameters is
unreadable in positional form, and D39 exists so a person can read a query and
understand it.

#### Columns are declared, and the SQL is not

This is where DeepSeek was wrong and the error would have broken D8. It said
fragments may change only the `WHERE`, `ORDER` and `LIMIT` text and never the
select list.

That is the opposite of D8. Fragments exist to change the select list. Padding
a missing column with `NULL AS "name"` is a change to the select list, and it
is the central mechanism.

The rule is that the column SET is fixed and the SQL is not. `QueryDef.Columns`
declares the columns once, and every fragment of every version produces exactly
those columns under exactly those names. Gemini stated this correctly.

A client therefore knows the result shape before it runs anything, and the same
scan code works against every supported version. That declared list is also
what the generator emits scan code from, which is D30's reason for not needing
to introspect.


### D40. Three support tiers, and a trigger that can remove a version. Decided.

D20 keeps PostgreSQL back to 9.6, which is one release below what `psql`
supports. Both external reviews accepted that only on two conditions, and both
are adopted here.

#### The tiers

`README.md` declares which tier every database version is in. `gen.go` writes
that table from the models, under D31, so it cannot drift from the code.

**Tested.** Tests run on every change, in CI. This is what D24 puts in CI: the
latest release of PostgreSQL, MySQL and SQLite3. A fault here is a bug and it
gets fixed.

**Verified.** Tests exist and run on a development machine before a release,
not on every change. This is the D24 local matrix. A fault here is a bug and it
gets fixed, and it is found later than a tested one.

**Archived.** The queries exist and were checked once against a real server,
and nothing runs them now. The tier records the date and the image digest of
that check. A fault here is fixed only if someone supplies a test with the
report.

Never write "supported" without saying which tier. Both reviews made this point
and DeepSeek put it plainly: a frozen untested path is an unverified
compatibility claim.

PostgreSQL 9.6 starts in Verified. It is in the D24 local matrix like every
other release below the latest, and its image runs today.

#### The trigger

A version can leave. Without a rule that can fire, "we can always keep it"
means nothing can ever be removed.

D21 governs every version inside upstream support. It is unchanged.

For a version outside upstream support, which today is 9.6 alone, two triggers
apply and either is enough.

1. Its pinned image cannot be pulled or started on two consecutive attempts.
2. Its tests fail and are not fixed within one release cycle.

When either fires, the version moves from Verified to Archived. It is not
deleted at that point, because the queries still document how that release
answered and they cost nothing to keep once nothing runs them.

Deletion follows D21 only. A version leaves the tree when its queries obstruct
a change the supported versions need, and the release notes say so.

#### Record where each query came from

A query translated from a tree that upstream no longer ships must say so, next
to the query: the release of the checkout and its commit.

This is not bookkeeping. Below release 10 there is no current `psql` to compare
against, so the provenance line is the only way a later reader can check the
translation at all. `QUERIES.md` says the same thing about reading the right
tree for the right release.


### D41. Every model ships its fixtures beside its queries. Decided.

A model is not finished when its queries are written. It is finished when a
real server has answered them, and that needs a schema containing one of every
object the queries read.

The fixture lives in `models/<driver>/fixture`, beside the model, for two
reasons.

The queries and the fixture change together. Adding a query for event triggers
means adding an event trigger to the fixture, and those belong in one change
rather than two repositories.

The fixture is useful to other projects, which is the stronger reason and the
one that makes it exported API rather than a test artifact. `dbtpl` can
generate against a schema that is known to exercise every object kind instead
of writing one of its own.

A fixture is only SQL text. It needs a driver to execute and none to define, so
it stays inside the root module and D26 holds. The `test` module imports it and
runs it.

#### Versioned like a query, skipped unlike one

A fixture varies by release for the same reason a query does. `CREATE TRIGGER
... EXECUTE FUNCTION` is release 11 syntax, and writing the deprecated
`EXECUTE PROCEDURE` everywhere would test syntax nobody writes on a current
server. Reuse `Fragment`, `Choice` and `Stmt` rather than inventing a parallel
type, because the fixture is public and a caller should not learn two ways to
say the same thing.

One behavior differs. A query with no applicable alternative is refused, and
`ErrVersionTooOld` is the right answer, because asking for publications on 9.6
is asking for something that is not there. A fixture step with no applicable
alternative is skipped, because there is nothing to create and the query that
reads it is refused anyway. Express that by checking `errors.Is` against the
same sentinel rather than by adding a second resolver.

#### Additive

A fixture is additive. A later release may add an object to one and will not
rename or remove what is there, so code written against it keeps working. A
change that is not additive means a new fixture beside the old one.

The cost of exporting it is that the schema becomes part of the public
contract, not just the Go types. Renaming a table to suit a new test would
break a consumer generating against it.

#### The invariant this buys

With a fixture at every release, the expected output needs no golden file per
release. `Field.Min` already declares the release each column arrived in, so a
test asserts generically that a field the server is too old for is NULL in
every row. Ten releases times forty eight queries is four hundred and eighty
combinations that nobody would maintain, and one assertion covers them.


### D42. Four releases per push, every release nightly. Supersedes D24, amended by D69.

CI runs the integration tests against PostgreSQL 9.6, 12, 15 and 18 on every
push, and against all ten releases on a nightly schedule.

#### Why four, and why those four

Testing the newest release alone is not enough, and there is evidence rather
than intuition for that. Six real faults have been found by running the queries
against real servers. Here is the release that exposed each:

| Fault | Exposed on |
| --- | --- |
| a column gated at 12 that arrived in 15 | 12, 13, 14 |
| `EXECUTE FUNCTION` being release 11 syntax | 9.6, 10 |
| a field padded with an empty string, not NULL | every release below 15 |
| four fields wrongly marked as padded | 10 to 14 |
| a NULL access list collapsed into an empty string | every release |
| a NULL `check_clause` scanned into a string | every release |

Four of the six were invisible at the newest release. Testing 18 alone would
have caught two.

The floor, the ceiling and one release on each side of the middle catch all
six. That was checked rather than assumed.

#### Why not a smaller set, and why not all ten on a push

A covering set over the version gates is not the right idea, and it is worth
saying why, because it looks right. The gates sit at 10, 11, 12, 13, 15, 16 and
17, and `{9.6, 18}` covers every one of them, since 9.6 is below all of them
and 18 is above all of them. That pair would have caught two faults out of six.

The reason is that these tests do not check that the gates work. They check
that the gates are **correct**. A wrong gate is only visible between the
release it claims and the release that is true, and nothing predicts that
window. `colliculocale` was gated at 12 and arrived at 15, so it was fine at
9.6 and fine at 18 and broken at 12, 13 and 14.

That argues for all ten, and all ten do run, nightly. Four is what a push
carries, because a push has to stay fast enough that a contributor does not
avoid it, and four demonstrably catches what has been found so far.

#### The rule for the next database

Do not hand maintain a list per database. The model already declares every
`Field.Min` and every fragment minimum, so generate the matrix from the gates:
the floor, the ceiling, and a release on each side of the densest gates. For a
database whose gates cluster, that gives three rather than ten by itself.

Two adjustments the shape of a database forces:

An embedded database has no container and no server version. SQLite3 and DuckDB
version with the Go module, so they belong in the unit job with a pinned module
version rather than in a container matrix.

A flavor is a separate target, not a second dimension. MariaDB and MySQL share
a driver and are different products, so the matrix is a flat list of pairs such
as `mariadb:11.4` and `mysql:8.4`, and the rule above applies to each on its
own.

A slow database goes nightly from the start. Oracle and Cassandra take minutes
to become healthy, and a push should not wait for them.


### D43. Ask several models before a dialect is declared finished. Decided.

When a new dialect is implemented, consult at least two independent AI models,
such as Gemini, DeepSeek and Astra, about the queries that the first pass could
not answer. Ask each one to sort them into three groups: absent from the
product, present under another name, and derivable from several catalog reads
or one complex statement. Then verify every claim against a running server.

#### Why

A first pass finds the objects that the source product names the same way. It
misses the objects that the target product keeps under a different name, and it
misses the ones that no single catalog table holds. MariaDB is the case that
proved it. A first pass answered 19 of the 48 queries. Asking Gemini about the
other 29 found `information_schema.TABLESPACES` for tablespaces, `mysql.servers`
for foreign servers, `mysql.func` for aggregates, and an engine test on
`information_schema.TABLES` for foreign tables, and it named
`information_schema.PERIODS` and `EVENTS` as catalogs the model was not reading
at all.

One model is not enough, because each one has its own gaps. Two models that
agree on an analogue raise the confidence that it is real. Two that disagree
mark the place to check on a server.

#### The rule

A model's answer is a lead, never a result. Every analogue it names is run
against the oldest and the newest supported release before it ships. An
analogue that is a stretch rather than a true match is left unsupported:
`ErrNotSupported` is an honest answer, and a column filled with something that
resembles the answer is not. Record the stretches that were rejected in
`COVERAGE.md`, with the reason, so that the next person does not find them
again and reach the other conclusion.

### D44. A version key names the product. A number alone never does. Decided.

Two products that share a dialect gate on a named version key, not on the
number. MariaDB records its version under the key `mariadb`, MySQL under
`mysql`, and a fragment written for one names that key. A server that does not
report the key does not meet the gate, however new its numbers are.

The proposal this replaces was to fold the product into the version itself,
with `V(Major(11), Minor(13), Patch(15), Variant("mariadb"))`. Gemini and
DeepSeek both rejected it, and for the same reason. A version exists to be
ordered, and a product does not order: `mariadb` is neither greater nor less
than `mysql`. Putting an incomparable thing inside the comparable type breaks
the one contract `Version` has, and it costs a signature change at 134 call
sites to do it. The named key is the same mechanism with none of that, and it
was already in the code for Cassandra, which reports its release, its CQL and
its protocol version separately.

#### The three rules

A fragment naming a key the server did not report never applies.
`VersionSet.Get` returns an unknown version for a missing key, and unknown
sorts above every known version, so reading it alone makes an absent key look
like the newest possible server. `VersionSet.Has` answers the real question and
`Gate.Met` calls it.

Within one `Choice`, a named key beats the empty key, and among alternatives
sharing a key the highest `Min` wins. A fragment written for one product is
more specific than one written for the family.

Two alternatives naming different keys, both met, is `ErrAmbiguousFragment`. It
is a fault in the model. Nothing decides between them, and picking the higher
number would compare releases that mean different things.

A parser sets a key only for a product it actually detected. Never set a key
speculatively, because the absence of a key is the fact everything above rests
on.

#### Wrong product and old server are different answers

A `Choice` where the server could not meet any alternative on any release
returns `ErrNotSupported`. A `Choice` where the server reports the key and sits
below the `Min` returns `ErrVersionTooOld`. The first is a fact about the
product and no upgrade changes it. The second is a fact about the release and
an upgrade fixes it. `Query.Support` renders the statement and reports
`NotSupported` for the first, so a caller listing what a server answers is told
the truth before it runs anything.

#### The fault this fixes, which was already shipped

The MariaDB model gated its check constraint column at 10.2 with no key. MySQL
recorded check constraints from 8.0.16 and reports 8 or 9, both below 10.2, so
that column was padded with NULL on every MySQL server that had it. The comment
beside the gate said "MariaDB 10.2 and MySQL 8.0.16" and the code said neither.
The sequence query had the same shape: gated at 11.5, it reported MySQL as too
old for an object MySQL has never had at any release.

A numeric coincidence standing in for a product test is the pattern. It reads
correctly, it happens to work for one case, and it fails the case it was
written for.

#### Forks, and what a key cannot do

CockroachDB and Redshift are the next case, and a key handles half of it.
CockroachDB has its own release scale and lies in `server_version`, so
`{Key: "cockroach", Min: V(23, 1)}` is right. Redshift is a fork of PostgreSQL
8.0 whose number is honest and useless: PostgreSQL 8.0 fragments mostly work,
PostgreSQL 14 fragments do not, and Redshift has features PostgreSQL 8.0 never
had. No version predicate expresses that. The key tags the product, and
answering what such a fork can do needs a capability probe, which is a separate
question and is not settled here.

#### When to stop sharing a dialect

Count the fragments that need a key. If most of them do, the shared model is a
partition wearing a trenchcoat, and a partition belongs in the type system: two
`Dialect` constants, two packages, shared helpers. The MariaDB and MySQL model
needs a key on seven pieces out of several hundred, so one dialect is right
today. Revisit it when that ratio moves.

#### How it is tested

`TestMySQLAgainstMariaDB` builds the same fixture on one server of each product
and compares the answer to every query that narrows to one schema, row by row
and column by column. Two kinds of difference are expected and recorded: an
object only one product has, and a column each product spells its own way.
Anything else fails. It found the fault where `external_language` is NULL on
MariaDB and `SQL` on MySQL, which would have failed to scan into a field that
is not nullable.

### D45. A query may answer partially, once, and must say so. Decided.

`Constraints` on SQLite returns primary key, unique and foreign key rows and
never returns a check constraint. It is the only query in `dbmeta` that answers
part of a question rather than all of it or none of it.

#### Why this one is allowed

D34 says a database that cannot answer reports `ErrNotSupported` rather than an
empty result, and D43 says an analogue that is a stretch is left unsupported.
Neither rule covers this case. SQLite can answer three quarters of the question
exactly, from pragmas, and the missing quarter is missing for a reason that
will not change: a check constraint exists only as text inside the
`CREATE TABLE` statement in `sqlite_schema.sql`, and `dbmeta` does not parse
DDL.

Both Gemini and DeepSeek were asked and both said the same thing. Three kinds
read exactly are worth more than refusing all four over the fourth. A caller
asking what constrains a table gets the primary key, the unique constraints and
the foreign keys, which is most of what it wanted.

#### The conditions

A partial answer is allowed only when all four hold. The part that is returned
is exact, not approximate. The part that is missing is missing structurally,
not because nobody wrote the query yet. The field description names what is
missing, in the API, where a caller reads it. A test asserts the absence, so
that it stays a decision rather than becoming a bug.

The SQLite fixture creates two check constraints and `TestSQLiteConstraints`
asserts that neither appears. Without that test this would be indistinguishable
from a query that forgot them.

#### What it is not a licence for

Do not use this to ship a query that half works. The question to ask is whether
a caller reading the result would be wrong about anything. Here it would not:
it would be missing something the field description told it would be missing.
A query that returns a wrong value, or that silently drops rows a caller would
expect, is not a partial answer. It is a defect.

#### The rejection this sits beside

`Aggregates` on SQLite went the other way, and the contrast is the point.
SQLite reports `sum`, `count` and `group_concat` with the same type code as
`row_number` and `rank`, because both groups can be used over a window. Gemini
said to map that code to aggregates and called it exact. DeepSeek said to map
only the other code. Running it against a server showed that the first would
list `row_number` as an aggregate and the second would omit `sum`. Every
available answer is wrong about something, so there is no exact part to return,
and `Aggregates` is unsupported.

Exact but incomplete is allowed. Complete but wrong is not.

### D46. Five object kinds are missing, and two consumers say which. Decided.

`dbmeta` answered 48 object kinds and neither consumer could move onto it.
Measuring both said why, and the two lists overlap. Add these five before
telling anyone to migrate.

The evidence is in `USQL.md` and `DBTPL.md`, both measured rather than read.

#### The three that both consumers need

**Routine parameters.** `usql` calls them `FunctionColumns` and `dbtpl` calls
them `ProcParams`. `dbmeta` has `Function.ArgTypes`, which is one string. A
person can read it and a code generator cannot use it. The kind needs a name, a
position, a direction, a type and a size, which is the union of what the two
ask for.

**Constraint columns.** `usql` calls them `ConstraintColumns` and `dbtpl` calls
them `TableForeignKeys`. `dbmeta` has `Constraint.Definition`, which is again
one string. The kind needs the column, its position, and for a foreign key the
catalog, schema, table and column it points at. A composite key needs the
position, or it cannot be put back together.

This is the largest gap. `dbtpl` generates code from a foreign key, and parsing
`author_id -> author(author_id)` out of a string is not something to ship.

**Column statistics.** `usql` reads them for `\ss`, which `psql` does not have
and `usql` added. Six of its drivers implement it. The kind needs the average
width, the null fraction, the distinct count, the minimum, maximum and mean,
and the most common values with their frequencies.

`\ss` is the one command that would regress on every database if `usql` moved
today, so this is not optional either.

#### The two that only dbtpl needs

**Enum values as rows.** `Types` reports `Kind` as `enum` and joins the labels
into `Type.Elements` with commas. `dbtpl` generates a Go constant per label and
needs a row each, with the sort order. Splitting the string is not good enough,
because a label can contain a comma. MySQL has no enum type, only an enum
column, so the kind has to allow a name that came from a column.

**The definition of a view.** `dbtpl.Table.ViewDef` holds the SQL of a view.
`dbmeta.Table` has no such field and no query returns one.

#### Two smaller things to decide with them

A routine needs a stable identity. `dbtpl` reads routines and then reads the
parameters of each, joined on the PostgreSQL oid. `Function` has no identifier,
and the name is not enough, because PostgreSQL allows two functions with one
name and different arguments. Decide what a parameters query joins on before
writing it.

`Column.IsPrimaryKey`. `dbtpl` reads it per column and `dbmeta` answers it
through `Constraints` or `Indexes`, which is a second query and a join in Go.
Ask whether the column should carry it.

#### What this does not change

None of these is a new kind of thing. Every one follows the existing shape: a
struct in `object.go`, a `NewQuery` value, a binding per model, a fixture object
to read, and an entry in `COVERAGE.md` for the databases that cannot answer.
The padding rule and the version gates apply unchanged.

Adding them raises the count from 48 to 53. It does not raise what any database
answers by itself, because a model has to implement each one, and a database
without column statistics reports `ErrNotSupported` like anything else.

#### Why they were missed

The 48 came from `psql`, which is D2 and is still right. `psql` prints a
constraint and a routine signature as text, because a person is reading it. A
code generator and a completer need the parts. Following `psql` for the shape
of an answer was correct, and following it for the granularity of an answer was
not, in these two places.

That is worth remembering for the next consumer. The object set was complete
against `psql` and it had never been checked against anything else.

#### Done

All five shipped, and the count went to 54. D47 sets the policy that allowed them
and the cost test for the next one. Four of the five turned out to be standard
`information_schema` views, so the shared model answers them too, which nobody
predicted. `COVERAGE.md` has the matrix and `USQL.md` and `DBTPL.md` say what
each consumer can now do.

### D47. dbmeta supplies the data. The consumer decides what to show. Decided.

`dbmeta` returns the facts a database holds. It never withholds one, reorders
one, or formats one so that somebody's output looks right. Deciding what to
print, what to leave out and how to lay it out belongs to the consumer, every
time.

`usql` aims to be compatible with `psql`, so it will show at least what `psql`
shows and will drop columns `psql` does not print. That is `usql`'s work, not
this library's. `dbtpl` wants the same facts in a different shape and shows
none of them. One set of queries serves both because neither presentation is
baked in.

#### What psql still decides

D2 stands, narrowed. `psql` sets the object model: which kinds exist, what they
are called, and the shape of an answer when two databases disagree. It does not
set the column set of a kind, and it never did: `psql` prints what a person
reads at a terminal.

That distinction is what D46 got wrong by omission. `psql` renders a constraint
and a routine signature as formatted text because a human is reading them, and
`dbmeta` copied the text. A code generator cannot use it.

#### When a field may be added

The test is cost, and it is checkable.

A field may be added when one statement can produce it: a column already in a
selected row, a column reached by a join, or a correlated subquery whose plan
stays bounded as the catalog grows. A field may not be added when it needs a
second statement, a per row round trip, or a scan whose cost grows with the
whole catalog rather than with the rows returned.

Gemini wanted correlated subqueries banned outright, on the grounds that they
cause N+1 plans. That is wrong and DeepSeek said so. N+1 is a client issuing
one query per row, which the one statement rule already forbids. A correlated
subquery is one statement. The rule would also outlaw code that already works:
the PostgreSQL model aggregates enum labels that way, and the whole SQLite
model rests on correlated table valued pragma joins, which is the only way
SQLite exposes the columns of more than one table at a time.

Check a new field with `EXPLAIN` against a catalog with thousands of tables,
not against a fixture with five.

#### Prose and parts

Where a fact exists as both, the parts are authoritative and the prose is kept.

Both reviews said to drop the prose. They are right that parts cannot be
recovered from it, and wrong about the cost of removing it. `psql` prints
`pg_get_constraintdef`, `usql` must show at least what `psql` shows, and making
every consumer rebuild that string for every dialect is work this library
already did once. `Constraint.Definition` and `Function.ArgTypes` stay.

A new fact arrives as parts. Prose is never the only form of anything.

Raw DDL is not prose. A view's definition and a trigger's body are what the
catalog stores, not a rendering of it, so they are returned as text and that is
the structured answer.

#### Granularity

A child of an object is its own kind with flat rows, never a slice on the
parent. `ConstraintColumns` and `RoutineParameters` are separate queries
carrying the parent's identity and an ordinal, and the consumer groups them.

Both reviews agreed and the reason is the iterator. Filling a slice on the
parent needs either a second statement per parent, which D33 forbids, or
`string_agg` and `json_agg`, which differ in every dialect and lose types.
Relational databases return flat rows, so `dbmeta` yields flat rows.

#### Session state and statistics are in scope

Gemini drew the line at durable DDL in the catalog and would reject the current
schema as session state and column statistics as runtime data. That line is
wrong for this library, because it excludes two of the five things the
consumers measured in D46 actually asked for, and `usql` would lose the `\ss`
command by migrating.

The line is this instead. `dbmeta` answers anything the database will tell it
about itself in one read only statement. A kind says in its documentation
whether it is durable, session dependent or runtime, so a caller knows what it
is holding. `CurrentSchema` is session dependent and says so. `ColumnStats` is
runtime, may be stale, and says so.

What is still out of scope: anything that writes, anything that needs a second
statement, and anything about the data rather than the schema. Row counts are
statistics and are in. The rows themselves are not metadata.

#### Compatibility once a kind ships

An exported field is API. Do not rename one, do not change its type, and do not
change what it means. Add a field rather than repurpose one.

Adding a field to a shipped struct is allowed, and it is why this decision is
written down before five kinds arrive at once. A caller that builds one of
these structs with an unkeyed literal breaks, and `go vet` already reports an
unkeyed literal of a struct from another package, so the compiler and the
standard tooling cover it.

Both reviews wanted an unexported `_ struct{}` field on every struct to force
keyed literals. That is a real technique and it is not taken, because `go vet`
runs the same check by default and nobody constructs these structs anyway: they
come out of a `Scan` that `dbmeta` wrote.

A field only some databases can fill is normal and is already the rule. The
model that cannot fill it selects `NULL AS "name"`, the field carries `Desc`
saying why, and `Field.Present` tells a caller whether the NULL means absent or
genuinely null. That is the padding rule and nothing here changes it.

### D48. cgo is allowed in the test module, and nowhere else. Amends D26, D29 and D35.

The root module has no driver and no cgo, and that does not change. A consumer
builds it with `CGO_ENABLED=0` and cross compiles it, because there is nothing
in it but the standard library.

The `test` module may use cgo. Its own `go.mod` keeps it out of everything a
consumer builds, which is the reason the module exists, and that isolation is
as true for a C compiler as it is for a driver version.

#### What changed

Hard rule 10 said no cgo anywhere, including the `test` module, and named
`mattn/go-sqlite3` as a driver never to use. That was wrong about SQLite.
`mattn/go-sqlite3` builds the real SQLite source, it is what most people run,
and testing only `modernc.org/sqlite` tests a reimplementation rather than the
database.

#### Test both drivers where both exist

Where a database has a canonical cgo driver and a pure Go one, test both. They
are not interchangeable. The cgo driver compiles the upstream source and the
pure Go one is a translation of it, they ship different library versions, and a
consumer that cannot use cgo runs the second. A query that works on one and not
the other is a fault worth finding.

SQLite is the case today: `mattn/go-sqlite3` and `modernc.org/sqlite`. The
SQLite tests run against each in turn, as subtests named for the driver, so a
failure says which one.

#### Which drivers need cgo

Two, and only two are known: `mattn/go-sqlite3` and the coming DuckDB driver.
Everything else on the list has a pure Go driver that is the right choice on
its own merits, and using the pure Go one there is not a concession.

`godror` stays banned, and the reason is different from cgo. It needs Oracle
client libraries installed on the machine, not just a C compiler, so it cannot
be built by a contributor who has not first installed a product. `sijms/go-ora`
speaks the wire protocol and needs nothing.

#### What CI has to do

The `test` module needs a C compiler, which `ubuntu-latest` has. Nothing in
the root module does, and the unit job must keep building with `CGO_ENABLED=0`
so that the promise about the root module is checked rather than assumed.

### D49. One method on the interface, and a NOT NULL is not a constraint row. Decided.

Two answers, both unanimous between Gemini and DeepSeek, both recorded in full
in this file, under each decision, before they were taken.

#### Querier has one method

`DB` is gone. `Querier` replaces it and declares `QueryContext` and nothing
else.

The old interface declared four methods and `dbmeta` called two. One of the
other two was `ExecContext`, so the interface of a read only library advertised
the single operation hard rule 8 forbids it from performing. `Dialect.Version`
used `QueryRowContext` and now reads its one row through `QueryContext`, which
costs four lines and removes a method from the contract.

`database/sql.DB`, `Tx` and `Conn` all satisfy one method.
`database/sql.Stmt` satisfies neither this nor the old four, because its
`QueryContext` takes no statement.

The name changed with the shape. It is a querier, not a database, and calling
it `DB` invited the reading that it stands in for `sql.DB`.

#### The interface is documentation, not a seam for a mock

This is the part worth writing down, because the goal it was added for cannot
be reached and someone will try again.

A fake cannot satisfy it. `sql.Rows` is a struct with unexported fields that
only `database/sql` constructs, from a registered driver, so nothing outside
that package can return one. That was as true of the four method version.

Both reviews rejected defining a `Rows` interface to fix that. It would change
the `Scan` signature of every binding, give up `ColumnTypes`, `RawBytes` and
`NextResultSet`, allocate per row, and break every consumer that hands a
`*sql.Rows` to something else.

Mock at the driver level. `database/sql/driver` is the seam Go provides and
`examplefake_test.go` already uses it, replaying recorded rows with no database.

What the interface does buy is the thing to keep: one method states in the type
system that this library reads and does nothing else. A reader establishes that
from the declaration rather than by searching for `ExecContext`.

#### A NOT NULL constraint is not reported on any release

PostgreSQL 18 records a NOT NULL constraint in `pg_constraint` with `contype`
`n`. Every earlier release records it only as `pg_attribute.attnotnull`.

`Constraints` and `ConstraintColumns` both exclude it, on every release. The
fact is reported by `Column.Nullable`, which is filled everywhere.

Reporting it would make the same schema answer differently on release 17 and
release 18, for a reason that has nothing to do with what either server can do.
That is the leak this library exists to prevent.

Synthesizing the rows on older releases from `attnotnull` was rejected, and the
reason is sharper than inventing names. Release 18 lets a NOT NULL constraint
be named explicitly, so a synthesized `<table>_<column>_not_null` would be
wrong some of the time, which is worse than absent.

#### The gap this found in the padding rule

The padding rule governs the column set. It says nothing about rows, and until
now nothing needed it to.

A release that starts recording an existing fact as a catalog row is a version
leak the padding rule does not catch. The rule to add: a query returns the same
rows for the same schema on every release, and where a release records
something new about an object that was always true, report it the way every
release can report it or do not report it at all.

`TestNotNullIsNotAConstraintRow` holds this one, and it runs on all ten
releases, so it fails on 18 alone if the filter is ever removed.

#### What is still not answered

A migration tool on release 18 that wants the name of a NOT NULL constraint, in
order to drop it by name, cannot get it here. Both reviews called that
PostgreSQL specific and outside the normalized model.

If it is wanted later, the shape that does not leak is a field on `Column`
holding the constraint name where the release has one and absent where it does
not, with `Field.Min` saying which. That is the padding rule doing its job, and
it is a decision rather than a translation.

### D50. Documentation lives in docs, and the decision log stays one file. Decided.

Three files in the repository root: `README.md`, because GitHub renders it,
`CLAUDE.md`, because an agent reads it first, and `CONTRIBUTING.md`, because
GitHub gives it its own behaviour. Everything else is in `docs/`.

Both reviews agreed on that much and on nothing else, and the measurement that
prompted it was 6116 lines of Markdown in 11 root files against 8261 lines of
Go.

#### The decision log is one file with an index

Gemini wanted this file split into one record per decision, the ADR
convention, on the grounds that 3384 lines is about 25,000 tokens and an agent
loads all of it to answer one question.

That is not taken, and the evidence is in this repository.

Six of the 49 decisions amend, supersede or withdraw an earlier one. D48 amends
D26. D24 supersedes D22. D11 is amended by D26. D19 is half overtaken. D6 and
D38 are amended. One in eight, and the rate rises rather than falls, because a
project that runs long enough learns things.

Split, an amendment lives in a different file from the decision it amends. An
agent greps a topic, lands on the older record, reads a rule that was
overturned, and gets no signal that it was. A slow answer becomes a wrong
answer, which is worse than a slow one.

There are 217 references to a decision by bare number in this file and 336 more
in the other documents and in the Go source. Split, every one of those is a
filename to guess, because `D8` does not say whether the file is
`0008-metadata.md` or `0008-abandon-the-subpackages.md`.

Gemini's cost is real and the index is the answer to it. The table at the top
of this file gives the number, the title and the status of every decision in 55
lines. An agent reads the table, jumps to one decision, and loads that. Nobody
had tried it before deciding to split.

#### The status column is the part that matters

A decision is read through its status. `Decided` means it stands.
`Amended by D26` means read both. `Superseded by D24` means read the other one.
Put the status in the heading when you add a decision, and the index picks it
up.

#### One rule per file where the rule is expensive

`NULLS.md` stays its own file. Gemini wanted it deleted and merged into
both `CLAUDE.md` and `CONTRIBUTING.md`, which would put the most expensive rule
this project has learned in two places and guarantee they drift. It is linked
as a requirement from both instead.

#### Two indexes, because there are two readers

`README.md` lists the documents for a person arriving from pkg.go.dev.
`CLAUDE.md` holds a routing table for an agent: what to read before touching a
given thing. They answer different questions and neither replaces the other.

#### What was not archived, and a correction

Both reviews were told `QUERIES.md` and `EVALUATION.md` were written once and
never updated, and both suggested archiving or renumbering them on that basis.
The description was wrong and the advice followed from it.

`QUERIES.md` is cited from `object.go`, `models/postgres/postgres.go` and the
information_schema test. It is the survey that justifies the object set, and a
reference document that is still correct does not need updating to be live.
`EVALUATION.md` is the procedure for choosing the supported versions of a
database that is not covered yet, which is a thing the project will do dozens
more times.

Both stay in `docs/` as reference. Nothing is archived, because nothing here is
stale.

#### REVIEW.md is gone

It held the argument behind decisions already taken, which is what this file
holds. Two places for the same reasoning is the failure this decision is about,
so its contents were folded into the decisions they argue for and the file was
removed.

An open question goes in the open questions section at the end of this file. A
decided one becomes a decision. There is no third state that needs a document.

#### The thing neither review raised

`COVERAGE.md`, `USQL.md` and `DBTPL.md` are generated from measurement and go
stale silently. Filing them better does not fix that. A test that fails when
the counts drift would, and `container/workflow_test.go` already does exactly
that for the CI matrix. That is worth more than any amount of organizing and it
is not done yet.

### D51. There is no alias for a nullable type. Decided.

A field a database may report as NULL is declared `sql.Null[T]` and never a
named alias of one. `Text` and `Int` are gone.

#### What was inconsistent

Two of the four nullable kinds were aliased and two were not: 93 fields as
`Text`, 5 as `Int`, and five written out as `sql.Null[bool]` or
`sql.Null[float64]`. Two structs declared next to each other read differently
for no reason a caller could see.

#### Why not alias all four instead

Both reviews rejected that and so did Ken, for the same reason, and the reason
is what a reader actually sees.

`go doc` in a terminal prints plain text. With an alias a reader of
`go doc dbmeta.Sequence` sees `Cycles Text` and has to run a second command to
learn that it can be absent. Without one they see `Cycles sql.Null[bool]` and
already know. A code review diff and an editor's field list behave the same
way, and pkg.go.dev's clickable link is the only place where the alias costs
nothing.

The other two would have had to be called `Bool` and `Float`, which read like
primitives and hide the single thing a caller has to know about the field.
Gemini's phrasing: a name like `Text` is dangerous for a nullable type, because
a reader assumes it behaves like a string and does not expect to check `Valid`.

Neither review thought 93 against 5 argued for keeping one alias. DeepSeek
called that status quo bias, and it is. Frequency makes a name dominant, not
clear.

#### Defined types were never an option

`type Text sql.Null[string]` does not inherit the methods of its underlying
type, so it loses `Scan` and `Value` and every `rows.Scan(&v.Comment)` in all
every binding stops working. The aliases worked only because they were aliases.

#### Where the lesson went

The `Text` doc comment held the canonical account of this project's most
expensive mistake, that collapsing a NULL access list into an empty string made
"the owner has full access" read identically to "nobody has any access". That
is in `NULLS.md` in full, where it always was, and the package documentation in
`object.go` now points there.

An invariant that governs the whole project should not have been hanging off a
type alias. Deleting the alias fixed that as a side effect.

#### The shape of the change

98 field declarations in `object.go` and nothing in `models/`, because every
binding names the struct field rather than the type. It was mechanical, and it
was cheap only because no release is tagged: `Text` was exported.

### D52. A test driver is the one usql uses, or it is the wrong driver. Amended by D59.

The `test` module imports, for each database, the same driver package `usql`
imports for that database. Not the same version, which each module pins for
itself, but the same package.

`usql` marks them: every driver import carries a `// DRIVER` comment, and
`grep -rn "// DRIVER" usql` is the list. Consult it before adding a driver, and
again before adding a dialect.

#### Why the package and not the version

`dbmeta` exists to be read through `usql`. A query that works on the driver
this repository tests and fails on the one `usql` ships is a query that does
not work, and the failure surfaces in someone else's project.

Drivers are not interchangeable. They differ in how they present a type, in
what they do with a NULL, and in which protocol extensions they use, and those
are exactly the things a metadata query touches. The NULL scan fault this
project has hit twice is driver visible behaviour.

The version is a different matter. Each module pins what it needs, and a
consumer picks its own, which is hard rule 1 and does not change.

#### What was wrong, and how it was found

The DuckDB work was written against `github.com/marcboeker/go-duckdb/v2`,
which is the widely known driver and is not the one `usql` uses. `usql` uses
`github.com/duckdb/duckdb-go/v2`, the successor under the DuckDB organisation.
Ken caught it before it was committed.

The other four were already right, and that was luck rather than method:
`go-sql-driver/mysql`, `jackc/pgx/v5/stdlib`, `mattn/go-sqlite3` and
`modernc.org/sqlite` all match `usql`. This decision makes it method.

#### The list, as it stands

| Database | Driver | usql driver directory |
| --- | --- | --- |
| PostgreSQL | `github.com/jackc/pgx/v5/stdlib` | `pgx` |
| PostgreSQL | `github.com/lib/pq` | `postgres` |
| MariaDB and MySQL | `github.com/go-sql-driver/mysql` | `mysql` |
| SQLite | `github.com/mattn/go-sqlite3` | `sqlite3` |
| SQLite, pure Go | `modernc.org/sqlite` | `moderncsqlite` |
| DuckDB | `github.com/duckdb/duckdb-go/v2` | `duckdb` |

`usql` has two drivers for PostgreSQL and two for SQLite, and both pairs are
tested. SQLite is tested on both because one compiles the upstream source and
the other is a translation of it.

PostgreSQL was tested on `pgx` alone, on the reasoning that `lib/pq` speaks the
same protocol and is in maintenance. That reasoning named the wrong boundary.
The protocol is not where a metadata query fails. The scan is, and the two
drivers reach it differently: `pgx/stdlib` asks for binary result formats and
`lib/pq` asks for text, so a value arrives at `Scan` having taken a different
path. It also named the wrong driver as primary. A person typing `postgres://`
into `usql` gets `lib/pq`, and `pgx` is a second scheme they have to ask for.

So both are tested, as subtests named for the driver, the way SQLite already
was. Every query, the shared `information_schema` model and the password
escaping run twice.

#### What the measurement found

Nothing, which is the outcome worth recording. Every metadata test passes
identically on both drivers at 9.6, 12, 15 and 18. No scan failure, no NULL
that became an empty string, no row that appeared on one and not the other.
The old conclusion was right and its reason was not, and only one of those is
worth keeping.

One thing does differ, and it is the driver rather than the database:

```
pgx     tablespaces refused: ERROR: permission denied for tablespace pg_global (SQLSTATE 42501)
lib/pq  tablespaces refused: pq: permission denied for tablespace pg_global (42501)
```

`test/testdata/parity.txt` records that string verbatim, so the parity file is
coupled to the driver wherever a query is refused. Parity therefore runs on the
first driver only. It measures what the server answers a principal, which is
not a driver question, and running it twice would buy a per-driver section for
every refusal in the file in exchange for re-measuring the database.

#### Which databases have a second driver at all

`grep -rn "// DRIVER" usql` gives the whole list. Four of the eight models here
have more than one, and two of the four are tested on both.

| Database | usql drivers | Tested on |
| --- | --- | --- |
| PostgreSQL | `postgres` (lib/pq), `pgx` | both |
| SQLite | `sqlite3` (mattn), `moderncsqlite` (modernc) | both |
| MariaDB and MySQL | `mysql` (go-sql-driver), `mymysql` (ziutek) | `mysql` only |
| Oracle | `oracle` (go-ora), `godror` | `oracle` only |
| SQL Server, Cassandra, ClickHouse, DuckDB | one each | that one |

`godror` was already excluded and the reason has not changed. It needs Oracle
client libraries installed on the machine rather than only a C compiler, so a
contributor cannot build the tests without first installing a product.

`mymysql` is excluded on a measurement rather than a rule, and the measurement
is worth keeping because the question will be asked again. It is a genuine
second implementation and `usql` points it at the same metadata reader, so it
looked like the MariaDB version of the SQLite case. It is not, for two reasons
found by trying it.

It cannot reach MySQL at all. It has no `caching_sha2_password`, which is the
default from MySQL 8.0, so 8.4 answers `#1251 Client does not support
authentication protocol requested by server` and 26.7 drops the connection.
Both supported MySQL releases are out of reach, and the plugin it does speak is
disabled by default from 8.4.

It cannot build the fixture on MariaDB either, where it does connect. The
aggregate step is a compound `BEGIN ... END` body, and `mymysql` returns
`reply is not completely read` rather than running it. Rule 9 makes the fixture
part of the model, so a driver that cannot create the objects cannot test the
queries that read them.

A driver that reaches neither MySQL product and cannot set up the one it does
reach is not a second measurement. Revisit if `mymysql` gains the auth plugin,
and not before.

#### Correcting hard rule 10

Hard rule 10 named `gocql/gocql` for Cassandra. `usql` uses
`github.com/MichaelS11/go-cql-driver`, which is the `database/sql` driver that
wraps `gocql`. `dbmeta` needs a `database/sql` driver, so the rule named a
package that cannot be used. Corrected.

#### When usql does not have one

A database `usql` does not support has no driver to match, and the choice is
open. Say so in the commit, and prefer the driver `usql` would most likely
adopt, which is usually the one the database's own organisation publishes.

### D53. One canonical expectation, checked in, that every database must meet. Decided.

`TestConformance` builds the same core schema on every database, projects each
answer onto the facts that are portable, and compares the result against one
checked in file. `testdata/conformance.txt` has a section per database.

It does not replace `TestMySQLAgainstMariaDB`, which compares raw values within
one family and catches what this cannot. Goldens catch a regression and
pairwise catches a divergence, and they are different faults.

#### The fixtures came first, and that was the largest part

Nothing had ever checked the fixtures against each other and they had drifted:
32 steps on PostgreSQL, 19 on MariaDB, 14 on SQLite, 12 on DuckDB, and three of
the four built a region and a shipment table where SQLite did not.

A comparison built on drifted fixtures reports the fixtures. So the fixtures
were aligned first: SQLite gained region and shipment, PostgreSQL's identity
and generated columns moved off author and book onto a table of their own, and
every view selects the same two columns.

`TestEveryFixtureBuildsTheCoreObjects` holds it, in the root module, needing no
database. It matches a `CREATE` rather than the name anywhere, which the first
version did not: renaming the region table did not fail the test, because
shipment's foreign key still said `REFERENCES region(country, area)`.

#### Values are compared, not only names

Gemini said no value can be compared across families and DeepSeek said a subset
can. DeepSeek is right and the evidence is local: the fault that justified the
MariaDB comparison was a NULL that would not scan, which is a value fault that
names and row counts would have missed.

The portable subset is what the standard makes every database record the same
way: whether a column accepts NULL, where it sits, whether it is in the primary
key, whether it has an explicit default, and which columns a constraint covers
in what order with what it references.

#### Why one file rather than ten pairwise comparisons

Five databases pairwise is ten comparisons and a failure does not say which
side is wrong. One expectation is five comparisons and every failure names the
database.

It works only because the canonical projection is release independent. A raw
golden would need one file per product per release, because PostgreSQL 9.6 and
18 disagree about raw values. Nullability and ordinal position do not change
between releases, so one file covers every release of every database, and the
PostgreSQL job checks it on all ten.

#### The mechanism that stops a model being made to lie

This is the part that matters, and Gemini's answer to it was a principle where
DeepSeek's was a mechanism. The mechanism is taken.

Every function in `canonical.go` takes a value and returns a new one. None
takes a pointer to a model struct and none writes to a field.

`canonicalFields` names every field the projection drops, folds or maps, with
the reason, and `TestCanonicalFieldsAreRecorded` checks it by reflection. A
field on the model that is not on the canonical struct and not in the list
fails the build. It found eight undocumented drops the first time it ran.

The raw values stay asserted by each database's own tests, which this cannot
weaken. Where the projection maps a difference away, the raw behaviour is
pinned by a test named in the entry.

#### What it found

Eleven differences, all real, none of them faults:

SQLite reports a primary key column as nullable, because an `INTEGER PRIMARY
KEY` there genuinely accepts NULL unless declared otherwise.

PostgreSQL reports a default on a primary key because `serial` is a `nextval`
default, where `AUTO_INCREMENT` is not a default at all.

MariaDB records the four character string `NULL` as the default of a nullable
column declared without one, where MySQL and everything else report no default.
`TestMySQLNullDefault` pins both, and it is the entry that justifies the one
mapping in `canonicalFields`.

MariaDB reports a view's columns as not nullable where the others say nullable.

Only PostgreSQL and DuckDB report the columns of a check constraint.

23 of the canonical lines are identical across all four.
`TestConformanceAgreementHolds` fails if that number falls, so something that
was uniform becoming non uniform is a decision somebody makes rather than a
thing that happens.

#### Rejected: a step carrying every dialect's DDL

Gemini proposed `Exec map[Dialect]string` on a fixture step. DeepSeek's
objection is the one that decided it: a missing dialect key skips the step
silently, the expectation is regenerated, the test passes, and that database is
never exercised.

Silence is the failure mode this project has been bitten by most. Separate
fixtures fail loudly, and the core object test now catches what they miss.

#### What productSpecific keeps doing

`TestMySQLAgainstMariaDB` keeps its hand written list of columns that
legitimately differ, and the list keeps its second job, which neither review
noticed: it is where `int(11)` against `int`, and MySQL parenthesizing a view
definition, got written down at all. A difference absorbed silently into a
file stops being knowledge. `canonicalFields` is the same idea for the
canonical comparison, which is why every entry carries a reason rather than a
flag.

### D54. SQL Server covers every release that ships a Linux container. Amended by D63.

Four releases, 2017, 2019, 2022 and 2025, and every one of them at the Tested
tier. CI runs all four on every push. 2016 and older are Archived.

#### The floor is a container fact, not a query fact

Microsoft shipped SQL Server on Linux from 2017. `mcr.microsoft.com/mssql/server`
carries 284 tags and not one of them names 2016, 2014 or 2012. So the floor is
not a judgement about which releases deserve support. It is the oldest release
anybody can run in CI, and there is nothing below it to argue about.

Two facts about the images. Microsoft publishes no bare release tag, so the tag
is `2017-latest` and never `2017`, which is why `product` carries a
`tagSuffix`. The 2017 image is built on an older base and installs sqlcmd at
`/opt/mssql-tools` where the other three use `/opt/mssql-tools18`, which is why
`container.SQLServer` overrides the readiness command for that one release.

#### Why all four are Tested rather than two Tested and two Nightly

Gemini proposed 2019 and 2022 on every push with 2017 and 2025 nightly, on
installed base. That reasoning fits a product with ten releases. This one has
four.

Every version gate the model has sits below 2017, so these four releases differ
by what they added and not by what they lack. There is no old branch for a
nightly job to protect. Four service containers cost four parallel jobs, and
the claim they buy is the whole one: dbmeta is tested on every SQL Server that
runs on Linux.

#### The gates below the floor, which is the part that needed deciding

The model carries two gates and both sit below 2017. `sys.sequences` and
`sys.dm_db_stats_properties` arrived in 2012, and `sys.external_tables`,
`sys.tables.is_external` and `sys.tables.temporal_type` arrived in 2016. CI
reaches the new branch of each and can never reach the old one.

The two reviews split on this, and the split is the useful part.

Gemini said keep them. The `sys` views are additive and backward compatible, so
a gate written from Microsoft's documentation will not surprise anybody, and
2008 R2 through 2016 go in an Archived tier with wording that says CI never
touched them.

DeepSeek said delete them and raise the floor, because the old branch of a gate
no test reaches is dead code. If they are kept, it said, fake the version in a
test and say plainly that the old path is not integration tested.

DeepSeek's objection is the right one and its remedy is the one this project
already has. A statement resolves against a version set, and a version set is a
value, so resolution below the floor is testable with no server at all. That is
`models/sqlserver/version_test.go`, and it holds three things: each gated query
refuses with `ErrVersionTooOld` below the release that added its catalog view
and reads that view at or above it, the one gate that pads rather than refuses
never names `temporal_type` or `is_external` on a release that has not got
them, and the set of statements that build on 2008 R2 is the set that builds on
2025 less exactly the three a gate names.

So the gates stay, and they are no longer a claim nobody checks.

#### What may honestly be said about 2014 and 2016

This much: the statement resolves, it names only catalog views that release
documents, and a reviewer read it. Not that it ran, because it cannot.

Write it that way. Do not write supported, do not write compatible, and do not
put an old release in a table beside one that CI runs without saying which is
which. D40 gives three tiers and none of them fits a release with no container,
so such a release is Archived and Archived means nothing is claimed.

#### What this does not decide

Whether `Query.Support` should answer no for a server too old to build the
statement. Today it answers yes and `Build` then returns `ErrVersionTooOld`, which
`TestWrongProductIsNotSupported` fixes deliberately: Support answers a question
about the product, and the release is the error's business. Writing the test
above raised the question of whether a caller is well served by that, because a
caller that trusts Support walks into a query it cannot build. It is left as it
is, and it is for Ken.

D63 answers it: Support gained a fourth value and now says so itself.

#### Oracle, recorded and not decided

The same question is coming for Oracle and the container facts are these.
`gvenzl/oracle-xe` has 18.4 and 21.3, `gvenzl/oracle-free` has 23, and there is
nothing for 11g or 12c. So Oracle gets the same hard floor for the same reason.

Both reviews agree that Express Edition answers the core catalog and that it is
not a stand-in for Enterprise Edition everywhere, and they name the same gaps.
`DBA_HIST_*` needs the Diagnostics Pack and is absent. The partitioning views,
`ALL_PART_TABLES` and `ALL_TAB_PARTITIONS` among them, exist and stay empty
because XE cannot partition. `ALL_POLICIES`, the Database Vault and Label
Security views, and the encryption columns are absent or empty. `ALL_TABLES`
has in-memory columns that report nothing.

Two things matter more than the feature list. `ALL_*` shows the caller only what
the caller may see, which is the `information_schema` problem this project
already knows, and `DBA_*` needs `SELECT_CATALOG_ROLE` that an ordinary user
does not have. So the Oracle model must choose between the two deliberately and
the test user must be a named one with fixed grants. See the requirements on the
container harness above.

### D55. The current user moves here. Changing a password does not. Decided.

`usql` carries database specific SQL outside its metadata readers. All of it
was audited, and it is three hooks on `drivers.Driver` and nothing else.

| Hook | What it runs | Reads or writes | Moves |
| --- | --- | --- | --- |
| `Version` | `SHOW server_version`, `SELECT sqlite_version()`, five `SERVERPROPERTY` calls | reads | already here, D38 |
| `User` | `SELECT current_user`, and `SELECT user FROM dual` on Oracle | reads | yes, as `CurrentUser` |
| `ChangePassword` | `ALTER USER`, `ALTER ROLE`, `ALTER LOGIN` | writes | no |

Everything else on that struct is Go behaviour rather than SQL: `Process`
rewrites a statement, `ColumnTypes`, `RowsAffected`, `Err` and the `Convert`
family read a result, and `Copy` generates inserts, which is data movement and
not metadata.

Two statements the audit turned up outside those hooks are already in scope.
The `Catalogs` readers for DuckDB and Trino run
`SELECT database_name FROM duckdb_databases() WHERE NOT internal` and
`SHOW catalogs`, and both are metadata, answered here by `Databases`.

#### CurrentUser, and why it reports two names

`CurrentSchema` already exists and describes the connection rather than the
database. The current user is the same shape of question, so it is the same
shape of answer: one row, read with `First`.

It reports two names because most products have two. The effective user is who
the session acts as now and the session user is who it authenticated as. They
differ after `SET ROLE` on PostgreSQL, they differ on MariaDB when the
connection matched a wildcard host, and on SQL Server they always differ,
because a connection authenticates as a server login and acts as a database
user. Connecting as `sa` answers `dbo` and `sa`.

Both come from one statement on every product that has them, so D47 says
report both. `usql` reports one, which is the login on SQL Server, because
`ALTER LOGIN` is what its password change needs.

SQLite answers neither and the query is not registered there, so it reports
`ErrNotSupported` rather than inventing a name. DuckDB answers the fixed string
`duckdb` and has no session user, so that column is NULL under the padding
rule rather than a copy of the name.

The shared model gains it too, which takes it from 11 kinds to 12. It is the
one binding there that reads no view at all, because `CURRENT_USER` and
`SESSION_USER` are standard SQL expressions rather than `information_schema`
tables. Two lines of standard SQL for a whole kind is the cheapest addition
this model has had.

#### Why changing a password stays in usql

D5 is the first reason and it would be enough on its own. `dbmeta` reads. A
consumer can hand it a read only connection and reason about what it can do,
and one write would end that.

The second reason is the one that settles it even for somebody willing to
reopen D5. A password statement cannot bind a parameter. Both were tried on a
real server:

```
postgres=# PREPARE t AS ALTER USER postgres PASSWORD $1;
ERROR:  syntax error at or near "ALTER"

1> EXEC sp_executesql N'ALTER LOGIN sa WITH PASSWORD = @p', N'@p nvarchar(50)', @p=N'X'
Msg 102, Level 15, State 1: Incorrect syntax near '@p'.
```

Every statement here binds its arguments. Moving this one would mean `dbmeta`
interpolating a secret into SQL text, and carrying a quoting and escaping rule
per product to do it safely. That is a new class of risk in a library that has
none today, in exchange for moving one line per driver.

`RequirePreviousPassword` is a flag on the same struct and points the same way:
this is a client concern, and the client already knows how to prompt for it.

#### The rule this sets

A statement moves here when it reads. A statement that writes stays with the
client, whatever it is about. That is D5 restated, and the audit found no case
that argues against it.

### D56. The password statement is built here and run by the caller. Amends D5.

D55 audited `usql` for database specific SQL and found `ChangePassword`, and
refused to move it because `dbmeta` reads. That refusal is reversed. The
knowledge moves here and the authority does not.

[Dialect.ChangePassword] returns statement text. It takes a
[PasswordChange] and a [Quoting], and it takes no database, so there is no way
for it to run anything. Everything `dbmeta` executes is still a read, and a
consumer still hands it a read only connection.

#### What changed the answer

Two things, and the second is the one that matters.

The first is that D55 weighed the wrong risk. It treated leaving the statement
in `usql` as the safe option. `usql` concatenates the password into the
statement with no escaping at all, in seven drivers, so the status quo was not
safe anywhere. The choice was never between safety here and safety there.

The second is that the escaping rule needs the server, and `dbmeta` is the
thing that reads the server. Whether a backslash escapes inside a string
literal is session state: `sql_mode` on MySQL and MariaDB, and
`standard_conforming_strings` on PostgreSQL. A consumer that writes this itself
has to learn to read those, per product. This module already reads settings.

Both reviews recommended against the move and both were answered by that
second point, which neither raised. Gemini's objection was the read only
invariant, and returning text keeps it. DeepSeek's was that a static escaper
cannot be correct without session state, which is an argument about where the
state is read rather than about whether the statement belongs here.

#### Why it cannot simply be a parameter

It cannot be bound. PostgreSQL will not prepare the statement at all:

```
postgres=# PREPARE t AS ALTER USER postgres PASSWORD $1;
ERROR:  syntax error at or near "ALTER"
```

and SQL Server rejects `@p` in `ALTER LOGIN`. The password goes into the text,
so the escaping has to be right.

#### What wrong escaping does, demonstrated

Quote doubling alone is not enough. Sent to a real MariaDB with the password
`x\`:

```
CREATE USER t3@'%' IDENTIFIED BY 'x\';
SELECT 'statement completed' AS result;
```

The backslash escaped the closing quote, the literal swallowed the semicolon
and the line after it, and the second statement never ran. That is not a
mangled password. That is the password consuming the rest of the statement.

[Quoting] carries the state, and a product that needs it and does not have it
returns [ErrQuotingUnknown] rather than guessing. SQL Server needs none,
because T-SQL has no such setting and a backslash is an ordinary character,
verified on 2022 where `LEN('a\b')` is 3.

#### The shape

`Quoting` is read with [Dialect.Quoting], which runs a query and is a read like
any other here, or with [Dialect.QuotingQuery] and [Dialect.ParseQuoting] for a
caller that runs its own statements. That pair is the same shape as
[Dialect.VersionQuery] and [Dialect.ParseVersion], deliberately.

A password and a user name are quoted by different rules, because they are
different things: PostgreSQL takes a role as an identifier and a password as a
literal, SQL Server takes a login in brackets, and MySQL takes an account as
two literals joined by an at sign. [QuoteLiteral] and [QuoteIdentifier] are
exported, because the rule is the thing worth reviewing once and a model
outside this repository needs the same one.

MySQL is the one product here where this is an addition rather than a move.
`usql`'s mysql driver declares no `ChangePassword` at all, so it cannot change
a password today, and the product whose escaping is hardest is the one that
had none.

#### What the caller still owns

The returned text contains the password in clear, because the server requires
that. A caller must keep it out of its logs, and `dbmeta` cannot do that for
it. This is the first code here that handles a secret, and it is worth saying
so rather than leaving it implied.

#### How it is tested

A unit test would not have caught the fault this exists for. So each case sets
a real password on a real server and then opens a new connection with it,
across seven passwords chosen to break a naive escaper.

Removing the backslash rule proves both failure modes. One password raises a
syntax error, and one sets the wrong password successfully and is caught only
by the login failing afterwards. The second is why the test connects rather
than comparing text.

### D57. A Windows machine is how a pre 2017 SQL Server gets tested, and it is Verified. Decided.

SQL Server on Linux begins at 2017. D54 put everything older in Archived,
which claims nothing, because no container exists and CI cannot run one. This
is how that changes: a Windows virtual machine, provisioned without a person
watching, hosting one old SQL Server.

| SQL Server | Windows host | dockur `VERSION` |
| --- | --- | --- |
| 2008 R2 SP2 Express | Windows Server 2008 R2 | `2008r2` |
| 2012 SP4 Express | Windows Server 2012 R2 | `2012r2` |
| 2014 Express | Windows Server 2012 R2 | `2012r2` |
| 2016 SP2 Express | Windows Server 2016 | `2016` |

2008 R2 is the floor, and it is a media floor rather than a judgement.
Microsoft still publishes the Express installer for 2008 R2, 2012 and 2014, and
the 2012 release page is gone while every 2012 service pack page is still
there. The plain 2008 page is gone entirely.

#### The tier

Verified, never Tested. A machine needs KVM and the better part of an hour, so
CI cannot run one, and calling it Tested would put an untestable release in the
table beside a release CI runs on every push. D40 already has the right word
and this uses it. `container/windows_test.go` fails if a machine is given any
other tier.

That does add the thing D54 said was missing. Tested, Nightly and Verified all
mean "runs again", and a frozen release is one nobody ships anything for, so
one verified run stays true. That is a property of the release rather than a
fourth tier, and it did not need a new word after all.

#### One machine per release

SQL Server installs side by side, so four releases could share two machines.
Both reviews said not to, for the same two reasons, and both are right.

A second release on a host has to be a named instance. A named instance takes a
dynamic port and needs the SQL Server Browser, where a default instance is
1433 and needs neither. And the releases disagree about prerequisites, because
2008 R2 and 2012 want .NET Framework 3.5 where 2016 wants 4.6, so a shared host
is a host where at least one release is installed unusually. The whole point is
to see what a normal installation of that release reports.

They run one at a time rather than together.

#### Where it lives

The same shape as the Linux side, because it is the same problem. The list is
Go data in `container/windows.go` and `dbrun provision` reads it from there.
One copy, and a test that fails when the payload and the list disagree.

#### Licensing, and what this deliberately does not do

Every Windows image is a Microsoft evaluation edition, fetched from Microsoft
by dockur: the Server 2016 ISO is `Windows_Server_2016_Datacenter_EVAL`, and the
2008 R2 one is `GRMSXEVAL`. An evaluation edition is free for 180 days of
testing and needs no product key and no activation, and `slmgr /rearm` extends
it, which is Microsoft's own mechanism.

So nothing here activates Windows. There is an existing script outside this
repository that does, by installing a generic volume licence key and pointing
`slmgr /skms` at a public KMS emulator. That is circumventing licensing rather
than complying with it, and it is also unnecessary, because the evaluation
editions already permit exactly this use. It was not carried over and it should
not be.

SQL Server Express is free on the same footing, and it is enough: every catalog
view these queries read exists in Express, and the fixture builds nothing that
Express cannot.

#### Three things that were not obvious, and one that was wrong

The installer is downloaded on the Linux host. Windows Server 2008 R2 has no
TLS 1.2 and cannot reach Microsoft's download servers, so fetching it inside
the machine works on three releases and fails on the oldest.

The listening port has to be written to the registry after setup. Express
installs with TCP disabled on a dynamic port whatever `TCPENABLED=1` says, and
the instance key is named for the release, `MSSQL10_50` through `MSSQL13`. A
key for the wrong release puts the port where nothing reads it, setup reports
success, and the machine is unreachable with nothing in any log.
`TestTheRegistryKeyMatchesTheRelease` holds that mapping.

2008 R2 differs twice: `[SQLSERVER2008]` rather than `[OPTIONS]` as the section
header, and setup refuses `/IACCEPTSQLSERVERLICENSETERMS`, which arrived in
2012.

The one that was wrong is worth recording. Readiness was first tested by
opening the published port, and the container runtime publishes that port when
the container is created, so it answered twenty seconds in and every machine
was declared ready before Windows had begun installing. Readiness is a query
now. A check that cannot fail is worse than no check, because it is believed.

#### What may be claimed after a machine runs

That the queries ran against that release, on that build, on that date. Not
that they run today, because nothing runs again. Record the build in
`docs/COVERAGE.md` beside the claim, the same way the tested releases record
theirs.

### D58. One .gitignore, in the repository root. Decided.

There is one `.gitignore` and it is the one in the root. Do not add a second in
a subdirectory, however local the thing being ignored feels.

#### Why

The question "what does this repository ignore" has to have one answer in one
place. A reader who has to find every `.gitignore` before answering it will
miss one, and the one they miss is the one that matters.

That is not hypothetical here. Two were written for the test harness.
`test/vm/.gitignore` was committed and `test/oracle/.gitignore` was not, so the
Oracle build directory was unprotected, and 2.9 GB of Oracle's checkout stayed
out of a commit only because that work happened to be uncommitted when the gap
was noticed. Nobody had done anything wrong. The arrangement simply had no
place where the omission was visible.

A single file also makes the rule auditable by the tool. `git check-ignore -v`
names the file and the line, and when there is one file that output is an
answer rather than a starting point.

#### What this does not mean

It does not mean the repository ignores much. It ignores `state/`, which is
where the test harness would put virtual machine disks and fetched checkouts
if somebody pointed `DBMETA_VM_STATE` or `DBMETA_ORACLE_STATE` back into the
working tree. Those live under `$XDG_DATA_HOME/dbmeta` instead, because a
Windows disk is tens of gigabytes and a working tree holding one is a tree
where every grep and every editor index walks it.

So the entry is insurance rather than routine. That is the point: an ignore
rule earns its place by covering the case nobody meant to create.

#### For an agent working here

Ignore a new artifact by adding a line to the root file, with a comment saying
what produces it. Do not create a `.gitignore` next to it. If a pattern needs
to be scoped to one directory, write the path in the root file rather than
moving the rule closer to the thing.

### D59. Oracle is tested with go-ora v2 until v3 tags its fix. Amends D52.

D52 says a test driver is the one `usql` uses or it is the wrong driver.
Oracle is the first exception, and it is not a preference.

`usql` pins `github.com/sijms/go-ora/v3 v3.0.1`. That version cannot connect to
Oracle 11g or 18c. It does not return an error. It panics:

```
panic: runtime error: slice bounds out of range [41:32]
	go-ora/v3/network.newAcceptPacketFromData
	go-ora/v3/network/accept_packet.go:61
```

The cause is in the source. `newAcceptPacketFromData` reads

```go
NegotiatedOptions2: binary.BigEndian.Uint32(packetData[41:]),
```

unconditionally, and the accept packet an older server negotiates is 32 bytes.
v2 talks to every release here. Measured against all six:

| Driver | 11g | 18c | 19c | 21c | 23ai | 26ai |
| --- | --- | --- | --- | --- | --- | --- |
| `go-ora/v2 v2.9.0` | yes | yes | yes | yes | yes | yes |
| `go-ora/v3 v3.0.1` | panic | panic | yes | yes | yes | yes |
| `go-ora/v3` at master | yes | yes | yes | yes | yes | yes |

The last row is the important one. It is fixed upstream and not yet tagged, in
sijms/go-ora issue 759, and the fix was checked here against all six servers at
v3.0.2-0.20260914154503-360b4b7ac9e9 rather than taken on trust.

#### Why this does not weaken D52

D52 exists so that a query passing here cannot fail on what a user runs. That
reasoning is about the SQL, and it still holds: the statements are the same
whichever major version of the driver carries them, because the difference is
in the wire protocol negotiation and not in what the server parses.

What D52 could not anticipate is a driver that cannot reach the server at all.
Holding to v3 would not make the old releases work in `usql`. It would only
stop dbmeta from testing them, and then nobody would know they are broken.

#### This reaches usql, and only until v3 tags

`usql` switched to v3 recently and pins v3.0.1, so a `usql` user who connects
to Oracle 11g or 18c today gets a Go panic rather than a message. A panic takes
the program down, which is worse than not supporting the release.

Ken is moving the `xo` database projects back to v2 for now. The fix being
already upstream makes that an interim measure rather than a direction.

#### The driver to want, which now exists

`godror` was Oracle's sanctioned driver and hard rule 10 bans it, because it
needs the Oracle Instant Client rather than only a C compiler.

Oracle has since published a pure Go one: `github.com/oracle/go-oracledb`,
package `oracle`, registering the driver name `oracledb`. It depends on
`golang.org/x/crypto` and `golang.org/x/text` and nothing else, so hard rule 10
has no objection to it. It was tagged v0.0.1 on 2026-08-18, and the module also
carries v26.0.1-beta.

It is not adoptable yet and it is worth watching. Tried against these servers
it reached authentication on 11g, 19c and 26ai, so the protocol side works
across the range that go-ora v3 cannot, and it then refused the credentials
because its connection string is not the URL form the rest of this project
uses. That is a matter of reading its documentation rather than a fault, and it
was not chased further, because a v0.0.1 beta is not what a test suite should
depend on.

Revisit when it reaches a stable release. It is the only driver that is both
pure Go and Oracle's own, and if it reaches 11g it settles this decision and
hard rule 10's Oracle clause together.

#### When this ends

When v3 tags the fix. Then move Oracle back to v3, match `usql` again, and mark
this superseded. There is nothing else to decide at that point.

Pinning v3 at the master commit instead was considered and not taken. It would
keep the same package `usql` uses, and a pseudo-version is tolerable in a test
module in a way it is not in a shipped binary. But v2.9.0 is a tagged release
that reaches every server here, and choosing a tagged release over an untagged
commit needs no explanation later.

This amendment covers Oracle and nothing else. Every other database still uses
the driver `usql` ships, which D52 requires and this does not change.

### D60. The Oracle model reads ALL_ views, and there is no DBA_ variant. Decided.

Every Oracle query reads an `ALL_` view. Two read `DBA_`, because the fact is
in no `ALL_` view at all. Whether `dbmeta` must also offer a `DBA_` variant of
the rest is open, and this decision records the evidence rather than settling
it.

#### What the model does today

Prefer `ALL_`. Read `DBA_` only where the fact is not in an `ALL_` view, and
say so in the field or query documentation so a reader knows why a permission
error is possible. Do not reach for `DBA_` to get more rows out of a question
`ALL_` already answers.

`Roles` and `RoleGrants` are the two the rule is for. They are not absent from
Oracle: they live in `DBA_ROLES` and `DBA_ROLE_PRIVS`, and there is no `ALL_`
equivalent, because an ordinary user sees only its own roles through
`USER_ROLE_PRIVS` and `SESSION_ROLES`. Answering them at all means reading
`DBA_`. Neither is written yet, and the rule is what they will be written
against.

`V$` follows the same rule and already does. The version query reads
`v$version` because no dictionary view carries the banner before 18c, and
`Settings` will read `V$PARAMETER` for the same reason when it is written.

#### Why a DBA_ query returns an error rather than nothing

A query that is unsupported says the product has no such object. That would be
a lie here: Oracle has roles, and a connection with the privilege can list
them. Reporting `ErrNotSupported` would tell a caller to stop asking, when the
truthful answer is that this particular connection may not see it.

So the query exists, it reads `DBA_`, and an unprivileged caller gets
`ORA-00942: table or view does not exist` passed back. That is a fact about the
connection rather than about the database, and the caller is the one who can do
something about it.

This is the third behaviour this project has met for the same situation, and
they are worth keeping apart. PostgreSQL shows a caller everything. SQL Server
narrows the answer silently, which `models/sqlserver` documents. Oracle refuses
outright, and that refusal is the most useful of the three, because nothing is
hidden and nothing is guessed.

#### What was open, and what closed it

An `ALL_` view and its `DBA_` twin are the same question asked with two
different privileges, and the difference is invisible to the caller.

Measured on 26ai against `ALL_TAB_COLUMNS` and `DBA_TAB_COLUMNS`:

| Connection | ALL_ rows | DBA_ rows |
| --- | --- | --- |
| a full DBA | 2255 | 2255 |
| a `SELECT_CATALOG_ROLE` user | 180 | 2255 |
| a user with no grant on the schema | 0 | ORA-00942 |

The column sets are identical, 91 and 91. So the two differ in rows only, and
only for a caller who is not a DBA.

The third row is the problem. An `ALL_` query returns no rows where the caller
cannot see the schema, and no rows is also what an empty schema returns and
what a schema that does not exist returns. A caller cannot tell the three
apart. `DBA_` raises an error in the same case, which is a worse answer for a
caller who has no privilege and a better one for a caller who has it and typed
the name wrong.

`usql` often knows which case it is in, because it knows who connected. A
consumer that knows it is a DBA would rather ask `DBA_` and get the error.

Three mechanisms have been sketched and none chosen:

1. A keyed fragment, so the same query reads `DBA_` when the caller says so.
   This keeps one query and one column set, which rule 3 already asks for. It
   needs a gate that is not a version, and no such gate exists today.
2. An option on `Meta`, set when the caller is built. This puts the choice
   where the dialect and the versions already sit, and it makes the choice a
   property of the connection rather than of the call.
3. A second dialect, `oracle-dba`. This is the least code and the worst
   answer, because it doubles a model that is otherwise identical.

None was chosen, and D61 is the reason. The parity harness measured what a
principal actually gets, and an Oracle local user that owns the objects
receives the administrator's answer to every query. There was no gap to close
for the case that matters.

So the model stays `ALL_` only and nothing is added. What remains unanswered
is the third row of the table above: a caller asking about a schema it has no
grant on gets no rows, and cannot tell that from an empty schema or from one
that does not exist. `Tablespaces`, `Collations` and `Databases` stay
unsupported for the same reason, which `docs/COVERAGE.md` records.

Reopen this if a consumer asks for the difference. The three mechanisms above
are the candidates and the measurements are here.

### D61. Every dialect is measured against every principal the product has. Amended in place.

A dialect is not finished until every query has been asked as the
administrator and as each lesser kind of principal the product has, and the
differences are written down. `test/parity_test.go` does it and
`test/testdata/parity.txt` is the record.

#### Why

D60 asked whether Oracle should read `DBA_` views, and the argument rested on
`ALL_` showing a caller only what the caller may see. Nobody had asked whether
the other products do the same thing. They do, and one of them is worse than
Oracle.

Measured against the fixture schema, with every principal given the same
rights over it, so that the only thing varying is what kind of principal it
is:

| Database | Principal | Queries that answer differently |
| --- | --- | --- |
| Oracle 26ai | local user | none |
| SQL Server 2022 | contained user | none |
| SQL Server 2022 | server login | `roles` |
| PostgreSQL 18 | schema owner | `settings`, `tablespaces` |
| PostgreSQL 18 | grantee | `settings`, `tablespaces` |
| Cassandra 5.0 | granted role | `privileges`, `role_grants`, `roles`, `settings` |
| ClickHouse 26.9 | granted user | `constraints`, `databases`, `foreign_servers`, `index_columns`, `indexes`, `privileges`, `role_grants`, `roles`, `tablespaces` |
| MySQL 8.4 | grantee | `foreign_servers`, `functions`, `role_grants`, `roles`, `user_mappings` |
| MariaDB 13.0 | grantee | `aggregates`, `column_stats`, `foreign_servers`, `role_grants`, `roles`, `user_mappings` |

`current_user` and `current_schema` are left out of that table and are in the
file. They are supposed to differ, because they answer a question about the
connection, and a run where they agreed would be the fault.

One release needed a section of its own. PostgreSQL 12 grants public SELECT on
six columns of `pg_subscription` and not on `subsynccommit`, so an ordinary
role is refused `Subscriptions` there and served from 13 on. A section may
therefore be written `product@major`, and that one wins for a server reporting
that major. The query is not gated for it: a superuser on 12 reads the column,
and padding it would withhold a fact from the caller who is allowed it.

Cassandra behaves like the MySQL dialect and for the same reason: `roles`,
`role_grants` and `privileges` read `system_auth`, and `settings` reads
`system_views`, and a role with every permission on its own keyspace is
refused all four outright. It has no containment either, so a role belongs to
the cluster and a keyspace is only a grant scope.

Oracle is the cleanest of the five, which is the opposite of what D60 assumed.
A local user owning the objects gets the administrator's answer to every
query. The MySQL dialect is the worst: a user holding ALL PRIVILEGES on its
own database has queries refused outright, six on MariaDB and four on MySQL,
because they read `mysql.proc`, `mysql.column_stats`, `mysql.servers`,
`mysql.roles_mapping`, `mysql.role_edges` and `mysql.user`. Those are tables
in the `mysql` database rather than views that filter themselves, so the
server answers with error 1142 and the query fails. That is the same shape as
Oracle's `DBA_` problem and it was never recorded. PostgreSQL refuses
`tablespaces` on `pg_global` and hides parameters from `pg_settings`.

MariaDB and MySQL do not agree with each other, which is why the section is
named for the product rather than the dialect. `mysql.proc` was removed in
MySQL 8.0 and MariaDB still has it, so `aggregates` is refused on one and
answered on the other.

#### A principal is not one thing

SQL Server has three and they are not interchangeable. A sysadmin. A server
login mapped to a database user, which is the ordinary model. And a contained
database user, whose password is in the database and which has no login at the
server, which needs `CONTAINMENT` set to `PARTIAL`.

Oracle has the same three from 12c. `SYSTEM` is the administrator. A common
user exists in the container database and in every pluggable database at once,
which is what a server login is. A local user authenticates against one
pluggable database and has nothing above it, which is what a contained
database user is.

PostgreSQL has no containment, because a role belongs to the cluster rather
than to a database. The nearest three are the superuser, the owner of the
objects, and a role holding only grants.

MySQL and MariaDB have no containment either. A user is a name and a host at
server level and a database is only a grant scope, so there are two.

SQLite and DuckDB have no user, no role and no grant. There is no second
connection to make, so the rule does not reach them and cannot.

#### The rule

A dialect ships its queries, its fixture, its documentation and its parity
targets. Those are one deliverable and not four, the same way rule 9 makes the
fixture part of the queries. A dialect with queries and no parity target is not
nearly finished. It is one whose answers have been measured for exactly one
kind of user.

`TestEveryDialectIsMeasuredForParity` holds it. Every dialect must have a
target or an entry in `parityExempt` giving the reason it has none, and one
with neither fails. `TestPrivilegeParity` cannot do this job: it skips a target
whose server is not running, and it says nothing at all about a target that was
never written, so a dialect added without one would pass every test here.

Only three are exempt. SQLite and DuckDB have no user to be, and
`infoschema_over_postgres` is a test registration of the shared model over a
PostgreSQL server rather than a product.

Add a dialect, add its principals to `parityTargets` in
`test/parity_test.go`, run `go test -run TestPrivilegeParity -update`, and
read the diff. A product with a kind of principal that no target covers is not
finished. Say in `docs/COVERAGE.md` which queries differ and why, because a
consumer choosing a connection needs to know which answers depend on who is
asking.

A difference is not a failure. A principal with no privilege on another schema
has no business seeing it. The file records the set so that a change in the
set is what fails, the same way `conformance.txt` works.

#### Two principals nothing covers yet

An Oracle common user cannot be made from inside a pluggable database, and
every Oracle target now names a pluggable database. Covering it needs a
connection to `CDB$ROOT`, which `container/oracle.go` already records as a
target worth having and which is not written.

A SQL Server sysadmin that is not `sa` is not covered either. It would answer
the same as `sa` and nothing suggests otherwise, so it is not worth a target.

#### A scene can be newer than the server

A contained database arrived in SQL Server 2012. On 2008 R2 `sp_configure` has
no `contained database authentication` option and refuses the name, so the
contained scene cannot be prepared there at all. A scene therefore carries a
`min`, and a server older than it is skipped with the reason, the same way a
fixture step the server is too old for is skipped rather than refused.

A kind of principal that a release does not have is not a gap in coverage. It
is the product, and recording it as a skip says so where a reader sees it.

### D62. CQL cannot compute, so the Cassandra model computes in Scan. Decided.

A CQL statement selects columns and nothing else. There is no CASE, no
expression, no function that turns one value into another. Every other model
computes a field in the statement and this one cannot, so the Cassandra model
selects the raw catalog column and `Scan` maps it.

Four things follow, and each was measured against Cassandra 5.0.9 and 3.11.19
rather than read.

#### A derived field is derived in Go

`Column.Nullable` and `Column.PrimaryKey` are both read from
`system_schema.columns.kind`. A column of kind `partition_key` or `clustering`
is in the primary key, and the primary key is the only thing in Cassandra that
cannot be null. The statement selects `kind` twice, once under each name, so
the column count still matches the field count, and `Scan` turns each into its
boolean.

That is a deviation and it is confined. The invariant the tests enforce is
that a query returns as many columns as it declares fields, not that a column
maps to a field untouched. `Scan` is Go, it already exists on every binding,
and there is nowhere else for the work to go.

#### A filter cannot be optional, so no query filters

Every other model writes `(@schema IS NULL OR col LIKE @schema)`. CQL has no
`OR`, no `IS NULL` outside a materialized view definition, and a partition key
takes only `=` or `IN`. There is no `NOT IN` either, so the system keyspaces
cannot be excluded.

So every query returns every row. The filter parameters are still declared,
and every description says plainly that Cassandra ignores it. Declaring them
is what keeps a caller passing one from getting `ErrUnknownParam`, and `usql`
already narrows the result itself to match `psql`, so it gets the right output
from the whole one.

The alternative considered and rejected was declaring no parameters, which is
more honest and makes every consumer special case Cassandra. Over-returning is
the lesser fault: it never hides a row, and the cost is bounded because
`system_schema` is small. On the server this was written against it is 48
tables and 313 columns.

#### There is no order across partitions

CQL orders rows within one partition, by a clustering column. A result that
spans partitions arrives in token order, and the same query can return the
same rows in another order on another cluster. No query here writes `ORDER BY`,
because writing one would not make the answer ordered and would suggest it
was.

#### Padding is a type hint, and every alias is quoted

`NULL AS "name"` is refused: "Cannot infer type for term NULL in selection
clause (try using a cast to force a type)". The form that works is
`(text)NULL`, which is CQL's type hint, and it is what `docs/NULLS.md` asks
for: a fact Cassandra does not have is NULL and never a literal. A literal is
used only where it is true of every row the query returns, such as the `type`
of a row from `system_schema.tables`.

Every alias is quoted. `schema` is a reserved word and `SELECT keyspace_name
AS schema` is a syntax error, which cost an afternoon because the driver
reported an empty result rather than the error. `table`, `index`, `default`,
`set` and `primary` are reserved too. Quoting every alias sidesteps the whole
class, and it also stops CQL lower casing one, which it does to an unquoted
identifier.

#### The driver cannot report a null, so a pad is discarded

`(text)NULL` selects, and it does not arrive as a null. gocql decodes a null
of any type as the zero value of that type and go-cql-driver hands that to
`database/sql`, so scanning one into `sql.Null[string]` gives a valid empty
string. Verified directly: `SELECT (text)NULL` comes back `Valid` with `""`.

Left alone that makes every padded field look present and empty, which is
what `docs/NULLS.md` exists to prevent. It showed up in the conformance
projection as `has_default=true` on every column of a database that has no
defaults.

So a padded column is scanned into `pad`, a `sql.Scanner` that discards, and
the field keeps its zero value, which is the invalid Null. The column is still
selected, because the statement should say what it returns and because a query
returns as many columns as it declares fields.

This does not rescue a real catalog column that is null, and nothing can with
this driver. `docs/COVERAGE.md` says so.

#### A fixed filter is allowed, and 4.0 decides which one

An optional filter is impossible and a fixed one is not, which is how the
constraint queries return the primary key and not every column. The predicate
tests `position` rather than `kind`: a key column carries its position within
the partition key or the clustering key counting from zero, and everything
else carries -1.

`kind IN ('partition_key', 'clustering')` reads better and 3.11 refuses it,
with "IN predicates on non-primary-key columns (kind) is not yet supported",
because that arrived in 4.0. The two forms select the same rows on 5.0, 102 of
them, and no regular or static column has a position at or above zero on
either release. One form that works everywhere beats a fragment that makes the
same query mean two things on two releases.

### D63. Support says when a release is too old. Amends D54.

`Query.Support` has a fourth value, `TooOld`. It means the model is present,
the product has the object, and this release of it does not. `Query.Build` then
returns `ErrVersionTooOld`, as it always did.

#### What it replaces

D54 left this open. `Support` answered a question about the product and the
release was the error's business, so a query gated above the server reported
`Supported` and then refused to build. `TestWrongProductIsNotSupported` fixed
that on purpose and D54 recorded the doubt: a caller that trusts `Support`
walks into a query it cannot build.

Cassandra made it concrete. `Settings` reads `system_views`, which arrived in
4.0, so on 3.11 `Support` said yes and `SQL` said no. Every smoke test already
carried the same two step dance, catching `ErrVersionTooOld` after being told
the query was supported.

#### Why a fourth value rather than folding it into NotSupported

Because they are different answers and a caller acts differently on them.
`NotSupported` means stop asking: no upgrade changes it. `TooOld` means this
server cannot and a newer one can, which is something a person can act on.
Folding them together would lose that, and a consumer showing a person what it
can offer would say "this database does not have roles" when the truth is
"yours is too old".

#### The ordering

`TooOld` is last in the constant block rather than in order of how supported
each value is, so that `NotBuilt`, `NotSupported` and `Supported` keep their
numbers. They are compared by equality and never by rank, and nothing in the
tree orders them.

### D64. The Verified tier is checked against the document. Decided.

`TestEveryVerifiedReleaseIsDocumented` fails when a release the Go list calls
Verified is not named in `docs/COVERAGE.md`.

#### Why the tier needed anything

D40 gives three tiers and two of them have a machine behind them. CI runs
Tested and Nightly, and `TestWorkflowReadsTheList` fails when the workflow
and `container/container.go` disagree. Verified has nothing: it means a person
ran the release on a development machine, and no test can prove that.

What a test can prove is that the two lists agree, and that is the failure that
happens. Oracle 18c spent months connecting to the container database while
every other release connected to a pluggable one, and nothing noticed, because
nothing compared the list to the document.

#### What it does not do

It checks one direction. A release the Go list calls Verified has to appear in
the document, so adding one and not writing it down fails. It cannot check the
other direction, because the document says what a release was verified against
in prose a person wrote, and parsing that to find a claim with no release
behind it would be guessing.

It also cannot prove anybody ran anything, and it does not pretend to. Recording
a date and a checked-in log was considered and not taken: it is stronger
evidence and one more thing to keep current by hand, and the failure it would
catch is not the one that has happened.

### D65. A Windows machine rearms its evaluation before it expires. Decided.

`test/cmd/dbrun/oem/rearm.bat` runs at every startup as a scheduled task, reads the
grace period, and spends a rearm only when fewer than ten days are left.

#### Why not on every boot

The rearm count is finite, three on most of these editions, and it cannot be
reset. A machine that is started often would spend the whole budget in a week
and be no better off. Reading `GracePeriodRemaining` first turns that into one
rearm every 170 days.

#### Why it does not reboot

A rearm applies at the next start. `dbrun` starts a machine and waits for
SQL Server, and a reboot underneath that looks exactly like a machine that
failed to come up. There is more than a week of grace left when the rearm runs,
so the next ordinary start is soon enough.

#### When the rearms are gone

`C:\OEM\rearm.log` says so and nothing else happens. At that point the machine
is rebuilt, which is about an hour, or the release drops to Archived under D40
and nothing is claimed for it. Neither is automatic, because both are a
person's decision about how much a pre-2017 SQL Server is worth.

### D66. The order the remaining dialects are written in. Amended by D67 and D77.

Impala first, then ClickHouse, then the products that run in a container,
then the ones that need an account. A product that cannot be started cannot be
supported, and that decides the order more than anything about the product.

#### Impala first, because usql is waiting on it. D67 found this wrong

`usql` has five hand written metadata readers: postgres, mysql, oracle,
impala and informationschema. Four of them are reimplemented here. Impala is
the last, and until it exists `usql` cannot retire its own metadata package,
which is the point of this project. Nothing else on this list blocks anybody.

It runs in a container, `apache/impala`, so the usual rules apply to it.

That reasoning does not survive contact with the product. D67 has the
measurement: Impala has no queryable catalog and its usql reader is not SQL, so
it cannot move here at all, and ClickHouse is first instead.

#### Then ClickHouse, the one gap in the default build

`usql` builds clickhouse, csvq, duckdb, mysql, oracle, postgres, sqlite3 and
sqlserver by default. Every one of those has a model except ClickHouse and
csvq, and csvq reads files rather than a catalog.

ClickHouse has a real one. `system.tables`, `system.columns`,
`system.databases`, `system.functions`, `system.settings`,
`system.data_skipping_indices` and `system.dictionaries` carry the engine, the
partition key, the sorting key, the TTL and the column codecs, none of which
its `information_schema` emulation exposes. `clickhouse/clickhouse-server`
starts in seconds.

#### Then the products that run in a container

In this order, and the order is what the native catalog adds over
`information_schema`, not the size of the user base:

| | Product | Image | Why here |
| --- | --- | --- | --- |
| 3 | Trino | `trinodb/trino` | federated engine, wide use, connector and session metadata |
| 4 | Presto | `prestodb/presto` | probably a flavor key on the Trino model rather than a model |
| 5 | Vertica | `vertica/vertica-ce` | `v_catalog` is rich and nothing else reaches it. Blocked, see below |
| 6 | SAP HANA | `saplabs/hanaexpress` | enterprise install base, deep `SYS` catalog. Done, see D76 |
| 7 | Firebird | `firebirdsql/firebird` | the `RDB$` catalog answers more than most of this list. Done, see D74 |
| 8 | Exasol | `exasol/docker-db` | `EXA_` catalog, analytic install base. Blocked, see D77 |
| 9 | Hive | `apache/hive` | metastore, and it is the shape Impala already teaches. Done, see D78 |

#### Vertica cannot be started, measured 2026-09-26

The image in that table no longer exists. `vertica/vertica-ce` returns "object
not found" on Docker Hub, and there is no `vertica` namespace there at all.

Vertica's maintained image moved to OpenText and is `opentext/vertica-k8s`,
which is healthy: 91 tags, amd64, and 26.2.0-2 rebuilt three weeks before this
was written. It cannot be started outside Kubernetes without reimplementing
what the operator does. It declares no entrypoint and no command, it ships no
`admintools`, and it has no `dbadmin` user, because the operator creates the
user and drives `vcluster` itself. Pulling it and trying was how this was
found, which is what step 2 of `docs/EVALUATION.md` warns about.

The supported standalone path is the Oracle 19c pattern exactly. Vertica
publishes a Dockerfile, an entrypoint and a Makefile at
`vertica/vertica-containers/one-node-ce`, and the image is built from a
Community Edition RPM that a person downloads after registering at
vertica.com/try. Nothing here can fetch it, the same way nothing here can
fetch Oracle's archive, and D71's `dbrun build` already has the shape for it.

#### A Kubernetes VM was considered and is not the answer

The obvious next thought is to run the operator properly: a Talos Linux VM as a
single node cluster under KVM, install the VerticaDB operator, apply a
VerticaDB resource, and freeze the result the way D57 freezes the Windows
machines. Gemini and DeepSeek were both asked and both said no, plainly, and
the reason is better than the recommendation.

The stack is not a VM, it is five things. Kubernetes on Talos, cert-manager for
the operator's admission webhook, communal object storage because the operator
runs Vertica in Eon mode only and Eon needs S3, so MinIO as well, then the
operator, then the custom resource. All of that to read catalog tables.

The part that settles it is that the analogy to the Windows machines fails.
Those work as frozen baselines because a Windows machine has exactly one time
sensitive thing in it, the evaluation licence, and D65 handles that by rearming
at every boot. A Kubernetes cluster has many: etcd leases, node heartbeats, API
server certificates and service account tokens all expire while the snapshot
sits on disk. It boots and then needs a person. A frozen baseline that needs a
person is not frozen, and the whole value of the Verified tier is that a
release can be measured a year later without an archaeology session first.

Both models independently named the same third route, and it is route B: put
the Community Edition package on an ordinary machine and let `admintools`
create a single node database. That is exactly what `one-node-ce` does.

#### And then the download went away too

Checked on 2026-09-26, after Rocket Software took Vertica over from OpenText.
`vertica.com/try` answers 403. The community edition download page still
answers 200 and now serves OpenText's generic Information Management marketing
with no download on it. Rocket's own Vertica pages answer 403 from here, which
may be geography or bot filtering rather than absence, so that one is not
proven either way.

So route B is blocked as well, and not on a registration anybody can complete.

What does still work is an unmaintained third party image.
`saadmairaj/vertica:10.1.1-RHEL6`, published in 2021, starts cleanly on a
current host, creates its database, and answers with a complete `v_catalog`:

	Vertica Analytic Database v10.1.1-0

That is a real Vertica and the queries could be written against it. It is not
a release anybody runs, it is five years old, it is built by a stranger, and
nothing about it can be rebuilt or reproduced. `docs/EVALUATION.md` step 2
rejects it, and D40 forbids calling a version supported without naming its
tier, and there is no tier for "verified once against an unmaintained image of
a dead release". Writing a model on it would satisfy rule 9 in the letter and
not at all in the spirit: the queries would be verified against something no
consumer will ever connect to.

So Vertica waits until a current release can be started. It is not next.

The four products after it on this list all have live images that need no
account: `saplabs/hanaexpress` last rebuilt in November 2025,
`firebirdsql/firebird` and `apache/hive` rebuilt the day before this was
written, and `exasol/docker-db` two weeks before. Firebird is next.

Below those and worth a model only if somebody asks: Couchbase, Ignite,
VoltDB, YDB and Databend. Each runs the real engine in an image and none of
them is shaped much like the 55.

Avatica is not on the list at all. It is a wire protocol in front of whatever
database somebody put behind it, so it has no catalog of its own to read.

#### An emulator is not the same thing as a container

The first pass at this conflated two things and the distinction turned out to
be the whole answer.

Most of what looks cloud-only here is not a cloud service. Vertica CE, Exasol,
SAP HANA Express, YDB, Databend, ClickHouse, Trino, Presto, Hive, Impala,
Firebird, Couchbase, Ignite and VoltDB all ship the real engine in an image.
The catalog in the container is the catalog in production, and a query written
against one is a query that works against the other. Those are containers and
the ordinary rules apply.

The genuine cloud services are different, and their emulators do not carry a
catalog worth testing against. The Spanner emulator implements a basic
`INFORMATION_SCHEMA` with tables and columns and no roles, no privileges and
no change streams, so a metadata query passes there and fails in production.
The DynamoDB and Cosmos DB emulators have no SQL catalog at all, because
neither product has one: metadata is a control plane API call rather than a
table.

So an emulator never counts as a container for D40's purposes. A product whose
only local option is an emulator is in the same position as one with no local
option: it is last, and it is Archived on arrival. That this agrees with
excluding DynamoDB, Cosmos DB and Tablestore as non relational is a
coincidence worth noticing rather than the reason.

#### Last, the ones that need an account

Snowflake, BigQuery, Databricks, Athena, MaxCompute and Alibaba Tablestore
have no local emulator that both reviews agreed on. Both were asked whether
credits make them testable and both said the same thing: a free tier exists
for each, and using it from CI means pre-created credentials.

That is a different kind of dependency from a container and a worse one. A
secret in CI, an account that expires, a bill that can arrive, and a test that
fails for everybody when somebody else's trial ends. D40's tiers assume a
release can be started on demand, and none of these can.

So they are last, and a model for one of them is Archived on arrival unless
the account question is answered first. The two reviews disagreed about
MaxCompute and Tablestore, one calling them trial only and the other naming an
official emulator, which is a lead to run before either is scheduled.

#### What gets no model at all

Four are another driver for a product already here, and want a flavor key or
nothing: `pgx` is PostgreSQL, `mymysql` is MySQL, `moderncsqlite` is SQLite
and `godror` is Oracle. `netezza` is PostgreSQL derived and may be a flavor
key, which is a lead rather than a fact.

Three are not relational and the 55 do not apply: DynamoDB, Cosmos DB and
Tablestore are key value stores with no SQL catalog to read.

Five are not products: `adodb` and `odbc` are bridges to whatever sits behind
them, and `csvq`, `chai` and `ql` read files or embed, with no catalog beyond
what `information_schema` already covers.

#### The rule this follows

Order by whether it can be started, then by whether anybody is blocked, then
by what the native catalog adds. Not by popularity: Snowflake and BigQuery
would be near the top on user count and are last here, because a query that
has never run against a real server is not finished and neither of them can be
run on demand.

### D67. Impala cannot be a dbmeta model, and ClickHouse goes first. Amends D66.

Impala is off the list. It has no catalog a statement can read, and the reader
D66 wanted moved here does not use SQL, so hard rule 1 forbids moving it.
ClickHouse takes first place.

#### What the server said

A four container quickstart was stood up at 4.5.2 and asked directly. There is
no `information_schema` and no `sys` database: `SHOW DATABASES LIKE` returns
zero rows for both, and selecting from `information_schema.tables` is an error.
`SHOW` is a statement rather than a relation, so `SELECT * FROM (SHOW
DATABASES) t` does not parse either.

Metadata comes from `SHOW` and `DESCRIBE`, one statement per scope. So dbmeta
could answer three of the 55: `Schemas` from `SHOW DATABASES`, which returns a
name and a comment in one statement, `CurrentSchema` from `current_database()`
and `CurrentUser` from `user()`. `Tables` would need `SHOW TABLES IN` once per
database and `Columns` would need `DESCRIBE` once per table, which is the per
row round trip rule 13 forbids.

#### Why usql is not blocked after all

D66 put Impala first because `usql` could not retire its metadata package
until the last of its five readers moved here. That reader does not run a
statement. It calls `GetSchemas`, `GetTables` and `GetColumns` on the driver,
which are HiveServer2 metadata operations in the protocol rather than queries.

Hosting that here would mean importing the Impala driver into the root module,
and hard rule 1 forbids a database driver there outright. So the reader cannot
move, and it is already where it belongs: it is a property of the wire
protocol, which is the driver's business. `usql` keeps it and loses nothing.

#### The cost, for completeness

`apache/impala` publishes no whole server. It publishes components, and a
running Impala is four containers, a Hive Metastore, statestored, catalogd and
impalad, on a shared network with a warehouse volume. That is a harness on the
scale of the Windows machines in `test/vm`, in exchange for three queries.

A three query model is a small job if somebody ever wants `\dn` against
Impala, and it buys nothing today and needs the harness anyway.

#### What this changes

ClickHouse is first. It is one container that starts in seconds, it is the
only driver in usql's default build with no model, and `system.tables`,
`system.columns`, `system.databases`, `system.functions`, `system.settings`
and `system.data_skipping_indices` are a real catalog rather than an
`information_schema` emulation. The rest of D66's order stands.

#### The rule this adds

Before scheduling a dialect, check that the product has a catalog a single
statement can read. A product whose metadata is a protocol operation or a
`SHOW` per object cannot be a model here, however popular it is and however
well it runs in a container. D66 ordered by whether a product could be started
and that was one question short.

### D68. Every container is started by the runner and named product-release. Amended by D70.

Nobody reaches for podman or docker by hand. One command starts every
container this project uses, and every container is named
`<product>-<release>`, which is what `container.Server.Name` returns:
`postgres-18`, `clickhouse-26.9`, `oracle-26ai`.

#### Why it needed saying

Because the machine filled up with containers nobody could place. A session
debugging one thing left `ch268`, `pg12`, `pg96` and `chplain` behind, on ports
chosen by whoever typed the command, while the runner used its own names and
its own ports for the same releases. Two sets of the same servers, and the only
way to tell which was which was to read the image tag.

It is worse than untidy. `version` could not reach two servers that were
plainly running, because the port it computes is not the port somebody typed.
A container named for the release but started by hand is the confusing case,
not the obviously wrong one.

#### What the runner had to grow to make the rule keepable

A rule that cannot be followed is a rule that gets broken, and the reason
people went around it is that it only knew how to start a server, test it and
throw it away. It now does the things a person actually wants:

| | |
| --- | --- |
| `start` | start it and leave it running, then print its DSN |
| `stop` | stop it, keeping it so `start` resumes it |
| `remove` | stop and delete it |
| `status` | what is running, with a URL for each |
| `version` | connect and print what dbmeta reads, per server |
| `dsn` | the dburl style URL, running or not |
| `usql` | connect to it with usql |
| `all` | every server, spelled the way a person says it |

`help` lists them. It runs from anywhere, which the shell version did not:
it had to find its own directory to find anything, and `./test/run.sh --help`
failed with a path error. D70 is why that is no longer possible.

#### Two faults the rule exposed

The port a server gets was its index in the filtered list rather than in the
whole one, so every server started on its own got the first port. Two of them
collided, and the loser sat in Created state holding a name that
`podman rm --force` does not free. A port is now a server's place in
`container.All`, so a release always gets the same one and a URL a person
learned keeps working.

It also hid the runner's error behind "could not start", which is what made
that take an afternoon. It prints what the runner said, and for a name held in
storage it prints the one command that clears it.

### D69. The workflow builds its matrix from the Go list. Amends D42.

The CI workflow names no release, no image and no port. It asks the runner
for the list as JSON and expands it with `fromJSON`, and each job runs the
runner against one server.

#### What it replaces

D42 put the release matrix in `container/container.go` and had the workflow
repeat it in YAML, with a test failing when the two disagreed. That worked and it was a second copy: twelve jobs, one per product,
each with its own service block, its own image, its own health command and its
own environment. 690 lines.

Every one of those images was unqualified, which is its own fault. `mariadb:13.0`
resolves against whatever the runner's search list holds, so the same YAML
means one thing locally and another in CI, and `container/container.go` has
written every image in full for exactly that reason since it was created.

#### What it is now

Six jobs and 276 lines. A `releases` job reads the list and hands it on, a
`server` job runs the Tested tier, a `nightly` job runs the rest, and `unit`,
`embedded` and `compare` are unchanged. Each server job is three steps and the
last one is `go run ./cmd/dbrun test "${{ matrix.server }}"`, which is
the same entry point a person uses, which is what D68 asks for.

#### What the drift test became

There is nothing left to drift, so the test that compared two lists is gone.
Three take its place and they hold the property rather than the agreement: the
workflow must read both tiers from `dbrun list --json --names`, every image it
still names must carry its registry, and every image it still names must be one
`container.All` knows.

Only the comparison job names any, because it needs MariaDB and MySQL running
at once and the runner starts one server at a time.

#### What this does not change

The list is still `container/container.go` and it is still the only copy. D42
decided that and it stands. What changed is that the workflow reads it instead
of repeating it.

### D70. The runner is a Go command called dbrun. Amends D68 and D12.

`test/cmd/dbrun` starts every database this project tests against. It replaced
`test/run.sh`, and no shim was left behind: a shim that only execs the Go is
one more name for the same thing, and the workflow and the documents were the
only callers. `tool/servers`, `tool/vms` and `tool/version` are gone with it,
and so is `test/vm/provision.sh`.

`docs/RUNNER.md` is the design and this records what the design decided.

#### Why a language

The list of servers was already Go, in `container`. `tool/servers` printed it
with fields separated by U+001F, because an argument can contain a space, and
the shell read them back into arrays. That whole layer existed to carry a
`[]string` across a language boundary. `version` already ran a second Go
program because it needs a driver, and `test` ran `go test`. The shell was a
launcher for Go written in the one language where quoting a command is
something you can get wrong.

It is in the `test` module, because `version` opens a connection, which means
a driver, which hard rule 1 keeps out of the root module.

#### The command line, not a Go client

`DBMETA_RUNNER` names podman or docker outright. Otherwise podman is preferred
and docker is used when podman is absent, so the command works on a machine
with either and nobody has to say which.

A Go client library was considered and rejected. It would be a dependency for
something the two commands already do identically, it would have to speak two
socket protocols to cover both, and the places they differ are one line each:
`image exists` against `image inspect`, and `rm --storage`, which podman needs
and docker has no state for.

#### What folded in

Two images are built rather than pulled, and both were shell scripts.
Cassandra's published image refuses a user defined function, a materialized
view and a role, which three queries read. Oracle publishes no free 19c image
at all, only Dockerfiles and a three gigabyte installer archive that no command
can fetch from them, because their download needs an account and a browser
session. It is fetched from a mirror when it is not already on the machine,
and the SHA-256 Oracle publishes is what says the file is theirs. A copy that
fails the check is deleted, so a bad download is not kept. `start` and `test` build a missing image so that neither caller has to
remember, and `build` rebuilds one on demand. The Cassandra Containerfile is
embedded with `//go:embed`, so the command carries its own build input. What
stays shell is Oracle's own build script, which is theirs and which rewriting
here would mean owning a build we do not control.

`provision` folded in too, and that reverses the design's own recommendation.
It said to leave provisioning a script and exec it, because the interface is
what D68 is about and because changing the implementation costs an hour per
attempt. The second half was wrong. `--render` writes the OEM directory and
stops, so the templating is checked in a second, and the port was verified by
rendering all four releases both ways and diffing: twelve files, identical to
the byte. Then a machine was built from the Go and answered
`dbmeta.SQLServer.Version`. The hour is the Windows install, and the port does
not touch the Windows install.

The OEM payload is embedded. The script had to locate its own directory to
find it, which is the same fault that made `./test/run.sh --help` fail with a
path error. A binary that carries its payload has nothing to find.

#### The one bug the diff caught

2008 R2 wants the section header `[SQLSERVER2008]` rather than `[OPTIONS]`,
and `ConfigurationFile.ini` carries a comment above the header saying so. The
shell wrote `sed 's/^\[OPTIONS\]$/[SQLSERVER2008]/'`, anchored to a whole
line. The Go wrote `strings.Replace` with a count of one, which rewrote the
comment and left the header alone. It renders, it installs for forty minutes,
and it fails at the end.

A translation is not finished because it compiles. It is finished when its
output has been compared to the output of the thing it replaced.

#### One selector changed

The draft wrote `--all` for both "every release of this product" and "every
product". Two model reviews said the same thing about it, which is that one
word doing two jobs reads as one job until it does not. Widening a product is
now `--releases` and everything is the selector `all`. A selector is a noun
and a flag modifies it.

#### What this does not change

The release list is still `container/container.go` and still the only copy.
D42 decided that, D69 kept it, and this is a different program reading the
same list.

Nothing about what the tests do. This is how a database gets started, not what
is asked of it once it is up.

### D71. Nothing here is generated. The models are written. Amends D2, D12 and D30, supersedes D11.

No generator runs in this repository. There is no `tool` directive in either
`go.mod`, no `go:generate` anywhere, and no file carries a generated header.
Every line under `models/` was written, and the queries in it were written by
an agent working against a running server, checking each statement as it went.

`dbtpl` generates nothing here and never did. It is a consumer of `dbmeta`,
the same as `usql`, and that is the only relationship between the two projects.

#### Why this needed a decision of its own

Because the plan said otherwise in three places and the instructions repeated
it. D2 said `dbtpl` generates the model code. D11 pinned `dbtpl` with the
`tool` directive so that two agents on two machines would produce the same Go
from the same SQL. D30 corrected half of it, saying `dbtpl` is not used, and
then put generation in a sub-package that was never built.

`CLAUDE.md` carried the consequence. It told a reader that `models/<driver>`
holds generated files, that they must not be edited, and that a change goes
into the SQL and is generated again through `go tool dbtpl`. All three are
wrong, and the first two are worse than wrong: they tell somebody not to touch
the only files there are to touch.

`docs/EVALUATION.md` carried it too, requiring a pinned image digest beside
each model so that generation would be reproducible. Nothing is reproduced,
so nothing needs the digest.

#### What replaces it

A model is written, read and edited like any other Go. A query is written
against a live server, `dbrun` starts that server, and rule 9 makes the fixture
part of the model rather than something a generator would emit.

What D2 decided about layout stands. One package per driver under `models/`,
one package covering every supported release, with the version differences held
as data inside it, which is D8. Only the claim that a tool produced it is gone.

What D11 reasoned about build dependencies stands too, and it is D26 that
carries it: a driver does not belong in the root module. There is simply no
build dependency left to place.

D6a went the same way and is marked superseded by this. It put the NULL scan
fix in a generator's flags. The fix is in the code, the rule is D6 and
`docs/NULLS.md`, and `TestNoCoalesceOnCatalogColumns` guards it.

#### What this does not mean

It is not a rule against generating code here later. If a generator is written,
it will be ordinary Go in this repository, it will mark what it emits, and it
will get a decision of its own. Until then, a file under `models/` is a file
somebody wrote, and the honest thing is to say so.

### D72. Trino reads system.jdbc, and a catalog is a real level. Decided.

Three things had to be decided for `models/trino` rather than found by running
statements. The rest of what it can and cannot answer is measurement and lives
in `COVERAGE.md`.

#### The catalog is a real level, and this is the first model to use it

Every other model returns an empty catalog or repeats the database name into
it, because the products have two levels of namespace and `psql` has three.
Trino has all three: a table is `catalog.schema.name`, a catalog is a
configured connector rather than a database, and one server reaches many at
once.

So `models/trino` is the first to answer a catalog filter. `dbmeta.Args` has
carried the field since it was written and nothing had used it. A consumer
that passes `Args{Catalog: "memory"}` to any other model gets every catalog,
because there is only one; passing it to this one narrows.

`Databases` reads `system.metadata.catalogs`, so `\l` lists the catalogs a
server can reach. That is the closest thing Trino has to the question `psql`
asks, and it is more useful than reporting nothing.

#### The source is system.jdbc, not the per catalog information_schema

Trino ships an `information_schema` inside every catalog and a `system`
catalog beside them. They differ in reach, and the difference decides this.

A query against `memory.information_schema.tables` sees the memory catalog
alone. The catalog cannot come from a bind parameter, because it is an
identifier in the `FROM` clause, so a filter naming a second catalog would
return no rows rather than an answer. That is a wrong answer wearing the
clothes of an empty one, which rule 13 does not allow. `system.jdbc` spans
every catalog the server has.

It is also richer. `system.jdbc.columns` carries the column comment in
`remarks`, and `information_schema.columns` has no column for a comment.

Views is the exception and it has to be. No cross catalog source holds a view
definition: `system.metadata.materialized_views` covers materialized views
only, and a plain view's definition lives in its own catalog's
`information_schema.views`. Reading every catalog would be one statement per
catalog, which rule 13 forbids. So Views reads the session catalog, and its
parameter description says so where a caller reads it rather than in a
document they will not open.

#### A boolean is not an integer

Every other model writes `@with_system = 1` and the server coerces. Trino
applies no implicit conversion between a boolean and an integer and refuses
the statement outright:

```
Cannot apply operator: boolean = integer
```

Seven queries were written the other way first and all seven failed on the
first run against a server, which is the cheapest way for that to be found and
the reason rule 9 exists.

#### What this does not decide

The floor. `container/trino.go` holds it with the measurement behind it, and
476 is set by what the memory connector can build rather than by what the
catalog can answer.

Whether `dbtpl` could generate from Trino. It could not, and `DBTPL.md` says
why: no foreign key means no relationship to follow. That is a fact about the
product rather than a decision about the model.

### D73. Presto is its own dialect, and not a flavor of Trino. Decided.

`models/presto` is a model of its own. Presto and Trino do not share a dialect
the way MariaDB and MySQL do.

D66 guessed the other way, saying Presto was "probably a flavor key on the
Trino model rather than a model". That guess was made before either existed.
The measurement changed it.

#### What they share

`system.jdbc` exactly: all thirteen tables, the same columns, the same
meanings. The same SQL dialect, the same `array_agg` and `array_join`, the
same strict typing that refuses `boolean = integer`, and the same five
catalogs in the official image including a writable `memory`. They are the
same program forked in 2019, when the original authors left Facebook and
renamed PrestoSQL to Trino, and it shows.

#### What they do not

Measured on Trino 483 and Presto 0.299, which was the newest of each.

| | Trino | Presto |
| --- | --- | --- |
| `version()` | answers | not registered |
| `node_version` | `483` | `0.299-7d50721` |
| `system.metadata.table_comments` | present | absent, and no table comment is readable at all |
| `current_catalog`, `current_schema` | both resolve | neither resolves |
| `system.jdbc.columns.remarks` | carries the comment | always NULL |
| `information_schema.columns` | 8 columns | 13, including `comment` |
| `memory` with `NOT NULL` | from 476 | refused |
| role views on `memory` | answer nothing | raise `NOT_SUPPORTED` |
| Go driver | `trinodb/trino-go-client` | `prestodb/presto-go-client` |
| DSN the driver takes | `http://` with query parameters | `presto://` with a path |
| `CREATE CATALOG` | in the grammar | not |

#### Why that means two dialects and not one with gates

The version query settles it on its own. A dialect registers one
`VersionQuery`, and Trino's `version()` does not exist on Presto. The
numbering cannot be compared either: Presto is `0.299` and Trino is `483`, so
there is no version to gate on, only a product to branch on.

A fragment that branches on which product it is talking to is not a version
fragment. Two models asked independently and both said the same thing. Gemini
put it best: once you are branching on engine identity rather than engine
version, you no longer have one dialect with versioned fragments, you have two
dialects sharing a struct. DeepSeek added the maintenance case, that a reader
would have to hold both products in their head at every object.

MariaDB and MySQL are not the counter example they look like. They share
decades rather than six years, their catalogs still agree on nearly
everything, and they use the same Go driver. The fragments there are genuinely
about version, which is what D44 is for.

#### What Presto answers

9 of the 55, against Trino's 13. The four it does not are absences in the
product rather than gaps in the model. Comments has no source. CurrentSchema
has no expression. Roles and RoleGrants read the standard views and the memory
connector raises rather than answering nothing, and D34 says a query dbmeta
offers must run, so they are not offered.

Its conformance section is its own, and that is the second reason a shared
dialect would not have worked. Presto's `memory` connector refuses `NOT NULL`
on the newest release there is, so every column in its fixture is nullable
where Trino's is not. One section cannot describe both.

#### What this does not decide

Whether a future product that forks from one of them is a flavor. The test is
the one applied here: if telling the two apart needs a product branch rather
than a version gate, they are two dialects.

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

## How a model is written

Nothing generates one. D71 has the whole of it: no `tool` directive, no
`go:generate`, no generated header, and no `dbtpl` in the loop. This section
once described `dbtpl query` and its flags, and following it produced nothing
because none of it was ever wired up.

A model is written against a running server. Start one with `dbrun`, write the
statement, run it, read the columns back, and declare the fields to match. The
NULL rules in `docs/NULLS.md` are the part that takes the care: a field gets
`sql.Null[T]` whenever the column can be absent, and a release with no source
for a column selects `NULL AS "name"` rather than a literal.

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
D34 settled this in favor of a capability report.

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

Half of this objection is already answered. Containers are started by `dbrun`,
which drives the podman or docker command line, so no Go container library is
needed. See D70. The `usql` tests use `ory/dockertest` only because they predate that
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

- `describe.c` is 7400 lines.
- It holds 49 entry points. They are the `describe*`, `list*`,
  `permissionsList` and `objectDescription` functions.
- It holds 68 version gates of the form `pset.sversion >= 110000`. They span
  release 11 to release 19.

These numbers moved when the checkout was updated, and one of the moves is not
a drift. Release 20 removed every `psql` code path for a server below release
10, in commit `831bec45924` on 2026-07-02. The gates below 11 are gone from the
current source. `QUERIES.md` part 2 explains what that does to D20, and the
short version is that the current tree can no longer tell you what release 9.6
needs.

The version gates are the work, not a detail. `psql` writes one query and
switches fragments on the integer server version. D8 requires one model per
version instead. Each gate you meet becomes a decision about which versions get
their own model.

The local checkout at `/home/ken/src/postgres` sits at
`REL_19_BETA1-1062-gd9de60c5e47` on `master`, which is release 20 under
development. The installed client is 18.6.

One checkout is not enough. D20 sets the floor at 9.6, and the current tree has
no code for anything below release 10. Translating the older releases means
checking out a release 15 or older tree and reading `describe.c` there.

Record which tree each fragment came from, beside the fragment. A reader who
cannot tell which source a gate was translated from cannot check it.

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

Expect gaps. `information_schema` does not describe most of the objects that
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
both here. Start the databases with `dbrun`, which D68 requires, and compare
with `reflect.DeepEqual` or with a written comparison. Do not copy the `usql` test harness.

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
`github.com/xo/dburl` the place the taxonomy lives, so read it from there
rather than copying it here. D19 records why `dbmeta` does not import it.
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
2. Pin the image digest, not only the tag. A tag moves, and a test that means
   to meet one release must not quietly meet another. Nothing here needs a
   digest to reproduce a file, because nothing here is produced. See D71.
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

### D74. Firebird has no schemas, and none is invented. Decided.

`models/firebird` answers 24 of the 55 against Firebird 3.0, 4.0 and 5.0.
`docs/COVERAGE.md` holds the measurements. Three things had to be decided
rather than discovered, and they are here.

#### Schemas report NotSupported, and every object returns an empty one

Firebird 3.0 through 5.0 has no schemas. Every object lives in one namespace
and names are unique across the database. Firebird 6.0 adds SQL schemas and is
out of range.

The alternative was to invent one. SQLite reports `main` and ClickHouse
reports the database name, so there was a shape to copy, and a consumer that
wants to qualify a name has an easier time with something in the field.

It is rejected. `main` is a name SQLite itself uses and a ClickHouse database
is a real namespace, so neither model is inventing anything. Firebird has
nothing to report, and a value put there would be indistinguishable from a
real schema to a caller that cannot see the server. D34 already says an empty
result must never stand in for `NotSupported`, and the converse holds as
firmly: a fabricated row must never stand in for an absent level.

So `Schemas` and `CurrentSchema` report `NotSupported`, every other query
returns an empty schema with a field description saying why, and
`TestFirebirdSchemasAreNotSupported` holds both halves.

The cost is one of the nine reads `dbtpl` makes, which `docs/DBTPL.md`
records. A generator would have to be told there is no schema to qualify by.
That is a true statement about Firebird and the honest thing for it to be told.

#### Roles keeps SEC$USERS, although the driver has a fault around it

Firebird splits what PostgreSQL keeps in one place. `SEC$USERS` holds users
and belongs to the server, `RDB$ROLES` holds roles and belongs to the
database, and `Roles` is one statement over both with `can_login` separating
them.

Keeping `SEC$USERS` was a decision because of what it costs. After a
`CREATE USER` on a connection, `nakagami/firebirdsql` answers a later read of
`SEC$USERS` on that same connection with EOF: the server drops the attachment
and the pool's next connection fails its handshake, so every query after it
reports a protocol error. A few statements in between make it reliable, which
is why it looked intermittent until it was pinned down. The parity test
reproduced it four times out of four and recorded between 18 and 22 differing
queries where the truth is three.

Dropping `SEC$USERS` would remove the fault and make the parity record stable
at once, which was measured: the same run then reported three differences
every time.

It is rejected, because the fault cannot reach a consumer. `dbmeta` issues no
user management statement at any time and the one statement it is allowed to
build, `Dialect.ChangePassword`, returns text and runs nothing. Only a caller
that runs `CREATE USER` itself, on the same connection it then reads metadata
on, can trigger it. The parity test is exactly such a caller, so it runs every
user management statement on a connection of its own and closes it, and the
record is stable with `SEC$USERS` in place.

D52 says a query that fails on the driver `usql` ships is a query that does
not work. This one does not fail on that driver. It fails after a statement
`usql` would have to be asked to run, and the reason is written down in
`docs/COVERAGE.md` so that the next person to see EOF from Firebird knows
within a minute what it is.

#### Languages stays unsupported, as a stretch

`RDB$FUNCTIONS.RDB$ENGINE_NAME` and the same column on `RDB$PROCEDURES` name
the external engine a routine is written for, so the engines in use are
derivable in one statement.

That is a list of languages in use and not a catalog of languages installed,
and Firebird has no catalog of the second. An engine that is installed and
unused would be missing and a caller could not tell which kind of answer it
had. Rule 14 says to leave an analogue that is a stretch unsupported and to
record the reason, and this is that case. The fact itself is not lost:
`Function.Language` carries it per routine, which is where Firebird records it.

#### What the measurement gave back

Firebird's conformance section is identical to PostgreSQL's except for three
lines, and on all three Firebird agrees with the other eight databases while
PostgreSQL is the outlier, because its fixture declares the keys `serial`.
That is the closest any model has come to the reference, and it is worth
saying after two query engines that could answer neither a constraint nor an
index.

### D75. Every container is bounded, and four run at once. Decided.

`container.MemoryLimit` is `4g` and every container this project starts is
given it. `dbrun` starts a fifth server by stopping the one that has been
running longest, and says which.

Neither existed before and the machine showed why. A session that starts a
server per question ends with a dozen up, all idle. Thirteen were running at
once on the development machine, including two query engines and a Windows
machine, and a database given the whole host will take it: SAP HANA alone had
taken 3.1g and Oracle 2.3g while neither was being used.

#### The limit is one value, the way the password is

`MemoryLimit` is a constant rather than a field with a default, for the same
reason `Password` is. These are throwaway servers holding fixture data, none
of them is doing real work, and a per product number is a thing to tune rather
than a thing to read.

One product does not fit and it carries the exception on `Server.Memory`. An
exception is allowed only where the product was measured and the number is
written down beside it. SAP HANA is the only one, at `8g`, and
`container/hana.go` holds the measurement.

#### Why the eviction rather than a refusal

The alternative was to refuse the fifth start and name what to stop. It is
rejected because `dbrun start all` and `dbrun test all` both walk the list,
and a refusal turns either of those into an error on the fifth server rather
than into a run.

Evicting the longest running is the right one to lose. The server somebody is
using is the one most recently started, rebuilding a container is a minute,
and the line says what happened rather than leaving a server mysteriously
down. A Windows machine is never evicted: D57 keeps one because rebuilding it
is an hour.

#### What it cost to get right

Two fields on `container.Server` that no product needed before SAP HANA, and
both are data rather than behavior. `RunFlags` goes before the image and
`Args` after it, because HANA's entrypoint takes the initial password and the
licence agreement as command line arguments and reads no environment variable
for either. `Startup` is a third, because HANA answers in 108 seconds where
every other product here answers within 90.

The first attempt set `--ulimit nofile=1048576:1048576`, which rootless podman
refuses outright: the host's hard limit is 524288 and a container already gets
it. The flag was both impossible and unnecessary, which is the sort of thing
only running it finds.

### D76. SAP HANA reads SYS, and answers more than anything but PostgreSQL. Decided.

`models/hana` answers 32 of the 55 against SAP HANA 2.0 SPS 08.
`docs/COVERAGE.md` holds the measurements. Four things had to be decided.

#### Ken agreed to the SAP licence

SAP HANA, express edition will not start without `--agree-to-sap-license`,
which accepts the SAP Developer Center Software Developer License Agreement.
Ken agreed to it on 2026-09-26 for this project's test containers, and the
flag is in `container/hana.go` with that recorded beside it.

It is written down because it is the only product here that needs an
affirmative licence acceptance to run at all, and because the next person to
read that flag should not have to wonder who decided.

#### Access methods are the row store and the column store

A HANA table is held by row or by column and the choice is per table. That is
the question a MySQL storage engine and a Trino connector answer, so
`AccessMethods` reports the two and `Tables` says which one each table uses,
reporting row table or column table rather than table.

It is an analogy and rule 14 says to leave one that is a stretch unsupported,
so the case for this one has to be made. It is not a stretch: the choice
decides how the table is stored, how it is scanned and what it is good for,
which is what an access method is. What makes it imperfect is that HANA keeps
no catalog of the kinds, so the query counts the tables that name each one and
a kind nothing uses does not appear. The field description says so.

The fixture builds a row table as well as a column table so that the answer is
never trivially one row, and `TestHANARowAndColumnStore` reads both.

#### Subscriptions answers and Publications does not

This looks like an oversight and it is the product. HANA replicates by
subscribing to a remote source, so `SYS.REMOTE_SUBSCRIPTIONS` is the
subscriber half and there is no publisher object anywhere in the catalog.
Firebird is the other way round: it publishes and configures the subscriber
in a file. Recording both halves separately is why the two kinds are separate
kinds.

#### The column grant column stays, and is always empty

`SYS.GRANTED_PRIVILEGES` has a `COLUMN_NAME` column and HANA 2.0 SPS 08 has
no `GRANT` syntax that fills it. All three spellings of a column list are a
syntax error, which was measured rather than read.

The decision is to keep reading the column rather than to drop it and hard
code an empty string. The catalog has it, a later release may fill it, and a
query that reads a column it cannot demonstrate is exactly the thing that rots
silently. So `TestHANAHasNoColumnGrant` asserts both halves: that the grant is
still refused, and that `column_access` is still empty. If SAP adds the
syntax, that test fails and tells somebody to look.

#### What the measurement gave back

Eleven queries answer differently for a grantee, which is the most of any
product here, and four of those return the same rows with different values
rather than fewer rows. Functions, sequences, triggers and views all carry a
definition, and HANA returns the row and withholds the text from a reader
without the privilege. A consumer that treats a definition as always present
is wrong on HANA, and nothing but D61 would have found it.

### D77. Exasol will not run here, and Hive goes ahead of it. Amends D66.

Exasol is number 8 in D66's order and Hive is number 9. Hive goes first,
because Exasol does not start and four separate things had to be got past
before that became clear.

There is no `models/exasol`, no dialect constant and no container entry. A
constant with no model and a container entry that cannot start are both worse
than nothing: they claim something this project cannot do. What was learned is
here instead, so that whoever tries again starts from the fourth problem
rather than the first.

#### What was got past

Each of these is a real fix and each one revealed the next. Measured on
`exasol/docker-db:2026.1.2`, rootless podman, on 2026-09-26.

The container needs a bridge network. Exasol picks its own address by looking
for the first interface whose state is UP, and rootless podman's default
networking gives an interface whose state is UNKNOWN, so initialization fails
before anything else happens:

	exadt:: searching for the first interface with state UP
	IndexError: list index out of range

The container needs to be privileged, which Ken granted on 2026-09-26. The
image's own README says so: privileged mode is required for permissions
management, UDF support and environment configuration. `--cap-add SYS_ADMIN`
alone gets past `sethostname`, which is the first thing to fail, and then
`bucketfsd` restarts forever with "Master authentication service rejected
authentication: Unauthenticated". With `--privileged` the initialization runs
to "All stages finished", which it never does otherwise.

The container needs a longer readiness budget than anything but SAP HANA. It
builds a single node cluster on first start.

#### What it did not get past

The database process starts, runs for about three minutes and aborts:

	*** Exception caught in init of ObjectMgmt:
	    ObjectClient: Invalid hash value ***

Then the controller shuts down cleanly and the container stays up with nothing
listening on 8563, so the symptom a caller sees is a readiness timeout rather
than an error. That is the worst shape a failure can have and it is why this
took as long as it did.

Two hypotheses were tested and both were wrong. Memory is not it: the error is
identical at 4g and at 8g, and the container was using 357MB when it failed.
The password is not it either: `init-sc` has an `--encode-passwd` flag and its
sibling `--root-passwd` documents that a password is expected already encoded,
so passing the password as cleartext looked like exactly what "Invalid hash
value" would say. Encoding it changes nothing.

The evidence points at storage and that is where the next person should start.
Exasol's device is a 6GB file at `/exa/data/storage/dev.1` and it sits on
overlayfs. The image's README says the host must support O_DIRECT, which
overlayfs does not, and `--no-odirect` is already passed. "Invalid hash value"
reads as an object checksum failure against that device rather than anything
to do with a password.

So the next thing to try is a real volume for `/exa`, which the README
documents under managing disks and devices. That needs a volume field on
`container.Server` and a host directory for `dbrun` to create and remove,
which is machinery no other product here needs.

#### Why that was not tried

Judgement rather than difficulty. Exasol had by then cost more than the whole
Firebird model did, including its queries, fixture, tests, conformance, parity
and documentation. Four gates were already behind it and the fifth needed a
new field in a shared package.

Hive needs none of it. It is Apache 2.0, multiarch, was rebuilt the day before
this was written, and asks for no licence, no privileged container and no
special networking. Taking the cheap one first is the same reasoning D66
already uses to put the products that run in a container ahead of the ones
that need an account.

Exasol is not struck the way Impala was in D67. Impala cannot be a model
because it has no queryable catalog. Exasol has `EXA_` and there is every
reason to think the queries would be good. It is blocked on starting the
server, which is a different thing and may take one volume mount to fix.

### D78. Hive reads sys, and is a model. Decided.

`models/hive` answers 16 of the 55 against Apache Hive 4.2.1.
`docs/COVERAGE.md` holds the measurements.

An earlier version of this decision said Hive could not be a model. That was
right about the evidence at the time and wrong about the conclusion, and
both halves of what changed it came from outside this project.

#### Hive is not Impala, which is what D66 left open

D66 put Hive last with the note that it is the shape Impala already teaches,
and D67 struck Impala because it answers only through `SHOW` and `DESCRIBE`,
which are statements rather than relations.

Hive passes that test. Hive 3.0 added a `sys` database that exposes the
metastore as 57 external tables over the JDBC storage handler, and they
answer ordinary SQL.

`sys` is not there when a server starts. The script that creates it ships in
the image and needs a running HiveServer2, because the tables are external
tables pointed at the metastore. That is neither an image layer nor a
fixture, so `container.Server.Init` was added: a command run once the server
answers and before anything reads it. It has to be safe to run twice,
because `dbrun start` runs it every time, and Hive's script is, because
every statement in it is CREATE IF NOT EXISTS or CREATE OR REPLACE.

#### The protocol cannot bind, and the driver had to change

`TExecuteStatementReq`, the Thrift request that carries a statement to
HiveServer2, has five fields and none of them is parameters:

	SessionHandle  Statement  ConfOverlay  RunAsync  QueryTimeout

So HiveServer2 cannot bind server side at all, and Hive's own JDBC
`PreparedStatement` substitutes on the client. No Go driver can offer real
binding. Saying it that way matters, because "the driver is broken" invites
somebody to go looking for a better driver and this does not. The `usql`
session found this by reading the protocol after reading the driver, which
is the check that pays: read what the driver is a client of.

`sqlflow.org/gohive`, which `usql` shipped, had two defects on top of that
and either one alone rules it out.

It accepted bind parameters and discarded them. `args` is a parameter of its
`execute` and appears nowhere in the body, and `NumInput` panics with "not
implemented", so a statement reached Hive with its question marks still in
it and came back as a Thrift frame size error rather than a refusal.

Worse, it could not represent NULL. Measured:

	SELECT CAST(NULL AS string)   valid=true  ""
	SELECT ''                     valid=true  ""
	SELECT CAST(NULL AS bigint)   valid=true  0

A NULL and an empty string were the same value, and a NULL and a zero were
the same value. Every nullable field in a model built on it would have been
a lie, silently, and nothing about the model would have looked wrong.
`docs/NULLS.md` is the shortest document here and the one that cost the most
to learn, and that driver breaks all of it.

`usql` is replacing it with `github.com/beltran/gohive/v2`, which Ken
confirmed. v2 refuses parameters with a message instead of mangling them,
and it tells NULL from empty. `TestHiveTellsNullFromEmpty` asserts the
second, because a driver change that regressed it would leave no other
trace.

One trap the `usql` session recorded and this keeps: `beltran/gohive` v1 has
no `database/sql` driver at all, only a Connect and Cursor client, and it
was already in the module graph as an indirect dependency. It looks like a
candidate and is not. The driver arrived in v2.

Two things about v2 are worth knowing before anybody debugs it. It panics
rather than returning an error when the auth mode is missing or unknown, so
`auth=NONE` is not optional. And it requires the DSN to keep its `hive://`
scheme.

#### What dburl had to change, and what it found

`dburl` generated the Hive DSN with `GenFromURL("truncate://localhost:10000/")`,
which strips the scheme and emits `localhost:10000/default`, and
`beltran/gohive/v2` rejects anything that does not begin with `hive://`. The
requirement was recorded here first, while the `dburl` session was not
running, and that session has since shipped it as its D16: the scheme is kept,
the `hive2` alias normalizes to `hive`, and `auth=NONE` is defaulted.

The `auth` default is not tidiness and it came out of a form table run
against the server here. The driver panics rather than returning an error
when `auth` is missing:

	hive://hive:pw@host:10000/default                 panic: Unrecognized auth
	hive://hive:pw@host:10000/default?auth=NONE       connects
	hive://hive:pw@host:10000/default?auth=CUSTOM     connects

`dburl` v0.28.0 emitted exactly the first shape, so every caller was one
`Open` from a crashed process. That is worth keeping here because it is the
clearest case yet for the rule the `usql` session wrote into its driver gate:
the check that pays is not reading the driver, it is reading what the driver
is a client of, and then asking what it does with nothing rather than with
something wrong.

`transport` behaves differently from `auth` and the difference matters.
Measured on 4.2.1:

	no transport option                connects
	transport=binary                   connects
	transport=http                     fails cleanly, the server is binary
	transport=nonsense                 panic: Unrecognized transport mode

So an absent `transport` is safe where an absent `auth` is not, and only a
wrong value panics. Nothing needs to default it.

The `dburl` session then found the mechanism, which turns the distinction
from a judgement into something checkable. `ParseDSN` fills in
`TransportMode: "binary"` and `Service: "hive"` when it builds its struct
and does not fill in `Auth`, so `Auth` reaches the connect path as the empty
string that panics while the other two arrive with working values. The form
table and that literal say the same thing from opposite ends.

There is a second trap one layer in, which that session hit while writing
the test for its own fix. `?auth=` and a bare `?auth` both parse to an empty
string, so a mechanism that overrides per key regardless of value hands back
exactly the value that panics, and the spelling most likely to be typed by
somebody trying to clear the option is the one that breaks. Checking what a
driver does with nothing is not enough on its own: whatever supplies the
default has to be able to tell nothing from empty.

Two facts the `dburl` session measured from the source, recorded so that
nobody re-measures them: the driver supplies port 10000 itself when the DSN
omits it, and TLS is selected by the `sslcert` and `sslkey` options together
rather than by a scheme suffix, so there is no `s` alias.

`container/hive.go` writes its own DSN and was never blocked on any of this.
Hard rule 1 keeps dbmeta out of dburl's taxonomy, so none of it is work for
this project.

### D79. A dialect that cannot bind renders its values. Decided.

[dbmeta.Info.Literal] renders a parameter value as a SQL literal. A dialect
that sets it gets its statements with the values in them and no bind
arguments. Apache Hive is the only one, because it is the only product here
whose protocol has no parameter channel at all. See D78.

Ken decided this. The alternative was to leave Hive unsupported, which is
where D78 stood before.

#### Why it is safe enough, and what that argument does not cover

The statements are written in this repository. No caller supplies one, and
nothing a caller passes becomes part of the statement's structure. What gets
rendered is a filter value for a parameter this project declared.

The value does come from outside. In `usql` it is a pattern somebody typed
and in `dbtpl` it is a schema name from a configuration, and in both the
person supplying it already has full SQL access through the same session, so
there is no privilege boundary for an injection to cross. That is Ken's
argument and it holds for both consumers.

It does not hold for every consumer. `dbmeta` is a library, and something
that put an untrusted name into a filter and ran it against Hive would have
a boundary to cross. So the escaping is written as though it mattered,
because for somebody it will:

`TestLiteral` checks the break out shapes directly, and
`TestHiveEscapingHoldsOnTheServer` asks a real Hive for tables named
`x' OR '1'='1` and three others, and fails if any of them matches more than
nothing. A unit test can show the string looks right. Only the server shows
what it means.

#### The dialect supplies the function rather than setting a flag

The first design was a boolean and a shared escaper. Hive killed it.

[dbmeta.QuoteLiteral] doubles the quote, which is the standard's rule and
right for every other product here. Hive does not accept a doubled quote.
Measured on 4.2.1:

	SELECT 'a''b'  ->  ab
	SELECT 'a\'b'  ->  a'b

The first is read as two literals written next to each other and joined, so
a doubled quote loses the quote and returns a wrong answer rather than an
error. Hive needs C style backslash escapes.

So escaping is per product knowledge and cannot be a flag, which is the same
conclusion D56 reached for `ChangePassword`: the escaping is the product's
and the value cannot be bound. This is the second instance of that rule
rather than a new exception to anything.

#### What it refuses

An implementation returns [dbmeta.ErrInvalidParam] rather than guessing. The
Hive one refuses a type it does not know, because every parameter this
project declares is a string or a boolean and an unknown type means a caller
passed something a query did not declare. It refuses a NUL for the reason
`ChangePassword` does: it can end a string early in a layer below this.

#### What it is not

It is not a general literal mode and there must not be one. A caller cannot
reach it, a dialect that can bind must leave it nil, and `Query.Build`
returns no argument values when it is set, so a dialect cannot half use it.

### D80. The driver registry is dburl's, and reading it is not importing it. Decided.

`dburl` v0.29.0 describes every scheme it registers. `dburl.Scheme` gained
`Desc`, `Home`, `GoPackage`, `DriverURL`, `RequiresCGO` and `Deployment`, so
the question step 3 of `docs/DIALECT.md` asks has an authoritative answer in
one place for the first time. Step 3 and hard rule 10 now name that registry.

Nothing in the root module changes and nothing is imported. Hard rule 1
forbids the dependency, D19 removed it once already, and none of this is a
reason to bring it back: `dbmeta` takes a `DB` and a `Dialect` and has no URL
to parse. The registry is a document here, read by a person adding a dialect
and by nothing at build time.

#### Why the grep it replaces gave a wrong answer

Step 3 said `grep -rn "// DRIVER" ~/src/go/src/github.com/xo/usql`. That finds
an import that carries the comment. Oracle does not have one: `oracle` and
`godror` both register through `orshared.Register`, so the grep answers for
every product except the one this project spent a day getting wrong. The
registry names `github.com/sijms/go-ora/v3` for `oracle` and
`github.com/godror/godror` for `godror`, and puts `RequiresCGO: true` on the
second, which is D48's rule written as data rather than as prose here.

Keep the grep as a second look. It shows what `usql` actually imports, and the
registry shows what it says it imports. Where the two disagree, one of the two
projects has a defect and the disagreement is the finding.

#### What the registry does not answer

The version. D52 requires the same package and allows a different version, and
the version lives in `usql`'s `go.mod`. Step 3 now reads two things: the
registry for the package, and that `go.mod` for the version `usql` pins.
Oracle is the standing example, because D59 holds `dbmeta` on `go-ora/v2`
while the registry and `usql` both name v3.

A scheme with no `GoPackage` is not a hole. It is how the registry says the
scheme borrows another scheme's driver, and it is the same fact
`URL.UnaliasedDriver` reports at parse time. Seven schemes are in that state
today: `cockroachdb`, `memsql`, `redshift`, `tidb`, `vitess`, `oleodbc` and
`file`.

#### Scheme.Deployment and Info.Embedded both stay

`DeploymentEmbedded` and `dbmeta.Info.Embedded` say the same thing about the
same products and neither replaces the other. `dbmeta` cannot read the first,
which settles it, and the two do not even count the same objects: `dburl`
marks `sqlite3`, `moderncsqlite` and `duckdb`, which is three schemes, and
`dbmeta` marks two models, because one model covers both SQLite drivers.
Neither number is wrong and neither is derivable from the other.

### D81. The Cassandra dialect is cql. Decided.

`dbmeta.Cassandra` is `"cql"`. It was `"cassandra"` and that was wrong.

`Dialect` is documented as the `dburl` driver name and twelve of the thirteen
were. `cassandra` is not a driver name. It is an alias of the `cql` scheme,
alongside `ca`, `datastax`, `scy` and `scylla`, and `cql` is what
`github.com/MichaelS11/go-cql-driver` passes to `sql.Register` and what `usql`
registers at `drivers/cassandra/cassandra.go`. Nothing anywhere answers to
`cassandra`, so the old value named a driver that does not exist.

The `dburl` session found it. D80 sent a request there for a field saying
which product a scheme drives, Ken asked whether the thirteen values matched
the registry, and that session checked all of them. This was the one.

#### Why dburl could not absorb it instead

`Scheme.Driver` has to be the exact string the Go driver registers, because
that is what a caller hands to `sql.Open`. Renaming the scheme to `cassandra`
would emit a name nothing answers to and every Cassandra connection would
fail. Adding a second scheme named `cassandra` would do the same thing with
more steps. The fault was here.

#### What changed with it

The constant name stays `Cassandra`, which is the whole point of the name and
the value differing.

The golden section in `test/testdata/parity.txt` is named from the dialect, so
`[cassandra/same/grantee]` is now `[cql/same/grantee]`.

`dbrun` builds the environment variable from the dialect, as
`"DBMETA_" + strings.ToUpper(string(d))`, so the variable that carries the
Cassandra DSN is `DBMETA_CQL` and no longer `DBMETA_CASSANDRA`. The three
places in the `test` module that read the old name were changed with it. CI
needed nothing, because it runs `dbrun test` and never writes the name.

Nothing else moved. The model package is still `models/cassandra`, the
container product is still `cassandra`, the build tag is still `cassandra`,
and `testdata/conformance.txt` is keyed by the test's own name rather than by
the dialect, so its `[cassandra]` section is unchanged. Those are four
different strings that happen to have agreed, and only one of them was the
dialect.

Verified against Cassandra 5.0.9 on 2026-09-26. The parity, smoke, fixture and
conformance tests all pass with the renamed section and the renamed variable.

#### The lesson, which is the reason this is a decision and not a commit

A value that is also a word is not checkable by reading it. `cassandra` looks
right in every place it appears, and it was wrong in exactly one of them. The
registry is what told the difference, which is D80 paying for itself the week
it was written.

Read these as constants and never as literals. A consumer that wrote
`dbmeta.Dialect("cassandra")` breaks here and one that wrote
`dbmeta.Cassandra` does not.

### D82. CI compiles once and every job runs the binary. Decided.

One job builds `dbrun` and the test binary and uploads them. Every job in
both release matrices downloads those two and compiles nothing. Ken asked
whether `dbrun` could be built once, and the answer is that it can, along
with the thing that costs four times as much.

#### What it cost before

Measured on the nightly run of 2026-09-26, job `mariadb-11.4`, which is an
ordinary one rather than the worst:

| | |
| --- | --- |
| the step | 103 seconds |
| building `dbrun`, and starting the server | 47 seconds |
| compiling the tests | 55 seconds |
| running the tests | 1.4 seconds |

Every job downloaded 63 modules and compiled them. There were 23 jobs.

The test module links every driver dbmeta tests against, and two of them are
cgo: `mattn/go-sqlite3` builds the SQLite source and `duckdb/duckdb-go` links
a prebuilt library of about a hundred megabytes. That is the whole cost and it
is paid once per job for a binary that is identical in all of them.

#### Why the cache did not do this already

`actions/setup-go` caches and the cache was hitting. It was 33 megabytes,
which is the root module, because the key is the hash of the root `go.sum`
and the root module has no dependencies at all. Whichever job saved first
saved the smallest possible cache, and a key cannot be written twice, so no
later job could replace it with a useful one.

Widening the key to cover `test/go.sum` would have helped and it would not
have been enough. A restored build cache still relinks, and the link of a
121 megabyte binary is not free. Building once and shipping the result skips
the question.

#### How

`DBMETA_TEST_BINARY` names a test binary built by `go test -c`. When it is
set, `dbrun test` runs that binary rather than `go test`. Nothing sets it for
a person, so `dbrun test postgres` still compiles what they just changed,
which is what they want.

This keeps D68 intact. CI still runs `dbrun test`, which is the same entry
point and the same code path, so CI cannot start a container a way nobody
else does. What changed is which binary runs the tests, not who starts the
server.

The matrix jobs no longer set up Go at all. They still check out, because the
tests read `testdata/` at run time, and they `chmod +x` what they download,
because an artifact does not carry the mode bit.

`TestTheMatrixJobsCompileNothing` fails when a matrix job runs `go run` or
`go test`, or does not set `DBMETA_TEST_BINARY`. That test exists because
this is invisible from a passing run: a job that compiles gives the right
answer and simply costs ninety seconds to give it.

#### What this does not fix

The artifact is 121 megabytes and about 46 compressed, downloaded once per
job. That is real and it is far less than what it replaces.

Wall clock is not 23 times better. The jobs already ran in parallel, so the
run was bounded by the slowest job rather than by the sum, and SAP HANA at
five minutes is mostly a server starting. What this recovers is the ninety
seconds inside every job, the runner minutes behind them, and the second wave
when the matrix is wider than the concurrency limit.

### D83. A server is ready when it can run a query, not when it answers one. Decided.

The readiness check for Presto and Trino reads a table. It read a constant
and that was not the same thing.

`presto-cli --execute "SELECT 1"` succeeded and the fixture's first statement
then failed:

	NO_NODES_AVAILABLE: No nodes available to run query

`EXPLAIN (TYPE DISTRIBUTED)` says why, on Presto 0.299 and Trino 483 alike:

| query | plan |
| --- | --- |
| `SELECT 1` | one SINGLE fragment |
| `SELECT * FROM (VALUES 1) t(x)` | one SINGLE fragment |
| `SELECT count(*) FROM system.runtime.nodes` | a SINGLE and a SOURCE fragment |

A SINGLE fragment is evaluated by the coordinator on its own. A SOURCE
fragment has to be scheduled on a node. So a constant is answered in the
window between the HTTP port opening and a worker registering, and a table
read is not. Both checks now read `system.runtime.nodes`.

Trino is changed on the same evidence rather than on a failure of its own. It
plans identically and the image has the same shape, so the difference is that
nobody has been unlucky with it yet.

#### D82 is what exposed it

The race was always there and the compile was hiding it. Every job spent
ninety seconds building the tests between `dbrun` declaring the server ready
and the first statement running, which was ample for a worker to register.
D82 removed that and the gap closed to nothing.

This is worth stating plainly, because the obvious reading is that D82 broke
Presto. It did not. It removed an accidental delay that a readiness check was
quietly depending on, and a readiness check that needs a ninety second pause
after it is not one. The same reasoning applies to anything else in
`container/` whose check is cheaper than the work that follows it.

#### What was not done

The check does not assert that the count is not zero, and it does not need
to. If no node is active the query cannot be scheduled and fails, so the exit
code already carries the answer, and a shell wrapper to compare the number
would add quoting for nothing.

Verified by removing both containers and running `dbrun test presto-0.299`
and `dbrun test trino-483` from cold. This machine starts both too fast to
reproduce the race, which is why the plans were measured rather than the
timing.

## Open questions for Ken

An open question lives here until it is answered, and then it becomes a
decision above. The argument behind a decision belongs with the decision, which
is why there is no separate document for it. See D50.

One question is open, at the end of this section. None of the older ones are.
The floor question that the upstream change reopened has been answered:
D20 keeps 9.6, and D40 adds the tiers and the removal trigger that the review
asked for in exchange.

Everything else raised in this document has been answered, and every decision
is marked Decided or Superseded.

Nothing is deferred either. Both of the items that were are closed, and each
is worth a line here because both were read as live design space after they had
stopped being it.

D4 and question 4 as it was: whether `dbmeta` exports interfaces at all, and
under what names. It waited for D13 to deliver the models, and D13 delivered
eight. The root package exports `Querier` and `AnyQuery` and no per object
interface, an object kind is a `Query` value, and D49 settled the one that
remained. D4 says so and points at the decisions that hold the current shape.

D35 was listed here and it is not deferred any more. D48 answered it: cgo is
allowed in the `test` module, so DuckDB is reached through its driver like
every other database. The question was worded as "how is DuckDB reached, given
pure Go only", and the answer is that pure Go only was never a constraint on
the `test` module. It is a property of the root module, which has no driver in
it at all.

D60 was the last one and it is answered. D61 measured what a principal
actually gets, and an Oracle local user that owns the objects receives the
administrator's answer to every query, so there was no gap to close. The model
stays `ALL_` only.

Raise a new question here rather than deciding one alone.

### Open. How does a consumer get from a dburl URL to a dialect when a product has two drivers?

`dbmeta.Dialect` is the `dburl` driver name, and `dburl.URL.Driver` selects a
dialect directly for every product with one driver. `dburl` registers a scheme
per Go driver, so a product with two has two: `pgx` is PostgreSQL,
`moderncsqlite` is SQLite and `godror` is Oracle. None of those three words is
a dialect here, and a consumer that maps `URL.Driver` straight to a `Dialect`
finds no model for any of them.

This is not hypothetical. `usql` builds all three, and rule 10 makes `dbmeta`
test two of the three pairs, so the case is the rule rather than an edge of
it. The `Dialect` doc comment and `docs/COMMANDS.md` said the mapping was
direct until this was found, and both now say it is not.

The five wire compatible schemes are already handled and are not part of this.
`cockroachdb`, `redshift`, `memsql`, `tidb` and `vitess` carry an `Override`,
so `URL.Driver` is already `postgres` or `mysql` and `URL.UnaliasedDriver`
carries the flavor. That is rule 1 working exactly as written.

Three answers are possible and each belongs to a different project:

`dburl` names the product. A field beside `GoPackage` saying that `pgx` and
`postgres` are one product would answer it for every consumer at once, and it
is the same kind of fact D80 welcomed. `Desc` almost carries it today,
"PostgreSQL PGX" against "PostgreSQL", and prose is not a key.

The consumer keeps the mapping. Three entries, and `usql` already knows which
product each of its drivers is, so it costs `usql` nothing. It costs the next
consumer the same three entries again, which is how two copies start
disagreeing.

`dbmeta` accepts the alias. Rule 1 forbids it, and it is written here only so
that nobody proposes it a second time without reading why.

Ken decides. The first is the one this session would pick, and it is a change
to `dburl` rather than to anything here.
