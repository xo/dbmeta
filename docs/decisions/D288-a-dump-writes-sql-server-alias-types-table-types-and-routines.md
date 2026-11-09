# D288. A dump writes SQL Server alias types, table types and routines in order

Status: Decided.

## Decision

A dump of SQL Server writes its alias types, its table types, its synonyms and its
routines, so that the northwind fixture copies into an empty database with the same
objects and the same rows. The `usql` session found the gap on 2026-11-09, and Ken
asked for a dump with no exclusions.

## Alias types and table types

- A column of an alias type has `data_type` such as `[hr].[email]`, with the
  schema, and no collation, because the server refuses COLLATE on it.
- `Domains` for SQL Server gives the base type with its length, such as
  `nvarchar(254)`. The reader keeps it as a domain, and the style writes
  `CREATE TYPE ... FROM ... NOT NULL`.
- A table type has a table of its own in `sys`, with the type TT. The queries of
  columns, constraints, indexes and their columns name it by the type, and put it
  in the schema of the type. The reader gives each table type a place before it
  reads columns, so the readers that fill a table fill it too. `Schema.TableTypes`
  holds the result.
- A key of a table type has a name that the server made, so the script writes it
  with no name.

## Routines, views and synonyms

SQL Server reads the tables that a function names when it creates the function.
`Style.RoutinesAfterTables` says so. A routine that a column, a default or a check
calls is written before the tables, and so is a routine that one of these calls.
The other routines are written in the Post stage with the views, each after the
objects that its text names. The text names are found by words and not by a parser,
so a name that two schemas share finds both, which only makes the order stricter.

`dbmeta.Synonyms` reads `sys.synonyms`, and the script writes `CREATE SYNONYM`.

## Keys, indexes and comments

- A key has NONCLUSTERED or CLUSTERED when its index is not stored as the default for
  a key (`Style.KeyKind`).
- An index has CLUSTERED, NONCLUSTERED or COLUMNSTORE.
- A comment is the `MS_Description` extended property.
- A `uniqueidentifier` that the driver gives as 16 bytes is in the order of the
  server, and the loader puts it in the order of the text (`Style.MixedEndianUUID`).
- A table that a partition scheme lays out is written as one table, and the snapshot
  says so in `Skipped`.

## What it does not do

- Roles and grants are not read. See `docs/BACKLOG.md`.
- A partition function and a partition scheme are not written.
