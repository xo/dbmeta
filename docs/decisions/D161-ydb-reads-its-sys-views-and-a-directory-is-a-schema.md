# D161. YDB reads its .sys views, and a directory is a schema

Status: Amends D45.

## The decision

Ken chose YDB as a dialect. `models/ydb` answers 7 of the 56 under the
dialect `ydb`, which is the name dburl gives the scheme. The driver is
`github.com/ydb-platform/ydb-go-sdk/v3`, which dburl names, through the
`database/sql` driver it registers as `ydb`. usql opens YDB the same way.
Both releases in `container/ydb.go`, 26.2.1.14 and 26.3.1.17, move from
Staged to Tested, the cadence each recorded (D119, D120).

The mapping below is for Ken to review.

## Where the facts are

YQL has no information_schema and no catalog of columns. The only source a
SELECT reads is the directory `.sys` in each database. It holds 32 views on
26.3. The ones this model reads are these:

| View | What it holds |
| --- | --- |
| `auth_owners` | every path in the database, with its owner, and nothing that says what kind of object the path is |
| `partition_stats` | every partition of every table, by path, including the tables that implement an index |
| `auth_permissions` | the explicit grants on each path |
| `auth_users`, `auth_groups`, `auth_group_members` | the users, the groups, and who belongs to which group |
| `ds_storage_pools` | the storage pools of the database |

The schema of an object, which holds its columns, its indexes, its key and
its changefeeds, is given only to a gRPC call per object, such as
DescribeTable. No SELECT reaches it. D146 allows a walk for Impala only, so
every kind that needs the schema of an object is unanswered.

PostgreSQL syntax gives `pg_catalog` and `information_schema`, and the server
refuses it unless a feature flag is set. A consumer reaches the server it is
given, so the model reads only what a default server serves.

## The mapping

YDB keeps its objects in a tree of directories. A database is a path, such as
/local, and a table is a path inside it.

| dbmeta | YDB | Read from |
| --- | --- | --- |
| catalog | the database, by its path, such as /local | the shortest path in `auth_owners` |
| database | the one database the connection names | the same |
| schema | a directory, by its path relative to the database, such as dbmeta/dbmeta_fixture | the parent of each path in `auth_owners` |
| table | a row table or a column table | a path in both `partition_stats` and `auth_owners` |
| tablespace | a storage pool, whose kind a column family names | `ds_storage_pools` |
| role | a user or a group | `auth_users` and `auth_groups` |
| role grant | a member of a group | `auth_group_members` |
| privilege | the explicit grants on a path | `auth_permissions` |

A table at the root of the database has the empty schema, and the root is
not listed as a schema. A nested directory is a schema of its own, so dbmeta
and dbmeta/dbmeta_fixture are two schemas. The parts of a path are joined
with a slash, which is how a YQL statement names the table.

The database is the shortest path in `auth_owners`, because it is a prefix
of every other path. It cannot be found as the parent of `.sys`, because a
user can name a table `.sys` in any directory, and measured on 26.3 the
server accepts it. No YQL function returns the database.

The system objects are the directories `.sys`, `.metadata` and `.sys_health`
at the root. A user can name a directory with a leading dot, so the test
names these three and not the dot.

## Two partial answers, which is why this amends D45

D45 allowed one query to answer part of a question, SQLite constraints, under
four conditions. Two queries here meet the same four.

Tables lists the row tables and the column tables, and no view. A view is a
path in `auth_owners` with no partitions, and so is a topic, an empty
directory and every other kind of object. No view tells them apart. The
tables are exact. The views are missing because no source names them, and
the field description of type says so. `TestYDBFixtureObjects` builds a view
and asserts that it is not listed.

Schemas lists every directory that holds a path, and no empty directory. An
empty directory is a path with no partitions and no children, which is what a
view is too. `TestYDBFixtureObjects` makes an empty directory and asserts
that it is not listed.

Privileges lists every path, and the type of a path is table, directory or
object. Object is the true answer for a path no view names the kind of, and
the field description says what it covers. That is not a partial answer.

## What it leaves out on purpose

Every table is reported as a table, and a column table is not called one.
`hive_tablets` says whether the tablet of a partition is a column shard, and
a new column table reports tablet 0 in `partition_stats` for about a minute,
measured on 26.3. The join then calls it a row table for that minute, which
is a wrong answer and not a partial one.

Indexes are left out although their names are exact. The table that
implements an index is in `partition_stats` at
`<table>/<index>/indexImplTable`. Nothing says whether the index is unique,
and `Index.Unique` is a plain bool, so a unique index reads as not unique,
which is wrong.

The current user is left out. `CurrentAuthenticatedUser()` returns the empty
string for root and for dbmetauser. `.sys/query_sessions` holds the sid of
every session, and the only way to find this session's row is to match the
text of the statement. Two connections that ask at once can each read the
other's row, so the answer is not exact.

The current schema is left out. A YDB session has no current directory. A
name resolves against the database, or against a prefix that one statement
sets with `PRAGMA TablePathPrefix`, which nothing reads back.

The size of a database is left out. It is a sum over every partition in
`partition_stats`, which grows with the catalog, and hard rule 13 refuses
that.

## A fault in the server, and how the statements avoid it

Measured on 26.3, a read of a `.sys` view fails with an internal error when
the filter is false before any row is read: "requirement
!Meta->GetReads()[0].GetKeyRanges().empty() failed". A filter on a parameter
alone is decided before the read, so `WHERE $p0 = 'x'` fails, and `WHERE
$p0 = 'x' OR Path IS NULL` returns no rows. Tables reports one type, so its
types filter names no column of its own, and a type that matches nothing
failed the scan test. The filter now also tests the name, which is never
absent.

## Parameters

ydb-go-sdk names a value that has no name `$p0`, `$p1` and so on, counting
from zero, and YDB takes a parameter that no DECLARE names. So the
placeholder is `$p` and the position less one, and a consumer needs no
option in the connection string.

## Who can ask

Every `.sys` view refuses a user that is not an administrator. The dbrun
setup gives dbmetauser the right to read and describe /local/dbmeta, and
the user is refused every query. The parity file records the seven
refusals. A grant on a `.sys` view is a change to the server, and this
decision does not make it.
