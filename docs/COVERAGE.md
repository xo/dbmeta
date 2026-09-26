# What Each Database Can Answer

`dbmeta` asks every database the same 55 questions. PostgreSQL answers all of
them, because PostgreSQL is the model. No other database answers all of them,
and this document says which ones each database answers, which ones it cannot,
and why.

Forty eight of the questions come from `psql`. The other seven exist because a
consumer needs them and `psql` has no command for them: the columns of a
constraint, the parameters of a routine, the labels of an enumerated type, the
statement a view selects, the statistics over a column, the schema an
unqualified name resolves in, and the user the connection is authenticated as.
D47 sets the rule for adding one, and D55 added the last.

A question that a database cannot answer returns `dbmeta.ErrNotSupported`. It
never returns an empty result. Those are different facts and a caller has to be
able to tell them apart: an empty result means the server holds none of that
object, and `ErrNotSupported` means the server has no such object at all.

Read `COMMANDS.md` for the `psql` command that each Go value answers. Read
`PLAN.md` D43 for the rule about finding these analogues in the first place.

Adding a database means adding a section here. [`DIALECT.md`](DIALECT.md) holds
every step and this is one of them, so read that first if you are adding one
rather than reading one.

## The count

| Model | Answers | Of | Tested against |
| --- | --- | --- | --- |
| `models/postgres` | 55 | 55 | PostgreSQL 9.6 through 18 |
| `models/mysql` | 29 on MariaDB, 26 on MySQL | 55 | MariaDB 10.6 to 13.0, MySQL 8.4 to 26.7 |
| `models/sqlite3` | 14 | 55 | both drivers: mattn/go-sqlite3 and modernc.org/sqlite |
| `models/duckdb` | 20 | 55 | duckdb/duckdb-go, the driver usql uses |
| `models/sqlserver` | 32 | 55 | SQL Server 2017, 2019, 2022 and 2025 |
| `models/oracle` | 25 | 55 | Oracle 11g, 18c, 19c, 21c, 23ai and 26ai |
| `models/cassandra` | 17 | 55 | Cassandra 3.11, 4.0, 4.1 and 5.0 |
| `models/clickhouse` | 23 | 55 | ClickHouse 25.3, 25.8, 26.8 and 26.9 |
| `models/trino` | 13 | 55 | Trino 476 and 483 |
| `models/presto` | 9 | 55 | Presto 0.299 |
| `models/firebird` | 24 | 55 | Firebird 3.0, 4.0 and 5.0 |
| `models/hana` | 32 | 55 | SAP HANA 2.0 SPS 08 |
| `models/hive` | 16 | 55 | Apache Hive 4.2 |
| `models/informationschema` | 12 | 55 | any database with a standard `information_schema` |

The shared `information_schema` model answers eleven: tables, schemas, columns,
functions, privileges, constraints, sequences, constraint columns, routine
parameters, views and the current schema. It is the floor. A native model
exists to beat it, and `models/mysql` beats it by seventeen.

Four of those eleven arrived with the kinds D47 added, and they arrived for
free: the standard defines `key_column_usage`, `parameters`, `views` and
`schemata`, so every database close to the standard answers them.

## MariaDB and MySQL

These are two products sharing one dialect and one model. Most of what follows
is true of both. Where they differ, the model gates on the product rather than
on the release number, because MariaDB is at 11.8 and MySQL at 9 and neither
number says anything about the other. See D44.

MariaDB answers 29 of the 55 and MySQL answers 26. The three MySQL cannot
answer are sequences, which it has never had, aggregates, which it has no form
of and whose catalog table it dropped in 8.0, and column statistics, below.

### What it answers with the same thing PostgreSQL has

Schemas, databases, tables, columns, indexes, index columns, constraints,
triggers, sequences, partitioned tables, comments, functions, collations,
settings, roles, role grants, privileges.

A schema and a database are the same object in MariaDB. `dbmeta.Schemas` and
`dbmeta.Databases` both answer, and they answer with the same rows. That is the
one place the object model does not fit, and it is not hidden.

### What it answers with an analogue

| Question | What MariaDB reads | Why it is a fair answer |
| --- | --- | --- |
| `AccessMethods` | `information_schema.ENGINES` | A storage engine decides how a table is stored and searched, which is what an access method decides. `psql` prints the access method of a table in `\d`, and the engine is what belongs there. |
| `Extensions` | `information_schema.ALL_PLUGINS` | A plugin adds capability to a running server and can be loaded and unloaded, as an extension can. |
| `Aggregates` | `mysql.proc` where `aggregate` is `GROUP`, and `mysql.func` where `type` is `aggregate` | These are aggregates, created with `CREATE AGGREGATE FUNCTION`. The two tables hold the two forms, stored and compiled. |
| `ForeignServers` | `mysql.servers` | `CREATE SERVER` names a remote server and its wrapper, which is what `CREATE SERVER` does in PostgreSQL. |
| `UserMappings` | `mysql.servers` | A MariaDB server carries one credential and every local user reaches the remote server with it. That is the mapping PostgreSQL writes for `PUBLIC`. |
| `ForeignTables` | `information_schema.TABLES`, filtered on the engine | A table on `FEDERATED`, `CONNECT`, `SPIDER`, `OQGRAPH` or `S3` reads data this server does not hold, which is what a foreign table is. |

Four of these need `SELECT` on the `mysql` schema, because `mysql.proc`,
`mysql.func` and `mysql.servers` are tables rather than views and MariaDB does
not grant them to an ordinary user. PostgreSQL makes the same information
readable by everyone. A caller without that privilege gets an error from the
driver, not from `dbmeta`.

`ForeignTables` reports the engine in the `server` column, because
`information_schema` does not publish the `CONNECTION` setting that names the
server. The field says so.

### Where the two products differ

| Question | MariaDB reads | MySQL reads |
| --- | --- | --- |
| `Constraints`, the check clause | `information_schema.CHECK_CONSTRAINTS` from 10.2, joined on the table as well as the name | the same view from 8.0.16, joined on the schema and the name, because it has no table column |
| `Settings` | `information_schema.SYSTEM_VARIABLES` | `performance_schema.global_variables` on 8.4, which holds the value alone, and `variables_metadata` joined to it from 9, which adds the type and the scope |
| `Roles`, whether it can log in | `mysql.user.is_role` | `mysql.user.account_locked`, because MySQL marks a role by locking the account and has no such column |
| `RoleGrants` | `mysql.roles_mapping`, which names the member and the role it holds | `mysql.role_edges`, which names the role it came from and the account it went to, so the columns are read the other way round |
| `Extensions` | `information_schema.ALL_PLUGINS` | `information_schema.PLUGINS`, which MySQL has instead |
| `Sequences` | `information_schema.SEQUENCES` from 11.5 | nothing: MySQL has no sequences at any release |
| `Aggregates` | `mysql.proc` and `mysql.func` | nothing: MySQL dropped `mysql.proc` in 8.0 and has no aggregate of its own |
| `ColumnStats` | `mysql.column_stats` joined to `mysql.table_stats` | nothing: `information_schema.COLUMN_STATISTICS` holds one JSON histogram and no width, null fraction or distinct count |

`Functions` reads `routine_body` for the language and never
`external_language`. MariaDB leaves `external_language` NULL for a SQL routine
and MySQL writes `SQL`, and the field is not nullable, so reading it fails to
scan on MariaDB. The cross product test found that.

### What the two products answer the same way, and what they spell differently

`TestMySQLAgainstMariaDB` builds the same fixture on one server of each product
and compares every query that narrows to one schema. Eleven queries compare,
and every column agrees except two.

`Columns.DataType` and `Columns.Default`. MariaDB keeps the display width of an
integer and MySQL dropped it, so a column reads `int(11)` on one and `int` on
the other. MariaDB quotes a string default and MySQL does not, so the same
default reads `'red'` and `red`.

`Constraints.Definition`. MariaDB records the check clause as written and MySQL
rewrites it with the character set introducer, so ``` `title` <> '' ``` becomes
``` (`title` <> _utf8mb4'') ```.

`RoutineParameters.DataType`. The same display width difference, reaching a
parameter through `dtd_identifier` rather than a column.

`Views.Definition`. Neither product stores a view as written. Both rewrite it,
and they rewrite it differently: MySQL parenthesizes the `WHERE` clause and
MariaDB does not.

Those four are named in the test. Any other difference fails it, so a query
written for one product and run against the other is caught here rather than by
a user. It has caught three faults so far, the most recent in CI: the
comparison keyed a row by its name and ordinal without the routine it belongs
to, so two routines' return values, both reported with no name at ordinal zero,
looked like one row whose name kept changing.

### What it cannot answer, and why

MariaDB has no equivalent of any of these. Reporting them unsupported is the
whole answer.

| Question | Why |
| --- | --- |
| `Types`, `Domains` | MariaDB has no `CREATE TYPE` and no `CREATE DOMAIN`. The type of a column is one of a fixed set. |
| `Operators`, `OperatorClasses`, `OperatorFamilies`, `OperatorFamilyOperators`, `OperatorFamilyFunctions` | Operators are built into the parser and cannot be created, so there is nothing to list and no index strategy to group them into. |
| `Casts`, `Conversions` | Conversion is implicit and is not declared, so there is no catalog of it. |
| `Languages` | A stored routine is SQL and nothing else. The `language` column of `mysql.proc` is an enum with one value. |
| `LargeObjects` | A large value is a `LONGBLOB` column, not an object with its own identity. |
| `EventTriggers` | A trigger fires on a row, never on a statement that changes the schema. |
| `RoleSettings`, `DefaultACLs` | A setting belongs to a session or to the server, never to a role. A grant applies to what exists, never to what will be created. |
| `Publications`, `PublicationTables`, `Subscriptions` | MariaDB replicates a binary log, not a named set of tables. There is no publication to name and no subscription to list. |
| `TextSearchParsers`, `TextSearchDictionaries`, `TextSearchTemplates`, `TextSearchConfigs` | Full text search is a `FULLTEXT` index with a built in tokenizer. None of its four parts is a nameable object. |
| `ExtensionObjects` | A plugin owns no SQL objects, so there is nothing to list as belonging to one. |

### Analogues that were found and rejected

Two AI models were asked what MariaDB holds for the 29 questions the first pass
could not answer, which is the rule D43 sets. Both named analogues that do not
survive a look at a running server. They are recorded here so that the next
person does not find them again and reach the other conclusion.

`information_schema.TABLESPACES` for `Tablespaces`. Both models named it. The
table exists and it has the right columns, and on MariaDB 11.8 it returns no
rows and can never return any: it is the MySQL Cluster table, kept for
compatibility. `information_schema.INNODB_SYS_TABLESPACES` does return rows,
and each row is one file behind one table, not a named place an administrator
created. Listing `dbmeta_fixture/book` as a tablespace would teach a caller
something false.

`information_schema.ENGINES` for `ForeignDataWrappers`. An engine is already
the answer for `AccessMethods`, and an engine is not a wrapper: it is not named
by `CREATE SERVER` and a server does not point at one. `mysql.servers.Wrapper`
holds a real wrapper name, but only for wrappers already in use, and `\dew`
lists the wrappers a server has rather than the ones it uses.

`mysql.column_stats`, `mysql.table_stats` and `mysql.index_stats` for
`ExtendedStats`. These hold engine independent statistics, one row per column.
`\dX` lists the statistics objects `CREATE STATISTICS` makes, and the point of
those is that they cover several columns at once. A single column histogram is
a different thing with a similar name.

`information_schema.PLUGINS` with `PLUGIN_TYPE` of `FTPARSER` for
`TextSearchParsers`. A full text parser plugin genuinely is a text search
parser, so this one is close. It was rejected because the built in parser is
not a plugin and does not appear, so a default install answers with no rows
while full text search works. `psql` lists the built in parser for `\dFp`, and
an empty answer here would read as "this server has no full text search".

`mysql.slave_master_info` for `Subscriptions`. MariaDB does not have that
table. It keeps master information in a file, and `SHOW SLAVE STATUS` is the
way to read it, which is not a catalog query.

