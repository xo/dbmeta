# What usql Answers Today, and What dbmeta Changes

`usql` is the reason the object model here follows `psql`. This measures what
`usql` can answer today, per database, and what changes if it reads `dbmeta`
instead of its own readers.

Everything below was measured rather than read. A program built `usql` with the
`most` build tag, constructed every driver's metadata reader, and asked each
one which reader interfaces it satisfies. The counts come from that.

Measured against `usql` at commit `382e1da` on `main`, with 47 drivers built.

## How usql reads metadata today

`usql` defines 14 leaf reader interfaces in `drivers/metadata/metadata.go`,
one per object kind, aggregated by `ExtendedReader`. The file declares 19
interfaces in total and 16 of those end in `Reader`, because two are composites
and three are `Writer`, `CatalogProvider` and `Result`, so counting with grep
gives a different and less useful number. `BasicReader` embeds 3 of the 14. A driver implements the ones it can. The writer asks for a reader
by type assertion and leaves out the section when the driver does not have one,
so a command degrades rather than failing.

The 14 kinds: Catalogs, Schemas, Tables, Columns, ColumnStats, Indexes,
IndexColumns, Triggers, Constraints, ConstraintColumns, Functions,
FunctionColumns, Sequences and PrivilegeSummaries.

Eleven metacommands register in the Describe family in `metacmd/descs.go` and
all of them dispatch through `Describe`:

	\d \da \df \di \dm \dn \dp \ds \dt \dv \l

Seven readers decide whether a command runs at all: TableReader, ColumnReader,
FunctionReader, IndexReader, SchemaReader, PrivilegeSummaryReader and
CatalogReader. The other seven leaf readers decide how much a command prints
once it does run. SequenceReader, IndexColumnReader, TriggerReader,
ConstraintReader and ConstraintColumnReader add sections inside
`DescribeTableDetails`, FunctionColumnReader inside `DescribeFunctions`, and
ColumnStatReader serves `\ss` alone. `\l` needs Catalogs. `\dn` needs Schemas. `\dt`, `\dv`, `\dm`,
`\ds` and bare `\d` need Tables. `\d NAME` needs Tables and Columns to print
anything, and adds a section per further reader. `\di` needs Indexes. `\df`
and `\da` need Functions. `\dp` needs PrivilegeSummaries. `\ss` needs
ColumnStats and is outside the Describe family, as is `\z`.

Two earlier versions of this section were wrong in the same way, and both
errors were a right count of the wrong thing. It said eight commands, which
counted readers. Then it said eight gating readers, which counted seven gating
readers plus one that is not. The `usql` session measured both.

## Which databases answer what

Counting needs a unit and a build, and this is the unit: registered names,
which includes aliases, because `Register` puts an alias in the same map as its
own key.

	51 registered names, 21 with a reader, 30 without    built with -tags all
	13 registered names, 12 with a reader, 1 without     the default build

A package is a different count and reconciles with neither. There are 46
packages under `drivers/`, 42 calls to `drivers.Register` with a literal
scheme, which undercounts because Oracle and godror go through
`orshared.Register`, and 15 packages that define a reader of their own. More
schemes answer than that, because cockroachdb, redshift, tidb, vitess, memsql
and nzgo have no package and register against another driver's reader.

An earlier version of this section said 47, 20 and 27 without saying which
build, so it could not be reproduced. The figures here were measured by the
`usql` session with a program that asserts each registered reader against the
interfaces the dispatch requires, rather than by reading code.

A driver with no reader answers no metadata command: `\dt` on it prints
nothing useful.

| Driver | Commands | Missing |
| --- | --- | --- |
| postgres, pgx | 11/11 | none |
| cockroachdb, redshift | 11/11 | none |
| sqlserver | 11/11 | none, and it reads `information_schema` with sequences and constraints off |
| duckdb | 11/11 | none |
| trino | 11/11 | none |
| mysql, mymysql | 10/11 | `\l`, no `CatalogReader` |
| memsql, tidb, vitess | 10/11 | `\l` |
| snowflake, databend, nzgo | 10/11 | `\l` |
| oracle, godror | 10/11 | `\dp`, no `PrivilegeSummaryReader` |
| sqlite3, moderncsqlite | 9/11 | `\dp` and `\l` |
| clickhouse | 8/11 | `\di`, `\dp` and `\l` |
| impala | 6/11 | `\da`, `\df`, `\di`, `\dp` and `\l` |

The floor is higher than a bare count suggests, and the gaps are two features
rather than a long tail. Every driver that fails fails on the same two
commands. `\l` is missing across the whole MySQL family and Snowflake,
Databend and Netezza, because none implements `CatalogReader`. `\dp` is
missing on Oracle, godror, SQLite, ClickHouse and Impala, because none
implements `PrivilegeSummaryReader`. Nothing is missing `\dt`, `\dn` or `\d`.

