# D289. A dump of Oracle

Status: Decided.

## Decision

Oracle is a source and a target of a dump. The schemas NORTHWIND and HR of the
northwind fixture copy into schemas of the same database with the same objects and
the same rows, which `TestOracleCopiesTheNorthwindFixture` checks. Ken asked for it
on 2026-11-09.

## What the models read

- `Columns.data_type` is the declared type, such as `VARCHAR2(200 CHAR)` and
  `NUMBER(14,2)`, and a type that a person made has its owner in quotes. It was
  the name of the type alone.
- `Columns.identity` is `always` or `by default`. It was `YES`.
- `Sequences.start` is `last_number`, the value that the next `NEXTVAL` gives.
- `dbmeta.Source` gives the lines of `all_source`, which hold the text of a function,
  a procedure, a package with its body, a type with its body and a trigger. The
  reader joins them and writes `CREATE OR REPLACE` before the text.
- `dbmeta.TypeAttributes` gives the attributes of an object type. A collection type
  has its element in `Types.elements`.
- `dbmeta.MaterializedViews` gives the query and the BUILD, REFRESH and ON clauses.
  The table of the same name that holds the rows is not a table of the dump.
- `dbmeta.Synonyms` reads `all_synonyms`.

## How a script runs

- A schema is a user. The script makes it with `NO AUTHENTICATION`, which needs 18c,
  and grants `CREATE MATERIALIZED VIEW` and `CREATE TABLE`, so that the owner can
  have a materialized view. The owner of the script gives the password later.
- Every object is written with its schema. The text of a view, a trigger and a
  routine names its tables with no schema, so the script runs `ALTER SESSION SET
  CURRENT_SCHEMA` before such an object (`Style.UseSchema`).
- A foreign key to another schema needs `GRANT REFERENCES` to the owner of the child
  table first (`Style.ReferenceGrant`).
- The semicolon after `END` belongs to a block, and every other closing semicolon is
  taken off (`Style.KeepTerminator`).
- Types, packages and routines follow the rule of D288. A type that a column uses is
  written before the tables.
- An identity column is `BY DEFAULT` while the rows load. The Post stage gives it its
  kind and a start of `LIMIT VALUE`, which needs 12.2 (`Style.IdentityAfter`).
- A statement holds one row, because a release before 23ai has no list of rows in
  `VALUES`.
- A date is `DATE '...'` and a timestamp is `TIMESTAMP '...'`, so no format of the
  session matters (`Style.TimeLiteral`). The session time zone is UTC.
- A binary value of more than 2000 bytes is a temporary BLOB that a block fills,
  because Oracle has no such literal and cannot join two (`Style.LongBinary`).
  A text of more than 4000 bytes is joined `TO_CLOB` pieces.

## Object types and collections

The driver cannot read an object column. The reader gives such a column to the loader
as the text of the expression that builds the value, such as
`"HR"."POSTAL_ADDRESS"('1 Market St','Seattle')`, and the loader writes the text as it
is (`Style.SelectColumn`, `Style.ExpressionColumn`). The expression comes from the
attributes of the type. A collection is joined with `LISTAGG`, which holds 4000
characters. A value of such a column has no placeholder form, so `Loader.Args` refuses
it.

## What it does not do

- Roles, grants and the users themselves are not read. The Oracle model has no roles
  query. See `docs/BACKLOG.md`.
- A partition is written by its bound. Subpartitions, interval partitions and
  reference partitions are not.
- The storage clauses of a table, an index and a materialized view are not written.
- Only 26ai Free was run. The other releases of the tiers are not measured.
