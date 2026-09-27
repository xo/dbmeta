# D20. PostgreSQL goes back to 9.6. Every other database starts at the maintained floor

Status: Decided.

There are two rules here, not one, because PostgreSQL is not an ordinary
database in this project.

**PostgreSQL: support 9.6 and newer.** The ceiling is the newest stable
release, 18 today. That is ten major versions: 9.6, 10, 11, 12, 13, 14, 15, 16,
17 and 18.

The unit is the major version, not the point release. `dbmeta` does not treat
14.1 and 14.2 separately. This is safe rather than convenient: every one of the
11 version gates in `describe.c` sits on a major boundary, and PostgreSQL does
not change a catalog in a patch release, because that would change the on disk
format. `EVALUATION.md` records the check and the command to repeat it.

Note that "major" means two different things across this range. Before release
10 a major version is the first two numbers, so 9.5 and 9.6 are different
majors. From 10 onward it is a single number. `EVALUATION.md` explains what
that does to the integer the server reports.

**Every other database: the floor is the oldest release with a maintained
container image**, which usually matches the oldest release its vendor still
supports. See `EVALUATION.md` for the method and for the evidence behind each
choice.

## Why PostgreSQL is special

D9 makes `psql` the primary model. The goal of `usql` and of this project is to
bring the `psql` command line experience to every other database. Compatibility
with `psql` is the product, so PostgreSQL is not one supported database among
many. It is the specification.

The whole of PostgreSQL is also open. Every release is public, the catalog
history is readable, and `describe.c` states its own version rules. Nothing has
to be guessed. That is not true of Oracle or SQL Server, where old behavior can
only be learned by running an old server.

So for PostgreSQL the ordinary cost argument does not apply. Go as far back as
the source allows, and 9.6 is the line.

## Upstream moved, and the floor was reviewed

The checkout was updated on 2026-09-24 from a release 15 tree to `master`,
which is release 20 under development. One upstream change bears on this
decision and it was reviewed rather than absorbed quietly.

Commit `831bec45924`, on 2026-07-02, removed every `psql` code path for a
server older than release 10. Its message gives the reason:

> Our current policy is to support at least 10 previous major versions, so this
> bumps the minimum to v10 for the v20 release.

## What upstream actually gave as its reason

The commit messages were read, because the reason matters more than the fact.
There is no metadata specific rationale anywhere. The removal was one commit
covering all of `psql`, and metadata was simply the largest part of it: of 288
deleted lines, 244 were in `describe.c`.

The 2026 reason is policy. `831bec45924` says in full:

> Per discussion, it seems like a good time to bump the minimum supported
> version for various applications. Our current policy is to support at least
> 10 previous major versions, so this bumps the minimum to v10 for the v20
> release.

It does not claim that the old servers cannot be tested, or that the old code
was wrong, or that maintaining it was costly. It cites a policy about how far
back to support.

The 2021 reason was technical, and it is the one that would transfer. Commit
`cf0cab868a`, which set the previous cutoff at 9.2, says:

> Per discussion, we'll limit support for old servers to those branches that
> can still be built easily on modern platforms, which as of now is 9.2 and up.

That is a real constraint: a server you cannot build is a server you cannot
test against. It would apply to `dbmeta` too, and it is exactly the condition
D40's trigger watches for. It is not the reason given in 2026, and 9.6 was
verified to run on this host on 2026-09-24.

PostgreSQL also sets a precedent for keeping a capability it cannot easily
test. The matching pg_dump commit from 2021, `30e7c175b8`, notes in passing:

> (As in previous changes of this sort, we aren't removing pg_restore's ability
> to read older archive files ... though it's fair to wonder how that might be
> tested nowadays.)

So upstream removes the code that reads from an old server, and keeps the code
that reads an old format, on the grounds that the second costs little once
written. That is close to the argument for keeping 9.6 here.

The conclusion is narrow and worth stating exactly. Upstream's 2026 reason is a
scope policy for a C project with a ten version commitment. It is not evidence
that these queries cannot be maintained or cannot be tested, and `dbmeta`
adopting it would be copying a conclusion without its premise.

## The gap is one release, not four

State the size of the problem before arguing about it, because it is smaller
than it first appears.

`psql` in release 20 supports servers from release 10, which
`command.c:4489` confirms with `pset.sversion < 100000`. Releases 10 and 11 are
inside that window and the current tree still describes both. Only 9.6 falls
outside it.

