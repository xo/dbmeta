# D60. The Oracle model reads ALL_ views, and there is no DBA_ variant

Status: Decided.

Every Oracle query reads an `ALL_` view. Two read `DBA_`, because the fact is
in no `ALL_` view at all. Whether `dbmeta` must also offer a `DBA_` variant of
the rest is open, and this decision records the evidence rather than settling
it.

## What the model does today

Prefer `ALL_`. Read `DBA_` only where the fact is not in an `ALL_` view, and
say so in the field or query documentation so a reader knows why a permission
error is possible. Do not reach for `DBA_` to get more rows out of a question
`ALL_` already answers.

`Roles` and `RoleGrants` are the two the rule is for. They are not absent from
Oracle: they live in `DBA_ROLES` and `DBA_ROLE_PRIVS`, and there is no `ALL_`
equivalent, because an ordinary user sees only its own roles through
`USER_ROLE_PRIVS` and `SESSION_ROLES`. Answering them at all means reading
`DBA_`. Neither is written yet, and the rule is what they will be written
against.

`V$` follows the same rule and already does. The version query reads
`v$version` because no dictionary view carries the banner before 18c, and
`Settings` will read `V$PARAMETER` for the same reason when it is written.

## Why a DBA_ query returns an error rather than nothing

A query that is unsupported says the product has no such object. Here that is
a lie: Oracle has roles, and a connection with the privilege can list them.
Reporting `ErrNotSupported` tells a caller to stop asking, when the truthful
answer is that this particular connection cannot see it.

So the query exists, it reads `DBA_`, and an unprivileged caller gets
`ORA-00942: table or view does not exist` passed back. That is a fact about the
connection rather than about the database, and the caller is the one who can do
something about it.

This is the third behavior this project has met for the same situation, and
they are worth keeping apart. PostgreSQL shows a caller everything. SQL Server
narrows the answer silently, which `models/sqlserver` documents. Oracle refuses
outright, and that refusal is the most useful of the three, because nothing is
hidden and nothing is guessed.

## What was open, and what closed it

An `ALL_` view and its `DBA_` twin are the same question asked with two
different privileges, and the difference is invisible to the caller.

Measured on 26ai against `ALL_TAB_COLUMNS` and `DBA_TAB_COLUMNS`:

| Connection | ALL_ rows | DBA_ rows |
| --- | --- | --- |
| a full DBA | 2255 | 2255 |
| a `SELECT_CATALOG_ROLE` user | 180 | 2255 |
| a user with no grant on the schema | 0 | ORA-00942 |

The column sets are identical, 91 and 91. So the two differ in rows only, and
only for a caller who is not a DBA.

The third row is the problem. An `ALL_` query returns no rows where the caller
cannot see the schema, and no rows is also what an empty schema returns and
what a schema that does not exist returns. A caller cannot tell the three
apart. `DBA_` raises an error in the same case, which is a worse answer for a
caller who has no privilege and a better one for a caller who has it and typed
the name wrong.

`usql` often knows which case it is in, because it knows who connected. A
consumer that knows it is a DBA prefers to ask `DBA_` and get the error.

Three mechanisms are sketched and none is chosen:

1. A keyed fragment, so the same query reads `DBA_` when the caller says so.
   This keeps one query and one column set, which rule 3 already asks for. It
   needs a gate that is not a version, and no such gate exists today.
2. An option on `Meta`, set when the caller is built. This puts the choice
   where the dialect and the versions already sit, and it makes the choice a
   property of the connection rather than of the call.
3. A second dialect, `oracle-dba`. This is the least code and the worst
   answer, because it doubles a model that is otherwise identical.

None was chosen, and D61 is the reason. The parity harness measured what a
principal actually gets, and an Oracle local user that owns the objects
receives the administrator's answer to every query. There was no gap to close
for the case that matters.

So the model stays `ALL_` only and nothing is added. What remains unanswered
is the third row of the table above: a caller asking about a schema it has no
grant on gets no rows, and cannot tell that from an empty schema or from one
that does not exist. `Tablespaces`, `Collations` and `Databases` stay
unsupported for the same reason, which `docs/COVERAGE.md` records.

Reopen this if a consumer asks for the difference. The three mechanisms above
are the candidates and the measurements are here.