### Catalogs MariaDB has that no question asks for

These hold real metadata and no `psql` command maps onto them, so `dbmeta` does
not read them. A later question could.

`information_schema.EVENTS` holds scheduled events, which PostgreSQL has no
form of. `information_schema.PERIODS` holds application time periods. It is present on
11.8 and absent on 10.6, so a query for it needs a version gate. `information_schema.PARAMETERS` holds routine
parameters, which `dbmeta.Functions` reports as absent because PostgreSQL packs
them into one string.

## SQLite

SQLite answers 14 of the 55. It is the smallest native model here and it still
beats the shared `information_schema` one, which SQLite does not have at all.

It is also the only database here with no server. SQLite is a library, so the
release under test is whichever one the Go driver was built with. There is
nothing to upgrade separately, nothing to run in a container, and no version
gate in the model: every pragma it reads arrived by SQLite 3.37 in 2021, and
the only way to reach an older one is to pin an old driver on purpose.

Two drivers are tested, as subtests named for each. `mattn/go-sqlite3` compiles
the upstream SQLite source and needs cgo, which is why it is the primary one:
it is the real database. `modernc.org/sqlite` is a pure Go translation and is
what a consumer who cannot use cgo runs. They ship different library versions
in general, and they happen to agree today. See D48.

### What it answers

Eleven from `psql` and three of the six D47 added: the columns of a constraint,
the statement a view selects, and the schema an unqualified name resolves in.
It cannot answer routine parameters, enum values or column statistics, and the
reasons are in the table above.

| Question | What SQLite reads |
| --- | --- |
| `Schemas`, `Databases` | `pragma_database_list`. An attached database is what SQLite calls a schema and it is also the only thing it calls a database, so both answer with the same rows. |
| `Tables` | `sqlite_schema`. A table backed by a module reports its type as `virtual` rather than `table`, because it does not behave like one. |
| `Columns` | `sqlite_schema` joined to `pragma_table_xinfo`. The xinfo form rather than info, so that a generated column and a virtual table's hidden column appear. |
| `Indexes` | `pragma_index_list`, which carries what `sqlite_schema` cannot: whether the index is unique and whether SQLite made it for a constraint. |
| `IndexColumns` | `pragma_index_xinfo`, filtered to the columns the index is on rather than the ones it carries to reach a row. |
| `Constraints` | Three pragmas joined. See below. |
| `Triggers` | `sqlite_schema`. The definition is the whole `CREATE TRIGGER` statement, because SQLite keeps nothing else. |
| `Functions` | `pragma_function_list`, folded to one row per name and kind. |
| `Collations` | `pragma_collation_list`. |
| `Settings` | One `SELECT` per pragma, joined by `UNION ALL`. See below. |

A correlated table-valued join is what makes most of these one statement
rather than one per table:

```sql
FROM sqlite_schema m JOIN pragma_table_xinfo(m.name) c
```

### The one query that answers incompletely

`Constraints` reports primary key, unique and foreign key, and never reports a
check constraint. SQLite records a check constraint only inside the
`CREATE TABLE` text in `sqlite_schema.sql`, and `dbmeta` does not parse DDL.

This is the one place a query here answers partially rather than not at all.
Both Gemini and DeepSeek argued for it and they are right: three kinds read
exactly are worth more than refusing all four over the fourth. The field
description says so, and the test creates two check constraints and asserts
that neither appears, so the gap is tested rather than assumed.

### Settings, which is assembled rather than written

There is no way to ask SQLite for the value of a pragma it names.
`pragma_pragma_list` gives names and no values, and reading a value means
naming the pragma in the SQL. So the query is built at startup from a list of
36 pragmas, one `SELECT` each, joined by `UNION ALL`.

Two things that only running it reveals. A pragma appears in
`pragma_pragma_list` whether or not it can be read as a table, so
`mmap_size`, `wal_autocheckpoint`, `wal_checkpoint`, `case_sensitive_like` and
`temp_store_directory` are left out: they are pragmas and they are not tables.
And `busy_timeout` returns a column called `timeout`, which is the only one in
the list not named after its own pragma.

`compile_options` is left out on purpose. A build flag is not a setting a
caller can change, which is what `\dconfig` means.

### Analogues that were found and rejected

Two AI models were asked what SQLite holds for the 37 questions the first pass
could not answer, which is the rule D43 sets. They agreed on almost all of it,
and every rejection below is theirs as well as mine.

`sqlite_sequence` for `Sequences`. It exists only when a table uses
`AUTOINCREMENT`, and it gets a row only after the first insert. It is a counter
SQLite keeps for itself, not an object anyone created, and nothing can read the
next value from it.

`pragma_module_list` for `Extensions`, `AccessMethods` or
`ForeignDataWrappers`. A module implements a virtual table. It is not an
extension, it is not an index method, and nothing wraps foreign data with it.

A virtual table for `ForeignTables`, the way a `FEDERATED` engine answers for
MariaDB. This one looked right and it is not: a virtual table is also how
SQLite does full text search, JSON and R-trees, so most of what the query
returned would be local data.

`pragma_compile_options` for `Settings`, rejected above.

`sqlite_stat1` for `ExtendedStats`. It holds one row per index, which is
PostgreSQL's `pg_statistic`, not the `CREATE STATISTICS` objects that `\dX`
lists.

`pragma_table_xinfo.type` for `Types`. A declared type in SQLite is an
unenforced affinity hint. Presenting a column of them as a type catalog would
suggest a check that does not happen.

`pragma_function_list` type `a` and `w` for `Aggregates`. This is the one the
models split on, and running it settled it against both. Gemini said to map
both and called it exact. DeepSeek said to map only `a`. On a real server,
`sum`, `count`, `avg` and `group_concat` all report as `w`, and so do
`row_number`, `rank` and `lag`. Type `a` matched one function, an extension.
So mapping `w` would list `row_number` as an aggregate and mapping `a` would
omit `sum`. Both mislead, there is no third option, and SQLite simply cannot
tell an aggregate from a window function. `Aggregates` is unsupported and
`Functions` reports the kind SQLite reports.

### What it has none of

SQLite has no users, no roles and no grants of any kind, so `Roles`,
`RoleGrants`, `RoleSettings`, `Privileges` and `DefaultACLs` are absent rather
than empty. It records no comment on any object. It has no type catalog, no
operators that can be created, no casts, no procedural languages, no
replication, no tablespaces and no partitioning.

## Cassandra

Cassandra answers 17 of the 55, verified against 5.0.9 and 3.11.19.

It is the first database here that is not SQL, and CQL is narrower than the
name suggests. D62 holds the four consequences and this is the short version.

### The image is built here

The Apache image refuses three things this model has queries for, and its
entrypoint maps only eight `cassandra.yaml` keys to environment variables,
none of them these. So every release is rebuilt from
`test/cmd/dbrun/image/cassandra.Containerfile`, which `dbrun` embeds and builds when the image is missing, so
nothing has to be done first, and CI builds its own from the same file.

It turns on user defined functions, so `Functions` and `Aggregates` have
something to read and the fixture can create one. It turns on materialized
views, which 5.0 ships off and 3.11 ships on, so `Views` behaves the same on
every release. And it sets `PasswordAuthenticator` and `CassandraAuthorizer`,
without which `system_auth` holds one role and no grant, and D61 has no second
principal to compare against.

The key names changed and the build handles both. 3.11 and 4.0 write
`enable_user_defined_functions` and `enable_materialized_views`, and 4.1 and
later write `user_defined_functions_enabled` and `materialized_views_enabled`.
usql publishes an image that does the same thing, as
`docker.io/usql/cassandra`, and its Dockerfile edits only the older spelling,
so it has no effect on 5.0. The build here checks the result rather than
trusting sed, because a sed that matches nothing changes nothing and says so
to nobody.

The superuser is `cassandra` and so is its password. It is the one product
here that does not use the shared test password, because Cassandra creates
that pair and it is the only one that works until somebody changes it.

### What it answers

Keyspaces as schemas, tables, columns, materialized views as views, user
defined types, indexes, index columns, the primary key as a constraint and its
columns, triggers, comments, functions, aggregates, roles, role grants,
privileges and settings.

`Settings` needs 4.0, where the `system_views` keyspace arrived. Everything
else answers on every release from 3.11 up. That is the only version fragment
the model has, which is why the tested pair spans it.

### No query filters, and every query returns the system keyspaces

CQL has no `OR`, no `IS NULL` outside a materialized view definition, and a
partition key takes only `=` or `IN`. The form every other model uses,
`(@schema IS NULL OR col LIKE @schema)`, cannot be written. There is no
`NOT IN` either, so the keyspaces Cassandra keeps for itself cannot be
excluded.

So every query returns every row, including `system`, `system_schema`,
`system_auth`, `system_distributed` and `system_traces`. The filter parameters
are still declared and every description says Cassandra ignores it. A consumer
narrows the result, which `usql` already does to match `psql`.

### A derived field is derived in Go

There is no `CASE` and no expression. `Column.Nullable` and
`Column.PrimaryKey` are both read from `system_schema.columns.kind`, which the
statement selects twice, and `Scan` turns each into its boolean. A column of
kind `partition_key` or `clustering` is in the primary key, and the primary key
is the only thing in Cassandra that cannot be null.

### No order

CQL orders rows within one partition and by a clustering column. A result that
spans partitions arrives in token order, so the same query can return the same
rows in another order on another cluster. No query writes `ORDER BY`, because
one would not make the answer ordered.

### What the fixture builds, and what it needs

`models/cassandra/fixture` creates the keyspace `dbmeta_fixture` with the core
objects D53 asks every fixture for, plus one of every Cassandra object the
queries read: a user defined type, a secondary index, a materialized view, a
function, an aggregate built on a second function, two roles, a grant between
them and a permission on the keyspace. Nineteen steps, none of them skipped on
either 3.11 or 5.0.

It needs the image this repository builds. Against the published one the
function, the view and the roles are all refused, and the test says so rather
than quietly building less.

Three tables show three shapes of primary key, because that is the one
constraint Cassandra has and the queries report it: `author` has a partition
key alone, `book` has a partition key and a clustering column, and `region`
has a two column partition key.

No trigger. A Cassandra trigger names a Java class that has to be on the
server's classpath already, so the fixture creates none and `Triggers` returns
no rows anywhere. It is the one registered query with no fixture object.

### A null arrives as an empty string

The driver cannot report a null. gocql decodes a null of any type as the zero
value of that type, so scanning one into `sql.Null[string]` gives a valid
empty string rather than an absent one.

For a column the model pads, this is handled: the value is discarded and the
field keeps the invalid Null that `docs/NULLS.md` asks for. See D62.

For a real catalog column that is null, it is not handled and cannot be. A
comment that was never set and a comment set to the empty string are the same
value to a caller. In practice Cassandra stores the empty string rather than a
null for a table with no comment, so the two agree, but a consumer should not
rely on `Valid` meaning anything on this dialect.

### What the conformance test says

Cassandra is in `test/testdata/conformance.txt` under `[cassandra]`, and it
agrees with the relational databases on less than they agree with each other.
That is why `TestConformanceAgreementHolds` now measures twice: the relational
databases against the floor they have always held, and every database against
a smaller one. Counting Cassandra with the rest would drop the floor from 23
lines to 6 and leave it too low to notice a regression anywhere.

Two differences and neither is a fault. There is no `table recent view` line,
because `Tables` returns no view: a materialized view is in
`system_schema.views` and CQL has no UNION to put the two together. And the
column ordinal is a position within the primary key rather than within the
table, because the catalog keeps no declaration order and holds a table's
columns alphabetically.

### What Cassandra has none of

No sequence, no domain, no enumerated type, no cast, no collation catalog, no
operator, no text search object, no extension, no foreign data wrapper, no
tablespace and no large object. No `Databases` either: a keyspace is the top
of the tree and the cluster above it is in `system.local`, which no single
statement can reach from `system_schema` because CQL has no join.

`CurrentSchema` and `CurrentUser` are absent and both models agreed. There is
no CQL expression for either. `system_views.clients` from 4.0 lists every
connection with the user on it and cannot say which one is asking, so it is
not the same fact.

