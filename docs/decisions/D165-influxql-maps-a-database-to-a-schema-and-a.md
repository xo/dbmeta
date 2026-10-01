# D165. InfluxQL maps a database to a schema and a measurement to a table

Status: Decided.

## The decision

Ken asked on 2026-10-01 for an InfluxQL dialect. `models/influxql` answers 7
of the 56 on InfluxDB 1.13.1 and 1.11.8, and the server answers 4 of those 7
on InfluxDB 2.9.1 and 2.8.0, through dbimp's influxdb driver, which dburl's
influxql scheme names. The dialect is `influxql`, which `dialect.go` already
named. Ken reviews the mapping below.

InfluxQL reads metadata only through SHOW statements. Each statement reads
one database, which ON names, and no statement reads every database at once.
So most kinds are a walk, which D159 allows. The cost of each walk is beside
its binding and in the table below.

The four releases move from Staged to the cadence each recorded: 1.13.1 and
2.9.1 are Tested, and 1.11.8 and 2.8.0 are Nightly (D120).

## The mapping

| Kind | InfluxQL | Source | Statements |
| --- | --- | --- | --- |
| `Schemas` | each database | SHOW DATABASES | one |
| `Databases` | each database, the same rows | SHOW DATABASES | one |
| `Tables` | each measurement, with the type `table` | SHOW MEASUREMENTS ON | one, and one for each database |
| `Columns` | time, and each tag and field of a measurement | SHOW FIELD KEYS ON, SHOW TAG KEYS ON | one, and two for each database |
| `Roles` | each user, InfluxDB 1 only | SHOW USERS | one |
| `Privileges` | each database, with every grant on it, InfluxDB 1 only | SHOW USERS, SHOW GRANTS FOR | one, and one for each user |
| `Settings` | each setting of the configuration, InfluxDB 1 only | SHOW DIAGNOSTICS | one |

### A database is the schema

InfluxDB names a measurement `"database"."retention policy"."measurement"`,
so the name has three parts, as a table in a catalog does. A retention policy
is not the schema, because a measurement does not belong to one. SHOW
MEASUREMENTS lists the measurements of a database, and on 1.13.1 with the
default index it refuses a filter on a retention policy:
"retention policy filter for measurements not supported for index inmem". So
the database is the schema, as on ClickHouse and Neo4j, and the catalog is
empty. Every database a user can read is listed, because ON reaches each one.

On InfluxDB 2 a database is a bucket, through a mapping of a database and a
retention policy to it. InfluxDB 2 maps a database named for each bucket to
that bucket by itself. So SHOW DATABASES lists the buckets, `_monitoring` and
`_tasks` among them.

`_internal` on InfluxDB 1 and 3, and `_monitoring` and `_tasks` on
InfluxDB 2, are the databases InfluxDB keeps for itself. They are system
objects, and `with_system` includes them.

### A measurement is the table, and its tags and fields are its columns

A measurement is the one kind of relation, so its type is `table`, as on
InfluxDB 3 (D152). SHOW FIELD KEYS gives each field and its type, and SHOW TAG
KEYS gives each tag. Every measurement also has a time column, which no SHOW
statement lists and which SELECT * always returns first, so the model adds it.

The ordinal is the position in the answer of SELECT *, measured on 1.13.1 and
3.11.5: time first, then every tag and field by name. A field and a tag can
have the same name. SELECT * then returns the field first and the tag as
`<name>_1`, so the field comes first here too.

The type of a field is the word InfluxQL writes: float, integer, unsigned,
string or boolean. The type of a tag is `tag`, which is how InfluxQL writes a
cast to a tag, as in `"host"::tag`. A tag holds a string, and `string`
hides which columns are tags. The type of time is `timestamp`.

A point with the same tags and time replaces the one before it, which acts
like a primary key. InfluxDB does not call it one, and the InfluxDB 3 model
reports no primary key for the same behavior, so this model reports none
either. Ken can reverse both.

### A user is the role

