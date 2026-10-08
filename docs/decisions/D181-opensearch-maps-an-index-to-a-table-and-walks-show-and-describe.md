# D181. OpenSearch maps an index to a table and walks SHOW and DESCRIBE

Status: Decided, amended by D189 and D191.

## The decision

Ken asked on 2026-10-08 for an OpenSearch dialect. `models/opensearch` answers 3
of the 56 on 3.9.0, and 2 of the 3 on 2.19.6, through dbimp's opensearch driver
at v0.14.0, which dburl v0.46.0 names. The dialect is `opensearch` and
`dialect.go` holds it. Ken reviews the mapping below.

OpenSearch answers SQL on `POST /_plugins/_sql`. Its SQL reads only. It has no
table that a SELECT reads, and its only metadata statements are `SHOW TABLES
LIKE <pattern>` and `DESCRIBE TABLES LIKE <pattern>`. `SHOW TABLES` with no
LIKE fails with HTTP 400. SHOW SCHEMAS, CATALOGS, FUNCTIONS, COLUMNS,
DATABASES, GRANTS and VARIABLES fail with the same message. `SELECT VERSION()`,
`SELECT USER()` and `SELECT DATABASE()` fail.

## The mapping

| OpenSearch | Kind | Why |
| --- | --- | --- |
| the cluster | database, and the catalog of every row | TABLE_CAT of SHOW TABLES names it. D176 chose it. No statement names a cluster that shows no index to the user, so this is a close call for Ken |
| an index | table | SHOW TABLES lists it as BASE TABLE |
| an alias | table on 2.19.6, and not listed on 3.9.0 | see below |
| a field of the mapping | column | DESCRIBE. An object and a nested field are rows with the types object and nested, and a subfield is a row of its own, such as `dims.h` |

There is no schema. D176 chose that. Every `schema` field is empty, and a
`schema` pattern other than the empty one matches nothing.

## An alias is not a view here

D176 said an alias is a view where the product lists it. SQL lists an alias on
2.19.6, with the type BASE TABLE, the same word as for an index. No row tells
the two apart, and `DESCRIBE` of an alias gives the columns of its index. 3.9.0
does not list an alias in SHOW TABLES, and it describes one when asked by name.
So Views is not answered on either release, and on 2.19.6 the alias
`dbmeta_recent` is a table. A consumer that reads Tables on 2.19.6 sees one
row more than it sees on 3.9.0. `TestOpenSearchFixtureObjects` asserts both.
The alias API tells an alias from an index, and it is HTTP and not SQL.

## The walks

D175 allows OpenSearch to walk. Each kind is a walk, and none is one statement.
The statements cannot take a pattern that works. An underscore in `SHOW TABLES
LIKE` is a wildcard with no escape, and a pattern in DESCRIBE merges every index
that matches into one table named for the pattern, so the rows cannot say which
index owns a column. So the walks ask for every index and match the patterns of
the caller in Go, with `dbmeta.Like`. The cost of each walk:

| Kind | Statements | Cost |
| --- | --- | --- |
| Tables | SHOW TABLES LIKE % | one statement |
| Databases | SHOW TABLES LIKE % | one statement, which yields the cluster once |
| Columns | SHOW TABLES LIKE %, then DESCRIBE TABLES LIKE '<name>' for each index | one statement and one more for each index that matches |

The cost check of D47 ran on 3.9.0. 200 indices of 5 fields were read in 181
milliseconds on a pool of one connection, which is about 1 millisecond for each
index. `TestOpenSearchManyIndices` keeps a smaller check of 100 indices. A walk reads the names to the end before it runs the first DESCRIBE, so
it holds one connection at a time. The server reads the mapping in the cluster
state and never a document. A caller that wants one index in a large cluster
pays for the list and for the DESCRIBE of the indices whose names match the
`parent` pattern, because the walk keeps only those.

A name goes into DESCRIBE as a literal in single quotes, because an index name
with a dot or a hyphen gives no row, and no error, when it is not quoted. The
walk doubles a quote inside the name with `QuoteLiteral`. A quote inside an index
name was not measured.

## An index that DESCRIBE refuses

The security plugin refuses DESCRIBE of an index that the role does not read,
with `no permissions for [indices:admin/mappings/get]`. The ordinary user of the
tests gets that for `secret_idx`, on both releases. SHOW TABLES lists the index
all the same, because the role holds `indices:admin/get` on every index (D176).
So the walk goes on to the next index when a DESCRIBE is refused, and the
refused index has no column. A walk does not skip an error while it reads the
rows of an answer, and it does not skip one when the context ends.

This is a choice for Ken to review. The walk cannot tell a refusal of the plugin
from a broken connection by its type, because the model cannot import dbimp. A
broken connection fails the first statement, SHOW TABLES, before any DESCRIBE.
The survey said that DESCRIBE of an empty index fails with HTTP 500 on 3.9.0.
That did not happen on 3.9.0: an index with no mapping, and one with an empty
mapping, both give no rows.

## What the driver cannot read

On 2.19.6 the answer of DESCRIBE declares every column keyword, and sends
numbers in some of them, such as 10 in NUM_PREC_RADIX and 2 in NULLABLE. dbimp's
driver fails the row with ErrInvalidValue for NUM_PREC_RADIX, even when the
caller scans the column into `any`. 3.9.0 declares integers and the driver reads
them. So Columns has no answer on 2.19.6 through the driver, and the reading
stops with that error at the first row. SHOW TABLES reads on both. This is
dbimp's open question 15, and the coordinator asked dbimp to fix it. The model
has no fallback: `SELECT * FROM <index> LIMIT 0` gives the top level fields of
an index, with no subfield and no position, and a walk cannot know the release
to choose it.