The 30 with no reader at all include avatica, awsathena, bigquery, chai,
cosmos, cql, csvq, databricks, exasol, firebirdsql, flightsql, h2, hdb, hive,
ignite, maxcompute, n1ql, ots, presto, ql, spanner, tds, vertica, voltdb and
ydb.

The seven at full marks get there the same way: they are PostgreSQL, or wire
compatible with it and reusing its reader, or they read `information_schema`.

One shape worth knowing, because D67 turns on it. Impala's reader returns a
value satisfying nothing when its handle is not a `*sql.DB`, so it degrades to
no metadata at all rather than reporting anything. It is the only driver here
that does that, and it is deliberate.

## What dbmeta changes

### It does not raise the command count for PostgreSQL

PostgreSQL already answers 8 of 8. What changes there is the number of object
kinds: 14 against 55. `COMMANDS.md` maps every `psql` metadata command to the
Go value that answers it, and 37 of them have no reader interface in `usql`
today. Tablespaces, types, domains, operators, text search, publications,
extensions, roles, access methods and the rest are all readable from `dbmeta`
and have nowhere to go in `usql` yet.

So for PostgreSQL the gain is not a command that starts working. It is that
`usql` could implement four times as many commands without writing SQL.

It also gains facts `psql` does not print. `Column.PrimaryKey` is the example
the policy was written around: `usql` reads it per column today and `dbmeta`
now returns it in the same row. D47 says when that is allowed, and the short
version is that the fact must come out of one statement.

### Which databases answer everything usql asks

`usql` has eight metadata commands and `\d NAME` has four sections. This is
what each `dbmeta` model answers of them, counted from the queries the models
actually register rather than from memory.

| Database | usql today | with dbmeta | What it still cannot answer |
| --- | --- | --- | --- |
| PostgreSQL | 8/8, 4/4 | 8/8, 4/4 | nothing |
| MariaDB | 6/8, 3/4 | 8/8, 4/4 | nothing |
| SQL Server | 8/8, 4/4 | 8/8, 4/4 | nothing |
| MySQL | 6/8, 3/4 | 7/8, 4/4 | `\ss`: the model registers ColumnStats for the shared dialect and MySQL the product has no such view |
| Oracle | 6/8, 1/4 | 7/8, 4/4 | `\l`: Oracle has one database per instance and no list to read |
| ClickHouse | 4/8, 0/4 | 6/8, 3/4 | `\ds` and `\ss`: no sequence and no column distribution. No trigger section |
| DuckDB | 8/8, 4/4 | 6/8, 3/4 | `\dp` and `\ss`. No trigger section |
| SQLite | 5/8, 1/4 | 5/8, 4/4 | `\ds`, `\dp` and `\ss`. It gains all four sections |
| Cassandra | none | 5/8, 4/4 | `\l`, `\ds` and `\ss`. It had no reader at all |

Three answer everything `usql` asks: PostgreSQL, MariaDB and SQL Server. Every
one of the others is short because the product has no such object rather than
because the model is unfinished, and each says so rather than returning
nothing.

Two rows need a note. DuckDB reads 8 of 8 through `usql`'s shared
`information_schema` reader today and 6 through `dbmeta`, and which of those
is right has not been measured: the shared reader answers `\dp` and `\ss` from
`information_schema` views, and whether what they report is true of DuckDB is
the D43 question and nobody has asked it. Cassandra had no reader in `usql` at
all, so every one of its five is new.

The count is per dialect, and MySQL and MariaDB share one. A query the model
registers for both is counted for both, so MySQL's `\ss` is listed as
answerable and returns nothing on the product. `docs/COVERAGE.md` keeps the
two apart, at 29 kinds for MariaDB and 26 for MySQL.

### It offers something to the 27 with nothing

`models/informationschema` answers 7 kinds for any database with a standard
`information_schema`, which is enough for `\dn`, `\dt`, `\d NAME`, `\df` and
`\dp`. A driver registers a profile saying how it differs from the standard,
which is tens of lines rather than a reader.

Which of the 27 have a usable `information_schema` is not measured here and
must not be guessed. Each one needs the D43 treatment: ask two models, then run
the queries against a real server.

## The gap, and what closed it

`usql` read three kinds that `dbmeta` could not answer for any database:
ColumnStats, ConstraintColumns and FunctionColumns. A migration would have lost
all three. D46 named them, D47 set the policy for adding them, and all three
now exist.

`dbmeta.ConstraintColumns` is the column level detail of a constraint: which
column, in what position, and for a foreign key which column of which table it
points at. Every model answers it, including SQLite.

