# D40. Three support tiers, and a trigger that can remove a version

Status: Decided.

D20 keeps PostgreSQL back to 9.6, which is one release below what `psql`
supports. Both external reviews accepted that only on two conditions, and both
are adopted here.

## The tiers

`README.md` declares which tier every database version is in. `gen.go` writes
that table from the models, under D31, so it cannot drift from the code.

**Tested.** Tests run on every change, in CI. This is what D24 puts in CI: the
latest release of PostgreSQL, MySQL and SQLite3. A fault here is a bug and it
gets fixed.

**Verified.** Tests exist and run on a development machine before a release,
not on every change. This is the D24 local matrix. A fault here is a bug and it
gets fixed, and it is found later than a tested one.

**Archived.** The queries exist and were checked once against a real server,
and nothing runs them now. The tier records the date and the image digest of
that check. A fault here is fixed only if someone supplies a test with the
report.

Never write "supported" without saying which tier. Both reviews made this point
and DeepSeek put it plainly: a frozen untested path is an unverified
compatibility claim.

PostgreSQL 9.6 starts in Verified. It is in the D24 local matrix like every
other release below the latest, and its image runs today.

## The trigger

A version can leave. Without a rule that can fire, "we can always keep it"
means nothing can ever be removed.

D21 governs every version inside upstream support. It is unchanged.

For a version outside upstream support, which today is 9.6 alone, two triggers
apply and either is enough.

1. Its pinned image cannot be pulled or started on two consecutive attempts.
2. Its tests fail and are not fixed within one release cycle.

When either fires, the version moves from Verified to Archived. It is not
deleted at that point, because the queries still document how that release
answered and they cost nothing to keep once nothing runs them.

Deletion follows D21 only. A version leaves the tree when its queries obstruct
a change the supported versions need, and the release notes say so.

## Record where each query came from

A query translated from a tree that upstream no longer ships must say so, next
to the query: the release of the checkout and its commit.

This is not bookkeeping. Below release 10 there is no current `psql` to compare
against, so the provenance line is the only way a later reader can check the
translation at all. `QUERIES.md` says the same thing about reading the right
tree for the right release.