`RoutineParameters` is the one that is present and not returned.
`system_schema.functions` holds `argument_names` and `argument_types` as two
parallel lists on the function's own row, and turning them into one row per
parameter needs an unnest that CQL does not have. The types are in
`Functions.ArgTypes` as one text.

`ColumnStats` is absent from the catalog. The statistics Cassandra keeps are
per SSTable and per node, in `system_views` from 4.0, which is a different
thing from the per column distribution `psql` prints.

`PartitionedTables` is a stretch and is left unsupported under the D43 rule.
Every Cassandra table is partitioned, so a list of the partitioned ones is a
list of all of them and says nothing.

## ClickHouse

ClickHouse answers 23 of the 55, verified against 26.9.2.8 and 25.8.33.6.

### system, not information_schema

ClickHouse ships both. `information_schema` is an emulation that reports what
the standard names and drops everything that makes a ClickHouse table what it
is: the engine, the partition key, the sorting key, the compression codec per
column, and the data skipping indices. Every query here reads `system`, which
is about 150 tables and the richest native catalog in this project after
PostgreSQL's.

### A database is a schema

ClickHouse has one level of namespace and calls it a database, so `Schemas`
and `Databases` read the same table under both names. `models/mysql` does the
same thing with `information_schema.SCHEMATA` for the same reason.

### What it answers

Schemas and databases, tables, columns, views, indexes, index columns,
constraints, comments, partitioned tables, types, collations, settings, roles,
role grants, privileges, functions, aggregates, tablespaces, foreign servers,
foreign tables, the current schema and the current user.

Two of those are an analogue rather than the same object, and both reviews
agreed they are real rather than a stretch. A storage policy names the volumes
and disks a table's parts live on, which is what a tablespace is for. A table
whose engine is MySQL, PostgreSQL, S3 or one of the others reads rows another
system holds, which is what a foreign table is, and a named collection is the
stored connection it uses.

### Two columns arrived in the same release

`system.constraints` and `system.functions.deterministic` are both absent on
25.3, 25.8 and 26.1, and both present on 26.8 and 26.9. They gate at 26.8,
which is the release they were first seen in rather than the release they
arrived in: somewhere in 26.2 to 26.8 is as precise as a check against running
servers can be, and gating late under reports rather than breaking.

`Constraints` is the whole query, so an older server is told it is too old.
Volatility is one column of `Functions`, so an older server gets it padded and
`Field.Min` says which is which.

### What Cassandra taught us to check here too

The conformance projection contributes no constraint lines for ClickHouse.
There is no primary key constraint, no foreign key and no unique constraint,
and `system.constraints` holds the expression a CHECK asserts rather than the
columns behind it, so the model registers `Constraints` and not
`ConstraintColumns`. Counting ClickHouse in the cross family agreement number
would take it from 23 lines to 14, so it is measured separately for a stated
reason, the way Cassandra is.

### What the fixture cannot build

A named collection. Creating one needs
`access_control_improvements.named_collection_control` in the server
configuration, and the official image exposes no variable for it. The
`ForeignServers` query is verified to run and returns no rows, which is the
same position Cassandra's triggers are in.

Everything else the fixture builds, including a user, a role and a grant,
which need `CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT` and `container/clickhouse.go`
sets it.

### What ClickHouse has none of

No sequence, no domain, no trigger, no operator catalog, no extension, no
large object, no text search object of the shape psql names, and no user
defined type: Enum, Array and Tuple are spelled inside a column's type rather
than declared.

`EnumValues` would mean parsing the enum out of a type string, which is not a
catalog read. `RoutineParameters` is absent because a ClickHouse function is
overloaded across types and `system.functions` records no signature.
`ColumnStats` is absent: `system.columns` carries compressed and uncompressed
sizes and nothing about distribution, and what `system.parts` holds is per
part rather than per column.

## Trino

Trino is a query engine rather than a store. It reads other people's data
through a connector and keeps almost nothing of its own, so most of what it
cannot answer is missing because the thing does not exist rather than because
the catalog hides it.

### A catalog is a real level, and Trino is the only one

Every other model here returns an empty catalog or repeats the database name
into it, because the products have two levels of namespace and `psql` has
three. Trino has all three. A table is `catalog.schema.name`, a catalog is a
configured connector, and one server reaches many at once.

So Trino is the only model that answers a catalog filter, and
[`dbmeta.Args`](../args.go) has carried the field all along waiting for it.

### system.jdbc, not information_schema

Trino ships an `information_schema` inside every catalog and a `system`
catalog beside them, and the two differ in reach. A query against
`memory.information_schema.tables` sees the memory catalog and nothing else,
and the catalog cannot come from a bind parameter, so a filter naming a second
catalog would return nothing rather than an answer. That is a wrong answer
rather than an empty one, which rule 13 does not allow. The tables under
`system.jdbc` span every catalog the server has.

`system.jdbc` is also the richer of the two. Its `columns` table carries the
column comment in `remarks`, and `information_schema.columns` has no column
for a comment at all.

### What it answers

13 of the 55. Catalogs as databases, schemas, tables, columns, views,
comments, types, access methods, roles, role grants, privileges, the current
schema and the current user.

A connector is the analogue for an access method, the same way a storage
engine is for MariaDB: both answer "how is this stored and reached". The
`comment` on a connector is the list of catalogs it backs, because one
connector can back several.

### The one query that cannot span catalogs

Views. No cross catalog source carries a view definition:
`system.metadata.materialized_views` has one and covers materialized views
only, and a plain view's definition lives in the `information_schema.views` of
its own catalog. Reading every catalog would mean one statement per catalog,
which rule 13 forbids.

So Views reads the session catalog, and its `schema` parameter says so where a
caller reads it. Every other query here spans catalogs.

### A boolean is not an integer

Every other model writes `@with_system = 1` and the server coerces. Trino
applies no implicit conversion between a boolean and an integer and refuses
the statement with `Cannot apply operator: boolean = integer`, so this model
writes `= true`. Seven queries were written the other way first and all seven
failed at once, which is the cheapest way for that to be found.

### What the fixture cannot build

No primary key, no foreign key, no unique constraint and no check. Trino has
no constraint of any kind at any release, so nothing enforces that a book has
an author. No sequence, no trigger, no index and no user defined type. No
default: Trino parses `DEFAULT` and the memory connector keeps none.

No role and no grant either, and that one is the connector rather than the
engine. A Trino role belongs to a catalog, only a connector that implements
role management has any, and the memory connector answers "does not support
role management" to `CREATE ROLE`. Roles, RoleGrants and Privileges are
verified to run and return no rows, and `TestTrinoRolesAreEmptyOnMemory` pins
that so it stays distinguishable from a broken query. A connector with
sql-standard security, such as Hive, populates all three.

No materialized view: the memory connector refuses to create one.

### What the conformance test says

Trino builds every core object D53 asks for, including the view, and the
column ordinals and nullability match every other database. It is left out of
the relational agreement count for the same reason ClickHouse is: with no
constraint catalog every column reads `primary_key=false` where the others
agree on the key, and there are no constraint lines to compare.

### What a second opinion found, and what it cost to check

D43 requires asking at least two models about the queries a new dialect cannot
answer. Gemini and DeepSeek were both asked to sort 42 unanswered kinds into
absent, present under another name, and derivable from one statement.

Every lead was run against a real server, which is the part of the rule that
matters. DeepSeek named eight sources that do not exist:

```
system.metadata.functions             ABSENT
system.metadata.partitions            ABSENT
system.metadata.types                 ABSENT
system.metadata.column_comments       ABSENT
system.metadata.session_properties    ABSENT
information_schema.parameters         ABSENT
information_schema.table_constraints  ABSENT
information_schema.key_column_usage   ABSENT
```

Three of its answers rested on `system.metadata.functions` alone: functions,
aggregates and operators. `SHOW CASTS` is not a statement Trino has, and no
table reports `table_type = 'FOREIGN'`. Gemini named nothing that does not
exist, and put all but six kinds in absent.

Two answers were worth the exercise.

**Functions cannot be read as a relation, and now that is settled.** Gemini
said `SHOW FUNCTIONS` cannot be wrapped in a subquery and it is right, though
not for the reason it gave. The parser does not reject it: it reads `SHOW` as
a table name and reports `Table 'memory.default.show' does not exist`. So
there is no table valued source for the function list, the column names carry
spaces, and Functions and Aggregates stay unanswered. `SHOW SESSION` has the
same shape, which is why Settings is unanswered too, and both models agreed
`SHOW STATS FOR` has no table valued form, so ColumnStats is as well.

**information_schema.columns has an undocumented column.** Gemini derived
partitioned tables from `extra_info = 'partition key'`. `SHOW COLUMNS` does
not list `extra_info` and the column resolves anyway, which a control
settled: a name that really does not exist fails with `Column
'definitely_not_a_column' cannot be resolved`, and `extra_info` returns 0 non
null values over 34 rows. It is real, and the memory connector never sets it.
A connector that partitions, such as Hive, does.

PartitionedTables is left unanswered on that basis rather than on absence. The
source exists and no connector in the test image populates it, so rule 9 has
no object to build and the query would be verified against nothing.

### Firebird

`models/firebird` answers 24 of the 55, against Firebird 3.0.14, 4.0.7 and
5.0.4.

### It reads RDB$, and there is no information_schema

Firebird has no `information_schema` at any release in range. Its catalog is
the `RDB$` tables, which are ordinary tables inside the database that a
statement reads the way it reads any other. Two other prefixes matter. `MON$`
is the monitoring set, and `MON$DATABASE` is where the attached database
describes itself. `SEC$` is a view on the server's security database, and it
is where users live, because a user belongs to the server and a role belongs
to the database.

Two properties of `RDB$` shape every query here, and both cost a round of
wrong answers before they were understood.

Every name is `CHAR(63)` and blank padded. Equality pads and `LIKE` does not,
so `RDB$RELATION_NAME LIKE 'AUTHOR'` matches nothing at all: the stored value
is AUTHOR followed by 57 spaces. A filter written without a trim looks correct
and returns an empty result rather than an error. Only the trailing blanks are
padding, so the model trims those alone and not a leading space, which a
quoted identifier is allowed to have.

Every bind parameter needs a cast. Firebird takes the type of a bare parameter
from what it is compared with, so `? = ''` types it `VARCHAR(0)` and the server
then refuses any value:

	arithmetic exception, numeric overflow, or string truncation
	string right truncation
	expected length 0, actual 6

An empty filter still passes, which is what makes it worth writing down. A
first run with no arguments reports nothing wrong.

### There are no schemas, and none is invented

Firebird 3.0 through 5.0 has no schemas. Every object lives in one namespace
and names are unique across the database. Firebird 6.0 adds SQL schemas and is
out of range.

So `Schemas` and `CurrentSchema` report `NotSupported`, and every other query
returns an empty schema. An invented name would be indistinguishable from a
real one to a caller that cannot see the server, and an empty result must never
stand in for `NotSupported`, which is D34. `TestFirebirdSchemasAreNotSupported`
holds both halves of that.

A Firebird database is a file rather than a name, and the server keeps no
catalog of the files it has served, so `Databases` returns exactly one row: the
database attached, named by its path.

### What it answers

Tables, columns, views, indexes, index columns, constraints, constraint
columns, triggers, event triggers, sequences, domains, functions, routine
parameters, types, collations, roles, role grants, privileges, comments,
databases, settings, publications, publication tables and the current user.

Three of those need 4.0 and report `TooOld` on 3.0, which D63 added the state
for: `Settings` reads `RDB$CONFIG`, and `Publications` and `PublicationTables`
read `RDB$PUBLICATIONS` and `RDB$PUBLICATION_TABLES`. None of the three exists
before 4.0.

Two answers take a second source and are worth naming.

A check constraint has no index, so it has no entry in `RDB$INDEX_SEGMENTS`
and the join every other constraint uses cannot reach its columns. Firebird
implements a check with a pair of system triggers and records the columns as
those triggers' rows in `RDB$DEPENDENCIES`, and nowhere else. That is the
second arm of `ConstraintColumns`, and `TestFirebirdCheckConstraintColumns`
exists so that it cannot quietly stop working: without it the result would
merely be a shorter list.