`dbmeta.RoutineParameters` is `usql`'s FunctionColumns: the name, position,
direction and type of each parameter. Every model except SQLite answers it,
and SQLite cannot, because a function there is compiled C with no named
parameters.

`dbmeta.ColumnStats` backs `\ss`. PostgreSQL, MariaDB and SQL Server answer it.
MySQL, SQLite and DuckDB cannot, and say so rather than returning rows that are
almost all absent. `usql` implements `\ss` today for PostgreSQL, DuckDB, Trino,
CockroachDB, Redshift and SQL Server. Two of those six are covered, and DuckDB
is not: `usql` prints statistics there that `dbmeta` refuses, because DuckDB
has no catalog of them.

### What is still missing for a lossless migration

Nothing, for the databases `dbmeta` models. `usql` implements `\ss` for six
drivers and `dbmeta` answers it for two of them, so a migration of the other
four waits on a model for Trino and on what CockroachDB and Redshift answer
through the PostgreSQL model, rather than on a missing kind.

Two `usql` reader kinds have no `dbmeta` equivalent by design.
ConstraintColumns replaces both ConstraintColumns and the column part of
Constraints, and FunctionColumns became RoutineParameters. The names differ and
the facts do not.

## How usql would use dbmeta

The shape is already there. `usql`'s `Reader` interfaces and `dbmeta`'s
`Query[T]` values line up one for one, so the move is per driver and reversible.

Read the version once, when the connection opens, and build a `Meta`:

```go
versions, err := dbmeta.PostgreSQL.Version(ctx, db)
if err != nil {
    return err
}
m, err := dbmeta.New(dbmeta.PostgreSQL, versions)
```

Then one `usql` reader method becomes one loop:

```go
func (r *reader) Tables(f metadata.Filter) (*metadata.TableSet, error) {
    args := dbmeta.Args{Schema: f.Schema, Name: f.Name, WithSystem: f.WithSystem}.Map()
    var out []metadata.Table
    for v, err := range dbmeta.Tables.All(r.ctx, r.meta, r.db, args) {
        if err != nil {
            return nil, err
        }
        out = append(out, metadata.Table{Schema: v.Schema, Name: v.Name, Type: v.Type})
    }
    return metadata.NewTableSet(out), nil
}
```

`dbmeta.Args` is already the same four filters `metadata.Filter` carries, and
`dbmeta.Querier` is one method, so whatever `usql` already holds satisfies it.

Two things the writer gets for free. `Query.Support(m)` says whether a database
can answer at all, so `usql` can tell "this database has no such object" from
"there are none", which today it cannot. And `Field.Min` and `Field.Present`
say whether a NULL means the value is null or the server is too old to have the
column, which `usql` has no way to express.

Do not move the writer. `dbmeta` only reads, and `tblfmt` renders. That
division is D5 and it does not change.

The order that loses nothing: move one driver, then the rest. PostgreSQL is the
wrong one to move first because it already works. MariaDB is the right one,
because it goes from 6 commands of 8 to all 8, and the difference is visible
the moment it lands.

The three kinds that blocked this are done. What is left is `usql`'s own work:
a reader per driver, and whatever filtering each command needs so that the
output still matches `psql`.

## The version line, measured

`usql` prints one line on connecting, and `dbmeta` builds the same line in
`VersionSet.Display`. Both were read from the same connection on 26 servers,
which is every release in `container.All()` plus the two SQLite drivers and
DuckDB. `usql`'s side is its own per driver `Version` function, or
`SELECT version()` where a driver declares none.

| Product | Releases | Result |
| --- | --- | --- |
| PostgreSQL | 9.6 to 18, all ten | identical, including the Debian build suffix |
| DuckDB | 1.5.5 | identical |
| MariaDB | 10.6 to 13.0, all six | `dbmeta` names the product, `usql` prints a bare number |
| MySQL | 8.4, 9.7, 26.7 | the same |
| SQL Server | 2017, 2019, 2022, 2025 | `dbmeta` adds the release year and the cumulative update |
| SQLite | one build, two drivers | the two disagree about the name |

Eleven lines match exactly. Thirteen are `dbmeta` reporting strictly more. Two
are the SQLite naming, which is the only real disagreement.

That table predates `models/oracle`, `models/cassandra`, `models/clickhouse`
and `models/trino`, and it compares what the two print rather than what they
run. The statements are compared below, because comparing only the output hid
a case where `usql` has no answer at all.

### The statements, compared

