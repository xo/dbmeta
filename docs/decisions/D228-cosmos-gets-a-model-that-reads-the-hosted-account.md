# D228. Cosmos DB gets a model that reads the hosted account

Status: Amends D223 and D225, amended by D229.

## The decision

D223 left Cosmos DB without a model, because the driver that dburl names read only
documents. Ken decided on 2026-10-11 to ask dbimp for statements that read the REST
resources, and dbimp tagged them as v0.17.0. dburl v0.50.0 names
`github.com/xo/dbimp/cosmos` for the cosmos scheme, so D154 is met. The model is
`models/cosmos`. It answers 10 of the 65 kinds: databases, schemas, tables, indexes,
partitioned tables, functions, triggers, roles, privileges and settings.

The test module moves to dbimp v0.17.0 and to dburl v0.50.0, and `dbrun` registers the
driver. dburl v0.50.0 also names the drivers of dbimp for BigQuery, Athena, Databricks and
Spanner, and the tests of those four do not move here. They keep the drivers of D220, D222,
D224 and D216 until Ken says otherwise, and the backlog says so.

## What it reads

Nine reserved names that the driver answers with one GET request each: `$databases`,
`$containers`, `$functions`, `$triggers`, `$users`, `$permissions` and `$offers`, and
`$account` and `$stored_procedures`, which no kind reads. A statement is
`SELECT * FROM "$name"`. It takes no list of columns and no LIKE, so the `Keep` function
of each binding narrows the rows in Go, as the Cassandra model does (D200).

A statement names no database and no container. The driver takes both from the path of the
URL, so a connection reads one database and, for the functions and the triggers, the one
container that the URL names. A URL with no container makes the driver refuse those two
statements before it sends a request. The grammar of the driver can name a database in a
WHERE, and the model does not use it, because a parameter of dbmeta is a pattern and the
key takes an exact name. Reading each database is a walk, and no walk is allowed for Cosmos
DB (D223).

The fixture is a list of REST requests in `models/cosmos/fixture`, and the tests sign and
send them with the key of the account. The SQL writes nothing and the driver reads only, so
this is the only way to make a container. The signature is the HMAC of the verb, the
resource and the date, and the standard library does all of it. The key never appears in an
error or a log line, because every error of the helper is a fixed message.

## The analogues

Each is a choice that Ken can reverse:

- A Cosmos DB database is a database and also a schema, the way an ArangoDB database is
  (D168). The catalog of every row is empty.
- A container is a table of the type `container`. Its partition key, time to live, unique
  keys, computed properties, conflict resolution and geospatial type are in `Table.Options`.
- The indexing policy is the one index of a container, named `indexing_policy`, and its
  definition is the whole policy as JSON text.
- A container is split by the hash of its partition key, so it is a partitioned table with
  the strategy `hash`.
- A user defined function is a function, and a trigger is a trigger. The definition of a
  trigger is its REST resource as JSON, because it has a type and an operation that no
  field holds.
- A user is a role and a permission is a privilege. The name of a privilege is the resource
  link, which is rid based, because the feed names the resource by nothing else.
- A throughput offer is a setting.

## What it does not answer

A document has no declared attribute, so there is no column. A sample of the documents is a
read of the data, and D47 does not allow it. A row of `$containers` holds many index paths,
many unique keys and many computed properties, and a binding returns one object for each row,
so index columns, constraints and constraint columns are not answered. Ken can decide whether
a binding can return several objects for one row. It needs one statement, so it is not a
walk, and the backlog holds the question. Stored procedures have a statement of their own,
and one statement answers one kind, so `Functions` reads the user defined functions. The
current schema and the current user have no statement.

## The version

Cosmos DB is a service with no release, and no statement reports one. The model declares no
version query, so the version is unknown. usql registers its driver with no `Version`, so it
runs no statement either.

## Parity

The principals are the primary key and the read-only key of the account, and both are account
wide. The read-only key is refused HTTP 401 by the feeds of users and permissions, so Roles and
Privileges are refused to it. Every other query gives the same rows.

## The cost

A feed costs 1 or 2 request units for a page, and `$permissions` is one request for the users
and one for each user. The cost grows with the number of containers and users and never with
the number of documents. A database with shared throughput holds few containers at the free
tier, so no catalog with thousands of containers was built.

## What was measured

On 2026-10-11, on the hosted account of dbsetup, in the database `dbmeta`. A vector embedding
policy, a vector index and a change feed retention are refused, because the account has neither
capability. A database is refused as the resource of a permission, so the permissions of the
fixture are on containers. The role feed that Gemini and DeepSeek named does not exist as they
wrote it: `/roledefinitions` is HTTP 401, because it is the role based access control of the
account, and `/dbs/{db}/roledefinitions` and `/dbs/{db}/roles` are HTTP 400. The database
`dbimp_test` of the account belongs to dbimp, and no test touches it.
