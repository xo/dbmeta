# Database Support Evaluation

This document records how `dbmeta` decides which versions of a database to
support, and how to make the same decision for a database that is not yet
covered.

Read it before you propose adding a database or changing a floor. The decisions
themselves live in `PLAN.md`. This file holds the method and the evidence.

## Why this document exists

The question "which versions do we support" comes up once per database, and
there will be more than forty of them. Answered case by case it produces
inconsistent floors that nobody can defend later. Answered by a method it
produces a table that anyone can check.

Later phases will ask other AI models and third party sources for data. This
document says which of their answers to trust and which to verify.

## The decision procedure

Work through these in order. Stop at the first one that gives a clear answer.

### 1. Is the database the model?

PostgreSQL is the primary model under D9, and the goal of this project is
compatibility with `psql`. For PostgreSQL only, go as far back as the source
allows. The floor is 9.6. Cost does not decide it.

No other database gets this treatment. If a future database is promoted to a
second reference, record that here first.

### 2. Does a maintained container image exist?

This is the decisive test for every database that is not PostgreSQL, because a
version that cannot be started cannot be tested, and an untested version is not
supported.

A maintained image is one the publisher still rebuilds. Check the registry
directly. For an official Docker Hub image:

```bash
curl -s 'https://hub.docker.com/v2/repositories/library/postgres/tags/13' | python3 -m json.tool
```

Read `last_updated` and the `images` list. A recent `last_updated` means the
image is still rebuilt. The `images` list must include `linux/amd64`.

The floor is the oldest release whose image is still rebuilt.

Warning: an image that exists is not an image that runs. A container built
several years ago sits on an old base system and can fail on a current host
over blocked syscalls or a C library mismatch. Pull it and start it before you
count a version as supported.

### 3. What does the vendor still support?

Find the published end of life date. D21 drops a version in the first minor
release after that date passes.

For most databases the vendor date and the image date agree, because the
publisher stops rebuilding when upstream stops patching. PostgreSQL 13 went end
of life on 2025-11-13 and its image was last updated on 2025-11-14. When the
two disagree, the image date decides, because it decides what can be tested.

### 4. Does an amd64 image exist?

D25 tests on `linux/amd64` only. A version whose image does not publish
`linux/amd64` cannot be tested and is not supported.

Do not evaluate any other platform, and do not record one as a reason to accept
or reject a version. The same database version is assumed to answer the same
way everywhere.

### 5. What does it cost?

Measure, do not guess. For a database with version gated queries, count the
gates that stay live at each candidate floor. The PostgreSQL worked example
below shows the method and the result.

A cost measurement does not overrule criteria 1 to 4. It tells you what you are
agreeing to.

## Which evidence to trust

Sources are ranked. Prefer a higher one when they disagree.

1. **The database source code.** Highest. It is the ground truth about catalog
   changes and version behavior, and for PostgreSQL it is public.
2. **A registry or vendor API.** High. Image dates and platform lists are facts
   and a command returns them.
3. **Published vendor lifecycle pages.** High for dates, but confirm the page
   is current.
4. **Documentation of comparable tools.** Medium. Useful for what a floor
   normally is. Projects state support they do not test.
5. **An AI model.** Low on its own. See below.
6. **Deployment share figures.** Lowest. Treat every number as an estimate.

## Consulting AI models

Models are useful here and they are not reliable here. Both were true in the
evaluation that produced D20.

What they did well: recalling release and end of life dates, naming what floor
comparable tools set, listing the considerations, and arguing a policy
position. Two models asked separately agreed on the deprecation trigger in D21,
and that agreement was worth having.

What they did badly:

- They gave confident figures for deployment share with no source. One model
  admitted, when asked, that reliable public telemetry does not exist.
- They named specific projects and described what those projects do, and the
  two models contradicted each other on the details. Those claims were recorded
  as unverified.
- One model asserted that database catalog changes are monotonic, meaning a
  column added in one release is never removed later. That is false, and it
  had been used to argue for a smaller test matrix.

Rules for using a model in this evaluation:

1. Ask at least two models separately. Agreement is weak evidence. Disagreement
   is a signal to go and check.
2. Never accept a claim that a command can check. Container availability,
   version gate counts, and catalog contents are all checkable.
3. Record every unverified claim as unverified, with the model named.
4. When two models disagree, settle it from a higher ranked source, not by
   preferring the model you like.

The catalog monotonicity question is the worked example. One model said catalog
changes are monotonic and a sampled test matrix is therefore safe. The other
said the opposite. Checking the PostgreSQL tree settled it: commit
`fe5038236c` is titled "Remove obsolete pg_attrdef.adsrc column", and
`describe.c` contains 3 gates of the form `pset.sversion < N`, at 11, 12 and
15, which exist only because something present in an older release is absent in
a newer one. The release 15 tree had 11 such gates. The count fell because
release 20 dropped support for servers below 10, not because the catalog became
monotonic.
The claim was false and the test matrix decision changed.