A foreign key names the unique constraint it references rather than the table,
so reaching the target column takes three more joins: to `RDB$REF_CONSTRAINTS`
for the referenced constraint, to its row in `RDB$RELATION_CONSTRAINTS` for the
table, and to that constraint's index segments for the column in the matching
position.

`Roles` is one statement over two places, because Firebird splits what
PostgreSQL keeps in one. `SEC$USERS` holds the users and `RDB$ROLES` holds the
roles, and `can_login` is what tells them apart.

A member of a package is left out of `Functions`, the way `models/oracle`
leaves one out. It is not callable by name on its own, and the package is what
a caller names.

### What it cannot answer, and why

Thirty-one kinds have no answer and every one of them is absent from the
product rather than hidden by the catalog.

There are no schemas, no tablespaces, no user defined casts, no operators, no
user defined aggregates, no enumerated types, no extensions, no foreign tables
or foreign data wrappers, no text search objects, no operator classes or
families, no partitioned tables, no extended statistics, no default privileges
and no per role settings. Firebird has one index kind and no pluggable access
method, so there is nothing for `AccessMethods` to list.

Three deserve a reason rather than a word.

`LargeObjects` has no answer because a Firebird BLOB is addressed from the row
that holds it and page chain that follows it. There is no catalog of them to
list, so this is absence rather than reach.

`ColumnStats` has no answer either, and the near miss is worth recording.
`RDB$INDEX_SEGMENTS.RDB$STATISTICS` holds a selectivity, but it is an index
prefix selectivity rather than a per column statistic, and it is none of the
things `ColumnStat` carries: no average width, no null fraction, no distinct
count, no most common values. Reporting it would be a different number under
the same name.

`Languages` is the one analogue left unsupported as a stretch, which rule 14
asks for explicitly. `RDB$FUNCTIONS.RDB$ENGINE_NAME` and the same column on
`RDB$PROCEDURES` name the external engine a routine is written for, so the
engines actually in use are derivable in one statement. That is a list of
languages in use and not a catalog of languages installed, and Firebird has no
catalog of the second. An unused engine would be missing and a caller could not
tell. The fact is not lost: `Function.Language` carries it per routine, which
is where Firebird records it.

`Subscriptions` is absent for a reason that is not obvious from the name.
Firebird 4.0 has logical replication and the publisher side is in the catalog,
which is why `Publications` answers. The subscriber side is configured in
`replication.conf` on disk and never reaches the database, so there is nothing
to read.

Firebird 5.0 added a partial index, whose predicate is in
`RDB$INDICES.RDB$CONDITION_SOURCE`. `dbmeta.Index` has no field to carry a
predicate, so the model does not read it and nothing here gates on 5.0. Adding
the field is a D47 question for every model rather than for this one.

### What a second opinion found

D43 and hard rule 14 require asking at least two models about the kinds a
first pass cannot answer, and this is the second time the rule has paid by
catching an invention rather than by finding a source. Trino was the first.

Gemini was asked about twelve concepts and answered absent for all twelve, and
every one of those answers was right. It also correctly identified
`RDB$INDEX_SEGMENTS.RDB$STATISTICS` as an index prefix selectivity rather than
a column statistic.

DeepSeek named seven sources. Six of them do not exist on a real Firebird 5.0
server and the seventh exists and is never filled:

| Named | What the server says |
| --- | --- |
| `RDB$INDEX_TYPES` | no such table |
| `RDB$INDICES.RDB$INDEX_TYPE_NAME` | no such column |
| `RDB$RELATION_FIELDS.RDB$STATISTICS` | no such column |
| `RDB$SUBSCRIPTIONS` | no such table |
| `RDB$SUBSCRIPTION_TABLES` | no such table |
| `RDB$FTS_CONFIG` | no such table |
| `RDB$FTS_STOPWORDS` | no such table |
| `RDB$FUNCTIONS.RDB$FUNCTION_TYPE = 2` for aggregates | the column exists and is NULL on every row |

Each was run rather than read, which is the whole of the rule. A lead is a lead
and nothing more.

### What the fixture cannot build

Nothing. The Firebird fixture builds all six core objects D53 asks for and all
33 of its steps run on 3.0, 4.0 and 5.0, except the one publication step, which
carries a gate and is skipped on 3.0 for the same reason `Publications` reports
`TooOld` there.

It creates no user, and that is deliberate rather than a gap. A Firebird user
lives in the server's security database, which every database on that server
shares, so creating one from a fixture would change a database the test never
opened. `test/parity_test.go` creates its principal and removes it again,
because a parity test has to.

### What the conformance test says

Firebird's section of `test/testdata/conformance.txt` is identical to
PostgreSQL's except for three lines, and on all three Firebird agrees with the
other eight databases rather than differing from them. PostgreSQL reports
`has_default=true` on `author_id`, `book_id` and `shipment_id` because its
fixture declares them `serial`. Firebird's keys are plain integers, the way
they are everywhere else.

Everything else matches exactly: the tables and the view, every column with its
ordinal, nullability and primary key flag, the composite primary key on
`region`, the composite foreign key from `shipment` into it with both target
columns, the unique constraint and the check.

### Two faults in the driver, and where they do and do not matter

Neither affects a consumer, and both cost enough to find that they are written
down here.

`nakagami/firebirdsql` returns a stale error for user management. After one
such statement fails, every later one on the same connection reports the first
error, while an ordinary query on that connection still works:

	DROP USER dbmeta_absent  -> record not found for user: DBMETA_ABSENT
	CREATE USER dbmeta_p1    -> record not found for user: DBMETA_ABSENT
	SELECT COUNT(*) ...      -> <nil>
	CREATE USER dbmeta_p2    -> record not found for user: DBMETA_ABSENT

Worse, a successful `CREATE USER` poisons a later read of `SEC$USERS` on the
same connection. The server answers EOF and drops the attachment, and the
pool's next connection then fails its handshake, so every query after that
reports a protocol error. It takes a few statements in between to become
reliable, which is why it looked intermittent until it was pinned down:

	CREATE USER ...          -> <nil>
	... ten metadata queries -> <nil>
	SELECT FROM SEC$USERS    -> EOF

`dbmeta` issues no user management statement at any time and never will, so
neither fault can reach a consumer and `Roles` keeps `SEC$USERS`. The parity
test creates a principal, so it runs every such statement on a connection of
its own and closes it. That was measured against Firebird 5.0.4 with
`nakagami/firebirdsql` v0.9.21.

## SAP HANA

`models/hana` answers 32 of the 55, against SAP HANA 2.0 SPS 08, which is
2.00.088. It is the second richest answer here after PostgreSQL's.

### It reads SYS, and there is a lot of it

SAP HANA has no `information_schema`. Its catalog is the SYS schema, a large
set of views that a statement reads like any other. That is why this model
answers as much as it does: a concept PostgreSQL has usually has a view here,
rather than an analogue that has to be argued for.

Three prefixes appear. A plain name such as `SYS.TABLES` is the catalog. A
name beginning `M_` is a monitoring view and reports what the server is doing
now rather than what is defined. A name beginning `_SYS_` is a schema the
server owns and not a view, which is why the system filter tests a prefix with
`LEFT` rather than with `LIKE`: an underscore is a single character wildcard
in `LIKE`, so `LIKE '_SYS%'` also matches `ASYS` and anything else of that
shape.

Two things about writing SQL for it, both measured rather than read.

A flag is the string TRUE or FALSE, and HANA refuses a bare comparison in a
select list. `SELECT IS_PRIMARY_KEY = 'TRUE' FROM SYS.CONSTRAINTS` is a syntax
error rather than a boolean, and so is comparing a CASE to FALSE, so the model
has both a `yes` and a `no` helper and writes the inversion out.

A bind parameter needs no cast, which is the opposite of Firebird. HANA infers
the type from the comparison, including where the same parameter appears twice.

### What it answers

Tables, schemas, columns, views, indexes, index columns, constraints,
constraint columns, triggers, sequences, partitioned tables, functions,
routine parameters, types, collations, roles, role grants, privileges,
comments, databases, settings, access methods, column statistics, extended
statistics, foreign data wrappers, foreign servers, user mappings, foreign
tables, subscriptions, text search configurations, the current schema and the
current user.

Four of those are the reason HANA is worth having. Smart data access is real
foreign data with a view per concept, so all four answer from a catalog rather
than by analogy:

| Kind | View |
| --- | --- |
| `ForeignDataWrappers` | `SYS.ADAPTERS` |
| `ForeignServers` | `SYS.REMOTE_SOURCES` |
| `UserMappings` | `SYS.REMOTE_USERS` |
| `ForeignTables` | `SYS.VIRTUAL_TABLES` |

No other model here answers all four. `SYS.REMOTE_SUBSCRIPTIONS` answers
`Subscriptions` as well, which is the subscriber half of replication.

Three more are worth naming. `SYS.DATA_STATISTICS` answers extended
statistics, `SYS.PARTITIONED_TABLES` answers partitioned tables, and
`SYS.TEXT_CONFIGURATIONS` answers text search configurations, which is the one
member of that family HANA has.

Two answers needed a decision rather than a view.

`AccessMethods` reports the row store and the column store. A HANA table is
held one way or the other and the choice is per table, which is the same
question a MySQL storage engine and a Trino connector answer. HANA keeps no
catalog of the kinds, so the query counts the tables that name each one, and
a kind nothing uses does not appear. `Tables` carries the same fact per table,
so the type is row table or column table rather than table.

`Constraints` derives the kind from two flags and the check text, because
`SYS.CONSTRAINTS` has no type column: a primary key sets both flags, a unique
key sets one, and a check sets neither and carries the condition. A foreign
key is not in that view at all and comes from
`SYS.REFERENTIAL_CONSTRAINTS`, which is the one place in this project where
the referenced column is on the row itself rather than reached through the
referenced constraint.

### What it cannot answer

23 kinds, and every one because HANA has no such object.

There is no `CREATE DOMAIN`, no enumerated type, no user defined cast,
operator or aggregate, no tablespace, no DDL or event trigger, no publication,
no default privilege and no per role setting. There is no catalog of
procedural languages either: `PROCEDURE_TYPE` and `FUNCTION_TYPE` say what one
routine is written in, which `Function.Language` carries, and there is no list
of the languages installed. That is the same stretch Firebird's engine names
are, and it is left unsupported for the same reason.

Two are worth a sentence rather than a word.

`LargeObjects` has no answer because a HANA LOB is addressed from the row that
holds it. There is no catalog of them.

`Publications` has no answer although `Subscriptions` does, and that is not an
oversight. HANA replicates by subscribing to a remote source, so the
subscriber half is in the catalog and there is no publisher object at all.

### What a second opinion found

Gemini was asked about twelve concepts. It answered absent for eleven and
named `SYS.REMOTE_SUBSCRIPTIONS` for the twelfth, which exists and is the
source `Subscriptions` now reads. That is the first time in three dialects
that a second opinion found a source rather than only confirming an absence.

DeepSeek named three and two do not exist:

| Named | What the server says |
| --- | --- |
| `SYS.TABLESPACES` | no such view |
| `SYS.PROCEDURAL_LANGUAGES` | no such view |
| `SYS.FUNCTIONS.FUNCTION_TYPE = 2` for aggregates | the column exists and holds BUILTIN or SQLSCRIPT2, never a number |

That is the third dialect running where DeepSeek invented a source and running
it was the only way to tell. See D43.

### What the fixture cannot build

Four things, and each is verified to run and return nothing rather than left
untested.

No remote source, virtual table or remote subscription: federating needs a
second database to federate to.

No text configuration: it is a repository object created outside SQL.

No data statistics object: `CREATE STATISTICS` needs rows, and the fixture
inserts none.

