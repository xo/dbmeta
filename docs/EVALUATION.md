# Database Support Evaluation

This document records how `dbmeta` decides which versions of a database to
support, and how to make the same decision for a database that is not yet
covered.

Read it before you propose adding a database or changing a floor. The decisions
themselves live in `decisions/`. This file holds the method and the evidence.

This is one step of adding a database. [`DIALECT.md`](DIALECT.md) holds all of
them, and this is its step 2.

## Why this document exists

The question "which versions do we support" comes up once per database, and
there will be more than forty of them. Answered case by case it produces
inconsistent floors that nobody can defend later. Answered by a method it
produces a table that anyone can check.

Later phases will ask other AI models and third party sources for data. This
document says which of their answers to trust and which to verify.

## Which databases qualify

A database qualifies when a person can get it and run it for development and
testing without paying. An open source license is not the test. A source
available release, an evaluation edition, a developer edition and a free
community edition all qualify.

Many of the databases here are commercial and qualify this way. SQL Server
runs as the Developer and evaluation editions, Oracle as Free and XE, SAP HANA
as the express edition, Exasol as the Community Edition, Vertica as the
community edition, and Db2 as the Community image.

If a release needs an account, a signup or an accepted license, record that in
its decision. D85 records the Exasol signup, and D76 records the SAP license
that the HANA container accepts. A release that no person can run
without paying cannot be tested, and step 2 below then decides against it.
See D90.

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

Some publishers never rebuild a tag. They build each release once, on the
day it is released, and only a new release gets a new build. Databend,
rqlite, libSQL, TDengine, Apache Pinot, Avatica, Apache Druid and most of
the products D118 added work this way. For such a product,
the floor is the newest release of the line before the newest, and the
ceiling is the newest release. A product with one line has one release. Ken
chose this rule on 2026-09-27 (D112).

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

Pin the bare major tag, such as `postgres:14`. A test must meet the newest
patch of that major, because that is what people run.

There is no second answer for generation, because nothing here is generated. A
model is written and checked against a server rather than produced by one, so
no image has to be exactly recoverable to reproduce a file. See D71.

### Which versions get tested where

D42 governs and it overrides any split by version. Four releases of each
product run on every push and the rest run nightly, and D40 names the tier each
one sits in.

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

1. Is it the model, or an ordinary database? Ordinary, unless `decisions/` says
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
versions CI covers and which are covered only on a development machine. D42
and D40 make that distinction, and the table must not claim more than is true.

## Databases evaluated so far