## Worked example: PostgreSQL

The evaluation that produced D20, in the order above.

**Criterion 1.** PostgreSQL is the model. The floor is 9.6 and the cost
criterion does not apply.

**Criterion 2, recorded anyway.** Checked against the Docker Hub API on
2026-09-24.

| Release | Image last updated | Platforms include amd64 |
| --- | --- | --- |
| 18, 17, 16, 15, 14 | 2026-09-19 | yes |
| 13 | 2025-11-14 | yes |
| 12 | 2025-01-14 | yes |
| 11 | 2022-06-23 | yes |
| 10 | 2022-06-23 | yes |
| 9.6 | 2022-02-12 | yes |

Maintained rebuilding stops after 14. Had PostgreSQL been an ordinary database,
the floor would be 14.

**Criterion 3.** Reported by a model and consistent with the published five
year policy, not independently checked: 18 through 14 are supported, 13 ended
on 2025-11-13, 12 ended on 2024-11-14. Release 14 ends on 2026-11-12, which is
under two months away.

**Criterion 5.** Counted from `describe.c` on the current tree, which is
`REL_19_BETA1-1062-gd9de60c5e47` on `master`, release 20 under development. It
holds 68 version gates spanning release 11 to release 19.

| Floor | Live gates | Gates that collapse |
| --- | --- | --- |
| 10 | 68 | 0 |
| 11 | 55 | 13 |
| 12 | 44 | 24 |
| 14 | 34 | 34 |
| 18 | 9 | 59 |

Count the gates with a command rather than by reading:

```bash
grep -oE 'pset\.sversion *(<|>=) *[0-9]+' src/bin/psql/describe.c | sort | uniq -c
```

A gate is dead at a given floor when it is always true or always false there.

**A warning about this criterion.** These numbers describe only the releases
the current `psql` still supports. On 2026-07-02, commit `831bec45924` removed
every `psql` code path for a server below release 10, stating the upstream
policy of supporting at least ten previous major versions. A release 15 tree
measured 76 gates spanning 9.3 to 16. The current tree measures 68 spanning 11
to 19.

So a cost measurement is only valid for the tree it was taken from, and a floor
below what upstream supports cannot be measured from the current tree at all.
Record the tree you measured, as this section does. Re-measure when it moves.

## The PostgreSQL version list

D20 sets the PostgreSQL floor at 9.6. This section gives the exact list and the
unit.

The unit is the major version. `dbmeta` does not support or test a point
release such as 14.1 or 14.2 separately.

### PostgreSQL changed what "major" means at release 10

Before release 10, a major version was the first two numbers. 9.5 and 9.6 are
two different major versions, and the third number was the patch. From release
10 onward a major version is a single number, and the second number is the
patch. So 9.6 is one major version, and 10.3 is release 10 at patch 3.

The change came with release 10 in 2017. The reason was that people read the
step from 9.5 to 9.6 as a minor update and replaced the binaries without
running `pg_upgrade`, which breaks the cluster.

This matters to `dbmeta` because the server reports an integer. `SHOW
server_version_num` returns 90600 for 9.6 and 100000 for release 10. Below 10
the integer packs three fields. At 10 and above it packs two. Comparison still
works across the boundary, because the integers increase, but do not try to
recover a human readable version by dividing by 10000 without handling both
schemes.

### The list

Ten major versions, from 9.6 to the newest stable release:

9.6, 10, 11, 12, 13, 14, 15, 16, 17, 18.

Release 18 is the newest stable as of 2026-09-24. Release 19 is in beta and
release 20 is in development.

### Verified: major granularity is safe

The claim to check was whether a catalog can change inside a major version,
which would force a gate at something like 14.3 and make major granularity
wrong.

It cannot, and `describe.c` demonstrates it. Every version gate in the file
sits on a major boundary. On the current tree the gate values are 110000,
120000, 130000, 140000, 150000, 160000, 170000, 180000 and 190000, and every
one ends at patch zero.

This matches the PostgreSQL rule that a patch release must not change the on
disk format, which a catalog change would do. A patch release is a drop in
binary replacement.

Recheck this when translating, with:

```bash
grep -oE 'pset\.sversion *(<|>=) *[0-9]+' src/bin/psql/describe.c | grep -oE '[0-9]+$' | sort -u
```

A value that does not end in `0000` for a release 10 or later gate, or in `00`
for an earlier one, would be a patch level gate and would break this
assumption.

### Two source trees are needed

The current tree describes releases 10 and newer only. Commit `831bec45924`
removed the older code paths on 2026-07-02.

Translate releases 10 through 19 from the current tree. Translate 9.6 from a
release 15 or older checkout. Record which tree each fragment came from, beside
the fragment.

### Which image tag to pin

Generation and testing want different answers, because they protect different
things.

