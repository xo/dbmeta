# D45. A query may answer partially, once, and must say so

Status: Decided.

`Constraints` on SQLite returns primary key, unique and foreign key rows and
never returns a check constraint. It is the only query in `dbmeta` that answers
part of a question rather than all of it or none of it.

## Why this one is allowed

D34 says a database that cannot answer reports `ErrNotSupported` rather than an
empty result, and D43 says an analogue that is a stretch is left unsupported.
Neither rule covers this case. SQLite can answer three quarters of the question
exactly, from pragmas, and the missing quarter is missing for a reason that
will not change: a check constraint exists only as text inside the
`CREATE TABLE` statement in `sqlite_schema.sql`, and `dbmeta` does not parse
DDL.

Both Gemini and DeepSeek were asked and both said the same thing. Three kinds
read exactly are worth more than refusing all four over the fourth. A caller
asking what constrains a table gets the primary key, the unique constraints and
the foreign keys, which is most of what it wanted.

## The conditions

A partial answer is allowed only when all four hold. The part that is returned
is exact, not approximate. The part that is missing is missing structurally,
not because nobody wrote the query yet. The field description names what is
missing, in the API, where a caller reads it. A test asserts the absence, so
that it stays a decision rather than becoming a bug.

The SQLite fixture creates two check constraints and `TestSQLiteConstraints`
asserts that neither appears. Without that test this would be indistinguishable
from a query that forgot them.

## What it is not a licence for

Do not use this to ship a query that half works. The question to ask is whether
a caller reading the result would be wrong about anything. Here it would not:
it would be missing something the field description told it would be missing.
A query that returns a wrong value, or that silently drops rows a caller would
expect, is not a partial answer. It is a defect.

## The rejection this sits beside

`Aggregates` on SQLite went the other way, and the contrast is the point.
SQLite reports `sum`, `count` and `group_concat` with the same type code as
`row_number` and `rank`, because both groups can be used over a window. Gemini
said to map that code to aggregates and called it exact. DeepSeek said to map
only the other code. Running it against a server showed that the first would
list `row_number` as an aggregate and the second would omit `sum`. Every
available answer is wrong about something, so there is no exact part to return,
and `Aggregates` is unsupported.

Exact but incomplete is allowed. Complete but wrong is not.
