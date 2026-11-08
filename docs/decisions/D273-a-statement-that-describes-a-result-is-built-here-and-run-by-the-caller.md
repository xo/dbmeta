# D273. A statement that describes a result is built here and run by the caller

Status: Amends D56, amended by D274.

psql's `\gdesc` shows the name and type of each column of the query in the
buffer, without running it. usql runs on database/sql, which cannot describe a
query without running it. usql asked on 2026-11-08 whether dbmeta owns this.
Ken accepted on 2026-11-08 that it owns the part that is database knowledge.

## Decision

Two methods build statements and return them as text. They take no database
and run nothing, in the way `Dialect.ChangePassword` does (D56). The caller
runs the statement. Both statements read. The first asks the server to
describe a query, which does not run it, and the second names types.

`Dialect.DescribeResult(query)` returns a `ResultDescription`. It holds the
statement and the names of the columns of its rows that carry the name and the
SQL type of each result column. SQL Server answers it with
`sp_describe_first_result_set`, which returns `name` and `system_type_name`. The
query goes into the statement as a literal prefixed N, quoted with
`QuoteLiteral`. Describing `WAITFOR DELAY '00:00:20'; SELECT 1` took 30
milliseconds, so the query does not run. The procedure describes the first
statement of a batch, so the caller refuses anything that is not one statement.

`Dialect.TypeNames(types)` takes the type number and modifier that a driver
reports for each result column. It returns a statement that gives each type
the name `format_type` gives, such as `character varying(10)`. It is built
from the numbers alone, so no string reaches the text. PostgreSQL and
CockroachDB answer it. CrateDB copies the PostgreSQL model and cannot answer,
because it has no `format_type` and its parser refuses the oid casts, so it
sets the field to nil. For PostgreSQL the description itself comes from the
driver in usql, which prepares the query and reads the fields without running
it.

Every other product returns `ErrNotSupported`. dbmeta does not map the type
names that a driver reports, such as INT4, to SQL names for MySQL, SQLite or
DuckDB, because those drivers drop the length and the precision and the map
invents them. The zero row wrapper stays in usql, because it runs the
query and a function with a side effect runs too. Oracle has
`DBMS_SQL.DESCRIBE_COLUMNS`, which needs a block with output variables and not
one statement, so it is not built until it is measured.

## What this changes in D56

D56 allowed one statement that is built and not run. This allows two more, for
the same reason: the statement is per product knowledge and the caller still
runs everything. The rule that dbmeta executes only reads does not change.

## Rejected

A `Query` that takes the text of the query as a parameter. It is the usual
shape here, but it needs a new object kind with a model for each product, and
the parity and scan tests run it with a query nobody wrote. Two methods
that return text are smaller, and usql needs no new type.