For generation, pin a digest. Generated code must be reproducible, so the
server that produced a model must be exactly recoverable. A tag moves when the
image is rebuilt, and a rebuilt base image can change a default that shows up
in generated output. Record the digest next to the generated model. D12 already
requires this.

For testing, pin the bare major tag, such as `postgres:14`. Testing should meet
the newest patch of that major, because that is what people run.

### Which versions get tested where

D24 governs and it overrides any split by version. CI runs the latest release
only. Every other major runs on a development machine, and D40 names the tier
each one sits in.

### Recorded dissent: both reviews argued for a higher floor

Gemini first recommended a floor of 10 on the grounds that 9.6 has been end of
life since 2021, its image has not been rebuilt since February 2022, and
release 10 introduced declarative partitioning, identity columns and logical
replication, so supporting 9.6 means a fallback path for a catalog without any
of them.

When the upstream removal was put to both models, Gemini recommended dropping
the old releases outright and DeepSeek recommended keeping them only as a named
tier with scheduled tests.

Ken kept 9.6. D20 records why the reasoning survived review, including that
upstream's 2026 reason was a scope policy rather than a finding that the old
releases cannot be tested, and that 9.6 was verified to run on 2026-09-24.

### Corrected: the old images do publish arm64

Gemini claimed that the 9.6, 10 and 11 images lack native `linux/arm64` builds
and would need emulation. That is wrong. Checked against the Docker Hub API on
2026-09-24, all three publish `linux/386`, `linux/amd64`, `linux/arm` and
`linux/arm64`.

The related warning that those images might not start at all is also now
answered. `postgres:9.6` was pulled and run under podman 6.1.2 on a current
Linux host on 2026-09-24. It became ready in four seconds and answered `\d` and
`\dt` correctly.

## Template for the next database

Record each database here as it is evaluated. Answer all five.

1. Is it the model, or an ordinary database? Ordinary, unless `PLAN.md` says
   otherwise.
2. Image evidence. The registry, the oldest release still rebuilt, the date
   checked, and whether `linux/amd64` is published. `linux/amd64` is required,
   and no other platform matters. State whether the image was actually started
   or only listed.
3. Vendor lifecycle. The end of life date for each candidate release, and the
   source.
4. Cost. How many version differences the floor implies, measured.
5. The floor chosen, the ceiling chosen, and which criterion decided it.

Add a row to the support table in `README.md` at the same time, and say which
versions CI covers and which are covered only on a development machine. D24
makes that distinction, and the table must not claim more than is true.

## Databases evaluated so far

| Database | Floor | Ceiling | Decided by |
| --- | --- | --- | --- |
| PostgreSQL | 9.6 | 18 | Criterion 1, it is the model |
| MariaDB | 10.6 | 13.0 | Criterion 3, the oldest long term release still maintained |
| MySQL | 8.4 | 26.7 | Criterion 3, 8.4 is the long term release |
| SQL Server | 2017 | 2025 | Criterion 2, and nothing else had to be asked |
| SQLite3 | none | none | No server. The release is whichever the driver embeds |
| DuckDB | none | none | No server. The release is whichever the driver embeds |

PostgreSQL covers ten major versions: 9.6, 10, 11, 12, 13, 14, 15, 16, 17 and
18. See the section above for the unit and the evidence.

## Worked example: SQL Server, where criterion 2 ended it

SQL Server is the clearest case the procedure has produced, and it is worth
recording because it took one step.

Criterion 2 asks whether a maintained image exists. Microsoft publishes one
image, `mcr.microsoft.com/mssql/server`, and its tag list answers the whole
question:

```bash
curl -s 'https://mcr.microsoft.com/v2/mssql/server/tags/list' | python3 -m json.tool
```

There are 284 tags. Every one of them names 2017, 2019, 2022 or 2025. There is
no 2016, no 2014 and no 2012, because Microsoft shipped SQL Server on Linux
from 2017 and never published a Linux image for an earlier release.

So the floor is 2017 and no further criterion applies. Criterion 3 would have
argued for 2019, because 2017 passed its end of extended support in October
2027 under the usual ten year term, and it does not get to: criterion 2 already
fixed the set at four, and testing all four costs four parallel jobs. D54
records the tier decision and what may honestly be said about 2016 and older.

Two facts about the images are worth writing down, because both cost time.
Microsoft publishes no bare release tag, so the tag is `2017-latest` and there
is no `2017`. The 2017 image is built on an older base and installs sqlcmd at
`/opt/mssql-tools` where the other three use `/opt/mssql-tools18`, so a
readiness command written for one of them fails on the other. Both are
recorded in `container/container.go` rather than in a script.

## What is still unevaluated

Oracle and Cassandra. The Oracle container facts are gathered and recorded in
D54, and the floor follows from them the same way SQL Server's did: the images
are `gvenzl/oracle-xe` at 18.4 and 21.3 and `gvenzl/oracle-free` at 23, and
there is none for 11g or 12c. The privilege question is the open one there, not
the version question.

Do not assume a floor for a database until it has been through the procedure
above.
