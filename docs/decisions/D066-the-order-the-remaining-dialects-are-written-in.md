# D66. The order the remaining dialects are written in

Status: Amended by D67, D77, D88, D91, D94 and D129.

Impala first, then ClickHouse, then the products that run in a container,
then the ones that need an account. A product that cannot be started cannot be
supported, and that decides the order more than anything about the product.

## Impala first, because usql is waiting on it. D67 found this wrong

`usql` has five hand written metadata readers: postgres, mysql, oracle,
impala and informationschema. Four of them are reimplemented here. Impala is
the last, and until it exists `usql` cannot retire its own metadata package,
which is the point of this project. Nothing else on this list blocks anybody.

It runs in a container, `apache/impala`, so the usual rules apply to it.

That reasoning does not survive contact with the product. D67 has the
measurement: Impala has no queryable catalog and its usql reader is not SQL, so
it cannot move here at all, and ClickHouse is first instead.

## Then ClickHouse, the one gap in the default build

`usql` builds clickhouse, csvq, duckdb, mysql, oracle, postgres, sqlite3 and
sqlserver by default. Every one of those has a model except ClickHouse and
csvq, and csvq reads files rather than a catalog.

ClickHouse has a real one. `system.tables`, `system.columns`,
`system.databases`, `system.functions`, `system.settings`,
`system.data_skipping_indices` and `system.dictionaries` carry the engine, the
partition key, the sorting key, the TTL and the column codecs, none of which
its `information_schema` emulation exposes. `clickhouse/clickhouse-server`
starts in seconds.

## Then the products that run in a container

In this order, and the order is what the native catalog adds over
`information_schema`, not the size of the user base:

| | Product | Image | Why here |
| --- | --- | --- | --- |
| 3 | Trino | `trinodb/trino` | federated engine, wide use, connector and session metadata |
| 4 | Presto | `prestodb/presto` | probably a flavor key on the Trino model rather than a model |
| 5 | Vertica | `usql/vertica`, copies of four community images | `v_catalog` is rich and nothing else reaches it. Done, see D88 and D100. The image this row first named was withdrawn, see below |
| 6 | SAP HANA | `saplabs/hanaexpress` | enterprise install base, deep `SYS` catalog. Done, see D76 |
| 7 | Firebird | `firebirdsql/firebird` | the `RDB$` catalog answers more than most of this list. Done, see D74 |
| 8 | Exasol | `exasol/nano` | `EXA_` catalog, analytic install base. Done, see D87, and D85 for the two tracks |
| 9 | Hive | `apache/hive` | metastore, and it is the shape Impala already teaches. Done, see D78 |

## Vertica cannot be started, measured 2026-09-26

The image in that table no longer exists. `vertica/vertica-ce` returns "object
not found" on Docker Hub, and there is no `vertica` namespace there at all.

Vertica's maintained image moved to OpenText and is `opentext/vertica-k8s`,
which is healthy: 91 tags, amd64, and 26.2.0-2 rebuilt three weeks before this
was written. It cannot be started outside Kubernetes without reimplementing
what the operator does. It declares no entrypoint and no command, it ships no
`admintools`, and it has no `dbadmin` user, because the operator creates the
user and drives `vcluster` itself. Pulling it and trying was how this was
found, which is what step 2 of `docs/EVALUATION.md` warns about.

The supported standalone path is the Oracle 19c pattern exactly. Vertica
publishes a Dockerfile, an entrypoint and a Makefile at
`vertica/vertica-containers/one-node-ce`, and the image is built from a
Community Edition RPM that a person downloads after registering at
vertica.com/try. Nothing here can fetch it, the same way nothing here can
fetch Oracle's archive, and D71's `dbrun build` already has the shape for it.

## A Kubernetes VM was considered and is not the answer

The obvious next thought is to run the operator properly: a Talos Linux VM as a
single node cluster under KVM, install the VerticaDB operator, apply a
VerticaDB resource, and freeze the result the way D57 freezes the Windows
machines. Gemini and DeepSeek were both asked and both said no, plainly, and
the reason is better than the recommendation.

The stack is not a VM, it is five things. Kubernetes on Talos, cert-manager for
the operator's admission webhook, communal object storage because the operator
runs Vertica in Eon mode only and Eon needs S3, so MinIO as well, then the
operator, then the custom resource. All of that to read catalog tables.

The part that settles it is that the analogy to the Windows machines fails.
Those work as frozen baselines because a Windows machine has exactly one time
sensitive thing in it, the evaluation license, and D65 handles that by rearming
at every boot. A Kubernetes cluster has many: etcd leases, node heartbeats, API
server certificates and service account tokens all expire while the snapshot
sits on disk. It boots and then needs a person. A frozen baseline that needs a
person is not frozen, and the whole value of the Verified tier is that a
release can be measured a year later without an archaeology session first.

Both models independently named the same third route, and it is route B: put
the Community Edition package on an ordinary machine and let `admintools`
create a single node database. That is exactly what `one-node-ce` does.

## And then the download went away too

