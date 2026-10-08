# D179. Solr maps a collection to a table under a fixed schema

Status: Decided, amended by D191.

## The decision

Ken asked on 2026-10-08 for an Apache Solr dialect. `models/solr` answers 4 of
the 56 on Solr 9.9.0, 9.10.1 and 10.0.0, through dbimp's solr driver at
v0.14.0, which dburl v0.46.0 names. The dialect is `solr`. Ken reviews the
mapping below.

Solr answers SQL with Apache Calcite, at `POST /solr/{collection}/sql`. The SQL
has no INFORMATION_SCHEMA and no SHOW. Its only catalog is the schema
`metadata`, with two tables, TABLES and COLUMNS, in the form of JDBC's
DatabaseMetaData. Every kind that the model answers is one statement over one
of them. Solr has no DDL in SQL.

## The mapping

| Solr | Kind | Why |
| --- | --- | --- |
| a collection | table | TABLES lists it, and COLUMNS lists its fields |
| an alias of a collection | table | SQL cannot tell it from a collection. TABLES lists it as TABLE and COLUMNS lists the fields of its collection |
| a field of a collection | column | COLUMNS. Solr adds `_nest_path_`, `_root_`, `_text_`, `_version_`, `_query_` and `score` to the fields of the schema |
| the schema `solr` | schema | fixed, see below. It is always listed, and it is the current schema, because an unqualified name resolves to a collection |
| the schema `metadata` | schema, system | listed only with with_system. Its two tables are tables of the type `system table` |
| the cluster | catalog `solr` | Solr names no cluster in SQL, so the model reports the fixed name `solr` |

## The schema name is fixed

TABLES and COLUMNS name the schema of a collection with the address of
ZooKeeper, such as `192.168.1.5:9983`. It changes with the machine, and a
consumer cannot depend on it. Ken chose a fixed name on 2026-10-07 (D176). The
statement turns every name but `metadata` into `solr`, and a filter on the
schema matches the name that the model reports and not the one that Solr gives.

## What the SQL cannot do, and how the statements work

The SQL module of Solr plans a statement over a collection with Calcite, and
plans metadata.TABLES and metadata.COLUMNS as plain tables. These facts were
measured on 10.0.0 and hold on the other two:

- A literal, a CASE, a CAST and a UNION ALL work over the metadata tables.
- A bare `NULL` in a projection fails. `CAST(NULL AS VARCHAR)` works, and so
  the model pads a field with it, which is a real NULL (docs/NULLS.md).
- A bare string literal is a CHAR of its own length, and a CASE pads the
  shorter branch with spaces. Every literal in a statement is cast to VARCHAR.
- The handler sets the lexical rules of MySQL, so a name between double quotes
  is a string, and the alias of every column is in backticks.
- STRPOS does not exist. The filter on the type searches the list with LIKE.
- The driver writes each parameter into the statement as a literal, so a
  filter can compare two literals, and Calcite folds it.

## What a column says

Every column reads nullable, with no default and no key. The SQL module reports
nothing else, and it does so for the unique key `id`, which the schema requires.
So `Column.Nullable` is always true, `Column.PrimaryKey` is always false and
`Column.Default` is always absent. The Luke handler holds the unique key and the
flags of each field, and it is an HTTP call, so no statement reads it.

The type is the JDBC type code that COLUMNS holds, written as a name: VARCHAR,
BIGINT, DOUBLE, TIMESTAMP and ANY. A boolean field reads VARCHAR, a
multi-valued field reads ANY, and a text field reads VARCHAR. The type of the
field in the schema is not there. The ordinal is the position that COLUMNS
gives, which is not the order of the schema: the fixed fields come first, then
the fields of the collection in the order of their names, then `_query_` and
`score`.

## No kind is a walk

D175 allows a walk for Elasticsearch and OpenSearch, and none is allowed for
Solr. A kind that only HTTP reads is not answered. That is the aliases
(`LISTALIASES`, so Views), the cluster state (`CLUSTERSTATUS`, so partitioned
tables), the users and roles (`security.json`), the configuration and the Luke
handler. D162 holds the same rule for a source that scans data, and these are
a different case: a Queryer runs SQL, and these are not SQL.

D176 asked that an alias be a view. Solr SQL lists an alias as a table, and
the request that tells it from a collection is not a statement, so the model
cannot do it. An alias is a table, and COVERAGE.md records that.

## The version, and who can read it

