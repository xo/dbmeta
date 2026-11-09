# D291. dbrun fixture dump makes a database that tests a dump

Status: Decided, amended by D293.

The maintainers agreed on 2026-11-10, with the `usql` session, to a fixture
that tests the dump of D274. It is separate from northwind and hr, because the
describe commands of `usql` compare their output with those two. A dump needs
objects that test the order of its statements, the quoting of names and the
variants of each kind of object. The specification is the `usql` item W82.
GPT-6 Astra, Gemini 3.1 Pro and DeepSeek reviewed it, and all three advised a
fixture that is separate from northwind.

## The fixture

`dbrun fixture dump <selector>` builds two schemas, so that an object can refer
to one in the other schema.

- PostgreSQL and SQL Server: the database `dump` with the schemas `core` and
  `extra`.
- MySQL and MariaDB: the databases `dump_core` and `dump_extra`.
- Oracle: the users `dump_core` and `dump_extra`.
- DuckDB: the file `dump.duckdb` with the schemas `core` and `extra`.
- SQLite: the file `dump.db`. dbmeta reads the schema `main` only, so the file
  is one schema and the tables of the second one start with `extra_` (D293).

Each product has one file in `test/cmd/dbrun/dump/`, with one section for each
area: types, routines, triggers, tables and indexes, dependencies and names.
MySQL and MariaDB share `mysql.sql` and add `mysql-only.sql` or
`mariadb-only.sql`. usql loads the files, so a file is plain SQL.

- Do not escape a semicolon as `\;`. If usql cannot load a statement, keep it
  and record it as a known skip, with the reason, in a comment at that place.
- Every product makes the table `people` in each schema, with rows. The table
  in `core` has a foreign key to its own table. The load fails if either table
  is empty.
- Each table has rows with a value in every column, with nulls and Unicode, so
  that a comparison of row values has data to compare.
- Where a product refuses an item, write a comment and use the nearest object,
  as D290 does.

## Every product

1. Foreign keys that refer to each other, and a foreign key to its own table.
2. A view that calls a function that reads a table.
3. A type in a function signature.
4. Names in mixed case, with a space, a reserved word, Unicode and an embedded
   quote.
5. One name used at once for a schema, a table, a column and a type.

## PostgreSQL

1. Types: a range type and its multirange, a domain over an enum, and an enum
   that gains a value with `ALTER TYPE ... ADD VALUE ... BEFORE`.
2. Routines: overloaded functions, OUT and INOUT parameters, RETURNS TABLE and
   SETOF, default arguments, SECURITY DEFINER, each volatility, and a SQL body
   written with BEGIN ATOMIC.
3. Triggers: a statement trigger, a WHEN condition, UPDATE OF a column, INSTEAD
   OF on a view, a deferrable constraint trigger, transition tables, a trigger
   on a partitioned table, a disabled trigger and an event trigger.
4. One user function that a check, a default, a generated column and an
   expression index all call.
5. Tables and indexes: a default partition, a subpartition, an index on the
   partitioned parent, EXCLUDE USING gist, an index with INCLUDE, and a column
   dropped after the table is created.
6. A view and a function that depend on each other in a loop.

## SQL Server

1. Functions: an inline and a multi-statement table-valued function, and a
   function WITH SCHEMABINDING that a view in the other schema uses.
2. A computed column that calls a user function.
3. Triggers: INSTEAD OF on a view, a database DDL trigger, a disabled trigger,
   and a trigger order set with `sp_settriggerorder`.
4. Procedures: OUTPUT parameters, parameter defaults, and a procedure of
   several statements with no BEGIN ... END around them. A dump copies that
   definition as written, and a client that splits at a semicolon can cut it.
5. Constraints and indexes: named default and check constraints, a filtered
   index, included columns and a columnstore index.
6. An XML schema collection, a security policy and a NEXT VALUE FOR default.

## Oracle

1. Types: a type body with methods, inheritance with UNDER, a VARRAY, a nested
   table column with STORE AS, and a REF column.
2. Triggers: a compound trigger, INSTEAD OF on a view, a statement trigger,
   UPDATE OF, WHEN, and a schema DDL trigger.
3. Routines: overloads in a package, a pipelined table function, AUTHID
   CURRENT_USER, and a deterministic function that a function-based index and
   a virtual column use.
4. Tables: an index-organized table, invisible columns, a DEFAULT
   `seq.NEXTVAL`, a deferrable foreign key and a materialized view log.
5. A view that calls a package function. The spec must load before the view,
   and the body can load after it.

## MySQL and MariaDB

1. The SET type, an EVENT, and two triggers on the same event ordered with
   FOLLOWS and PRECEDES.
2. Routines with DEFINER and SQL SECURITY INVOKER, routines marked
   DETERMINISTIC and NO SQL, and routines created under different `sql_mode`
   values.
3. Views: one WITH CHECK OPTION, a view on a view, and a view that calls a
   function.
4. Invisible columns, a character set and a collation on one column, and a
   spatial index.

## SQLite

1. Triggers: INSTEAD OF on a view, WHEN and UPDATE OF.
2. Tables: STRICT, WITHOUT ROWID, AUTOINCREMENT with its row in
   `sqlite_sequence`, and deferred foreign keys.
3. An FTS5 table. FTS5 creates hidden tables of its own, and a dump must not
   create them again.

## DuckDB

The MAP, UNION and fixed ARRAY types, a struct that holds a list of maps,
table macros, `nextval` defaults and foreign keys.

## The test

The round trip loads the fixture, dumps it, and loads the dump into an empty
database. It then makes four comparisons:

1. The snapshot of dbmeta for the copy against the snapshot for the source.
   This compares every object that dbmeta reads.
2. The rows of each table, by their values and not by their count.
3. The dump of the copy against the dump of the source.
4. What each object does: call each function, fire each trigger, and read the
   order of each enum and the next value of each sequence.

It is not decided whether the whole test lives in dbmeta, in usql or in both.
usql runs it with usql as the client, because the dump is a usql script.

dbmeta has the first comparison, as `TestDumpFixtureRoundTrips` in the `test`
module. The fixture holds objects that a dump does not write yet, so the test
does not stop at the first statement that fails. It writes down each statement
that failed and each difference between the snapshots, and compares that text
with a file in `test/testdata/dump_fixture/`. An empty file means a perfect
copy. A line that goes away means that the dump got better. The test covers
PostgreSQL, SQL Server, MySQL and DuckDB, and `docs/BACKLOG.md` lists the gaps
that it shows.
