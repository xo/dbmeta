# D249. Syntax says which dialects have BEGIN and END bodies

Status: Amends D143.

usql splits its input at each semicolon, and it sends each piece as a
statement. A stored program whose body sits between BEGIN and END holds
semicolons that do not end the statement, so the split cuts the body into
pieces and the server refuses each one. Four cases failed through usql:
a trigger on SQLite, and a procedure on MySQL, SQL Server and Oracle.

transit has the option `BeginEndBlocks` for its usql grammar. After CREATE of
a PROCEDURE, FUNCTION, TRIGGER or EVENT, it counts BEGIN and END, and a
semicolon inside the body does not end the statement. A transaction, written
`BEGIN; ...; END;`, stays split. Ken decided on 2026-11-01 that the knowledge
of which dialects need the option lives here, and not in a list in usql.

## Decision

`Syntax.BeginEndBlocks` is true when the body of a stored program can sit
between BEGIN and END with no quotes, and hold semicolons. It is in `Syntax`
because the client that reads it is the one that splits text, and it reads the
other lexical forms there. A client reads it with `Dialect.Syntax`.

A dialect that quotes the body, as PostgreSQL does with dollar quotes, is
covered by the quote flag. A dialect with no stored program is false.

## What was measured

`TestBeginEndBlocksIsMeasured` in the `test` module sends one CREATE, with a
semicolon inside the body, as a single statement. The answer must be the value
of the model. The test also sends the piece before the first semicolon alone
and logs the answer, which shows what a client sends when it splits there.
It skips a dialect when its variable, such as `DBMETA_MYSQL`, is not set.

True, and the server accepted the whole statement:

- SQLite3, libSQL and rqlite: a trigger.
- MySQL 26.7, MariaDB 13.0, SingleStore 9.1 and Vitess 24.0.5: a procedure.
  SingleStore writes `AS` before BEGIN.
- SQL Server 2025: a procedure.
- Oracle 26ai: a procedure. The piece before the first semicolon is also
  accepted alone, because Oracle creates the procedure with compile errors.
  The split does not fail there. It leaves a broken procedure.
- PostgreSQL 18 and CockroachDB 26.3.2: a function with `BEGIN ATOMIC`, which
  PostgreSQL added in release 14. A release before 14 refuses it, and the
  flag is true because a client cannot tell the release when it splits.
- Firebird 5.0, Exasol 2026.2 and SAP HANA 2.00: a procedure or a function.
- Trino 484: a function with `BEGIN`, in a catalog that stores functions.

False, and the server refused the form:

- TiDB 8.5.8 parses CREATE PROCEDURE and refuses to run it, error 8108.
  The model copies MySQL and sets the flag back to false.
- CrateDB 6.4.6, Vertica 25.1 and Databend 1.2: no such body. Vertica and
  Databend quote the body. CrateDB copies PostgreSQL and sets the flag back
  to false.
- Presto 0.299, ClickHouse 26.9 and DuckDB.

## What was not measured

BigQuery, Snowflake and Databricks are true from their documentation. No
server of them runs here without a credential, and nobody enters one. Their
value is a statement of the documentation and not a measurement. A person with
access runs `TestBeginEndBlocksIsMeasured` after adding a probe.

Every other dialect is false. It has no stored program with an unquoted body,
or its language has no CREATE of one. Redshift and Vertica quote the body.

## What this does not cover

An Oracle anonymous block, `BEGIN ... END` or `DECLARE ... BEGIN ... END`
with no CREATE, still splits with this option. transit holds it as a separate
question. If it lands, usql can need a second flag, and it is added here.
