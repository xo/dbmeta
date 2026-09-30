# D6. Fix the NULL scan defect once, and never hide a NULL

Status: Decided, amended in place, D6a superseded by D71.

## The amendment, and the bug that forced it

The first PostgreSQL model met this decision by writing `COALESCE(x, '')`
around every nullable column, so that every Go field could stay a plain string.
That was wrong and it shipped a real fault. It is corrected here rather than
quietly rewritten.

PostgreSQL reports three states for an access control list. A NULL means the
default privileges apply, so the owner has everything. An empty list means
every privilege was revoked, so nobody has anything. A list means explicit
grants. `psql` handles all three, and `printACLColumn` in `describe.c` prints
the empty case as `(none)`.

Coalescing the first two into an empty string makes "the owner has full access"
read exactly like "nobody has any access". This was demonstrated on a live
PostgreSQL 18 server:

```
    relname    | acl_is_null | acl_len | psql_shows | dbmeta_showed
---------------+-------------+---------+------------+---------------
 default_privs | t           |      -1 | <null>     |
 revoked_privs | f           |       0 | (none)     |
```

## The rule

Never wrap a nullable catalog column in `COALESCE`. Let the NULL through and
give the field the type `sql.Null[string]`. There is no alias. See D51.

`COALESCE` is still correct over an aggregate that matched no rows, because
there NULL and empty mean the same thing. "No members" and "an empty member
list" are one answer. Sixteen such wrappers remain and they are right.

Reading `.V` prints empty for an absent value, so a command line client behaves
as it did. Reading `.Valid` recovers the difference for a caller that needs it,
and a code generator does: a column with no default is not a column whose
default is the empty string.

`TestNoCoalesceOnCatalogColumns` in the model package guards the rule.

# D6a. The original decision: fix the defect at generation time. Superseded by D71.

It said the NULL scan fix belonged in a generator's flags rather than in hand
written patches. There is no generator, which D71 records, so there is nowhere
for it to go but the code. The rule that replaced it is in D6 above and in
`docs/NULLS.md`: a field that can be absent is declared `sql.Null[T]`, and a
`TestNoCoalesceOnCatalogColumns` in the model package guards it.

See "Known defects to fix once" in `docs/PLAN.md` for the evidence that produced it.
