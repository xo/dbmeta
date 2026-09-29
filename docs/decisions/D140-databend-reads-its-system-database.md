# D140. Databend reads its system database

Status: Decided.

## The decision

Ken asked on 2026-09-30 for dbmeta to answer for Databend, because usql
deletes its own readers in W21 and read Databend through its
information_schema reader until then. `models/databend` is a native model
that reads the system database, as the ClickHouse model does, and not the
shared information_schema model.

Databend's information_schema reports an ordinal position of 1 for every
column, measured on 1.2.881 and 1.2.948, and it has no view of indexes,
constraints, functions, sequences or roles. system has all of those, and
the table function show_sequences() lists the sequences.

## How it is reached

usql's databend scheme opens dbimp's driver, which replaced databend-go in
dbimp v0.6.0, so the test module reads Databend through it (D52). The driver
refuses every key but tls, auth, cancel and timezone, so the dbrun entry's
connection string has no query. The two releases move from Staged to
Tested, which is the cadence they kept (D120).

## What is its own

- system.columns has no position, so a table's columns are numbered in the
  order system.columns lists them, which is the order they were declared
  in.
- A function, a procedure and a sequence belong to the tenant and not to a
  database, so each is reported with an empty schema.
- Index columns and constraint columns are read out of lists that system
  records as text.
- Databend orders a union by a column name and refuses a position, so those
  statements order by name.

docs/COVERAGE.md has the rest, and what a lesser user is refused.
