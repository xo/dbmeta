# D52. A test driver is the one usql uses, or it is the wrong driver

Status: Amended by D59 and D154.

The `test` module imports, for each database, the same driver package `usql`
imports for that database. Not the same version, which each module pins for
itself, but the same package.

`usql` marks them: every driver import carries a `// DRIVER` comment, and
`grep -rn "// DRIVER" usql` is the list. Consult it before adding a driver, and
again before adding a dialect.

## Why the package and not the version

`dbmeta` exists to be read through `usql`. A query that works on the driver
this repository tests and fails on the one `usql` ships is a query that does
not work, and the failure surfaces in someone else's project.

Drivers are not interchangeable. They differ in how they present a type, in
what they do with a NULL, and in which protocol extensions they use, and those
are exactly the things a metadata query touches. The NULL scan fault this
project has hit twice is driver visible behavior.

The version is a different matter. Each module pins what it needs, and a
consumer picks its own, which is hard rule 1 and does not change.

## What was wrong, and how it was found

The DuckDB work was written against `github.com/marcboeker/go-duckdb/v2`,
which is the widely known driver and is not the one `usql` uses. `usql` uses
`github.com/duckdb/duckdb-go/v2`, the successor under the DuckDB organization.
Ken caught it before it was committed.

The other four were already right, and that was luck rather than method:
`go-sql-driver/mysql`, `jackc/pgx/v5/stdlib`, `mattn/go-sqlite3` and
`modernc.org/sqlite` all match `usql`. This decision makes it method.

## The list, as it stands

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

## What the measurement found

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
not a driver question, and running it twice buys a per-driver section for
every refusal in the file in exchange for re-measuring the database.

## Which databases have a second driver at all

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

## Correcting hard rule 10

Hard rule 10 named `gocql/gocql` for Cassandra. `usql` uses
`github.com/MichaelS11/go-cql-driver`, which is the `database/sql` driver that
wraps `gocql`. `dbmeta` needs a `database/sql` driver, so the rule named a
package that cannot be used. Corrected.

## When usql does not have one

A database `usql` does not support has no driver to match, and the choice is
open. Say so in the commit, and prefer the driver `usql` is most likely to
adopt, which is usually the one the database's own organization publishes.