No SQL statement returns the release. `SELECT CURRENT_USER` returns `sa` for
every user and `version()` does not exist. `GET /solr/admin/info/system` holds
`lucene.solr-spec-version`, and only an administrator can read it: the ordinary
user gets HTTP 403. So the model has no version statement and
`Dialect.Version` reports an unknown version, as it does for InfluxQL (D165).
A caller that reads the release over HTTP as the administrator passes it to
`Dialect.ParseVersion`, which reads `10.0.0`. This is Ken's answer in D176. The
ordinary user gets no release. A test of the refusal,
which D192 renamed, asserts it.

usql's driver reports the word Solr for the same reason (docs/USQL.md).

## The cost check of D47

Ken asked on 2026-10-08 for the cost of metadata.TABLES and metadata.COLUMNS
against a catalog with thousands of collections, before the model was written.
It was measured on 10.0.0 with the entry as it is: 1 GB of heap, one node. A
collection has one shard and a configuration set of its own, made through the
Collections API and the Configsets API, 6 at a time.

The measurement did not reach thousands, and the reason is the result.

- With the heap of the entry, 1 GB, the server died with an OutOfMemory error
  at about 300 collections.
- With a heap of 3 GB, set for the measurement only and restored after it, the
  server held 1062 collections and died between 800 and 1600 when the load
  continued. Each collection costs about 3 to 4 MB of heap. Thousands of
  collections need a node with a heap of many gigabytes, which is a limit of
  Solr and not of the model.
- Time is linear in the number of collections, and about 1 ms for each
  collection while the heap is not under pressure. On 3 GB: 400 collections,
  TABLES 20 ms and COLUMNS 0.35 s. 800 collections, TABLES 40 to 70 ms and
  COLUMNS 0.6 s. 1062 collections, TABLES 70 ms and COLUMNS 0.95 s.
- A filter does not prune. `WHERE tableName = 'cc_5'` on COLUMNS costs the same
  as no filter, because Calcite scans the table, so each statement reads every
  collection. COLUMNS for one collection cost 0.35 s at 400 collections and
  0.95 s at 1062.
- When the heap is nearly full, the same statements slow down by 10 to 30
  times. COLUMNS took 31 s and TABLES 9 to 11 s at 1065 collections on 3 GB.
- Under that pressure COLUMNS left collections out with no error. It listed
  951 of 1065. A consumer cannot tell a collection with no fields from a
  collection that the statement dropped.

Ken decided on 2026-10-08 to leave the heap of the entry at 1g and to record
the limit. A consumer that reads a large cluster must read one table at a time
with a filter on TABLES, which is cheap, and must not rely on COLUMNS to name
every collection. The queries stay as they are, because a field is added when
one statement produces it (rule 13), and these are one statement each. Their
cost is a cost of the SQL module of Solr.

## The fixture

Solr has no DDL in SQL. `models/solr/fixture` holds the requests for the four
core collections of D53, author, book, region and shipment, and the alias
`recent` of book, which is the core view. A test sends them as the
administrator: the Configsets API makes a copy of `_default` for each
collection, because collections made from `_default` share one managed schema
and a field added to one appears in all. Then the Collections API makes the
collection, the Schema API adds the fields, and the update handler adds the
documents. A step of a collection that exists is skipped. The fixture cannot
build a foreign key, a default, an index, a trigger, a sequence, a comment or
a type, so those kinds are not answered.

## The principals

Parity runs one lesser principal, the ordinary user of the entry, who has the
role `search` and can read a collection and run SQL on it. It sees the same
rows as the administrator for all four kinds, on all three releases. The
section `solr/same/user` in `testdata/parity.txt` is empty for that reason. A
reader with less than the role search, who reads one collection alone, was not
measured. The entry grants `read` on every collection, and a rule for one
collection needs another security.json. Neither principal can read the
version with a statement.

## The DSN and the driver

dbimp's driver sends each statement to the handler of the collection that the
path of the DSN names, and refuses a statement when the path is empty. The
entry printed a DSN with no path, so a test needed `WithDatabase`. The DSN of
each principal in `container/solr.go` now ends with `/dbmeta`, the collection
that Init makes, and the DSN is what `sql.Open` takes (D167). `dbrun usql`
reads the same URL.

The statement ends with no semicolon, because Solr refuses one. The model sets
the terminator to TerminatorStripped, so that usql removes it. The syntax
flags are the block comment and the backtick, the lexical rules of MySQL.

## The tier

The three releases were Staged with the cadences Tested for 9.9.0 and 10.0.0
and Nightly for 9.10.1. They keep them, and `container/solr.go` calls `add`
instead of `staged` (D119, D120). A Solr container takes 1 GB of heap and
about 2 GB in all, so a test starts one release at a time.
