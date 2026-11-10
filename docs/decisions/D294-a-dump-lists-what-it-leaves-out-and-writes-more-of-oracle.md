# D294. A dump lists what it leaves out and writes more of Oracle

Status: Amends D293, amended by D295 and D296.

`usql` ran the round trips of the dump fixture on v0.43.0 and sent what was left.
These are the changes that answer it.

## Warnings

A dump lists each object that the model reads and the dump does not write, as an
entry of `Snapshot.Skipped`, so that a client can warn about it.

- An event trigger of PostgreSQL and Oracle, and a database trigger of SQL
  Server, which fire on a DDL event.
- A trigger that belongs to no table or view.
- A row level security policy and a security policy.
- A PostgreSQL aggregate, and a routine that has no statement that creates it.
- A DuckDB type that is not an enum, such as a struct or an alias.

A MySQL event, an XML schema collection of SQL Server and a materialized view log
of Oracle have no kind in dbmeta yet, so the dump cannot list them. An attribute
that is dropped, such as the order of triggers, has no entry either.

## MySQL and MariaDB

- `Loader.SelectAll` names the database of the table, so a dump of several
  databases reads each table from its own.
- A foreign key to a table of another database names that database, whether the
  snapshot holds the other database or not.

## DuckDB

- A table that has a foreign key to itself loads one row in each INSERT, because
  DuckDB checks each row against the rows before it in the same statement.

## Oracle

- `Column.NestedStorage` names the table that stores a nested table column. The
  script writes `NESTED TABLE ... STORE AS`, and the storage table is not a table of
  the dump.
- `Column.Invisible` marks an INVISIBLE column. The model lists it, and the script
  writes `INVISIBLE`. The ordinal of such a column has no `column_id`, so it
  comes after the others.
- `Table.OfType` names the object type of an object table, and the script writes
  `CREATE TABLE ... OF`. A key on a hidden column that the system made is left out.
- An index organized table is written with `ORGANIZATION INDEX`.
- A foreign key keeps `DEFERRABLE` and `INITIALLY DEFERRED`.
- An index on a virtual column is written on the column, because 21c refuses an
  index on an expression that a virtual column holds.
- A view or a routine that names an object of another schema gets the grant that
  Oracle wants, `SELECT` or `EXECUTE`, before it.