So the question is whether `dbmeta` supports one release that upstream `psql`
no longer does, and the answer costs one extra source tree, not four.

## Verified: 9.6 runs today

This was flagged as unverified twice in earlier drafts. It is now checked.

`docker.io/library/postgres:9.6` was pulled and started with podman 6.1.2 on a
current Linux host on 2026-09-24. It became ready in four seconds, reported
`server_version_num` 90624, and both `\d` and `\dt` returned correct output
against a real table. The image is frozen, not broken.

## The review disagreed, and the useful part is where it agreed

Two models were asked. Gemini said drop the old releases and follow `psql`.
DeepSeek said keep them as a named legacy tier with scheduled tests, and drop
them otherwise. They agreed on the parts that matter.

Both rejected the argument that Go data is cheaper to maintain than C. Their
correction is the same and it is right: cheaper to edit, not cheaper to verify.
The SQL text of a frozen query does not rot. The environment that proves it
does. The cost is the container image, the runner, the driver, the auth method
and the person who triages an issue, and none of those becomes cheaper because
the difference is data.

Both insisted that a version must not be called supported unless something
tests it, and both asked for a falsifiable trigger to remove one. D40 sets
both.

## One argument against was checked and does not apply

Gemini's strongest objection was a refactoring tax. It argued that the padding
rule makes every new column expensive, because adding a column for a new
release means going back and patching every old version with
`NULL AS "new_column"`.

That is true of the design D8 abandoned, which gave each release its own query.
It is not true of the design D8 chose. A new column is one `Choice` with two
alternatives:

```go
Choice{
	{Query: `NULL AS "x"`},
	{Min: v20, Query: `real_expression AS "x"`},
}
```

The zero minimum alternative covers every release below 20, whether the floor
is 9.6 or 14. Adding a column costs the same number of alternatives regardless
of how many versions are supported. The tax Gemini describes is real for a
package per release and absent here.

This is worth recording because it was the main argument for dropping 9.6, and
it was aimed at the wrong design.

## The floor stays at 9.6

Ken reviewed this and kept 9.6. The reasoning holds up against the review.

The extraction is genuinely one time for a frozen release. 9.6 has been out of
upstream support since 2021 and its catalog will never change again, so the
query set has a final form rather than a moving one.

The image demonstrably runs, so this is not a claim of support that nothing can
check.

The gap is one release, and the cost of it is one additional source checkout at
release 15 or older.

What the review adds, and what D40 now requires, is that keeping a version is
not the same as claiming it is tested, and that something must eventually be
able to remove it.


## What a floor costs, measured

Counted from `describe.c` on the current tree, which holds 68 version gates.
These counts cover release 11 to 19 only, because the current tree has nothing
older. Measuring a floor below 11 needs an older checkout.

| Floor | Live gates | Gates that collapse |
| --- | --- | --- |
| 10 | 68 | 0 |
| 11 | 55 | 13 |
| 12 | 44 | 24 |
| 13 | 40 | 28 |
| 14 | 34 | 34 |
| 15 | 20 | 48 |
| 16 | 15 | 53 |
| 17 | 12 | 56 |
| 18 | 9 | 59 |

The gap between a floor of 10 and a floor of 14 is 68 gates against 34, so
exactly double. On the release 15 tree the same comparison ran 63 against 13,
close to five times. The ratio narrowed because the oldest gates went away with
the code that used them, not because the work got easier.


## Testing the old releases

Container images exist for every release back to 9.6, and all of them publish
`linux/amd64` and `linux/arm64`. They are frozen rather than gone:

| Release       | Image last updated |
| ------------- | ------------------ |
| 18 through 14 | 2026-09-19         |
| 13            | 2025-11-14         |
| 12            | 2025-01-14         |
| 11            | 2022-06-23         |
| 10            | 2022-06-23         |
| 9.6           | 2022-02-12         |

One thing is unverified and someone must check it before phase 1 relies on it.
The 9.6, 10 and 11 images were last built in 2022 on a Debian base of that era,
and a container that old can fail on a current host over blocked syscalls or a
C library mismatch. The image being listed is not proof that it runs. Pull each
one and start it before planning work around it.

D24 already puts these releases outside CI. They are tested on a development
machine, which is also where an old image that needs a workaround is least
disruptive.

## The floor moves for other databases, not for PostgreSQL

D21 drops a version when its upstream support ends. That rule governs the other
databases. It does not govern PostgreSQL, whose floor is fixed at 9.6 by this
decision and moves only if Ken says so.