InfluxDB 1 has users and no role that groups them. A user is an
administrator or is not. An administrator can do everything, and every other
user has only its grants. So Roles returns the users, with superuser,
create_role and create_db set for an administrator.

A grant is READ, WRITE or ALL PRIVILEGES on one database. SHOW GRANTS FOR
lists the grants of one user, so Privileges is a walk over the users: one
statement for SHOW USERS, and one for each user. D159 names a walk over the
databases and over the measurements, and a walk over the users is a third.
That was asked on 2026-10-01 and is open. If Ken does not allow it,
Privileges is dropped, and the rest stays. A row is one database, with
`user=privilege` for each grant on it. An administrator holds every privilege
without a grant, and SHOW GRANTS lists nothing for one, so no administrator is
named in a row.

### The configuration is the settings

SHOW DIAGNOSTICS on InfluxDB 1 reports the configuration as a series for
each section, such as `config-data`, with a column for each setting. A
setting is named `<section>.<setting>`, such as `data.cache-max-memory-size`.
The series `config` repeats `config-data` under a shorter name, so the model
skips it.

## InfluxDB 2 and 3

InfluxDB 2 answers SHOW DATABASES, SHOW RETENTION POLICIES, SHOW
MEASUREMENTS, SHOW FIELD KEYS, SHOW TAG KEYS, SHOW TAG VALUES and SHOW SERIES
through its v1 API. It answers "not implemented" for SHOW USERS, SHOW GRANTS,
SHOW DIAGNOSTICS, SHOW CONTINUOUS QUERIES, SHOW SHARDS, SHOW STATS, SHOW
QUERIES and the cardinality statements, measured on 2.9.1. Its users are v1
authorizations, which no InfluxQL statement lists.

InfluxDB 3 Core answers InfluxQL too, and dbrun sets the test variable of
influxql for it (D114). It parses only SHOW DATABASES, SHOW RETENTION
POLICIES, SHOW MEASUREMENTS, SHOW FIELD KEYS, SHOW TAG KEYS and SHOW TAG
VALUES, measured on 3.11.5. So Schemas, Databases, Tables and Columns give
the same rows on InfluxDB 1, 2 and 3, and the conformance section is the same
on all three. Its SQL is the dialect `influxdb`, which `models/influxdb`
reads, and nothing here changes that model.

## No statement names the release

No InfluxQL statement names the release on every release and for every user.
SHOW DIAGNOSTICS names it on InfluxDB 1, and only to an administrator, and
InfluxDB 2 and 3 do not have it. Only `GET /ping` names it, in the header
`X-Influxdb-Version`, and no statement reaches that. So the dialect has no
version statement, and `dbmeta.InfluxQL.Version` reports an unknown version.

The dialect still has a ParseVersion. A caller that reads the release from
the driver, as usql does through dbimp's `influxdb.Version`, passes it to
`dbmeta.InfluxQL.ParseVersion` and builds the meta with it. The tests do that,
which is how the parity file has a section for InfluxDB 1.

With an unknown version, a query cannot tell in advance that InfluxDB 2 or 3
lacks SHOW USERS, SHOW GRANTS and SHOW DIAGNOSTICS. So Roles, Privileges and
Settings report Supported on every release, and on InfluxDB 2 and 3 they
return the error of the server, wrapped. The model does not turn that error
into `dbmeta.ErrNotSupported`, because that means matching the text of a
server message. Ken can choose otherwise.

## The cost of SHOW FIELD KEYS and SHOW TAG KEYS

D47 leaves a kind unsupported when its only source reads the points rather
than an index or a catalog. Ken applied that to Neo4j on 2026-10-01. SHOW
FIELD KEYS and SHOW TAG KEYS read the index, and both were measured, each the
best of three runs:

