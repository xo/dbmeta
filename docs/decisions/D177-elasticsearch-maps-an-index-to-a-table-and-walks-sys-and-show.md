# D177. Elasticsearch maps an index to a table and walks SYS and SHOW

Status: Decided.

## The decision

Ken asked on 2026-10-08 for an Elasticsearch dialect. `models/elasticsearch`
answers 8 of the 56 on 8.19.22, 9.4.6 and 9.5.3, through dbimp's elasticsearch
driver at v0.14.0, which dburl v0.46.0 names. The dialect is `elasticsearch`
and `dialect.go` holds it. Ken reviews the mapping below.

Elasticsearch answers SQL on `POST /_sql`. Its SQL reads only. It has no table
that a SELECT reads, and it has the statements SYS TABLES, SYS COLUMNS, SYS
TYPES, SHOW FUNCTIONS and SHOW CATALOGS. A statement of that kind takes no
WHERE and no ORDER BY, and `SELECT ... FROM (SYS TABLES)` fails with an
unknown index.

## The mapping

| Elasticsearch | Kind | Why |
| --- | --- | --- |
| the cluster | database, and the catalog of every row | SHOW CATALOGS names it, and SYS TABLES and SYS COLUMNS put it in TABLE_CAT. D176 chose it. A remote cluster of cross cluster search is listed beside it. This is a close call for Ken |
| an index | table | SYS TABLES has the type TABLE |
| an alias | view | SYS TABLES has the type VIEW. D176 chose it. The model gives a name and nothing more, because the indices and the filter of an alias are in the alias API |
| a data stream | view | SYS TABLES lists it as a VIEW, the same as an alias. Its backing index, whose name starts with `.ds-`, is a hidden table |
| a field of the mapping | column | SYS COLUMNS. A subfield is a column of its own, such as the multi-field `name.raw` and the object field `dims.h` |
| a type of SQL | type | SYS TYPES, 38 rows. They are all built in |
| a function of SQL | function, or aggregate when SHOW FUNCTIONS says AGGREGATE | SHOW FUNCTIONS, 161 rows on 9.5.3. All are built in, so `with_system` lists them, as for Druid in D171 |
| `USER()` | current user | the one statement that names the user. It is a SELECT, so it is not a walk |

There is no schema. D176 chose that. Every `schema` field is empty, and a
`schema` pattern other than the empty one matches nothing. SYS TABLES and SYS
COLUMNS hold NULL there, and SHOW SCHEMAS gives no row. So Schemas and the
current schema are not answered.

A column reads the type of the mapping as SQL names it, in upper case, which
is TYPE_NAME of SYS COLUMNS: LONG, TEXT, KEYWORD, DATETIME. It is the name that
SYS TYPES uses. It is not the SQL type that DESCRIBE names, such as BIGINT.
Every column is nullable, because SYS COLUMNS reports NULLABLE 1 for every
field. SYS COLUMNS leaves out an object, a nested field and a field of a type
SQL cannot read, such as dense_vector and flattened, and its ordinal still
counts them, so a number can be missing.

## The walks

D175 allows Elasticsearch to walk. Each kind but the current user is a walk of
one statement, and none is a walk of several. The statement cannot take a
pattern, so the walk matches the patterns of the caller in Go, with
`dbmeta.Like`, and skips a row that does not match. The cost of each walk:

| Kind | Statement | Cost |
| --- | --- | --- |
| Tables and Views | SYS TABLES | one statement, one page for 3000 indices, 43 ms |
| Columns | SYS COLUMNS | one statement, 1000 rows to a page, and the driver follows the cursor. 24 pages and 1.7 s for 3000 indices of 8 fields, whatever the `parent` pattern is |
| Databases | SHOW CATALOGS | one statement |
| Functions and Aggregates | SHOW FUNCTIONS | one statement |
| Types | SYS TYPES | one statement |

The cost check of D47 ran on 9.5.3 with 3000 indices, one shard each, which
the survey had not done. SYS TABLES and SYS COLUMNS read the cluster state and
no document. The first row of Columns arrives after 250 ms, because a page is
1000 rows. A `parent` pattern does not make it cheaper. SYS COLUMNS takes a
pattern of its own, and then the TABLE_NAME of each row holds the pattern and
not the name of the index, so the model cannot use it. A caller that wants one
index and a large catalog pays for the whole catalog, which is bounded by the
mappings and not by the documents.

