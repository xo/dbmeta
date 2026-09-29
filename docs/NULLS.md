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

Four layers, and only the last two found any of these.

`TestNoCoalesceOnCatalogColumns` in the model package reads the statement text
and fails on a `COALESCE` around a column named `access`, `comment`, `default`
or `options`.

`TestPaddedFieldsAreNull` in the `test` module runs every PostgreSQL query
against each PostgreSQL release and asserts that a field whose `Field.Min` is
above the server version is NULL in every row. This is the one that works, and
it is also what replaces a golden file per release: ten releases times
fifty-five queries is five hundred and fifty combinations that nobody would
maintain, and `Field.Min` already declares what each of them should contain.

Running the queries. Every fault above was invisible to a test that only
assembled SQL.

`scanEveryQuery` in the `test` module reads every query a model supports
through its own Scan, with the system objects included. It found the other
half of the problem on 2026-09-29: a field that was a plain `string` or
`bool`, and a catalog that returns NULL for it. The CockroachDB databases and
tablespaces queries failed on every row, and PostgreSQL 18 failed on a cast
with no function and on the column of an expression index. Every other test
passed, because none of them scanned those rows.

Twenty two fields became `sql.Null`: a database's size, a tablespace's owner,
location and size, a cast's function and leakproof flag, an index column's
name, a collation's collate, ctype and deterministic flag, an access method's
handler, a function's result type, argument types and owner, an extended
statistic's name and owner, a view's definition, a type's owner, an
operator's function, and a privilege's schema, column access and policies.
HANA grants on a remote source, which has no schema. Where a model had no
source for one of these fields, it now selects NULL rather than an empty
string.

On Oracle it found that every `''` and every `NVL(x, '')` arrived as NULL,
which is the Exasol case below. So the Oracle model now scans a plain string
field through the same helper, `dbmeta.NullAsEmpty`, which is in the root
package because two models need it (D127).

It found two type faults as well. An Oracle sequence has 28 digits and an
int64 holds 19, so the default maximum, 9999999999999999999999999999, did
not scan. A sequence's start, minimum, maximum and increment are decimal text
now, on every database. HANA keeps those numbers as DECIMAL, and its driver
hands a DECIMAL over as a value that database/sql cannot convert, so the HANA
model casts them to BIGINT.

## For other databases

This is not a PostgreSQL problem and the next model will meet it.

MySQL reports an empty string where the standard says NULL in several
`information_schema` columns, so the same value means different things in
different databases. Oracle famously does not distinguish an empty string from
NULL at all, which means a query there cannot report the difference and the
model must say so rather than pretend. SQL Server and the rest each have their
own corners.

Exasol is Oracle's case with a twist the driver adds. It reads `''` as NULL,
so a filter written `? = ''` is never true and matched nothing at all, and a
statement that selects `''` for a field that is always empty gets NULL back,
which a plain Go string cannot scan. The Exasol model tests a filter with
`IS NULL`, scans a plain string field through `dbmeta.NullAsEmpty`, which
reads NULL as empty, and restores the three fields where the empty string means something,
such as a column that is not an identity, from the row. A field typed
`sql.Null` keeps its NULL. That is not the COALESCE above: on Exasol NULL is
the only way to write the empty string. D87 has it.

Vertica keeps the two apart, as PostgreSQL does, and still writes `''` in some
catalog columns to mean absent: a column with no default, and a function with
no definition or no comment. The model turns those into NULL with `NULLIF`,
because there the empty string is the catalog's way of saying there is
nothing, not a value somebody set.

Cassandra was the worst of them, and the driver was why. CQL has a null, and
the old driver, `go-cql-driver`, handed `database/sql` an empty string for
one. `SELECT (text)NULL` came back valid and empty, every padded column looked
present and empty, and the conformance projection reported `has_default=true`
on a database that has no defaults. The model scanned a padded column into a
target that discards the value, so that the field kept its invalid Null. See
D62.

The tests now use `github.com/xo/cql`, which reports a null as one. A real
catalog column that is null now reaches the caller as NULL. The change also
exposed a Scan that read a padded column into a plain string, which only the
old driver's empty string had kept working. See D93.

ScyllaDB goes through the same driver and adds one more limit. It accepts no
literal in a select list, not even `(text)NULL`, so a padded column cannot be
selected as NULL at all. The ScyllaDB fragment selects a real column of the
same table in its place, and Scan discards it on both products, the same way
it discards a padded column on Cassandra. The field keeps its invalid Null.
See D91.

The lesson generalises. Before trusting a padded NULL, check what the driver
does with one, not only what the database does. The two are different
questions and only the second is in the manual.

So the rule for a new model is: find out what the database means by NULL in
each column before choosing a Go type, find out what the driver does with one,
write the fixture so that both the absent and the empty case exist, and let
`TestPaddedFieldsAreNull` and its equivalents run against a real server before
believing any of it.

[`sql.Null[string]`]: https://pkg.go.dev/database/sql#Null