| Database | Floor | Ceiling | Decided by |
| --- | --- | --- | --- |
| PostgreSQL | 9.6 | 18 | Criterion 1, it is the model |
| MariaDB | 10.6 | 13.0 | Criterion 3, the oldest long term release still maintained |
| MySQL | 8.4 | 26.7 | Criterion 3, 8.4 is the long term release |
| SQL Server | 2008 R2 | 2025 | Criterion 2 for the containers, 2017 to 2025. 2008 R2 to 2016 have no Linux container and are Verified Windows machines (D57) |
| Oracle | 11g | 26ai | Criterion 2, from the free images. The container facts are in D54 |
| Cassandra | 3.11 | 5.0 | Criterion 2, then held one release up because 3.0 adds no answer |
| ClickHouse | 25.3 | 26.9 | Criterion 2. It goes stale faster than any other, because ClickHouse releases monthly |
| Trino | 476 | 483 | Criterion 5. Criterion 2 cannot decide it and criterion 3 gives a floor of one |
| Presto | 0.299 | 0.299 | Criterion 3, which gives a floor of one, and there is no second release to compare |
| Firebird | 3.0 | 5.0 | Criteria 2 and 3 agree, which is rare enough to record |
| SAP HANA | 2.00.076 | 2.00.088 | Criterion 3. SAP publishes an express edition of 2.0 only |
| Apache Hive | 4.0.1 | 4.2.1 | Criterion 2. Nothing older than 4.0 is published |
| Vertica | 7.2.1 | 25.1.0 | Criterion 2 gives a floor of one, 25.1, the only current release that starts outside Kubernetes. The three older releases are community images Ken chose so that a gate has something older to answer against, and D88 records why. The images are copies in `docker.io/usql/vertica` (D100) |
| Exasol | 2025.2.1 | 2026.2.0 | Criterion 2 for the container, which gives a floor of one. The floor is the Community Edition machine, a release line older, which D85 chose so that a gate has something to answer against |
| ScyllaDB | 2025.1 | 2026.3 | Criterion 2. 2025.1 is the oldest release the vendor still rebuilds. 6.2, the last open source release, was last rebuilt in February 2025. D90 says a source available release qualifies |
| Couchbase | 7.2.9 | 8.0.3 | Criterion 2. 7.2 is the oldest line the image still rebuilds, and 7.0 and 7.1 stopped in November 2024. The model's floor is 7.6, because 7.2 sends its columns in name order. 7.2.9 stays for the dbimp driver, and the model reports it too old (D104) |
| Neo4j | 5.26.31 | 2026.09.0 | Criterion 2. 4.4.48, 5.26.31 and 2026.09.0 were rebuilt on 2026-09-26, and a monthly release stops being rebuilt when the next one arrives. 4.4 is out, because its Enterprise image starts only with the commercial licence. The ceiling moves each month. There is no dbmeta model, and the entry is for dbimp's driver (D106) |
| SurrealDB | 2.7.0 | 3.3.0 | Criterion 2. 2.7.0 was rebuilt on 2026-09-23 and 3.3.0 on 2026-09-24, and between them 3.1.6 on 2026-09-01 and 3.2.4 on 2026-08-03. 1.5.6 was last rebuilt in November 2024, and 2.6.5 and 3.0.5 in March 2026. There is no dbmeta model, and the entry is for dbimp's driver |
| ArangoDB | 3.12.12 | 3.12.12 | Criterion 2. Only the 3.12 line is still built: 3.12.12 was rebuilt on 2026-09-24, and 3.11.14 was last built on 2025-05-24. The release moves with each patch. The entry is for dbimp's driver (D112) |
| InfluxDB | 1.11.8 | 3.11.5 | Criterion 2 for each line. 1.13.1, 1.11.8, 2.9.1 and 2.8.0 were rebuilt on 2026-09-19, and the InfluxDB 3 Core lines 3.9 to 3.11 in September 2026. Ken chose the releases in dbimp's D79. The entries are for dbimp's driver, and no model reads them, so they are Staged (D112, D114, D119) |
| CrateDB | 6.3.7 | 6.4.5 | Criterion 2. 6.4.5 and 6.3.7 were rebuilt in September 2026, and 6.2 last on 2026-07-09. Reached on the PostgreSQL port, with a dialect of its own, cratedb (D112, D123) |
| TDengine | 3.3.8.8 | 3.4.2.8 | The rule for an image that is never rebuilt: the newest release of each of the last two lines (D112). The entry is for dbimp's driver |
| Apache Pinot | 1.4.0 | 1.5.1 | The rule for an image that is never rebuilt (D112). Apache supports only the newest release. The entry is for dbimp's driver |
| Databend | 1.2.881 | 1.2.948 | The rule for an image that is never rebuilt (D112): the newest stable release and the newest weekly one. The weekly release moves almost every day. The entry is for dbimp's driver |
| rqlite | 9.4.5 | 10.3.6 | The rule for an image that is never rebuilt (D112). The entry is for dbimp's driver |
| libSQL | 0.24.33 | 0.24.33 | The rule for an image that is never rebuilt (D112). libSQL has one line, and its newest release was built on 2025-12-19. The entry is for dbimp's driver |
| chai, csvq, ql, moderncsqlite | none | none | No server. The release is whichever the driver embeds. `dbrun` knows them before their models (D116) |
| Avatica | 1.28.0 | 1.29.0 | The rule for an image that is never rebuilt (D112). The standalone server over HSQLDB, which the Calcite project builds. The entry is for dbimp's Avatica driver (D113) |
| Apache Phoenix | 2.0-5.0 | 2.0-5.0 | An exception to step 2, which Ken made (D113). The Phoenix project publishes no image, and the only one that runs in one container, boostport/hbase-phoenix-all-in-one, was last pushed on 2023-03-14. The entry is for dbimp's Avatica driver |
| Apache Druid | 36.0.0 | 37.0.0 | The rule for an image that is never rebuilt (D112). 38.0.0-rc1 is a candidate. The entry is for dbimp's Avatica driver (D113) |
| Qdrant | 1.18.3 | 1.19.1 | The rule for an image that is never rebuilt (D112). The entry is for dbimp's driver (D118) |
| Chroma | 1.4.1 | 1.5.9 | The rule for an image that is never rebuilt (D112). The 1.5.10.dev tags are development builds. The entry is for dbimp's driver (D118) |
| Weaviate | 1.38.17 | 1.39.7 | The rule for an image that is never rebuilt (D112). An older line still gets releases. The entry is for dbimp's driver (D118) |
| CouchDB | 3.4.3 | 3.5.2 | Criterion 2. 3.5.2 and 3.4.3 were rebuilt on 2026-09-19, and 3.3.3 last on 2025-04-29. The entry is for dbimp's driver (D118) |
| QuestDB | 9.4.3 | 10.0.1 | The rule for an image that is never rebuilt (D112). The entry is for dbimp's driver (D118) |
| Meilisearch | 1.53.2 | 1.54.0 | The rule for an image that is never rebuilt (D112). A minor release arrives about every two weeks, so the ceiling moves fast. The entry is for dbimp's driver (D118) |
| Typesense | 29.1 | 30.2 | The rule for an image that is never rebuilt (D112). 31.0 is a release candidate. The entry is for dbimp's driver (D118) |
| TerminusDB | 11.1.17 | 12.0.7 | The rule for an image that is never rebuilt (D112). 12.1-rc is a release candidate. The entry is for dbimp's driver (D118) |
| CockroachDB | 24.3.36 | 26.3.2 | The rule for an image that is never rebuilt (D112) gives 26.2.7 and 26.3.2. 24.3.36 is kept too, because 24.3 is the oldest line with long term support still patched. No dialect until models/postgres detects it (D118) |
| TiDB | 7.5.8 | 8.5.8 | Criterion 2. A tag is rebuilt while its line is maintained, and 7.5 is the oldest line with long term support still maintained. 8.1.2 is between them. No dialect until models/mysql detects it (D118) |
| MongoDB | 7.0.43 | 8.3.11 | Criterion 2. 8.3.11, 8.0.32 and 7.0.43 were rebuilt in September 2026, and 6.0 last in May. 8.0.32, the line with long term support, is between them. The entry is for dbimp's driver (D118) |
| Elasticsearch | 8.19.22 | 9.5.3 | Each tag is built once, and 8.19 is still patched, so it is the floor. 9.4.6 is between them. The entry is for dbimp's driver (D118) |
| Dgraph | 25.3.8 | 25.4.1 | The rule for an image that is never rebuilt (D112). The entry is for dbimp's driver (D118) |
| YDB | 26.2.1.14 | 26.3.1.17 | The rule for an image that is never rebuilt (D112). usql reaches it with ydb-go-sdk, and dbmeta has no model yet (D118) |
| Spanner | 1.5.58 | 1.5.58 | The rule for an image that is never rebuilt (D112). The emulator has one line. The image is built here, because Google's has no shell (D118) |
| BigQuery | 0.7.2 | 0.8.1 | The rule for an image that is never rebuilt (D112). A community emulator, with a smaller INFORMATION_SCHEMA than the service (D118) |
| GizmoSQL | 1.38.5 | 1.39.0 | The rule for an image that is never rebuilt (D112). The maintained Arrow Flight SQL server, for usql's flightsql driver (D118) |
| Virtuoso | 7.2.17 | 7.2.17 | Criterion 2. 7.2.17 was rebuilt on 2026-08-05 and 7.2.16 last on 2025-10-15. The entry is for dbimp's SPARQL driver (D118) |
| Alternator | 2025.1 | 2026.3 | The scylla entry's range, which D90 chose. The DynamoDB interface of ScyllaDB (D118) |
| Vitess | 23.0.6 | 24.0.3 | The rule for an image that is never rebuilt (D112). No dialect until models/mysql detects it (D118) |
| Milvus | 2.6.24 | 3.0.2 | The rule for an image that is never rebuilt (D112). The entry is for dbimp's driver (D118) |
| OpenSearch | 2.19.6 | 3.8.0 | The rule for an image that is never rebuilt (D112). The entry is for dbimp's driver (D118) |
| DynamoDB | 3.2.0 | 3.3.1 | The rule for an image that is never rebuilt (D112). DynamoDB Local, under a proprietary licence Ken accepted (D118) |
| Cosmos | EN20260907 | EN20260907 | The vNext emulator is a dated build each month, built once, so the rule in D112 gives the newest. A Microsoft licence Ken accepted (D118) |
| Stardog | 12.0.4 | 12.1.4 | The rule for an image that is never rebuilt (D112). Needs a licence file, and not yet measured (D118) |
| GraphDB | 11.4.3 | 11.5.1 | The rule for an image that is never rebuilt (D112). GraphDB 11 needs a licence file, and not yet measured (D118) |
| VoltDB | 14.1.0 | 15.2.0 | The rule for an image that is never rebuilt (D112). The developer edition needs a licence file, and not yet measured (D118) |
| Solr | 9.9.0 | 10.0.0 | Criterion 2. 10.0.0, 9.10.1 and 9.9.0 were rebuilt on 2026-09-26. 9.10.1 is between them. The entry is for dbimp's driver (D118) |
| Drill | 1.21.2 | 1.22.0 | The rule for an image that is never rebuilt (D112). The Java of 1.22.0 fails on a current host unless its container support is off (D118) |
| H2 | 2.4.240 | 2.5.252 | The rule for an image that is never rebuilt (D112), applied to the jars on Maven Central. The image is built here (D118) |
| Fuseki | 6.1.0 | 6.2.0 | The rule for an image that is never rebuilt (D112), applied to the jars on Maven Central. The image is built here (D118) |
| PostgREST | 14.18 | 16.4 | The rule for an image that is never rebuilt (D112). There is no 15. The image is built here, on PostgreSQL 18 (D118) |
| ksqlDB | 8.2.4 | 8.3.2 | The rule for an image that is never rebuilt (D112), from Confluent Platform's image, because confluentinc/ksqldb-server stopped in 2023. The image is built here, with the Kafka broker (D118) |
| SQLite3 | none | none | No server. The release is whichever the driver embeds |
| DuckDB | none | none | No server. The release is whichever the driver embeds |