No column level grant, and this one is a product fact rather than a fixture
limit. `SYS.GRANTED_PRIVILEGES` carries a `COLUMN_NAME` column and HANA 2.0
SPS 08 has no `GRANT` syntax that fills it. Every spelling is a syntax error:

	GRANT UPDATE(rating) ON t TO r        -> syntax error near "("
	GRANT SELECT (rating) ON t TO r       -> syntax error near "("
	GRANT UPDATE ON t(rating) TO r        -> syntax error near "("

So `Privilege.ColumnAccess` is always empty. The query keeps the column
because the catalog has it, and `TestHANAHasNoColumnGrant` asserts both halves
so that the absence stays a decision.

No user either. A HANA user belongs to the tenant database and outlives the
schema, so `test/parity_test.go` creates its principal and drops it again.

### What the conformance test says

HANA's section differs from PostgreSQL's in three places and each one is a
real difference rather than a gap.

The three `has_default=true` lines are PostgreSQL's `serial` keys, where HANA
agrees with the other nine databases.

A view's columns are reported NOT NULL where PostgreSQL reports them nullable.
HANA computes a view column's nullability from the column behind it, and
`book_id` and `title` are both NOT NULL, so the view says so. PostgreSQL
reports a view column nullable whatever is behind it.

There is no `constraint book check (title)` line, and the reason is
structural. A HANA check belongs to the table rather than to a column:
`SYS.CONSTRAINTS` leaves `COLUMN_NAME` and `POSITION` NULL on a check row, so
`ConstraintColumns` has nothing to report and the conformance line is built
from that query. The check itself is in `Constraints` with its condition, and
`TestHANAConstraints` reads it. Firebird reaches a check's columns through the
dependencies of the triggers that implement it, and HANA implements a check
without a trigger, so there is no equivalent to follow.

### Apache Hive

`models/hive` answers 16 of the 55, against Apache Hive 4.2.1. It is the
only model here that writes its filter values into the statement, and the
reason is in D78 rather than here.

### It reads sys, and Hive is not Impala

Hive keeps its metadata in a relational metastore that SQL cannot reach.
Hive 3.0 added a `sys` database that exposes that metastore as external
tables over the JDBC storage handler, and those answer ordinary SQL. There
are 57 of them and they are what this model reads.

That is the difference D66 left open. D67 struck Impala because it answers
only through `SHOW` and `DESCRIBE`, which are statements rather than
relations and cannot be filtered, joined or aliased. `sys` is relations, so
Hive passes the test Impala failed.

`sys` is not there when a server starts. The script that creates it ships in
the image and needs a running HiveServer2 to run against, because the tables
are external tables pointed at the metastore. That is neither an image layer
nor a fixture, so `container.Server.Init` was added for it: a command run
once the server answers and before anything reads it. Hive is the only
product that sets it.

### What it answers

Tables, schemas, columns, views, constraints, constraint columns,
partitioned tables, functions, roles, role grants, privileges, comments,
column statistics, access methods, the current schema and the current user.

Three answers are worth naming because Hive arranges the fact differently
from everything else here.

Nullability, a default and a primary key are not properties of a column in
Hive. They are constraints in `KEY_CONSTRAINTS`, so `Columns` reads them
with three outer joins rather than from the column row. They are joins
rather than correlated subqueries because Hive does not take a correlated
scalar subquery in a select list.

A partition key is not a column of the table. It is in `PARTITION_KEYS` and
it does not appear in `COLUMNS_V2` at all, so `Columns` does not report it
and `PartitionedTables` does.
`TestHivePartitionColumnIsNotAColumn` asserts both halves, because a caller
that reads only `Columns` sees a table without its partition column and that
absence would otherwise look like a bug.

A SerDe is how Hive reads and writes a table's rows, which is the question
an access method answers, so `AccessMethods` reports the SerDe classes in
use. Hive keeps no catalog of them, so the query counts the tables that name
each one and a SerDe nothing uses does not appear.

### The trap in KEY_CONSTRAINTS, which cost a round of wrong answers

Hive stores the two sides of a constraint the other way round from how the
names read, and only a foreign key has both. Measured on 4.2.1, with tables
115 and 116:

	kp_pk  type=0  child(tbl=0)    parent(tbl=115)
	kp_fk  type=1  child(tbl=116)  parent(tbl=115)

For a foreign key the child is the table that has the constraint and the
parent is the table it points at. For every other kind the parent is the
table that has it, and the child columns are zero rather than null.

Reading child as "the table this is on" is the obvious mistake and it was
made here first. It is worse than wrong: joining on a zero finds nothing
rather than dropping the row, so every primary key, unique, not null and
default goes silently missing while foreign keys keep working. A schema with
foreign keys in it looks correct. The model normalizes the two sides in a
derived table and the comment on it records the measurement.

### What it cannot answer

39 kinds, and almost all of them because Hive has no such object.

Indexes were removed in Hive 3.0 and there is no statement that makes one,
so `Indexes` and `IndexColumns` have no answer. There are no sequences, no
triggers, no domains, no collations, no user defined types, no operators and
no foreign data of any kind.

`RoutineParameters` has no answer for a reason worth stating: a Hive
function is a Java class registered under a name, and the metastore holds
the name and the class and nothing else. The parameters are in the class.
`Function.Source` carries the class name, which is the nearest thing to a
body Hive has.

`Settings` has no answer because Hive's configuration is read with `SET -v`,
which is a statement rather than a relation, and the `sys` tables do not
carry it.

### What a second opinion found

DeepSeek named three sources and two do not exist:

| Named | What the server says |
| --- | --- |
| `sys.IDXS` for indexes | no such table |
| `sys.TYPES` for a type catalog | no such table |
| `sys.SEQUENCE_TABLE` for sequences | exists, and holds something else entirely |

The third is the most interesting wrong lead in this project so far, because
it is not an invention. `sys.SEQUENCE_TABLE` is real and the name matches
what was asked for. It holds the metastore's internal identifier allocator:

	org.apache.hadoop.hive.metastore.model.MDatabase = 11
	org.apache.hadoop.hive.metastore.model.MRole = 11

Reporting that as `Sequences` would have shipped a wrong answer that no test
would catch, because the query runs and returns rows. A lead that exists is
more dangerous than one that does not, and running it is the only thing that
separates them. See D43.

### What the fixture cannot build

No index, and that is why Hive is not in the cross family comparison in the
root module. D53 asks every fixture for six core objects and one of them is
an index somebody created. Hive builds five.

No function, because `CREATE FUNCTION` registers a Java class by name and a
fixture that made one would depend on a class being on the server's path.
`Functions` is verified to run and returns nothing.

No sequence and no trigger, because Hive has neither.

Every constraint carries `DISABLE NOVALIDATE`, which Hive requires because
it enforces none of them. They are declarations for a planner and the
queries read them as the catalog records them.

### What the conformance test says

Hive's section matches PostgreSQL's on every table, view and column, and
differs in two ways that are both facts.

PostgreSQL reports `has_default=true` on the three `serial` keys, where Hive
agrees with the other nine databases.

Hive reports extra constraint lines. It records NOT NULL and DEFAULT as
constraints where every other database records them on the column, so a
Hive table has `not null` and `default` constraints that the same table
elsewhere does not. Those are additional rows rather than missing ones, and
`Columns` reports the same facts in the same place as everywhere else.

### Which answers depend on who is asking

None of them. The image configures no authorization, so a client states a
principal and HiveServer2 takes it, and the server then allows that
principal everything. That is the measurement rather than a gap in it, and
it is the same shape as Trino and Presto.

A Hive with SQL standard authorization configured would answer differently
and nothing here measures that, because the image does not configure it.

## Which answers depend on who is asking

Eleven queries answer differently for a user that is not the administrator,
which is the most of any product here. That is HANA rather than the model:
almost every SYS view filters itself by what the reader may see, so a grantee
sees fewer collations, fewer databases, fewer adapters and fewer settings, as
well as fewer roles and grants.

Four of the eleven return the same number of rows with different values:
functions, sequences, triggers and views. Those carry a definition, and HANA
returns the row and withholds the text from a reader without the privilege.
That is worth knowing before a consumer treats a definition as always present.

## Which answers depend on who is asking

None of them, and that is the measurement rather than a gap in it. Trino has
no users to create: a client states a principal on every request and the
server takes it, because the image configures no authenticator. With no access
control plugin the server then allows that principal everything, so the only
query that answers differently for a second principal is `current_user`, which
is the one that is supposed to.

## Presto

Presto is Trino's older self. They forked in 2019 and `models/presto` is a
model of its own rather than a flavor, which D73 decides and measures. Read
the Trino section first: everything there about a catalog being a real level
and about `system.jdbc` spanning catalogs is true here too, and this section
records only where the two differ.

### What it answers

9 of the 55. Catalogs as databases, schemas, tables, columns, views, types,
access methods, privileges and the current user.

Trino answers four more, and each is absent from the product rather than
missing from the model.

**Comments has no source.** Presto accepts a `COMMENT` clause on
`CREATE TABLE`, keeps nothing readable, and shows nothing in
`SHOW CREATE TABLE`. There is no `system.metadata.table_comments` for the
query to reach, so `Tables.comment` and `Views.comment` are padded absent and
the Comments kind is not registered. `COMMENT ON` is not a statement Presto
has at all: the parser rejects the word, so a column comment cannot be set
either and `system.jdbc.columns.remarks` is always NULL.

**CurrentSchema has no expression.** Neither `current_catalog` nor
`current_schema` resolves, and nothing in `system.runtime` carries the
session. `current_user` does resolve, so CurrentUser is answered.

**Roles and RoleGrants raise rather than answering nothing.** This is the
sharpest difference from Trino and the one most likely to surprise. Both
products read the same standard views. Trino's memory connector returns no
rows, which is a supported query with an empty result. Presto's answers:

```
NOT_SUPPORTED: This connector does not support roles
```

D34 says a query dbmeta offers must run, so those two are not registered here.
Privileges reads `information_schema.table_privileges`, which does answer on
the same connector, and returns nothing.

### The version query, and why it is not Trino's

Presto has no `version()` function:

```
Function version not registered
```

So the model reads the coordinator's row from `system.runtime.nodes` instead,
with `WHERE coordinator = true` so that a cluster cannot answer with a
worker's version. `usql` reads the same table for both products and omits that
filter.

The numbering is the other half of D73. Presto is `0.299-7d50721`, which is
release 299 and the build it was cut from. Trino is `483`. There is no version
to compare, only a product to tell apart.

### The two drivers want opposite DSNs

`prestodb/presto-go-client/v2`, which is what `usql` pins, takes the catalog
and schema in the path and refuses an `http://` scheme. It rejects the scheme
before any network call, and reads any unrecognised query parameter as a
Presto session property, so the server rejects the statement:

```
unsupported scheme "http": must be presto or trino
INVALID_SESSION_PROPERTY: Unknown session property schema
```

The form it takes is `presto://user@host:port/catalog/schema`, which is what
`container/presto.go` generates. Trino wants the opposite:
`trinodb/trino-go-client` takes `http://` with the catalog and schema as query
parameters. v1 of the Presto driver took that form too, so this is a v2 break
rather than a long standing fault.

This is another measure of how far apart the two have drifted, and `dburl`
found a sharper one. It had no `GenTrino` at all: the `trino` scheme was
registered against `GenPresto`, one generator serving both since Trino was
Presto, with the name left on the function that had quietly become the Trino
one. `dburl` is splitting them.

`dbmeta` is not affected either way. It depends on nothing and generates its
own DSNs in `container`, which is D19 and hard rule 1.

### What the fixture cannot build

Everything Trino cannot, and two more.

No `NOT NULL`. The memory connector answers "does not support non-null column"
on 0.299, which is the newest release there is, so every column is nullable.
That is the same error that sets Trino's floor at 476, and here there is no
newer release to move to.

No comment of any kind, as above. The view is created without one, because
Presto's `CREATE VIEW` takes `AS` or `SECURITY` after the name and treats
`COMMENT` as a syntax error rather than accepting and dropping it.

### What the conformance test says