| Release | Data | SHOW FIELD KEYS | SHOW TAG KEYS | SELECT count |
| --- | --- | --- | --- | --- |
| 1.13.1 | 1,000 points, 1 series | 0.0004 s | 0.0004 s | |
| 1.13.1 | 5,000,000 points, 1 series, 9 shards | 0.0004 s | 0.0003 s | 0.117 s |
| 1.13.1 | 100,000 series, 1 point each | 0.0006 s | 0.011 s | |
| 1.13.1 | 1,000,000 series, 1 point each | 0.0004 s | 0.122 s | |
| 2.9.1 | 2,000,000 points, 1 series | 0.0006 s | 0.0008 s | |
| 2.9.1 | 1,100,000 series, 1 point each | 0.0016 s | 0.0013 s | |

Neither grows with the points. SHOW FIELD KEYS does not grow with the series
either. SHOW TAG KEYS on InfluxDB 1 with the default index, which keeps the
index in memory, grows with the number of series, which is the size of the
index. On InfluxDB 2, whose index is on disk, it does not grow. A series is
an entry of the index, not a point, so this is the cost of reading a
catalog, and Columns is answered. If Ken reads a series as data, Columns is
dropped.

A catalog with thousands of measurements was measured too. On 1.13.1, with
5,000 measurements of one point each in one database, SHOW MEASUREMENTS took
0.002 seconds, SHOW FIELD KEYS 0.012 seconds and SHOW TAG KEYS 0.011
seconds, for answers of 49 KB, 414 KB and 289 KB. That is the cost of the
answer, and the walk adds no statement for each measurement.

## What is left unsupported, and why

Each of these has something close in InfluxDB, and each is a stretch:

| Kind | What InfluxDB has | Why it is not an answer |
| --- | --- | --- |
| `Tablespaces` | a retention policy, with a duration, a shard group duration and a replication factor | a tablespace belongs to the server and a retention policy to one database, so two databases each have an `autogen` and the name says nothing alone. A measurement is not placed in one either |
| `Views` | a continuous query, which SHOW CONTINUOUS QUERIES lists with its CREATE statement, on InfluxDB 1 only | a continuous query writes into another measurement on a schedule. No statement selects from it by its name, which is what a view is |
| `Subscriptions` | SHOW SUBSCRIPTIONS, on InfluxDB 1 only | an InfluxDB subscription sends each write to an outside address. A PostgreSQL subscription reads a publication from another server, which is the opposite direction |
| `ColumnStats` | SHOW TAG VALUES CARDINALITY ON a database WITH KEY = a tag, on InfluxDB 1 only | it counts the values of one tag across every measurement, so it is a statement for each tag, and a field has no count that does not read the points |
| `Indexes`, `IndexColumns` | the index of series, which holds every tag | it is not an object that a statement makes or names, and SHOW TAG KEYS is what lists the tags |
| `ExtendedStats` | SHOW SERIES CARDINALITY, on InfluxDB 1 only | it counts the series of a database or a measurement, and is not a statistics object that a statement makes |
| `CurrentSchema`, `CurrentUser` | the database of the request, and its user | no statement returns either. The request carries both |

The rest are absent from InfluxDB: constraints, triggers, sequences,
partitioned tables, comments, access methods, languages, conversions, casts,
collations, large objects, event triggers, functions, aggregates, routine
parameters, types, domains, operators and their classes and families, enum
values, role settings, role grants, default privileges, foreign data, the
publications, text search, extensions and their objects. InfluxQL has
functions such as MEAN, and no statement lists them. `docs/COVERAGE.md` holds
what Gemini and DeepSeek found.

## The dsn stays the HTTP address

The model reads InfluxDB 1 and 2 through dbimp's driver, which takes only the
`influxdb://` form. The two entries set `connectURL`, which D160 added for
libSQL, so dbrun gives the tests the `url` and connects `dbrun version` with
it. The `dsn` stays the plain `http://` address, so what dbimp's tools read
from `dbrun dsn` does not change. The tests add `sqlmode=disable` to the URL,
as dburl's influxql scheme does, so that InfluxDB 3 speaks InfluxQL too.

## Walks in the parity test

`TestPrivilegeParity` asked each query with its one statement, and a walk has
none, so it skipped every walk. It now asks a walk through its iterator and
compares the rows the same way. Impala is exempt from parity, so InfluxQL is
the first walk it measures.