The full reasoning for each is in the doc comment of its `container/` file,
which is where the template above says to put it. This table is the index to
those and not a second copy of them.

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

Nothing that runs. The products D118 added were the last through the
procedure, and every product in `container.All` has a row in the table above.
Most of them are Staged, because no model reads them (D119). The tier does not
change the range, which each row above still records. `TestEveryProductIsEvaluated` fails when one does not.
The products `usql` ran that have no entry in `container/` are listed under
"Candidates carried over from usql" below.

Two products were once evaluated as unable to start, and neither is any more.

Vertica had no image outside Kubernetes once `vertica/vertica-ce` was
withdrawn, which D66 recorded. A community copy of that image at 25.1 starts,
and three older community images start too, so Vertica has four releases, a
row above and a model. D88 records the decision, and it is the first product
here whose release range is set by community images. All four are now copies
in `docker.io/usql/vertica`, tagged by release and pinned by digest, which
D100 records.

Exasol's `exasol/docker-db` would not initialize under rootless podman, which
is D77, and Exasol now publishes `exasol/nano`, which starts unprivileged on
the default network in about five seconds. D84 has the measurement, and D87
records the model.

Exasol's range comes from two places. The nano images are a container like
any other and criterion 2 decides them. The Community Edition is an
appliance a person imports once and freezes, which is criterion 3 with a
floor of one, and it exists so that the model has a server older than the
nano line to answer against. D85 has the approach and says what does not
carry over from the SQL Server machines, and D86 is how `dbrun` imports it.

