# D274. A dump is built here as statements and run by the client

Status: Amends D5, D56 and D273.

usql asked on 2026-11-08 for a `\dump` command, a `--dump` flag, a target
dialect and a rewritten `\copy`, and dbmeta proposed a design in
[`DUMP.md`](../DUMP.md). The maintainers decided the points below on 2026-11-08.

## Decision

1. dbmeta builds the statements of a dump and a copy. This widens D5, which
   said that dbmeta does not render metadata, and D56 and D273, which allowed
   three statements that are built and not run. The rule that survives is the
   same one. dbmeta takes no connection for anything it builds and runs
   nothing. The client reads the rows, writes the file and runs the
   statements.
2. The code is in its own package, `dump`, in the root module. It uses the
   standard library and nothing else.
3. The first targets are PostgreSQL, SQLite, DuckDB and MySQL. SQL Server and
   Oracle follow. MariaDB follows MySQL and CockroachDB follows PostgreSQL.
4. dbmeta does no SQL translation, and neither does usql. dbmeta produces the
   statements that a client runs and does not care where they run. A view, a
   function, a procedure or a trigger is produced as the text of the source
   database, marked as not translated, for every target. It is not left out.
   The client decides what to do with it. usql warns, or stops, when it finds
   one and the target is not the source. Producing the data has no such
   barrier.
5. A type with no equivalent in the target fails by default, with an error that
   names the table, the column and the source type. The export stays possible.
   A client asks for a fallback and dbmeta reports what it did. The fallbacks
   are in the proposal and are refined with usql before they are built. For
   example, a float64 whose target has only a 32 bit float is a lossy mapping,
   and a client that allows it gets the warning. A client that asks for text
   gets the string type of the target for any type that has no numeric or
   other equivalent.

6. The idioms and the cast rules in `DUMP.md` are approved, and they serve the
   export only.
7. The `Snapshot` is the format that dbtpl moves to later.
8. A lossy mapping never happens without an explicit option that the client
   passes to `\dump`.
9. dbmeta adds the readers that the dump needs, and the snapshot reads get the
   cost test of D47 against a catalog with thousands of tables.

## Consequences

`dump.Statement` has a field that says the SQL is in the source dialect and was
not translated, and every mapping that is not exact has a note. The default for
a statement that cannot be built exactly is an error. A fallback never happens
without a flag that the client sets.

Nothing is built by this decision. Each step in `DUMP.md` is built, measured on
a real server and released on its own.