The tests skip what needs Columns on 2.19.6, under one condition,
`describeReadable` in `test/opensearch_test.go`. Remove it when dbimp fixes the
row, and the skips go with it. The conformance target skips the release too.
Parity records the release under `opensearch@2`, where no column answer
differs, because the driver fails for every principal.

## What a column says

A column reads the type of the mapping as DESCRIBE names it, in lower case,
such as long, text, keyword and timestamp. A date is `timestamp` on both
releases. There is no list of types to match it against, so the name is the
text of the server and nothing more. Every column is nullable, because DESCRIBE
reports NULLABLE 2, which is unknown, and a document can leave any field out.
None has a key, a default or a comment. The position is DESCRIBE's, which is not
the order of the mapping, and which starts at 0 on both releases.
DESCRIBE leaves out a multi-field, such as `name.raw`, and a field of a type SQL
cannot read, such as integer_range. The survey said that the ordinal starts at
1 on 3.9.0, that 3.9.0 lists every type, and that DESCRIBE of an alias gives no
row there. None of that held in this measurement. Both releases answered the
fixture the same way, except that 2.19.6 lists the alias in SHOW TABLES and the
driver cannot read its columns.

## The terminator and the syntax

usql said that opensearch-3.9.0 accepts a semicolon at the end of a statement.
Both releases accept it, for SELECT, SHOW and DESCRIBE, so the model keeps the
terminator, which is the zero value. It reads `--`, `#` and `/* */` as comments
and refuses `//`. A name between backticks works, and a name between double
quotes is read as a string, so `SELECT * FROM "dbmeta_author"` fails with no
such index. Both were measured on 3.9.0, and the first on 2.19.6 for `--` and
`/* */`. `TestOpenSearchSemicolon` asserts the semicolon.

## The version, and who can read it

No SQL statement names the release. `GET /` does, in `version.number`, to the
administrator. The ordinary user gets HTTP 403, `security_exception`, for
`cluster:monitor/main`. 3.9.0 sends the header `X-OpenSearch-Version` with every
answer, also to the ordinary user, and 2.19.6 sends none. D176 asked that the
model read the release over HTTP as the administrator. A model has only a
`Queryer`, so the model follows Elasticsearch (D177): `Dialect.Version` reports
an unknown version, and `Dialect.ParseVersion` reads the `version.number` that a
caller got from `GET /`, and gives the line `OpenSearch 3.9.0`. The tests read
it as the administrator, and `TestOpenSearchVersionForAnOrdinaryUser`
asserts the refusal (D191).

## The kinds that are not answered

53 kinds. Roles, privileges and role grants are in the security plugin, which is
HTTP. Settings are in the cluster settings API and in the settings of the SQL
plugin, which are HTTP too. The `_meta.comment` of a mapping, which the fixture
sets, is not in SQL: REMARKS is NULL. SHOW FUNCTIONS and SHOW SCHEMAS fail, and
SQL has no list of types. The types a cluster uses are the distinct TYPE_NAME
of its columns, and that is a list of use, not a list of types, so it is a
stretch and the model leaves it out. Indexes, constraints, triggers, sequences,
collations, domains and the other kinds have no source. An alias is not a view
(above). `docs/COVERAGE.md` holds the rest, and the second opinions.

## The fixture

OpenSearch has no DDL in SQL, so `models/opensearch/fixture` is a list of HTTP
requests that a test sends as the user of the DSN. Every index starts with
`dbmeta`, because the role of the ordinary user reads `dbmeta*`. The four core
tables of D53 are `dbmeta_author`, `dbmeta_book`, `dbmeta_region` and
`dbmeta_shipment`. The alias `dbmeta_recent` is over `dbmeta_book`. The fixture
also makes `dbmeta_types`, `dbmeta_empty` and `secret_idx`, which only the
administrator can describe. The package imports nothing.

## The role

D176 widened the role of the entry to `indices:admin/get` and
`indices:data/read/search` on every index, and `cluster:monitor/health`. It was
measured on 2.19.6 only. It was measured on 3.9.0 now. The ordinary user runs
SHOW TABLES, DESCRIBE of an index it reads, and a paged SELECT on both releases,
so the role needs no change. It can see the name of every index. It cannot
describe an index outside `dbmeta*`, on either release.

## The principals

Parity runs three. The ordinary user that the entry makes, a reader whose role
grants `read`, `indices:admin/get` and the mapping on `dbmeta_author` alone, and
a lister whose role grants `indices:admin/get` on every index and nothing else.
The reader is refused SHOW TABLES, because the plugin needs `indices:admin/get`
on every index to run it. So Tables, Databases and Columns are refused to the
reader, and no principal sees only one index, as one does on Elasticsearch. The
ordinary user and the lister run SHOW TABLES, and get fewer rows from Columns,
because they cannot describe `secret_idx`. The lister is refused every DESCRIBE,
and so gets no column at all, which the walk reports as no row and not as an
error. The current user is not answered. On 2.19.6 the Columns answer is an
error for every principal, so no difference is recorded there.

## The tier

Both releases were Staged with the cadence Tested, which
`container/opensearch.go` records. The entry calls `add` instead of `staged`
(D119, D120). 2.19.6 passes every test but the ones that need Columns, which
skip with the reason.
