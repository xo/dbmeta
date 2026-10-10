# D223. No Cosmos DB model, because the driver reads only documents

Status: Decided, amended by D228.

## The decision

dbmeta builds no model for Azure Cosmos DB with the API for NoSQL. The session
that measured it stopped before a model. Ken chose what comes next, and the last section
says what. The entry in `container/cosmos.go` stays Staged and the dialect `cosmos` has no
queries.

## Why

Cosmos DB has no catalog that a SELECT reads. A database, a container, its
partition key, its indexing policy, its unique key policy, its time to live, its
stored procedures, triggers and functions, its conflict policy and its throughput
are resources of the REST API. Each one is a GET request on a path such as
`/dbs`, `/dbs/{db}/colls` and `/dbs/{db}/colls/{c}/sprocs`. The SQL of Cosmos DB
reads the documents of one container and nothing else. A query cannot name a
database, a system container or a view of the catalog.

dburl v0.49.0 names `github.com/xo/dbimp/cosmos` for the scheme `cosmos`, and D154
says that the driver of the registry is the one the tests use. That driver is not
tagged. The module cache holds no release of dbimp that has it, and the dbimp
tree holds it as work in progress. It sends a statement as one query to the
documents of the container that the DSN names. Its package comment says so, and
`docs/COSMOS.md` of dbimp records the hosted account. It has no statement for the
catalog, and its `Exec` always fails. So the driver that dburl names cannot read
any of the metadata.

usql uses `github.com/btnguyen2k/gocosmos`, and that driver does answer two
statements of its own: LIST DATABASES and LIST COLLECTIONS (measured on the
hosted account on 2026-10-11, through `dbrun usql cosmos`). Neither is a SELECT,
the driver is not the one that dburl names, and the answers are the raw bodies of
the feeds. LIST COLLECTIONS gave the indexing policy as JSON text and the
resource links. It did not give the partition key, the unique key policy or the
time to live. DESCRIBE DATABASE and DESCRIBE COLLECTION answered "invalid
query".

## What a walk can do

A walk is several statements that the Queryer sends and Go matches (D146, D159,
D175). It is allowed for Impala, InfluxQL, Elasticsearch and OpenSearch. It does
not apply here, because the Queryer has no way to send a REST request on a
resource. No statement of either driver reads the partition key, the unique keys,
the functions, the procedures or the triggers. So a walk answers at most the
database list, the container list and the indexing policy as text, through the
driver that dburl does not name. Ken must decide one of three things:
allow a walk through gocosmos, ask dbimp for statements that read the resources,
or leave Cosmos DB without a model. The recommendation is the second, because it
keeps D154 and costs one request for each list and no request for each container.

## What was measured

The account of dbsetup, the database `dbmeta` with 400 request units a second
that its containers share. `dbrun usql cosmos` ran CREATE COLLECTION, LIST
COLLECTIONS, DESCRIBE, INSERT, SELECT and DROP COLLECTION on one container, and
the container is gone. Nothing touched the database that dbimp uses. The
recordings of dbimp on the same account cover the REST paths, and they are the
source of the answers in `docs/COVERAGE.md`.

## Second opinions

Gemini and DeepSeek agreed on every kind (rule 14 of AGENTS.md). Sequences, views
and foreign keys are absent. Every other kind is present under another name, and
every name is a field of the container body or a feed of its path. Columns can only
be inferred by sampling documents. Both named a role feed, and the paths they gave
for it disagree, so it stays unmeasured. Each lead for the feeds of a container
is recorded by dbimp on the hosted account, except roles, materialized views and
the vector and full-text policies, which no run read.

## What Ken decided

Ken decided on 2026-10-11 to ask dbimp for statements that read the REST resources of
Cosmos DB. dbmeta writes the model when the driver has them and is tagged. No walk is
allowed for Cosmos DB.
