# D293. A dump reads what the dump fixture showed it missed

Status: Amends D291, amended by D294.

`TestDumpFixtureRoundTrips` (D291) loads the dump fixture, runs the script of its
snapshot on an empty database and compares the two snapshots. It showed gaps in
the dump of five products. These are the changes that close them.

## MySQL and MariaDB

- A snapshot of several databases names each one in every statement, creates
  each with `CREATE DATABASE`, and chooses it with `USE` before a view, a routine
  or a trigger. A product whose schema is its database had one default database
  before.
- A routine is written. The catalog keeps the body, the parameters and the
  declarations apart, so the dump builds `CREATE FUNCTION` or `CREATE PROCEDURE`
  from them, with the definer, the data access and the security. `Function` has
  the new fields `DataAccess` and `SQLMode`. A script sets `sql_mode` to the mode
  of the routine before it makes the routine, and sets it back after.
- A view keeps `WITH CASCADED CHECK OPTION` and `WITH LOCAL CHECK OPTION`.
- A view and a routine find each other by the names in their text, so a view
  that calls a function follows the function.

## PostgreSQL

- A trigger that is disabled, or set for the replica role, is written with
  `ALTER TABLE ... DISABLE TRIGGER`. SQL Server does the same with
  `DISABLE TRIGGER`. See D292 for range types.

## SQL Server

- A nonclustered columnstore index has no included columns. The catalog marks
  each of its columns as included, and the script no longer writes `INCLUDE`.

## SQLite

- `Table.Definition` holds the text of `CREATE TABLE` as SQLite keeps it, and a
  script of SQLite writes that text. STRICT, WITHOUT ROWID, AUTOINCREMENT, a
  deferred foreign key and a virtual table are kept, because no catalog lists
  them.
- The shadow tables of a virtual table, such as the content table of FTS5, are
  left out, because the module makes them.
- dbmeta reads the schema `main` of SQLite and no attached one, so the fixture is
  one schema and the tables of its second schema start with `extra_`.

## DuckDB

- A macro and a table macro are written, with `CREATE OR REPLACE MACRO`.
- An enum is written with `CREATE TYPE ... AS ENUM`.
- A nested value, which is a list, a struct, a map or a union, has a literal
  that the driver's Go value builds. `Style.ValueLiteral` writes it.

## Every product

- An INSTEAD OF trigger belongs to a view. `View.Triggers` holds it, and the
  script writes it after the view.

## What the test still shows

The files in `test/testdata/dump_fixture/` hold what is left, and
`docs/BACKLOG.md` lists it.