| Product | dbmeta runs | usql runs | |
| --- | --- | --- | --- |
| PostgreSQL | `SHOW server_version` | the same | same |
| SQLite | `SELECT sqlite_version()` | the same | same |
| Cassandra | three columns from `system.local` | the same | same |
| MariaDB | `SELECT VERSION()` | no function, so the generic `SELECT version();` | same answer |
| MySQL | `SELECT VERSION()` | no function, so the generic `SELECT version();` | same answer |
| ClickHouse | `SELECT version()` | no function, so the generic `SELECT version();` | same answer |
| DuckDB | `SELECT version()` | `SELECT library_version FROM pragma_version()` | different statement, same answer |
| Trino | `SELECT version()` | `SELECT node_version FROM system.runtime.nodes LIMIT 1` | different statement, same answer |
| Presto | `SELECT node_version FROM system.runtime.nodes WHERE coordinator = true LIMIT 1` | the same, without the coordinator filter | same answer, and `usql` may read a worker on a cluster |
| SQL Server | the `@@VERSION` banner and four `SERVERPROPERTY` values | three `SERVERPROPERTY` values | `dbmeta` reads more |
| Oracle | `SELECT banner FROM v$version WHERE ROWNUM = 1` | no function, so the generic `SELECT version();` | **`usql` cannot answer** |

Measured on a live server of each, on 2026-09-26.

#### Oracle, where the generic fallback is not valid SQL

`usql` declares no `Version` function for its `oracle` driver, so
`drivers.Version` falls through to `SELECT version();`. Oracle has no such
function:

```
ORA-00904: "VERSION": invalid identifier
```

`drivers.Version` discards the error and returns `<unknown>`, so the failure
is silent and `usql` prints no version for Oracle at all. `dbmeta` reads
`v$version` and gets
`Oracle AI Database 26ai Free Release 23.26.3.0.0`.

This is the strongest single case for the move, and comparing the printed
lines would never have found it, because Oracle was not in the table above.

#### DuckDB and Trino, where the statement differs and the answer does not

DuckDB returns `v1.5.5` from both `version()` and `library_version` in
`pragma_version()`. Trino returns `483` from both `version()` and
`node_version` in `system.runtime.nodes`, byte for byte.

Neither is a divergence worth closing, and the scalar function is the better of
each pair. `pragma_version()` is a table function where `version()` is a plain
call. `system.runtime.nodes` has one row per node, so `usql`'s `LIMIT 1` with
no `ORDER BY` picks an arbitrary one, which on a cluster mid upgrade can be a
worker rather than the coordinator that parses the SQL.

### MySQL and MariaDB, where usql prints no product at all

The `mysql` driver declares no `Version` function, so `usql` falls through to
the generic `SELECT version()` and prints what comes back. On MySQL that is the
whole line:

```
usql     8.4.11
dbmeta   MySQL 8.4.11
```

On MariaDB the suffix carries the product, so `usql` reads
`11.8.9-MariaDB-ubu2404` and is at least identifiable, by accident rather than
by design. `dbmeta` reads the same string and names the product from the same
suffix its queries gate on, so the two cannot drift apart. See D44.

This is the one place `usql` would gain from the move without a new query.

### SQL Server, where usql is thinner than its own specification

```
usql     Microsoft SQL Server 16.0.4295.3, RTM, Developer Edition (64-bit)
dbmeta   Microsoft SQL Server 2022 16.0.4295.3, RTM-CU27, Developer Edition (64-bit)
```

`usql` selects `productversion`, `productlevel` and `edition`. `dbmeta` selects
those, plus `productupdatelevel` for the `CU27`, plus the release year cut from
`@@VERSION`, which is the only place the server states the name the product is
sold under. D38 always specified the longer form.

### SQLite, which is a disagreement rather than a gap

```
usql     SQLite3 3.53.4          with mattn/go-sqlite3
usql     ModernC SQLite 3.53.4   with modernc.org/sqlite
dbmeta   SQLite 3.53.4           with either
```

`usql` names the driver, so one SQLite build reports two different products.
`dbmeta` names the product, because it holds a dialect and never sees the
driver, and under D47 the consumer decides what to show. A consumer that wants
to name its driver prepends that itself.

This is the one line a move would change, so it is written down rather than
discovered later.

`TestTheDisplayLineNamesTheProduct` in the `test` module pins the shape per
product against a live server, so a model cannot quietly lose its product name
and start printing the bare number `usql` prints today.

## Changing a password

`usql` changes a password in seven drivers and escapes nothing. Each one
concatenates the new password into the statement, so a password holding a quote
or a backslash breaks the statement or sets something other than what was
asked.

`Dialect.ChangePassword` builds the statement instead and returns the text
for `usql` to run. It takes no database, so `dbmeta` still executes only reads.
The escaping needs the server, because whether a backslash escapes inside a
string literal is `sql_mode` on MySQL and MariaDB and
`standard_conforming_strings` on PostgreSQL, and `Dialect.Quoting` reads it.

That makes this the second thing a move would fix rather than merely relocate,
alongside the version line for MySQL. See D56.
