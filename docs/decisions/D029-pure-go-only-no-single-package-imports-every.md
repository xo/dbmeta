# D29. Pure Go only. No single package imports every driver

Status: Amended by D48.

The proposal put all driver imports in one sub-package. Both reviews rejected
that, and they were right, though the reason is narrower than Gemini stated.

Importing every driver into one package forces cgo on everyone who builds it,
breaks `CGO_ENABLED=0` and cross compilation, and invites a conflict between
two drivers over a shared transitive dependency.

## Verified: only DuckDB actually forces cgo

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

## The rule: pure Go only, everywhere

Ken has settled this beyond preference. `dbmeta` is pure Go, and so is the
generation sub-package. The root module uses no cgo and never will. The test
module can, and does for SQLite. D48 amends this: the separate `go.mod` is what
makes a C toolchain safe there and absent everywhere else.

Use the pure Go driver in the table above for every database that has one.
Never import `mattn/go-sqlite3` or `godror`.

Do not put every driver import in one package even though all of them are pure
Go. A conflict over a shared transitive dependency does not care about cgo.

## DuckDB has no pure Go driver, and D24 puts it in CI. D48 answered this

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
   root module, so DuckDB tests run there like any other. Something still
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