Checked on 2026-09-26, after Rocket Software took Vertica over from OpenText.
`vertica.com/try` answers 403. The community edition download page still
answers 200 and now serves OpenText's generic Information Management marketing
with no download on it. Rocket's own Vertica pages answer 403 from here. The
cause can be geography or bot filtering rather than absence, so that one is not
proven either way.

So route B is blocked as well, and not on a registration anybody can complete.

What does still work is an unmaintained third party image.
`saadmairaj/vertica:10.1.1-RHEL6`, published in 2021, starts cleanly on a
current host, creates its database, and answers with a complete `v_catalog`:

	Vertica Analytic Database v10.1.1-0

That is a real Vertica and the queries can be written against it. It is not
a release anybody runs, it is five years old, it is built by a stranger, and
nothing about it can be rebuilt or reproduced. `docs/EVALUATION.md` step 2
rejects it, and D40 forbids calling a version supported without naming its
tier, and there is no tier for "verified once against an unmaintained image of
a dead release". A model written on it satisfies rule 9 in the letter and not
at all in the spirit: the queries are verified against something no consumer
will ever connect to.

So Vertica waits until a current release can be started. It is not next.

D88 changed that on 2026-09-27. A community copy of the one-node-ce image
at 25.1 starts, and Ken chose to take this image and three older ones like it
as the release range.

The four products after it on this list all have live images that need no
account: `saplabs/hanaexpress` last rebuilt in November 2025,
`firebirdsql/firebird` and `apache/hive` rebuilt the day before this was
written, and `exasol/docker-db` two weeks before. Firebird is next.

Below those and worth a model only if somebody asks: Couchbase, Ignite,
VoltDB, YDB and Databend. Each runs the real engine in an image and none of
them is shaped much like the 55.

ScyllaDB was not on this list, and Ken asked for it on 2026-09-27. It is a
flavor of the Cassandra model rather than a model of its own, so it took no
place in the order. D91 is the decision.

Ken asked about Couchbase on the same day. It is in `container/` without a
model, for the n1ql driver's tests, and D94 records why the model waits.

Avatica is not on the list at all. It is a wire protocol in front of whatever
database somebody put behind it, so it has no catalog of its own to read.

## An emulator is not the same thing as a container

The first pass at this conflated two things and the distinction turned out to
be the whole answer.

Most of what looks cloud-only here is not a cloud service. Vertica CE, Exasol,
SAP HANA Express, YDB, Databend, ClickHouse, Trino, Presto, Hive, Impala,
Firebird, Couchbase, Ignite and VoltDB all ship the real engine in an image.
The catalog in the container is the catalog in production, and a query written
against one is a query that works against the other. Those are containers and
the ordinary rules apply.

The genuine cloud services are different, and their emulators do not carry a
catalog worth testing against. The Spanner emulator implements a basic
`INFORMATION_SCHEMA` with tables and columns and no roles, no privileges and
no change streams, so a metadata query passes there and fails in production.
The DynamoDB and Cosmos DB emulators have no SQL catalog at all, because
neither product has one: metadata is a control plane API call rather than a
table.

So an emulator never counts as a container for D40's purposes. A product whose
only local option is an emulator is in the same position as one with no local
option: it is last, and it is Archived on arrival. That this agrees with
excluding DynamoDB, Cosmos DB and Tablestore as non relational is a
coincidence worth noticing rather than the reason.

## Last, the ones that need an account

Snowflake, BigQuery, Databricks, Athena, MaxCompute and Alibaba Tablestore
have no local emulator that both reviews agreed on. Both were asked whether
credits make them testable and both said the same thing: a free tier exists
for each, and using it from CI means pre-created credentials.

That is a different kind of dependency from a container and a worse one. A
secret in CI, an account that expires, a bill that can arrive, and a test that
fails for everybody when somebody else's trial ends. D40's tiers assume a
release can be started on demand, and none of these can.

So they are last, and a model for one of them is Archived on arrival unless
the account question is answered first. The two reviews disagreed about
MaxCompute and Tablestore, one calling them trial only and the other naming an
official emulator, which is a lead to run before either is scheduled.

## What gets no model at all

Four are another driver for a product already here, and want a flavor key or
nothing: `pgx` is PostgreSQL, `mymysql` is MySQL, `moderncsqlite` is SQLite
and `godror` is Oracle. `netezza` is PostgreSQL derived and is perhaps a
flavor key, which is a lead rather than a fact.

Three are not relational and the 55 do not apply: DynamoDB, Cosmos DB and
Tablestore are key value stores with no SQL catalog to read.

Five are not products: `adodb` and `odbc` are bridges to whatever sits behind
them, and `csvq`, `chai` and `ql` read files or embed, with no catalog beyond
what `information_schema` already covers.

## The rule this follows

Order by whether it can be started, then by whether anybody is blocked, then
by what the native catalog adds. Not by popularity: Snowflake and BigQuery are
near the top on user count and are last here, because a query that has never
run against a real server is not finished and neither of them can be run on
demand.
