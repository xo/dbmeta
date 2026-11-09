# D259. usql applies the fixtures, and the northwind fixture is on five more products

Status: Amends D254, D256 and D257, amended by D290.

The maintainers decided on 2026-11-04 that the fixtures target usql, the command that
applies a script, and not the command line client of each product. usql works
like psql. It ends a statement at a semicolon and has no slash line, which comes
from SQL*Plus. The northwind fixture also went to five more products.

## usql applies the files

`dbrun fixture northwind` prepares the database on a connection of its own,
because a database cannot be dropped from inside itself. Then it runs
`usql -q <url> -f <file>` for each file. dbrun no longer splits any SQL. The
splitters, the `SET IDENTITY_INSERT` tracker and their tests are gone.

usql goes on after a statement that fails and exits with 1, so dbrun reports the
first lines of what usql wrote to the error output. After the load, dbrun counts
the orders and the customers with usql and prints them.

What the files changed for usql:

- The Oracle files end each statement with a semicolon. The line with only a
  slash is gone, and so is its meaning in `Syntax.AnonymousBlocks` (D252).
- The SQL Server file turns `IDENTITY_INSERT` off for a table before it turns it
  on for the next one, because dbrun does it no more.
- A semicolon inside a PL/SQL unit that has a declaration section, or no BEGIN,
  is written as `\;`, as in psql. usql turns it back into a semicolon before it
  sends the statement. A semicolon inside a BEGIN and END block stays plain.
  `oracle-hr.sql` has four such units. `oracle.sql` has none.

## Five more products

The files are made from `postgres.sql` by `convert/nw2sql.py`, and the procedures
are written by hand in a tail file for each product. No product but these gets a
northwind fixture, and the fixtures are not compared with each other.

| Product | File | How it differs from the PostgreSQL file |
| --- | --- | --- |
| MySQL, MariaDB | `mysql.sql` | `auto_increment`, `decimal(19,4)` for money, `longblob`, `datetime`, procedures that return a result set, in the database northwind |
| SQLite | `sqlite3.sql` | Keys and constraints inside CREATE TABLE, `numeric` for money, a view for the one function with no argument, in the file `northwind.db` beside the file of dbrun |
| DuckDB | `duckdb.sql` | Sequences for identity, keys inside CREATE TABLE, tables in the order of their references and rows in the order of their parents, table macros, in the file `northwind.duckdb` |
| Firebird | `firebird.sql` | `decimal(18,4)`, blob sub types, selectable procedures. Firebird has no schema, so dbrun drops the objects of the fixture by name and nothing else in the database |
| SAP HANA | `hana.sql` | `nvarchar`, `nclob`, `decimal(19,4)`, table functions, in the schema northwind |

The extended price of an order line is rounded to cents in every file. DuckDB
and HANA cast the discount to a double and the result to `decimal(19,2)`,
because they keep single precision through a sum otherwise. SQLite rounds the
sum.

There is no `hr` file for these products. A second file with schemas, enums,
partitions and so on is written where the product has them, and none of these
five was asked for one.

## What was measured

Each file loaded on MySQL 26.7, MariaDB 13.0, SQLite (mattn), DuckDB, Firebird
5.0 and HANA 2.00 with 91 customers and 830 orders, and loaded twice in a row.
PostgreSQL 18, SQL Server 2025 and Oracle 26ai load the same way through usql.

- HANA ran out of memory on a second load after a first one failed, and loaded
  twice in a row after the container restarted.
- usql with the hdb driver cannot print a DECIMAL column. It stops with an error
  from the JSON encoder of the driver, even for `CAST(1.5 AS DECIMAL(9,2))`.
  That is a fault of usql or of the driver and not of the fixture.
- The Oracle hr file was not loaded through usql with the `\;` form, because the
  usql that dbmeta ran against does not have it yet.
