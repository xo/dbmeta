# D8. Version differences are generated data, not packages

Status: Decided.

A metadata query must work against more than one release of a database. There
is one package per driver, and the version differences live inside it as
generated data.

This replaces an earlier form of D8 that gave each release its own package.
That form is abandoned. Two independent reviews called it a combinatorial
explosion and both were right. Do not reintroduce it.

## The shape

Each query is a list of fragments. A fragment carries the lowest server version
that it applies to and a piece of SQL:

```go
[]Fragment{
    {MinVersion: 120000, SQL: ...},
    {MinVersion: 150000, SQL: ...},
}
```

A selector reads the server version once per connection, merges the fragments
that apply, and runs the result. `SHOW server_version_num` returns the integer
that `psql` itself compares against, so 15.0 is 150000.

Version by feature range, not by release. Add a fragment when a query must
change. Do not add one per release.

## The rule that makes this work: pad the column list

A fragment must never change the set of columns that a query returns. When a
column has no source on a given server, the fragment still selects it, as a
literal NULL under the same name:

```sql
NULL AS "access_privileges"
```

The rule is symmetric, and an earlier version of this section got that wrong.
Both external reviews caught it. Padding is not only for a column that arrives
in a newer release. A column can exist on an older server and be removed from a
newer one, and then the newer fragment is the one that pads.

`pg_attrdef.adsrc` is the case. Commit `fe5038236c` in the PostgreSQL tree
removed it in 2018, so it is present up to release 11 and gone from 12. If the
canonical shape includes a field with no replacement, releases 12 and later pad
it. Most removals do have a replacement, as `adsrc` does in
`pg_get_expr(adbin, adrelid)`, and then both fragments select a real value
under the same name. The padding case is the one where nothing replaces it.

State it as: whichever side lacks a source pads, old or new.

## Padding does not cover a type change

A NULL pad fixes the presence of a column. It does not fix its type. Both
reviews raised this and it is a real limit of the mechanism.

If a column exists on both versions but its type differs, one Go field cannot
scan both. The fix is a cast in the fragment, so that every version returns the
same type under the same name, chosen to lose nothing:

```sql
CAST(col AS text) AS "col"
```

Do not reach for `any` or an empty interface to paper over this. That discards
the typed scanning the whole design rests on.

A NULL pad also needs a type where the database cannot infer one. PostgreSQL
accepts a bare `NULL AS name`, but a stricter database can require
`CAST(NULL AS text) AS "name"`. Write the cast when the server asks for it.

Gemini offered `pg_class.reltuples` as an example of a type change, saying it
went from `real` to `bigint` in release 14. That is wrong and it was checked:
it is still `float4` in the PostgreSQL tree. Release 14 changed its default and
its meaning, not its type. The category of risk is real even though that
example is not.

This rule exists because Go scans into a fixed struct. Without it, one query
returns 12 columns on one server and 13 on another, no single generated struct
fits both, and positional scanning breaks.

`psql` does not need this rule and mostly does not follow it. It renders a
table whose shape it discovers at run time, so it lets the column set vary. It
uses the padding technique once, at `describe.c:4506`, where it emits
`NULL AS "Access privileges"` for a server below 15 rather than the real
column. That one line is the technique. `dbmeta` applies it everywhere.

Verify this before you dismiss it. The version gates in `describe.c` add
columns rather than only rewriting expressions. Release 11 adds `pubtruncate`
as "Truncates". Release 13 adds `pubviaroot` as "Via root". Another gate adds
`am.amname` as "Access method". Each one changes the column count.

## What a model holds

A model is written against one concrete statement run on one live server. With
the padding rule, the newest supported version is that statement. It gives the
row struct and the scan code once per query, and both fit every version,
because every version returns the same columns in the same order.

The fragments are data beside the query, not something a server is asked
about.

## The subtlety to resolve

Padding makes two different facts look the same. A NULL now means either that
the value is genuinely null on this server, or that the column does not exist
at this version. A caller that cannot tell them apart will report a missing
feature as missing data.

Do not solve this with a sentinel value. Record, next to each query, which
fields are valid at the detected version, and let a caller ask. This is the
same mechanism D34 settles for a database that cannot answer at all, and the
two must be one mechanism rather than two.

## What this decision settles

Questions 3 and 4 are closed. There is no version directory to name, so the Go
rule about a trailing `/vN` import path element no longer applies. There is no
inheritance to arrange, because data needs none.

D2 still holds: one package per driver under `models/<driver>`. The version
segment that an earlier draft put below it is gone.
