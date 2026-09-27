# D76. SAP HANA reads SYS, and answers more than anything but PostgreSQL

Status: Decided.

`models/hana` answers 32 of the 55 against SAP HANA 2.0 SPS 08.
`docs/COVERAGE.md` holds the measurements. Four things had to be decided.

## Ken agreed to the SAP licence

SAP HANA, express edition will not start without `--agree-to-sap-license`,
which accepts the SAP Developer Center Software Developer License Agreement.
Ken agreed to it on 2026-09-26 for this project's test containers, and the
flag is in `container/hana.go` with that recorded beside it.

It is written down because it is the only product here that needs an
affirmative licence acceptance to run at all, and because the next person to
read that flag should not have to wonder who decided.

## Access methods are the row store and the column store

A HANA table is held by row or by column and the choice is per table. That is
the question a MySQL storage engine and a Trino connector answer, so
`AccessMethods` reports the two and `Tables` says which one each table uses,
reporting row table or column table rather than table.

It is an analogy and rule 14 says to leave one that is a stretch unsupported,
so the case for this one has to be made. It is not a stretch: the choice
decides how the table is stored, how it is scanned and what it is good for,
which is what an access method is. What makes it imperfect is that HANA keeps
no catalog of the kinds, so the query counts the tables that name each one and
a kind nothing uses does not appear. The field description says so.

The fixture builds a row table as well as a column table so that the answer is
never trivially one row, and `TestHANARowAndColumnStore` reads both.

## Subscriptions answers and Publications does not

This looks like an oversight and it is the product. HANA replicates by
subscribing to a remote source, so `SYS.REMOTE_SUBSCRIPTIONS` is the
subscriber half and there is no publisher object anywhere in the catalog.
Firebird is the other way round: it publishes and configures the subscriber
in a file. Recording both halves separately is why the two kinds are separate
kinds.

## The column grant column stays, and is always empty

`SYS.GRANTED_PRIVILEGES` has a `COLUMN_NAME` column and HANA 2.0 SPS 08 has
no `GRANT` syntax that fills it. All three spellings of a column list are a
syntax error, which was measured rather than read.

The decision is to keep reading the column rather than to drop it and hard
code an empty string. The catalog has it, a later release may fill it, and a
query that reads a column it cannot demonstrate is exactly the thing that rots
silently. So `TestHANAHasNoColumnGrant` asserts both halves: that the grant is
still refused, and that `column_access` is still empty. If SAP adds the
syntax, that test fails and tells somebody to look.

## What the measurement gave back

Eleven queries answer differently for a grantee, which is the most of any
product here, and four of those return the same rows with different values
rather than fewer rows. Functions, sequences, triggers and views all carry a
definition, and HANA returns the row and withholds the text from a reader
without the privilege. A consumer that treats a definition as always present
is wrong on HANA, and nothing but D61 would have found it.
