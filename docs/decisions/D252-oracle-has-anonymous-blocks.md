# D252. Oracle has anonymous blocks

Status: Amends D143.

transit built the option `AnonymousBlocks` for its usql grammar, and usql asked
for a flag here that says which dialect needs it. A statement that starts with
BEGIN or DECLARE is a block of code. It ends at the semicolon after its last
END, or at a line that holds only a slash.

## Decision

`Syntax.AnonymousBlocks` is true for Oracle and false for every other dialect.

## What was measured

Oracle 19c ran three blocks as one statement each: a BEGIN with two
statements, a DECLARE with a BEGIN, and a block with a loop that has its own END.
It refused half a block. `TestOracleRunsAnonymousBlocks` holds it to a server.

The others have no such statement, or it starts with another word:

- Firebird 5.0 refuses a bare BEGIN. Its block starts with EXECUTE BLOCK.
- SAP HANA 2.00 refuses a bare BEGIN and a bare DECLARE. Its block starts with
  DO.
- Exasol 2026.2 refuses a bare BEGIN.
- A bare BEGIN starts a transaction on PostgreSQL and the MySQL family, and
  DECLARE is a statement of its own on SQL Server.

transit made `BeginEndBlocks` treat EXECUTE BLOCK and DO BEGIN as openers
(transit D130), so Firebird and HANA need no flag here.