Presto builds every core object D53 asks for, including the view, with the
same names and ordinals as every other database. Every column reads
`nullable=true`, which is the fixture difference above, and that is why Presto
has its own section rather than sharing Trino's. It is left out of the
relational agreement count for the same reason Trino is, plus that one.

### Which answers depend on who is asking

None, the same as Trino and for the same reason. The image configures no
authenticator, a client states a principal on every request, and with no
access control plugin the server allows it everything. Only `current_user`
differs for a second principal.

## Which answers depend on who is asking

Every query has been asked as the administrator and as each lesser kind of
principal the product has. D61 requires it of every dialect,
`test/parity_test.go` does it, and `test/testdata/parity.txt` is the record.

The principals get the same rights over the fixture schema, so the only thing
that varies is what kind of principal they are.

| Database | Principal | Queries that answer differently |
| --- | --- | --- |
| Oracle 26ai | local user | none |
| SQL Server 2022 | contained user | none |
| SQL Server 2022 | server login | `roles` |
| SQL Server 2012, 2014, 2016 | both | the same as 2022 |
| SQL Server 2008 R2 | server login | the same as 2022. It has no contained user |
| PostgreSQL 18 | schema owner | `settings`, `tablespaces` |
| PostgreSQL 18 | grantee | `settings`, `tablespaces` |
| Cassandra 5.0 | granted role | `privileges`, `role_grants`, `roles`, `settings` |
| ClickHouse 26.9 | granted user | `constraints`, `databases`, `foreign_servers`, `index_columns`, `indexes`, `privileges`, `role_grants`, `roles`, `tablespaces` |
| MySQL 8.4 | grantee | `foreign_servers`, `functions`, `role_grants`, `roles`, `user_mappings` |
| MariaDB 13.0 | grantee | `aggregates`, `column_stats`, `foreign_servers`, `role_grants`, `roles`, `user_mappings` |
| MariaDB 10.6 | grantee | the same, plus `functions` |
| Firebird 3.0, 4.0, 5.0 | grantee | `roles`, `settings` |
| Apache Hive 4.2 | other principal | none. The image configures no authorization, so every principal is allowed everything |
| SAP HANA 2.0 SPS 08 | grantee | `collations`, `databases`, `foreign_data_wrappers`, `functions`, `privileges`, `role_grants`, `roles`, `sequences`, `settings`, `triggers`, `views` |

`current_user` and `current_schema` are left out of the table and are in the
file. They answer a question about the connection, so a run where they agreed
would be the fault.

The releases on Windows machines are measured too. They are Verified rather
than Tested, so `dbrun test sqlserver-2008R2` and its siblings run it rather
than CI. A contained database arrived in SQL Server 2012, so 2008 R2 skips that
scene and says so instead of failing.

Every dialect has a target or a recorded reason for having none, and
`TestEveryDialectIsMeasuredForParity` fails when one has neither. SQLite and
DuckDB are the products with no reason to have one: neither has a user.

### One release answers differently

`test/testdata/parity.txt` has a section per product, and two releases have one
of their own, written `postgres@12` and `mariadb@10`. A section named for a
release wins over the shared one for a server reporting that major.

PostgreSQL 12 grants public SELECT on six columns of `pg_subscription` and not
on `subsynccommit`, which the `Subscriptions` query reads as `synchronous`, so
an ordinary role is refused the whole query. PostgreSQL 13 widened the grant to
every column except `subconninfo`, so the same role is served from 13 on. A
superuser reads it on every release.

The query is not gated for this. A superuser on 12 can read the column, and
padding it would withhold a fact from the caller who is allowed it, which rule
13 forbids. What was wrong was the file claiming one answer covers every
release of a product.

It was found by CI rather than by the cross-release check that was supposed to
catch it. That check compared 9.6 against 18, and 9.6 has no `pg_subscription`
at all, so the query was never asked and the difference never showed.

MariaDB 10.6 is the second, and it is `Functions`.
`information_schema.ROUTINES` reports `routine_definition` as NULL to a user
that cannot read the routine's source, and the query selects that column as
`source`, so a grantee gets the same rows with the definition absent. MariaDB
11.3 made `SHOW CREATE ROUTINE` a grantable privilege, and
`GRANT ALL PRIVILEGES` on a database carries it, so 13.0 serves the definition
to the same principal. 10.6 has no such privilege to grant, and only the
definer or a user that can read `mysql.proc` sees it there.

Nothing is gated for this either, and for the same reason: an administrator on
10.6 reads the column, so padding it would withhold a fact from a caller who is
allowed it.

This one was hidden rather than missed. The workflow ran MariaDB with
`-run 'MySQL|InformationSchema|Conformance'`, which no parity test matches, so
parity had never been asked of MariaDB at all. D69 replaced those twelve
hand written jobs with one that runs the whole suite per release, and this was
the first thing it found.

### What each one means for a consumer

The MySQL dialect is the one to plan for. Queries are refused outright, not
narrowed, for a user holding ALL PRIVILEGES on its own database: six on
MariaDB and four on MySQL. They read `mysql.proc`, `mysql.column_stats`,
`mysql.servers`, `mysql.roles_mapping`, `mysql.role_edges` and `mysql.user`,
which are tables in the `mysql` database rather than views that filter
themselves, so the server answers with error 1142 and the query fails. A
consumer that offers these to an ordinary user has to be ready for the error.

The two products do not agree with each other, which is why the record names
the product rather than the dialect. `mysql.proc` was removed in MySQL 8.0 and
MariaDB still has it, so `aggregates` is refused on one and answered on the
other.

PostgreSQL refuses `tablespaces`, because reading `pg_global` needs a
privilege an ordinary role does not have, and narrows `settings` from 404
parameters to 375 because `pg_settings` hides the rest. Everything else
answers identically, because the model reads `pg_catalog`, which shows every
row to everybody. That is why `information_schema` is not used for the
PostgreSQL model: it filters by privilege and `pg_catalog` does not.

SQL Server narrows `roles` for a server login, from 8 to 6, because a login
cannot see every server role. A contained user answers identically to the
sysadmin on every query, which is the cleanest result of the four.

Oracle answers identically on every query for a local user that owns the
objects. That is worth saying plainly, because D60 began from the worry that
`ALL_` views would under-report. They do, but only for a caller asking about
a schema it has no privilege on, which is a different question, and that
measurement is what closed D60 with no change to the model.

## What every database agrees on

`TestConformance` builds the same core schema everywhere and compares the
canonical answer against `test/testdata/conformance.txt`, which is checked in.
23 of the canonical lines are identical across PostgreSQL, MariaDB, MySQL,
SQLite and DuckDB. Every difference below is real and none is a fault.

| Difference | Why |
| --- | --- |
| SQLite reports a primary key column as nullable | An `INTEGER PRIMARY KEY` in SQLite genuinely accepts NULL unless the column is declared NOT NULL. It is the oldest surprise in SQLite and it is not a fault. |
| PostgreSQL reports a default on a primary key | `serial` is implemented as a `nextval` default. `AUTO_INCREMENT` and DuckDB's plain key are not defaults. |
| MariaDB reports the literal `NULL` as the default of a nullable column with no default | MariaDB is saying the column defaults to NULL, which is true. MySQL reports no default, as PostgreSQL, SQLite and DuckDB do. `TestMySQLNullDefault` pins both. |
| MariaDB reports a view's columns as not nullable | The others say nullable. Each is inferring from the view body differently. |
| Only PostgreSQL and DuckDB report the columns of a check constraint | `KEY_COLUMN_USAGE` does not cover a check, and SQLite publishes nothing about one. |

The comparison is over the portable facts only: whether a column accepts NULL,
where it sits, whether it is in the primary key, whether it has an explicit
default, and which columns a constraint covers in what order with what it
references. A type spelling, a rendered default and a rendered definition are
per product and are dropped, and `test/canonical.go` records every one with the
reason.

## The seven kinds psql has no command for

These exist because a consumer measured in D46 needs them. D47 allows them:
`psql` sets the object model and does not set the column set, and `psql`
renders a constraint and a routine signature as text because a person is
reading them.

| Question | PostgreSQL | MariaDB | MySQL | SQLite | DuckDB | SQL Server | information_schema |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `ConstraintColumns` | yes | yes | yes | yes | yes | yes | yes |
| `RoutineParameters` | yes | yes | yes | no | yes | yes | yes |
| `Views` | yes | yes | yes | yes | yes | yes | yes |
| `CurrentSchema` | yes | yes | yes | yes | yes | yes | yes |
| `CurrentUser` | yes | yes | yes | no | yes | yes | yes |
| `EnumValues` | yes | no | no | no | yes | no | no |
| `ColumnStats` | yes | yes | no | no | no | yes | no |

Four of the seven are answered by every model, because the SQL standard defines
`key_column_usage`, `parameters`, `views` and `schemata` and every database
here has them. That was not the expectation: the two the consumers wanted most
turned out to be the two the standard already had.

`CurrentUser` is answered by every model but SQLite, which has no users. It is
the one question here that reads no catalog at all on the shared model, because
`CURRENT_USER` is a standard SQL expression rather than a view. It came from
auditing `usql` for database specific SQL outside its metadata readers, which
found it and found exactly one other thing. See D55.

### Where each one comes from

`ConstraintColumns`. PostgreSQL unnests `conkey` with ordinality and indexes
`confkey` by the same ordinal, which is what keeps a composite key together.
MariaDB, MySQL and the shared model read `key_column_usage`. SQLite unions
three pragmas, matching the three kinds its `Constraints` reports.

`RoutineParameters`. PostgreSQL unnests `proallargtypes`, falling back to
`proargtypes` where a routine has no output parameter. The others read
`parameters`. SQLite has none: a function there is compiled C and SQLite
publishes only how many arguments it takes.

`Views`. PostgreSQL calls `pg_get_viewdef` and lists a materialized view
alongside a plain one, as `psql` does. The others read `views`, except SQLite,
which returns the whole `CREATE VIEW` statement because that is all it stores.

`CurrentSchema`. It is session dependent and it says so. PostgreSQL returns the
first entry of `search_path` that exists. MariaDB and MySQL return the database
in use, and no row at all when the connection named none, which is honest: an
unqualified name resolves nowhere until a `USE` runs. SQLite returns `main`,
which is a constant, because SQLite looks in `temp` first and reports nothing
about that.

`EnumValues`. Only PostgreSQL has an enumerated type. MariaDB and MySQL have an
enum column rather than an enum type, and the labels exist only inside the
`enum('red','green','blue')` text of `COLUMN_TYPE`. Splitting that correctly
needs to track quoting, because a label may contain a comma or an escaped
quote, and no portable SQL does that. `Columns.DataType` returns the text
verbatim, so a caller that knows the product's quoting rules can parse it.
`dbtpl` splits it in Go today and has the same limitation.

`ColumnStats`. It is runtime and it can be stale, and a column never analyzed
has no row. PostgreSQL reads `pg_stats`. MariaDB reads `mysql.column_stats`,
which `ANALYZE TABLE ... PERSISTENT` fills, and derives the distinct count from
the average frequency and the row count. MySQL keeps only a JSON histogram in
`information_schema.COLUMN_STATISTICS`, with no width, no null fraction and no
distinct count, and only where somebody ran `ANALYZE TABLE ... UPDATE
HISTOGRAM`, so it reports the kind unsupported rather than returning rows that
are almost all absent. SQLite's `sqlite_stat1` holds one text string per index
and says nothing about a column's values.

### Two fields rather than kinds

`Column.PrimaryKey` is the case the policy was written around. MySQL has
`COLUMN_KEY` and SQLite has the `pk` column of `pragma_table_xinfo`, so it is
already in the row both select. PostgreSQL needs one more join, to the primary
key index of the relation. Free on two, one join on the third, and it saves
every consumer a second query and a join in Go.

`Function.ID` identifies a routine where the name does not. PostgreSQL returns
the oid, because it allows two functions with one name and different
parameters. Everything else returns the name or the specific name, because
nothing else here overloads. `RoutineParameters` carries the same value, so a
caller writes one join for every database.

## DuckDB

