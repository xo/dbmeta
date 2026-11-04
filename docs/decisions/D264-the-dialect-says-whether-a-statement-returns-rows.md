# D264. The dialect says whether a statement returns rows

Status: Decided.

usql chose between a query and an exec from the first words of a statement,
in `QueryExecType` of `drivers/qtype.go`. A statement with a RETURNING clause
on PostgreSQL, SQLite, MariaDB, CockroachDB and others, and one with an OUTPUT
clause on SQL Server, returns rows, and usql printed only a command tag. psql
prints the rows (xo/usql#540). Ken decided on 2026-11-04 that the
classification moves into dbmeta and reads the whole statement for each dialect.

## Decision

`Dialect.Classify(stmt) (tag string, rows bool)` takes the whole statement. It
returns the command tag that a client prints after an exec, and whether the
statement returns rows. A client passes a statement with rows to a query and
any other to an exec. `Info.EveryStatementIsAQuery` makes `rows` true, so a
client does not read that field again.

A small lexer reads the statement by the `Syntax` of the dialect. A string, a
quoted name and a comment is one token that no word matches, so `'returning'`,
`"returning"`, a comment and a dollar quoted body are not seen. A word counts
only at the top level, outside every parenthesis. `Syntax.Brackets` is new, for
a name between square brackets in SQL Server and SQLite.

`Info.Statements` holds what differs by product:

- `Returning` is `ReturningRows` for a RETURNING clause that sends rows to the
  client, `ReturningOutput` for SQL Server, where the OUTPUT clause sends rows
  unless it has an INTO, and `ReturningThenReturn` for Spanner. Oracle has none
  of them, because its RETURNING INTO puts the rows in variables.
- `SetListsSettings` is true for Impala and Hive, where SET alone, SET ALL
  and SET -v list the settings.
- `ExecuteReturnsRows` is true for SQL Server, where EXECUTE is EXEC.

## The rules

- A SELECT with an INTO at the top level, anywhere in it, returns no rows, with
  the tag SELECT INTO. This covers `SELECT ... INTO table` and `INTO @variable`.
  usql looked at the second word only.
- A WITH statement is classified by its first top level SELECT, VALUES, TABLE,
  INSERT, UPDATE, DELETE, MERGE, REPLACE or UPSERT. The CTEs sit in parentheses.
  A WITH that ends in an INSERT with no RETURNING returns no rows and has the tag
  INSERT, as psql prints it.
- A PRAGMA with an `=` returns no rows.
- ADMIN SHOW is a query. usql never matched it, because it looked up the first
  word only.
- A statement that starts with a parenthesis is a query.
- The tags are the table that usql held, in `classify_tags.go`.

The release does not matter. A release that lacks a clause refuses the
statement and the client prints the error.

## What was measured

`TestClassifyIsMeasured` in the `test` module runs statements against a server
and holds `Classify` to whether the result has columns. It covers PostgreSQL,
CockroachDB, SQLite, DuckDB, MariaDB, Firebird, SQL Server and Oracle.

## What was not measured

- Spanner's THEN RETURN is from the documentation.
- Oracle's RETURNING INTO needs an output variable, which the test does not
  bind.
- A string with a backslash escape, as MySQL reads `'it\'s'`, ends at the
  quote. A statement with one and a clause word after it can be misread.
- A procedure that returns no result set, such as `EXEC` of a batch that only
  sets NOCOUNT, has no rows, and Classify says it has. The client prints an
  empty result.
- DuckDB answers an INSERT, UPDATE or DELETE with a one column result named
  Count. Classify says no rows, as usql did, and the test holds that.
- Impala and Hive SET are from the pull request xo/usql#573 and were not run.
- EXECUTE of a prepared statement returns rows when the statement is a SELECT.
  Classify says no rows, as usql did.
