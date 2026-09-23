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
`describe.c` contains 11 gates of the form `pset.sversion < N`, which exist
only because something present in an older release is absent in a newer one.
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

**Criterion 4.** Counted from `describe.c`, which holds 76 version gates.

| Floor | Live gates | Gates that collapse |
| --- | --- | --- |
| 9.6 | 63 | 13 |
| 14 | 13 | 63 |

The 9.6 floor costs close to five times the version work. That is the price of
`psql` compatibility.

Count the gates with a command rather than by reading:

```bash
grep -oE 'pset\.sversion *(<|>=) *[0-9]+' src/bin/psql/describe.c | sort | uniq -c
```

A gate is dead at a given floor when it is always true or always false there. A
gate reading `>= 90400` is always true once the floor is 9.6.

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
the integer packs three fields as major, minor and patch. At 10 and above it
packs two, as major and patch. Comparison still works across the boundary,
because the integers increase, but do not try to recover a human readable
version by dividing by 10000 without handling the two schemes.

### The list

Ten major versions, from 9.6 to the newest stable release:

9.6, 10, 11, 12, 13, 14, 15, 16, 17, 18.

Release 18 is the newest stable as of 2026-09-24. Release 19 is in beta.

### Verified: major granularity is safe

The claim to check was whether a catalog can change inside a major version,
which would force a gate at something like 14.3 and make major granularity
wrong.

It cannot, and `describe.c` demonstrates it. Every version gate in the file
sits on a major boundary. There are 11 distinct gate values and all of them end
at patch zero:

```
90300 = 9.3.0    100000 = 10.0    130000 = 13.0
90400 = 9.4.0    110000 = 11.0    140000 = 14.0
90500 = 9.5.0    120000 = 12.0    150000 = 15.0
90600 = 9.6.0                     160000 = 16.0
```

Not one gate sits at a patch level. This matches the PostgreSQL rule that a
patch release must not change the on disk format, which a catalog change would
do. A patch release is a drop in binary replacement.

Recheck this when translating, with:

```bash
grep -oE 'pset\.sversion *(<|>=) *[0-9]+' src/bin/psql/describe.c | grep -oE '[0-9]+$' | sort -u
```

A value that does not end in `00` would be a patch level gate and would break
this assumption.

### Which image tag to pin

Generation and testing want different answers, because they are protecting
different things.

For generation, pin a digest. Generated code must be reproducible, so the
server that produced a model must be exactly recoverable. A tag moves when the
image is rebuilt, and a rebuilt base image can change a default that shows up
in generated output. Record the digest next to the generated model. D12 already
requires this.

For testing, pin the bare major tag, such as `postgres:14`. Testing should meet
the newest patch of that major, because that is what people run.

### Which versions get tested where

D24 governs and it overrides any split by version. CI runs the latest release
only. Every other major runs on a development machine.

For PostgreSQL that means CI tests release 18, and 9.6 through 17 are tested
locally before a release.

### Recorded dissent: Gemini argued for a floor of 10

Gemini was asked and recommended dropping 9.6 and starting at 10. Ken decided
9.6. The argument is recorded because it names what 9.6 costs.

Gemini's case: 9.6 has been end of life since 2021, its image has not been
rebuilt since February 2022, and release 10 introduced declarative
partitioning with `pg_partitioned_table` and `pg_class.relpartbound`, identity
columns through `pg_attribute.attidentity`, and logical replication with
`pg_publication` and `pg_subscription`. Supporting 9.6 means a fallback path for
a catalog without any of those, plus handling the pre-10 versioning scheme.

Ken's reason overrides it. Compatibility with `psql` is the product, so
PostgreSQL is the specification rather than one supported database. The source
for every release is public, so nothing has to be guessed.

### Corrected: the old images do publish arm64

Gemini also claimed that the 9.6, 10 and 11 images lack native `linux/arm64`
builds and would need emulation on Apple Silicon or Graviton. That is wrong.
Checked against the Docker Hub API on 2026-09-24, all three publish
`linux/386`, `linux/amd64`, `linux/arm` and `linux/arm64`.

The rest of Gemini's warning about those images stands and is unverified. They
were last built in 2022 on a Debian base of that era, so the archive
repositories for that base may be gone, which matters if a test installs
anything inside the container. Whether they start at all on a current host is
still unchecked. Pull each one and start it before planning work around it.

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

PostgreSQL covers ten major versions: 9.6, 10, 11, 12, 13, 14, 15, 16, 17 and
18. See the section above for the unit and the evidence.

Every other database is unevaluated. Do not assume a floor for one until it has
been through the procedure above.
