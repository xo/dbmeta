# Nulls, Padding and Types

Read this before writing a query for any database. It is short, it cost three
rounds of real bugs to learn, and every one of those bugs was found by running
against a real server rather than by reading the SQL.

The whole of it is one question asked twice.

## The rule

**Never hide a NULL.** A database returns NULL to mean something. Turning it
into an empty string, a zero or a false throws that meaning away, and nothing
downstream can get it back.

There are exactly two ways this package loses a NULL, and both have bitten.

## Mistake one: COALESCE over a nullable catalog column

Writing `COALESCE(x, '')` so that a Go field can stay a plain `string`.

This is the bug that cost the most. PostgreSQL reports three states for an
access control list:

| Catalog value | Means | psql prints |
| --- | --- | --- |
| NULL | the default privileges apply, so the owner has everything | blank |
| empty list | every privilege was revoked, so nobody has anything | `(none)` |
| a list | explicit grants | the list |

`COALESCE(array_to_string(relacl, E'\n'), '')` collapses the first two. "The
owner has full access" then reads exactly like "nobody has any access".
Demonstrated on a live PostgreSQL 18 server:

```
    relname    | acl_is_null | acl_len | psql_shows | dbmeta_showed
---------------+-------------+---------+------------+---------------
 default_privs | t           |      -1 | <null>     |
 revoked_privs | f           |       0 | (none)     |
```

**Do this instead.** Let the NULL through and give the field the type
[`sql.Null[string]`],
which is `sql.Null[string]`. Reading `.V` prints empty for an absent value, so
a command line client behaves as it would have. Reading `.Valid` recovers the
difference for anyone who needs it, and a code generator does.

**COALESCE is still right over an aggregate that matched no rows.** "No
members" and "an empty member list" are one answer, so
`COALESCE((SELECT string_agg(...)), '')` is correct. The test that guards this
checks the column, not the count.

## Mistake two: padding with a literal instead of NULL

D8 requires every version of a query to return the same columns, so a server
too old to have a column selects a placeholder under the same name. Writing
`, '' AS "identity"` rather than `, NULL AS "identity"` is the same bug in a
different coat: "this release has no such field" then reads as "the field is
present and empty".

**Do this instead.** Pad with `NULL`, cast where the database needs a type:
`NULL::bigint`, `NULL::boolean`.

## The distinction that decides which applies

Before padding, ask one question:

> On the old release, is the value unknown, or is it genuinely this?

**Unknown.** The catalog column does not exist, so there is no answer. Pad with
NULL, make the field nullable, and set `Field.Min`. Examples: a column's
identity kind below release 11, a collation's provider below 10, a sequence's
bounds below 10.

**Genuinely this.** The release had one behaviour and that behaviour is the
answer. Do not pad, do not set `Field.Min` to hide it, and say so in the
field's description. Three examples. A publication publishes no truncate below
release 11, because truncate could not be replicated at all. A role membership
always inherits below release 16, because there was no other option. A
collation is always deterministic below release 12.

Getting this backwards in either direction is wrong. Reporting NULL for a value
the release genuinely had loses information. Reporting a literal for a value
the release never had invents it.

## How this is caught

Three layers, and only the last one found any of these.

`TestNoCoalesceOnCatalogColumns` in the model package reads the statement text
and fails on a `COALESCE` around a column named `access`, `comment`, `default`
or `options`.

`TestPaddedFieldsAreNull` in the `test` module runs every query against a real
server at every supported release and asserts that a field whose `Field.Min` is
above the server version is NULL in every row. This is the one that works, and
it is also what replaces a golden file per release: ten releases times
forty-eight queries is four hundred and eighty combinations that nobody would
maintain, and `Field.Min` already declares what each of them should contain.

Running the queries. Every fault above was invisible to a test that only
assembled SQL.

## For other databases

This is not a PostgreSQL problem and the next model will meet it.

MySQL reports an empty string where the standard says NULL in several
`information_schema` columns, so the same value means different things in
different databases. Oracle famously does not distinguish an empty string from
NULL at all, which means a query there cannot report the difference and the
model must say so rather than pretend. SQL Server and the rest each have their
own corners.

So the rule for a new model is: find out what the database means by NULL in
each column before choosing a Go type, write the fixture so that both the
absent and the empty case exist, and let `TestPaddedFieldsAreNull` and its
equivalents run against a real server before believing any of it.

[`sql.Null[string]`]: https://pkg.go.dev/database/sql#Null