The driver follows the cursor. `TestElasticsearchPages` makes 30 indices of
50 fields, which are 1500 rows and two pages, reads them all, stops a walk on
its first row, and checks that no search context stays open on the node. The
walk streams the rows and never holds them in a slice.

## Hidden indices

Elasticsearch lists the hidden indices it makes itself, such as the backing
index of a data stream, in SYS TABLES and in SYS COLUMNS for the
administrator. SQL does not report the hidden setting. It names each such index
with a dot first. So `with_system` keeps a name that starts with a dot, and
without it the walk leaves it out. SYS COLUMNS lists the columns of a data
stream under its backing index and not under the data stream, so the view of a
data stream has no column until `with_system` is set. This is a rule of the
model and not of Elasticsearch, and a user can name an index with a dot too.

## The version, and who can read it

No SQL statement names the release. `SELECT VERSION()` fails, and the driver has
no function for it. Only `GET /` does, in `version.number`, to the
administrator. The ordinary user gets HTTP 403, `security_exception`, because
the role of the user holds no cluster privilege. D176 asked that the model read
it over HTTP as the administrator.

A model has only a `Queryer`, which holds no URL and no HTTP client, so it cannot
send that request. The model follows InfluxQL (D165). It has no version
statement, so `Dialect.Version` reports an unknown version, and no query
depends on the release. `Dialect.ParseVersion` reads the `version.number` that a
caller got from `GET /`, and gives the display line `Elasticsearch 9.5.3`.
`TestElasticsearchVersion` reads the release over HTTP as the administrator and
passes it, and `TestElasticsearchVersionRefusedToAnOrdinaryUser` asserts the
refusal. Ken can choose another rule: a function of the driver that sends the
request, which is a change in dbimp.

## The kinds that are not answered

48 kinds. Roles, privileges and role grants are in the security API, which is
HTTP, and in the restricted index `.security-7`, which an administrator can
read as a document store and which has native users only and no role name that
SQL lists. That is a stretch and the model leaves it out. Settings are in the
cluster and index settings API only. The `_meta.comment` of a mapping, which
the fixture sets, is not in SQL: REMARKS is empty for a table and NULL for a
column. Indexes, constraints, triggers, sequences, collations, domains, enum
values and the other kinds have no source. A data stream is not a partitioned
table here, because SQL reports no partition and no backing index of it.
`docs/COVERAGE.md` holds the rest, and the second opinions.

## The fixture

Elasticsearch has no DDL in SQL, so `models/elasticsearch/fixture` is a list of
HTTP requests that a test sends as the user of the DSN. The package imports
nothing, so OpenSearch can use it. Every index starts with `dbmeta`, because
the role of the ordinary user reads `dbmeta*` and no other name. The four core
tables of D53 are `dbmeta_author`, `dbmeta_book`, `dbmeta_region` and
`dbmeta_shipment`, and the core view is the alias `dbmeta_recent`. The
conformance report removes the prefix. The fixture also makes `dbmeta_types`
with the types that SQL reads differently and four that it cannot read, the data
stream `dbmeta_stream`, and `secret_idx`, which only the administrator can
read.

## The principals

Parity runs two. The ordinary user that the entry makes, who can read the
indices `dbmeta*`, and a reader that the test makes through the security API,
who can read `dbmeta_author` alone. Elasticsearch shows a user only the indices
it can read. So both principals get fewer tables and columns than the
administrator, who also sees `secret_idx`, and the reader gets fewer views. The
current user differs, as it does everywhere. No query is refused to either.

## The tier

All three releases were Staged with the cadences Tested and Nightly, which
`container/elasticsearch.go` records. 8.19.22 and 9.5.3 are Tested and 9.4.6 is
Nightly, and the entry calls `add` instead of `staged` (D119, D120). The three
answered the same, except where `docs/COVERAGE.md` says so.

## The terminator

usql found that Elasticsearch refuses a semicolon at the end of a statement. The
model sets `TerminatorStripped`, and the lexer flag for block comments. It
refuses `//`, `#` and a backtick, and it compares a name with its case.