DuckDB answers 20 of the 55, which is second only to PostgreSQL. Its catalog is
unusually complete for an embedded database: comments on most objects, real
enumerated types, sequences with their bounds, and a constraint catalog that
names both the columns of a key and the columns they reference.

Like SQLite it has no server, so the release under test is whichever one the Go
driver was built with, there is no container, and the model carries no version
gate. Unlike SQLite its driver needs cgo, which D48 allows in the test module.

### How the catalog is shaped

DuckDB publishes its catalog as table functions named `duckdb_something` rather
than as views, and they behave like tables in a `FROM` clause:

```sql
FROM duckdb_columns() c JOIN duckdb_tables() t ON t.table_oid = c.table_oid
```

Where PostgreSQL carries an array, DuckDB carries a list, and
`unnest(list) WITH ORDINALITY` turns one into rows. That is how the columns of
a constraint, the labels of an enum and the parameters of a function are read,
which is the same shape the PostgreSQL model uses for the same things.

Every catalog function carries an `internal` flag, so the system object rule is
one clause and there is no list of schema names to keep up to date. That is
better than every other model here manages.

### What it answers

Schemas, databases, tables, columns, indexes, constraints, constraint columns,
sequences, views, comments, types, enum values, functions, aggregates, routine
parameters, collations, settings, extensions and the current schema.

Two are worth naming. `Comments` gathers comments from five catalog functions,
because DuckDB puts a comment on the object rather than in a catalog of
comments. `Extensions` is a real answer rather than an analogue: a DuckDB
extension is installed and loaded separately, which is closer to a PostgreSQL
extension than anything else here manages.

### The one that looks answerable and is not

`IndexColumns`. `duckdb_indexes` has an `expressions` column that prints as
`[title]`, and its type is `VARCHAR` rather than `VARCHAR[]`. It is a rendering
of a list rather than a list, so `unnest` refuses it, and splitting the text on
a comma breaks the moment an index is on an expression containing one, such as
`concat(a, b)`.

`duckdb_constraints.constraint_column_names` is a real `VARCHAR[]`, which is
why `ConstraintColumns` works and this does not. The columns of a primary key
or a unique constraint are therefore readable and the columns of an index
someone created are not.

### Analogues that were found and rejected

`SUMMARIZE` or `PRAGMA storage_info` for `ColumnStats`. The two reviews split
on this and running it settled it against both. DeepSeek said `storage_info`
provides column statistics: it returns one row per row group per segment with
the statistics as a formatted string, `[Min: 1, Max: 199][Has Null: false]`,
which is prose rather than parts. Gemini said nothing provides them, and missed
`SUMMARIZE`, which returns genuinely good per column statistics including the
minimum, maximum, approximate distinct count, mean and null fraction.

`SUMMARIZE` is rejected for a different reason: it scans the data rather than
reading a catalog, and it takes one statement per table. That fails the cost
test in D47 and the one statement rule in D33. PostgreSQL and MariaDB keep
statistics the planner wrote; DuckDB computes them on demand and stores
nothing, so there is no catalog to read.

`duckdb_dependencies` for `ExtensionObjects`. Both reviews rejected it. It
records every catalog dependency, such as a view on a table, rather than what
an extension owns.

An attached PostgreSQL, SQLite or MySQL database for `ForeignTables`,
`ForeignServers` or `ForeignDataWrappers`. `ATTACH` makes the other database a
full catalog rather than a wrapped remote, so it is reported as a `Database`,
which is what it is.

Hive partitioning through `read_parquet()` for `PartitionedTables`. It is a
file layout read at scan time, not a catalogued object.

### A difference worth knowing

DuckDB generates its own constraint names and ignores one given in the DDL. The
fixture writes `CONSTRAINT shipment_region_fk` and DuckDB records
`shipment_country_area_country_area_fkey`. A caller matching constraints across
databases by name will not find them.

### NOT NULL, and why it is filtered here too

DuckDB records a NOT NULL as a row in `duckdb_constraints`, exactly as
PostgreSQL 18 does, and `Constraints` and `ConstraintColumns` both leave it
out.

D49 filtered it on PostgreSQL for cross release consistency, which does not
apply here because DuckDB has always recorded it. A second reason does:
PostgreSQL never reports a NOT NULL constraint and DuckDB would, so the same
schema would answer differently by database. `Column.Nullable` carries the fact
in both.

DeepSeek argued the other way, that `duckdb_constraints` is DuckDB's own
catalog and hiding a row loses information. The information is not lost. The
constraint name is, which D49 already records as a known gap.

### What it has none of

Everything that needs more than one user or more than one process. No roles, no
privileges, no triggers, no tablespaces. No casts, domains, operators,
procedural languages, large objects, event triggers, text search objects or
replication.

## Microsoft SQL Server

SQL Server answers 32 of the 55, which is more than any database here except
PostgreSQL. It is the only one besides PostgreSQL with roles, privileges,
tablespaces and DDL triggers, and the only one that keeps comments in a catalog
of their own rather than on the object.

It is tested on every major release that runs on Linux: 2017, 2019, 2022 and
2025, all four on every push. 2016 and older have no container, so they are
provisioned in a Windows virtual machine instead and are Verified rather than
Tested. D54 says what a Linux release claims and D57 says what a machine does.

| Release | How | Verified against |
| --- | --- | --- |
| 2017, 2019, 2022, 2025 | Linux container, every push | 14.0.3550.4, 15.0.4490.9, 16.0.4295.3, 17.0.5005.3 |
| 2016 | Windows Server 2016 machine | 13.0.5026.0 SP2 Express, on 2026-09-25 |
| 2014 | Windows Server 2012 R2 machine | 12.0.2000.8 RTM Express, on 2026-09-25 |
| 2012 | Windows Server 2012 R2 machine | 11.0.7001.0 SP4 Express, on 2026-09-25 |
| 2008 R2 | Windows Server 2008 R2 machine | 10.50.4000.0 SP2 Express, on 2026-09-25 |

The gates show up as a clean progression, and every step of it was checked
against a real server rather than a version set:

| Release | Answers | Refused as too old |
| --- | --- | --- |
| 2016 and newer | 32 | none |
| 2014, 2012 | 31 | `ForeignTables` |
| 2008 R2 | 29 | `ForeignTables`, `Sequences`, `ColumnStats` |

`ForeignTables` reads `sys.external_tables`, which arrived in 2016. `Sequences`
and `ColumnStats` read `sys.sequences` and `sys.dm_db_stats_properties`, which
arrived in 2012. Each is refused with `ErrVersionTooOld` rather than returning
nothing, which is the whole point of the gate.

The machines earned their cost immediately. Every gate in this model sits below
2017, so until one ran, the old branch of each was checked only by resolving a
statement against a version set with no server. Running 2012 and 2014 found
three faults that no container could have.

The fixture tore nothing down. Every teardown statement used `DROP ... IF
EXISTS`, which arrived in 2016, so on 2014 and older every drop was a syntax
error and the next test met a schema that was already there.

The display line printed the build twice on a release with no service pack.
`@@VERSION` reads `Microsoft SQL Server 2014 - 12.0.2000.8 (X64)`, where the
first parenthesis comes after the build rather than after the year, so cutting
there kept too much. Every release with a Linux container ships a cumulative
update and names it, so the shorter form never appeared.

`ColumnStats` was supported and had nothing to report. The query gates at 2012
and the fixture step that creates the statistics was gated at 2016, following a
comment that had the release wrong. The two gates disagreed, and only a server
between them could show it.

2008 R2 then found a fourth, of the same family. The fixture created a sequence
unconditionally, and `CREATE SEQUENCE` arrived in 2012, so the whole fixture
failed before any query ran. Both the create and the drop are gated at 2012
now. Guarding the drop with `IF OBJECT_ID(...) IS NOT NULL` is not enough,
because 2008 R2 rejects the word `SEQUENCE` when it parses the batch, whatever
the condition around it says.

The count is four faults from four machines, all of them in the part of the
model that no container can reach.

### The sys schema, not information_schema

SQL Server ships both, and the `sys` views carry what `information_schema`
cannot: whether an index is unique, what a foreign key points at, where a table
lives, and the identity of every object. Microsoft's own documentation says to
prefer them, and D9 says to prefer a native catalog anyway.

### What it answers

Schemas, databases, tables, columns, indexes, index columns, constraints,
constraint columns, sequences, views, triggers, event triggers, comments,
types, domains, collations, functions, aggregates, routine parameters, roles,
role grants, privileges, tablespaces, partitioned tables, foreign servers, user
mappings, foreign tables, column statistics, extended statistics, settings and
the current schema.

Four are worth naming. `Comments` reads `sys.extended_properties` for the
`MS_Description` property, which is the convention every SQL Server tool uses
and the closest thing the product has to `COMMENT ON`. `ForeignServers` and
`UserMappings` read `sys.servers` and `sys.linked_logins`, because a linked
server is what SQL Server has instead of a foreign server. `Tablespaces` reads
the filegroups, which is a genuine match rather than an analogue: a filegroup
is where a table's pages live and that is what the question asks.

### Visibility rather than refusal

A SQL Server catalog view shows the caller what the caller may see and returns
fewer rows otherwise. It does not refuse.

Three queries here depend on a privilege. `ColumnStats` reads
`sys.dm_db_stats_properties` and needs `VIEW STATISTICS`. `UserMappings` reads
`sys.linked_logins` and needs a server level permission. `ForeignTables` reads
`sys.external_tables`, which exists in every install and holds nothing until
PolyBase is configured. All three were run as a user holding `VIEW DEFINITION`
alone, and all three returned an empty result rather than an error.

That is worth knowing because it is a third behaviour. PostgreSQL shows a
caller everything, MariaDB refuses outright, and SQL Server quietly narrows the
answer. A consumer that treats an empty result as "there are none" is wrong on
SQL Server in a way it is not wrong on PostgreSQL.

### The gates, all of which sit below the oldest testable release

`sys.sequences` and `sys.dm_db_stats_properties` arrived in 2012.
`sys.external_tables`, `sys.tables.is_external` and `sys.tables.temporal_type`
arrived in 2016. Every one of those is below the 2017 floor, so CI reaches the
new branch of each gate and never the old one.

`models/sqlserver/version_test.go` holds the old branch instead. It resolves
each statement against a version set without a server and checks that a gated
query refuses below the release that added its view, that the padded
alternative never names a column an older release has not got, and that no
other query quietly depends on a release. See D54.

One thing the model avoids on purpose. `STRING_AGG` arrived in 2017 and would
be safe at this floor, and the model uses `STUFF(... FOR XML PATH(''))`
instead, which works from 2005. The floor is a container fact and the queries
do not have to inherit it.

### What it has none of

No enumerated type, so nothing for `EnumValues` to list. No operator catalog
and nothing a user can create, which rules out `Operators`,
`OperatorClasses`, `OperatorFamilies`, `Casts` and `Conversions`. No procedural
language catalog: T-SQL is the language and a CLR assembly is not one, which
rules out `Languages`. No text search objects of the shape `psql` names and no
logical replication, which rules out `TextSearchConfigs`,
`TextSearchParsers` and their relatives, `Publications` and `Subscriptions`.

### Analogues that were found and rejected

Hard rule 13 says to ask several models and then run the answer against a real
server. These were named, checked on SQL Server 2022, and left unsupported.

`Extensions` and `ExtensionObjects` from `sys.assemblies`,
`sys.assembly_modules` and `sys.assembly_types`. A PostgreSQL extension is a
versioned bundle of catalog objects that one statement installs and one removes.
A CLR assembly is compiled .NET code that a later `CREATE FUNCTION` can point
at, which is not the same thing. On a stock 2022 the view holds exactly one
row, `Microsoft.SqlServer.Types`, and `clr enabled` is 0, so the feature is off
by default and Azure SQL Database forbids it outright. Reporting one disabled
system assembly as the extensions of the database is worse than saying there
are none.

`DefaultACLs` from `sys.database_permissions` where `class = 3`. A default ACL
says what will be granted on objects created later. A schema scoped permission
is a grant that is already in effect and that new objects inherit at runtime.
The two produce a similar result and they are different facts, and the rows are
already reported by `Privileges`. On a stock 2022 the count is 0.