Oracle's container facts are in D54, and the floor followed from them the same
way SQL Server's did. The privilege question was the open one there rather than
the version question, and D60 and D61 answer it.

Cassandra went through step 2 and stopped there. On
`docker.io/library/cassandra`, 2.2 was last rebuilt in August 2021 and is dead,
3.0 and 3.11 were rebuilt in November 2025, and 4.0, 4.1 and 5.0 a week before
this was written, all with a linux/amd64 build. So the floor could be 3.0 and
it is 3.11, because 3.0 and 3.11 carry the same `system_schema` catalog and
3.11 is the release people ran. Below 3.0 the catalog is a different shape
entirely, in `system.schema_columnfamilies` and its siblings, and no image that
still runs has it. See `container/cassandra.go`.

Do not assume a floor for a database until it has been through the procedure
above.

## Candidates carried over from usql

`usql` kept a podman configuration per database in `contrib/`, started by
`podman-run.sh`. The products dbmeta already has a model for are started by
`dbrun` from `container/`, and that list is the one copy of their images,
ports, environment and passwords, so nothing of theirs was carried over.
CockroachDB, Flight SQL, H2 and YDB have Staged entries now, which D118 added
from the vendors' current images rather than from these. Apache Ignite is
gone: usql removed its driver on 2026-09-27 and dburl v0.31.0 dropped the
scheme, and a product nothing reads is removed (D118).

The one below has no entry in `container/`. Its container facts are recorded
here, as `usql` had them on 2026-09-27, so that its evaluation starts from them
rather than from nothing. dbmeta did not start it. It is a lead, the same as an
AI model's answer, and step 2 of the procedure above still has to be run.

| Product | Image | Ports | Environment and setup |
| --- | --- | --- | --- |
| Db2 | `icr.io/db2_community/db2` | 50000, 55000 | `LICENSE=accept`, `DB2INSTANCE=db2inst1`, `DB2INST1_PASSWORD`, `DBNAME=testdb`, and a volume at `/database` |

Two more `usql` targets are covered here and differ from what `usql` ran,
which is worth knowing when comparing the two.

`usql` started Oracle Enterprise 21.3.0.0 from `container-registry.oracle.com`,
which needs an Oracle account to pull. dbmeta runs 21.3.0 as the free edition
from `gvenzl/oracle-xe`, and builds 19c Enterprise itself. D54 has the list.

`usql`'s Exasol and Vertica configurations name `exasol/docker-db` and
`vertica/vertica-ce`. The first does not initialize under rootless podman,
which is D77, and dbmeta runs `exasol/nano` instead. The second was
withdrawn, which D66 records, and dbmeta runs a community copy of the same
image at 25.1 and three older releases, which D88 records.

The Db2 notes in `usql` are about installing IBM's ODBC client, which is
`usql`'s concern and stays there. The container itself needs nothing beyond
the table above.