`RoleSettings` from `sys.server_principals` and `sys.database_principals`.
Those carry `default_database_name` and `default_language_name` and nothing
else. A role setting in PostgreSQL is an arbitrary configuration parameter
attached to a role, and two fixed columns are not that.

`ForeignDataWrappers`. A linked server names an OLE DB provider, and the
providers are COM components registered with the operating system. They are
enumerated by an extended stored procedure and not by any catalog view, so
there is nothing to read. `ForeignServers` already answers the question the
linked server itself asks.

`AccessMethods`. SQL Server's index kinds are engine features rather than
catalog rows. They appear as a `type_desc` on `sys.indexes` and there is no
catalog of the methods themselves, and nothing can add one.

`LargeObjects`. Every binary value in SQL Server is a column value. FILESTREAM
and FileTable put the bytes on disk and keep them addressed by the row, so
there is no object with an identity and an owner of its own to list.

## Oracle

Oracle answers 25 of the 55. Every one is verified on six releases.

It needs no Windows and no virtual machine, which is the opposite of SQL
Server. The free Express images reach back to 11g Release 2 from 2010, so every
release runs in an ordinary container. See D57 for the SQL Server contrast and
`container/oracle.go` for the list.

### What it answers

Schemas, tables, columns, indexes, index columns, constraints, constraint
columns, sequences, views, the current schema and the current user. Then
comments, triggers, event triggers, functions, aggregates, routine parameters,
types, domains, operators, privileges, column statistics, partitioned tables,
foreign servers and foreign tables.

Verified on 11g, 18c, 19c, 21c, 23ai and 26ai. Every release runs 24 of them
and each returns the number of columns it declares. `Domains` is the
twenty-fifth and needs 23ai, where the SQL domain and `ALL_DOMAINS` arrived,
so 23ai and 26ai run all 25 and the four older releases report that the server
is too old.

### Which user the tests run as

Every Oracle service here names a pluggable database, so the fixture user is a
local user on every release that has the concept. A local user authenticates
against one pluggable database and has nothing at the container level, and it
is the Oracle equivalent of a SQL Server contained database user. A user
created in the container root is a common user instead, which exists in the
root and in every pluggable database at once, and that is the equivalent of a
SQL Server server login.

This was wrong until it was measured. XE answered as `CDB$ROOT` and FREE
likewise, so 18c, 21c, 23ai and 26ai built the fixture as a common user while
only 19c built it as a local one. Nobody had chosen that, and the queries were
being validated mostly against a principal a consumer does not use. The
gvenzl images set `common_user_prefix` to empty, which is why a plainly named
user in the root was accepted there and raised `ORA-65096` on the 19c image.

11g needs none of this. It predates multitenant, so it has no container
database and no `COMMON` column, and `XE` is the instance itself.

Nothing tests a connection to `CDB$ROOT` now. That is a real thing a DBA does
and it is worth a target of its own, named so that a failure reads root rather
than a release. It is not written yet.

### What the conformance test says

Oracle is in `test/testdata/conformance.txt` under `[oracle]`, which D53
requires. Its lines match SQL Server exactly apart from one, and the two
differences from PostgreSQL are both product differences rather than faults:

`recent.book_id` and `recent.title` are `nullable=false`. Oracle carries the
NOT NULL of a base column into a view, and so does SQL Server. PostgreSQL and
SQLite report a view column as nullable whatever it is selected from.

`author_id`, `book_id` and `shipment_id` are `has_default=false`. The Oracle
fixture gives them a plain `NUMBER(10)` primary key, because an identity
column is 12c and later and the fixture has to build on 11g. PostgreSQL uses
`serial`, which is a default.

The canonical projection folds identifier case. Oracle stores an unquoted name
in upper case and everything else here stores it in lower case, and that is a
difference in spelling rather than in structure. See `fold` in
`test/canonical.go`.

Adding Oracle found one fault. `Constraints` returned the fixture's check
constraint and `ConstraintColumns` returned no columns for it, because the
second query dropped every check rather than only the generated NOT NULL ones
that D49 excludes. The rule was written out twice and the two copies came
apart. Both now read `notGeneratedNotNull`.

### ALL_ rather than DBA_ or USER_

Every query reads the `ALL_` views. Both reviews agreed, and the reasoning is
the one this project already knows from `information_schema`: `ALL_` shows the
connected user what it may see and needs no special role, `DBA_` needs
`SELECT_CATALOG_ROLE` that an ordinary application user does not have, and
`USER_` shows only the caller's own schema and has no `OWNER` column at all.

The cost is the same one `information_schema` has. `ALL_` silently drops a row
the caller cannot see, so an unprivileged connection gets a smaller answer
rather than an error.

### A schema is a user

Oracle has no schema object separate from the user that owns it. `Schemas`
reads `ALL_USERS`, the owner of a schema is its own name, and the fixture
creates a user rather than a schema. That also makes the teardown one
statement, because dropping the user cascades to everything it owns.

### Four things the dictionary does that nothing else here does

`LONG` columns. `ALL_TAB_COLUMNS.DATA_DEFAULT`, `ALL_CONSTRAINTS.SEARCH_CONDITION`
and `ALL_VIEWS.TEXT` are all `LONG`, which cannot be joined, compared or
wrapped in a function. They are selected bare. Oracle added `VARCHAR2` twins
for two of them later, `SEARCH_CONDITION_VC` in 12c and `TEXT_VC` in 18c, and
those are gated so an older release reads the `LONG`.

No boolean before 23ai. Nullability is a `Y` or an `N`, uniqueness is the word
`UNIQUE`, and a cycling sequence is a `Y`. Every one is translated with a
`CASE`.

One letter constraint kinds. `P`, `U`, `R` and `C`, where `R` is a foreign key
because Oracle calls it referential. `C` covers a check and a `NOT NULL`
together, and D49 says a `NOT NULL` is not a constraint row, so the generated
ones are filtered out.

A system schema flag, from 18c. `all_users.oracle_maintained` says whether
Oracle created the user, and it is the authority where it exists: it finds
OJVMSYS on 19c, DGPDB_INT on 21c, and BAASSYS, GGSHAREDCAP and VECSYS on 23ai,
none of which a written list had. The written list stays for 11g, which has no
such column, and for PDBADMIN, which the flag calls a person's account from
19c because the script that creates the pluggable database creates it. So the
test is the list on 11g and the list and the flag from 18c.

### Releases, and what separates them

| Release | Version it reports | Why it is here |
| --- | --- | --- |
| 11g | 11.2.0.2.0 | the one release before multitenant: no identity column, no `CDB_` views, no `CON_ID`, 30 byte identifiers |
| 18c | 18.0.0.0.0 | where `version_full` and `TEXT_VC` arrived |
| 19c | 19.0.0.0.0 | the long term release most installations run |
| 21c | 21.0.0.0.0 | native JSON as a column type |
| 23ai | 23.0.0.0.0 | domains, and the vector and boolean types |
| 26ai | 23.26.3.0.0 | 23.26, not 26: a dozen more `ALL_` views than 23ai |

The version comes from the banner, which is the one source every release has.
`product_component_version.version_full` is more precise and does not exist
before 18c, and `v$version.banner_full` likewise, so both are a parse error on
11g. The banner also carries the name Oracle sells the release under, which
nothing else does, and its number separates 23ai from 26ai.

### The D43 pass, and what it found

Eleven of 55 was too few for a dictionary this rich, so both models were asked
to sort the other 44. They disagreed in a way that proves the rule: DeepSeek
marked `ForeignServers` and `ForeignTables` absent, and Gemini named
`ALL_DB_LINKS` and `ALL_EXTERNAL_TABLES` for them. A database link is exactly
what a foreign server is, and `usql`'s own Oracle reader already queries it.

The pass produced nineteen leads. D43 says to run each against a real server
before believing it, and running them removed six. Fourteen shipped, which
took Oracle from 11 to 25.

| Question | Reads | Note |
| --- | --- | --- |
| `Comments` | `ALL_TAB_COMMENTS`, `ALL_COL_COMMENTS` | the two are unioned, and a column is named table.column |
| `Triggers` | `ALL_TRIGGERS` | where `base_object_type` is a table or a view |
| `EventTriggers` | `ALL_TRIGGERS` | where `base_object_type` is `SCHEMA` or `DATABASE` |
| `Functions` | `ALL_PROCEDURES` | where `aggregate` is `NO` |
| `Aggregates` | `ALL_PROCEDURES` | where `aggregate` is `YES` |
| `RoutineParameters` | `ALL_ARGUMENTS` | |
| `Types` | `ALL_TYPES` | the element type comes from `ALL_COLL_TYPES` |
| `Domains` | `ALL_DOMAINS`, `ALL_DOMAIN_COLS` | 23ai, so gated |
| `Privileges` | `ALL_TAB_PRIVS` | one row per grant, folded into one list per object |
| `ColumnStats` | `ALL_TAB_COL_STATISTICS` | |
| `PartitionedTables` | `ALL_PART_TABLES`, `ALL_PART_KEY_COLUMNS` | |
| `Operators` | `ALL_OPERATORS`, `ALL_OPBINDINGS`, `ALL_OPARGUMENTS` | one row per binding |
| `ForeignServers` | `ALL_DB_LINKS` | a database link |
| `ForeignTables` | `ALL_EXTERNAL_TABLES` | |

### The five leads a real server disproved

D43 exists for this. Three of the views the models named do not exist on any
release here, and checking took one query:

| Lead | 11g | 19c | 26ai |
| --- | --- | --- | --- |
| `ALL_TABLESPACES` | no | no | no |
| `ALL_COLLATIONS` | no | no | no |
| `ALL_PDBS` | no | no | no |

`Tablespaces` is in `USER_TABLESPACES` and `DBA_TABLESPACES` only, which D60
decided against. `Collations` was said to have arrived in 12.2, and it did
not: 26ai has
no such view either. `Databases` has the same problem, and `ALL_PDBS` never
existed under that name.

Two more exist and were rejected on reading them:

`LargeObjects` from `ALL_LOBS`. The view describes the storage of a LOB column
of a table, not an object with an identity of its own. There is no id to
return for `LargeObject.OID`, because Oracle has no standalone large object.
A stretch, and D43 says leave a stretch unsupported.

`ExtendedStats` from `ALL_STAT_EXTENSIONS`. The view has the extension
expression and no list of statistic kinds. Putting the expression in a field
named `Kinds` would be shaping the answer so it fills a column, which rule 13
forbids.

`Settings` from `V$PARAMETER` is a lead that still stands and is not written.

### Analogues the pass rejected

`AccessMethods` from `ALL_INDEXTYPES` and `OperatorClasses` from
`ALL_INDEXTYPE_OPERATORS`. An Oracle indextype is a user written access method
for a domain index, which is a narrower thing than a PostgreSQL access method,
and neither view describes the built in ones. A stretch, and D43 says leave a
stretch unsupported.

`Publications` and `Subscriptions` from `ALL_PUBLISHED_COLUMNS` and
`ALL_SUBSCRIPTIONS`. Those belong to Change Data Capture, which Oracle
deprecated, and they describe something other than logical replication.

### What Oracle has none of

No enumerated type, so nothing for `EnumValues`. No procedural language
catalog, no cast catalog, no conversion catalog, no operator class or family,
no text search objects of the shape `psql` names, and no extension.

`Roles` and `RoleGrants` are a different case and are not absent. Oracle has
both, in `DBA_ROLES` and `DBA_ROLE_PRIVS`, and there is no `ALL_` equivalent:
an ordinary user sees only its own, through `USER_ROLE_PRIVS` and
`SESSION_ROLES`. Answering them means reading `DBA_`, which needs
`SELECT_CATALOG_ROLE`, and D60 decided not to reach for it. Neither query is
written.

`Databases` is the same shape of problem. Oracle has one database per
instance, and the nearest list is the pluggable databases, which no `ALL_`
view carries: `ALL_PDBS` does not exist on any release here. `DBA_PDBS` and
`V$DATABASE` do, and D60 decided against reading either.
