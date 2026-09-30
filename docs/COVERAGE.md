# What Each Database Can Answer

`dbmeta` asks every database the same 56 questions. PostgreSQL answers all of
them, because PostgreSQL is the model. No other database answers all of them,
and this document says which ones each database answers, which ones it cannot,
and why.

Forty nine of the questions come from `psql`. The other seven exist because a
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
D43 in `decisions/` for the rule about finding these analogues in the first place.

Adding a database means adding a section here. [`DIALECT.md`](DIALECT.md) holds
every step and this is one of them, so read that first if you are adding one
rather than reading one.

## The count

| Model | Answers | Of | Tested against |
| --- | --- | --- | --- |
| `models/postgres` | 56 | 56 | PostgreSQL 9.6 through 18 |
| `models/mysql` | 29 on MariaDB, 26 on MySQL | 56 | MariaDB 10.6 to 13.0, MySQL 8.4 to 26.7 |
| `models/sqlite3` | 14 | 56 | both drivers: mattn/go-sqlite3 and modernc.org/sqlite |
| `models/duckdb` | 20 | 56 | duckdb/duckdb-go, the driver usql uses |
| `models/sqlserver` | 32 | 56 | SQL Server 2017, 2019, 2022 and 2025 |
| `models/oracle` | 26 | 56 | Oracle 11g, 18c, 19c, 21c, 23ai and 26ai |
| `models/cassandra` | 17 on Cassandra, 18 on ScyllaDB | 56 | Cassandra 3.11, 4.0, 4.1 and 5.0, ScyllaDB 2025.1, 2026.1, 2026.2 and 2026.3 |
| `models/clickhouse` | 23 | 56 | ClickHouse 25.3, 25.8, 26.8 and 26.9 |
| `models/trino` | 13 | 56 | Trino 476 and 483 |
| `models/presto` | 9 | 56 | Presto 0.299 |
| `models/firebird` | 24 | 56 | Firebird 3.0, 4.0 and 5.0 |
| `models/hana` | 32 | 56 | SAP HANA 2.00.076, 2.00.082 and 2.00.088, which are SPS 07 and SPS 08 |
| `models/hive` | 16 | 56 | Apache Hive 4.0 and 4.2 |
| `models/exasol` | 25 | 56 | Exasol 2026.2.0 on the nano image, and 2025.2.1 on the Community Edition machine |
| `models/vertica` | 26 | 56 | Vertica 7.2.1, 9.1.0, 10.1.1 and 25.1.0, on copies of community images in `docker.io/usql/vertica` |
| `models/couchbase` | 12 | 56 | Couchbase 7.6.12 and 8.0.3, and 7.2.9, which is Tested and refused as too old |
| `models/cockroachdb` | 54 | 56 | CockroachDB 24.3.36, 26.2.7 and 26.3.2. 47 of its statements are the postgres model's (D123) |
| `models/cratedb` | 26 | 56 | CrateDB 6.3.7 and 6.4.5, where 6.3 answers one fewer, collations. 3 of its statements are the postgres model's (D123) |
| `models/questdb` | 11 | 56 | QuestDB 9.4.3 and 10.0.1, on the PostgreSQL interface with pgx |
| `models/tidb` | 19 | 56 | TiDB 7.5.8, 8.1.2 and 8.5.8, where privileges needs 8.5. 16 of its statements are the mysql model's (D133) |
| `models/vitess` | 20 | 56 | Vitess 23.0.6 and 24.0.3, on vttestserver. 19 of its statements are the mysql model's, and a schema is a keyspace (D135) |
| `models/databend` | 20 | 56 | Databend 1.2.881 and 1.2.948, from the system database, with dbimp's driver (D140) |
| `models/singlestore` | 23 | 56 | SingleStore 9.0 and 9.1, on the development image with no licence. 16 of its statements are the mysql model's (D141) |
| `models/snowflake` | 13 | 56 | not run: written from Snowflake's documentation before an account was provisioned (D144) |
| `models/redshift` | 11 | 56 | not run: written from Redshift's documentation before a cluster was provisioned (D144) |
| `models/impala` | 11 | 56 | Apache Impala 4.4.1 and 4.5.2, in one container dbrun builds. Most kinds are a walk of SHOW statements (D146) |
| `models/rqlite` | 14 | 56 | rqlite 9.4.5 and 10.3.6, through gorqlite until dbimp has a driver. Every statement is the sqlite3 model's (D148) |
| `models/informationschema` | 12 | 56 | any database with a standard `information_schema` |

The shared `information_schema` model answers twelve: tables, schemas,
columns, functions, privileges, constraints, sequences, constraint columns,
routine parameters, views, the current schema and the current user. It is the floor. A native model
exists to beat it, and `models/mysql` beats it by seventeen.

Four of those twelve arrived with the kinds D47 added, and they arrived for
free: the standard defines `key_column_usage`, `parameters`, `views` and
`schemata`, so every database close to the standard answers them.

## MariaDB and MySQL

These are two products sharing one dialect and one model. Most of what follows
is true of both. Where they differ, the model gates on the product rather than
on the release number, because MariaDB is at 11.8 and MySQL at 9 and neither
number says anything about the other. See D44.

MariaDB answers 29 of the 56 and MySQL answers 26. The three MySQL cannot
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
| `TextSearchParsers`, `TextSearchDictionaries`, `TextSearchTemplates`, `TextSearchConfigs`, `TextSearchConfigMaps` | Full text search is a `FULLTEXT` index with a built in tokenizer. None of its parts is a nameable object, so there is nothing to map either. |
| `ExtensionObjects` | A plugin owns no SQL objects, so there is nothing to list as belonging to one. |

### Analogues that were found and rejected

Two AI models were asked what MariaDB holds for the 29 questions the first pass
did not answer, which is the rule D43 sets. Both named analogues that do not
survive a look at a running server. They are recorded here so that the next
person does not find them again and reach the other conclusion.

`information_schema.TABLESPACES` for `Tablespaces`. Both models named it. The
table exists and it has the right columns, and on MariaDB 11.8 it returns no
rows and can never return any: it is the MySQL Cluster table, kept for
compatibility. `information_schema.INNODB_SYS_TABLESPACES` does return rows,
and each row is one file behind one table, not a named place an administrator
created. A list that names `dbmeta_fixture/book` as a tablespace teaches a
caller something false.

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
an empty answer here reads as "this server has no full text search".

`mysql.slave_master_info` for `Subscriptions`. MariaDB does not have that
table. It keeps master information in a file, and `SHOW SLAVE STATUS` is the
way to read it, which is not a catalog query.

### Catalogs MariaDB has that no question asks for

These hold real metadata and no `psql` command maps onto them, so `dbmeta` does
not read them. A later question can.

`information_schema.EVENTS` holds scheduled events, which PostgreSQL has no
form of. `information_schema.PERIODS` holds application time periods. It is present on
11.8 and absent on 10.6, so a query for it needs a version gate.

## SQLite

SQLite answers 14 of the 56. It is the smallest native model here and it still
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
reasons are in the table under The seven kinds psql has no command for.

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
did not answer, which is the rule D43 sets. They agreed on almost all of it,
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
returns is local data.

`pragma_compile_options` for `Settings`, rejected above.

`sqlite_stat1` for `ExtendedStats`. It holds one row per index, which is
PostgreSQL's `pg_statistic`, not the `CREATE STATISTICS` objects that `\dX`
lists.

`pragma_table_xinfo.type` for `Types`. A declared type in SQLite is an
unenforced affinity hint. A column of them presented as a type catalog
suggests a check that does not happen.

`pragma_function_list` type `a` and `w` for `Aggregates`. This is the one the
models split on, and running it settled it against both. Gemini said to map
both and called it exact. DeepSeek said to map only `a`. On a real server,
`sum`, `count`, `avg` and `group_concat` all report as `w`, and so do
`row_number`, `rank` and `lag`. Type `a` matched one function, an extension.
If the query maps `w`, it lists `row_number` as an aggregate. If it maps `a`,
it omits `sum`. Both mislead, there is no third option, and SQLite cannot
tell an aggregate from a window function. `Aggregates` is unsupported and
`Functions` reports the kind SQLite reports.

### What it has none of

SQLite has no users, no roles and no grants of any kind, so `Roles`,
`RoleGrants`, `RoleSettings`, `Privileges` and `DefaultACLs` are absent rather
than empty. It records no comment on any object. It has no type catalog, no
operators that can be created, no casts, no procedural languages, no
replication, no tablespaces and no partitioning.

## Cassandra

Cassandra answers 17 of the 56, verified against 5.0.9 and 3.11.19.

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
else answers on every release from 3.11 up. That is the only fragment gated on
a Cassandra release, which is why the tested pair spans it. The other
fragments gate on the `scylla` key.

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
one does not make the answer ordered.

### What the fixture builds, and what it needs

`models/cassandra/fixture` creates the keyspace `dbmeta_fixture` with the core
objects D53 asks every fixture for, plus one of every Cassandra object the
queries read: a user defined type, a secondary index, a materialized view, a
function, an aggregate built on a second function, two roles, a grant between
them and a permission on the keyspace. Twenty one steps. Cassandra runs
nineteen on every release and skips the two service level steps, which run on
ScyllaDB alone.

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

### A null arrives as a null

The tests use `github.com/xo/cql`, which reports a CQL null as NULL. The
driver before it sent an empty string for every null, and D62 worked around
that by discarding each padded column. D93 records the change.

A real catalog column that is null now reaches the caller as NULL, so `Valid`
means what it means on every other dialect. Cassandra itself stores the empty
string rather than a null for a table with no comment, so an uncommented table
reports a valid empty comment. That is what the catalog holds.

### What the conformance test says

Cassandra is in `test/testdata/conformance.txt` under `[cassandra]`, and it
agrees with the relational databases on less than they agree with each other.
That is why `TestConformanceAgreementHolds` now measures twice: the relational
databases against the floor they have always held, and every database against
a smaller one. If Cassandra counts with the rest, the floor drops from 23
lines to 6 and is too low to notice a regression anywhere.

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

## ScyllaDB

ScyllaDB answers 18 of the 56, verified against 2025.1.15, 2026.1, 2026.2 and
2026.3.1. It is a second product that speaks CQL, and `models/cassandra` reads
it. Cassandra is the reference product and ScyllaDB is the flavor, the way
MariaDB and MySQL share `models/mysql`. D91 is the decision.

### How the model knows which product it has

ScyllaDB puts a `supported_features` column in `system.local`, and Cassandra
has no such column. The version query reads the whole row as one JSON text,
with `SELECT JSON *`, because CQL refuses a statement that names a column the
table does not have. A row with that column is ScyllaDB, and the model records
the `scylla` version key.

Its `release_version` is 3.0.8 on every release measured. That is the
Cassandra release that ScyllaDB keeps compatible with, not its own release,
and it stays the main version, because it describes the catalog that ScyllaDB
offers. The ScyllaDB release is in `system.versions`, which Cassandra does not have,
so a second statement reads it, and only on ScyllaDB. D92 is the decision. A
role granted nothing is refused `system.versions` and served `system.local`,
so it learns that it is talking to ScyllaDB and not which release. The
version read does not fail for it.

### What differs from Cassandra

ScyllaDB keeps `system_schema` as Cassandra 3.0 laid it out, so most queries
need nothing. Four things differ, and each one is a fragment on the `scylla`
key:

1. Roles, role grants and permissions are in `system.roles`,
   `system.role_members` and `system.role_permissions`, with the columns that
   Cassandra has in `system_auth`. `system_auth.roles` is refused as an
   unconfigured table on both 2025.1 and 2026.3.
2. Settings are in `system.config`, which also records a type for each
   setting and where its value came from. The value arrives as JSON, so a text
   value is in double quotes. The `system_views` gate at Cassandra 4.0 does
   not reach ScyllaDB, because its main version is 3.0.8, so the ScyllaDB
   fragment is the one that answers.
3. No literal is allowed in a select list. 2025.1 refuses `(text)NULL`,
   `(boolean)false` and `CAST(false AS boolean)` as syntax errors. 2026.3
   takes the cast and still refuses every NULL. So where the Cassandra
   statement selects a literal, the ScyllaDB fragment selects a real column of
   the same table under the same name, and Scan discards that column on both
   products and sets the known value itself.
4. `RoleSettings` has a source. `system.role_attributes` holds a value set on
   a role, and `ATTACH SERVICE LEVEL` is what sets one. It gives the role's
   sessions the timeout and the share of the server that the service level
   names, which is the ScyllaDB form of `ALTER ROLE ... SET`. The row is per
   attribute, because CQL cannot group.

### A secondary index is also a view

ScyllaDB builds a secondary index as a materialized view. `Views` returns the
view that backs the fixture's index, `book_author_index`, beside the view the
fixture creates, so it reports two rows where Cassandra reports one. The
catalog holds the row, and hard rule 13 says to return it. A consumer can tell
the two apart with `Indexes`, which names the index.

### The container needs its settings as arguments

The published image `docker.io/scylladb/scylla` passes every argument that its
entrypoint does not know to `scylla` itself, so nothing is built. The
arguments turn on `PasswordAuthenticator`, `CassandraAuthorizer` and user
defined functions, and hold the server to one shard and one gigabyte.

2026.3 does not create the `cassandra` role that Cassandra creates. So the
superuser is named at startup with `--auth-superuser-name` and a salted
password, which 2025.1 also takes. The pair is `cassandra` and `cassandra`,
the same as the reference product, so one set of test helpers logs in to both.
The entrypoint writes the arguments into a file that a shell reads, so each
dollar sign in the salted password is escaped once.

### What the fixture does differently

Three steps have a ScyllaDB form, and two steps run on ScyllaDB alone:

1. The keyspace uses `NetworkTopologyStrategy` and turns tablets off.
   ScyllaDB refuses `SimpleStrategy`, because it places a new keyspace on
   tablets, and 2025.1 refuses a secondary index and a materialized view on a
   keyspace that uses tablets.
2. The two functions are written in Lua. ScyllaDB runs no Java, and it takes
   Lua and WebAssembly behind an experimental flag.
3. A service level is created and attached to `dbmeta_reader`, so that
   `RoleSettings` has a row. The teardown drops it last.

Twenty one steps run on ScyllaDB and none is skipped. Cassandra runs nineteen
and skips the two service level steps.

### What a second opinion found

Gemini and DeepSeek were both asked to sort the 38 unanswered kinds. Both
called 35 of them absent and named three leads. None of the three is a
source:

| Lead | What the server says |
| --- | --- |
| `Databases` from `system_schema.keyspaces` | a keyspace is already what `Schemas` returns, and one object cannot be both |
| `Languages` from `SELECT DISTINCT language FROM system_schema.functions`, from DeepSeek | refused: `SELECT DISTINCT` works only on partition key columns |
| `RoutineParameters` from `argument_names` and `argument_types` | they are two lists on the function's own row, and CQL cannot unnest a list into one row per parameter, the same as on Cassandra |

`RoleSettings` from `system.role_attributes` was found here rather than
offered, while reading the tables that the system keyspace holds.

### What the conformance test says

ScyllaDB gives the canonical answer that Cassandra gives, line for line, under
`[cassandra]` in `test/testdata/conformance.txt`. The flavor agrees with the
reference product on everything the projection reads.

### Which answers depend on who is asking

A role granted every permission on the fixture keyspace is refused the same
four queries as on Cassandra, from the `system` tables rather than from
`system_auth` and `system_views`, and is refused `RoleSettings` as well. All
four releases give the same answer, under `[scylla/same/grantee]` in
`test/testdata/parity.txt`.

## ClickHouse

ClickHouse answers 23 of the 56, verified against 26.9.2.8 and 25.8.33.6.

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
`ConstraintColumns`. If ClickHouse counts in the cross family agreement
number, the number drops from 23 lines to 14. So ClickHouse is measured
separately for a stated reason, the way Cassandra is.

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

To answer `EnumValues`, a query must parse the enum out of a type string,
which is not a catalog read. `RoutineParameters` is absent because a ClickHouse function is
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
[`dbmeta.Args`](../filter.go) has carried the field all along waiting for it.

### system.jdbc, not information_schema

Trino ships an `information_schema` inside every catalog and a `system`
catalog beside them, and the two differ in reach. A query against
`memory.information_schema.tables` sees the memory catalog and nothing else,
and the catalog cannot come from a bind parameter, so a filter that names a second
catalog returns nothing rather than an answer. That is a wrong answer
rather than an empty one, which rule 13 does not allow. The tables under
`system.jdbc` span every catalog the server has.

`system.jdbc` is also the richer of the two. Its `columns` table carries the
column comment in `remarks`, and `information_schema.columns` has no column
for a comment at all.

### What it answers

13 of the 56. Catalogs as databases, schemas, tables, columns, views,
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
its own catalog. A read of every catalog takes one statement per catalog,
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
no object to build and nothing to verify the query against.

### Which answers depend on who is asking

None of them, and that is the measurement rather than a gap in it. Trino has
no users to create: a client states a principal on every request and the
server takes it, because the image configures no authenticator. With no access
control plugin the server then allows that principal everything, so the only
query that answers differently for a second principal is `current_user`, which
is the one that is supposed to.

## Firebird

`models/firebird` answers 24 of the 56, against Firebird 3.0.14, 4.0.7 and
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
returns an empty schema. A caller that cannot see the server is not able to
tell an invented name from a real one, and an empty result must never
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
exists so that it cannot quietly stop working: without it the result is
a shorter list.

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

Thirty-two kinds have no answer and every one of them is absent from the
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
count, no most common values. A report of it gives a different number under
the same name.

`Languages` is the one analogue left unsupported as a stretch, which rule 14
asks for explicitly. `RDB$FUNCTIONS.RDB$ENGINE_NAME` and the same column on
`RDB$PROCEDURES` name the external engine a routine is written for, so the
engines actually in use are derivable in one statement. That is a list of
languages in use and not a catalog of languages installed, and Firebird has no
catalog of the second. That list misses an unused engine, and a caller cannot
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
shares, so a user that a fixture creates changes a database the test never
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

`models/hana` answers 32 of the 56, against SAP HANA 2.00.088, which is
Tested, and 2.00.076 and 2.00.082, which run nightly. It ties SQL Server for
the richest answer after PostgreSQL and CockroachDB, which shares the
PostgreSQL model.

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

The catalog keeps changing for about a minute after the server first answers.
Measured on 2.00.088 on 2026-09-30, the statistics service made more than 100
views in `_SYS_STATISTICS` in that minute, recorded its installation as done
in `_SYS_STATISTICS.STATISTICS_PROPERTIES`, and made the views of
`_SYS_TELEMETRY` about 12 seconds later. Nothing changed after that. A tier run
once read the tables while this happened and counted 896 views and then 895,
so checkTypes in `test/scan_test.go` reads again once when two counts
disagree, and fails only when the disagreement stays.

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

24 kinds, and every one because HANA has no such object.

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

### Which answers depend on who is asking

Ten queries answer differently for a user that is not the administrator,
which is the most of any product here. That is HANA rather than the model:
almost every SYS view filters itself by what the reader is allowed to see, so a grantee
sees fewer collations, fewer databases, fewer adapters and fewer settings, as
well as fewer roles and grants.

Three of the ten return the same number of rows with different values:
functions, triggers and views. Those carry a definition, and HANA returns the
row and withholds the text from a reader without the privilege. That is worth
knowing before a consumer treats a definition as always present.

Sequences were recorded as a fourth until 2026-09-29, and that was wrong. The
catalog keeps a sequence's numbers as DECIMAL, and the driver hands a DECIMAL
over as a value that holds a pointer, so its text differed on every read.
The query casts the numbers to BIGINT now, and both principals read the same
rows.

## Apache Hive

`models/hive` answers 16 of the 56, against Apache Hive 4.2.1, which is
Tested, and 4.0.1, which runs nightly. It is the
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
absence otherwise looks like a bug.

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

40 kinds, and almost all of them because Hive has no such object.

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

A model that reports that as `Sequences` ships a wrong answer that no test
catches, because the query runs and returns rows. A lead that exists is
more dangerous than one that does not, and running it is the only thing that
separates them. See D43.

### What the fixture cannot build

No index, and that is why Hive is not in the cross family comparison in the
root module. D53 asks every fixture for six core objects and one of them is
an index somebody created. Hive builds five.

No function, because `CREATE FUNCTION` registers a Java class by name and a
fixture that makes one depends on a class being on the server's path.
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

A Hive with SQL standard authorization configured answers differently
and nothing here measures that, because the image does not configure it.

## Presto

Presto is Trino's older self. They forked in 2019 and `models/presto` is a
model of its own rather than a flavor, which D73 decides and measures. Read
the Trino section first: everything there about a catalog being a real level
and about `system.jdbc` spanning catalogs is true here too, and this section
records only where the two differ.

### What it answers

9 of the 56. Catalogs as databases, schemas, tables, columns, views, types,
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
one. `dburl` has since split them, and `GenTrino` generates the Trino DSN.

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

## Exasol

`models/exasol` answers 25 of the 56. It was run against Exasol 2026.2.0 on
the nano image and 2025.2.1 on the Community Edition machine, and the two
answer identically, so the model has no version fragment. D85 says why there
are two, and D87 records what was decided here.

### It reads the EXA_ALL views

Exasol's catalog is a set of system tables in SYS, in three families.
`EXA_USER_` is what the current user owns, `EXA_ALL_` is what it can see, and
`EXA_DBA_` is everything and needs `SELECT ANY DICTIONARY`. The model reads
`EXA_ALL_`, for the reason the Oracle model reads `ALL_`: it answers every
principal with what that principal can see and asks for no special right.

Two queries read `EXA_DBA_`, because nothing else holds the answer.
`RoleGrants` reads `EXA_DBA_ROLE_PRIVS`. The views an ordinary user can read
list only the grants that user holds, which hides every other member's
grants from an administrator too. `UserMappings` reads the connection views,
which carry the remote user and every grant. Both are refused to a lesser
principal, and parity records it.

The system tables are not in the `EXA_ALL_` views, which list what users made.
`with_system` reaches them through `EXA_SYSCAT` and `EXA_SYS_COLUMNS`, for
tables, columns, schemas and the system scripts.

Three things about writing SQL for it, all measured.

An empty string is NULL. Exasol reads `''` as NULL, the way Oracle does, so
`? = ''` is never true and the first version of every filter matched nothing.
A filter tests for NULL instead. A statement that selects `''` for a field
that is always empty gets NULL back, so a plain string field is scanned
through a helper that reads NULL as empty, and three fields where the empty
string means something are restored from the row: identity and generated,
which are empty for an ordinary column, and a foreign key's catalog.

A correlated `EXISTS` in a select list is refused as not supported, so the
primary key flag on a column is a join. A correlated scalar subquery is
accepted, which is how the partition key and a virtual schema's properties are
folded into one row.

`CONNECT BY` runs before `WHERE`. Splitting the `SCRIPT_LANGUAGES` parameter
over `EXA_PARAMETERS` itself multiplied every level by every parameter and
returned 182 rows for four languages, so the parameter is selected first and
the generator runs over that one row.

### What it answers

Tables, schemas, columns, views, indexes, index columns, constraints,
constraint columns, partitioned tables, comments, functions, aggregates,
types, languages, roles, role grants, privileges, settings, the database,
foreign data wrappers, foreign servers, user mappings, foreign tables, the
current schema and the current user.

Five need a sentence.

A virtual schema is how Exasol reaches data it does not hold, and it lines up
with PostgreSQL's foreign data in three places:

| Kind | Exasol |
| --- | --- |
| `ForeignDataWrappers` | an adapter script, `EXA_ALL_SCRIPTS` where `SCRIPT_TYPE` is ADAPTER |
| `ForeignServers` | a virtual schema, with its properties as the options |
| `ForeignTables` | a virtual table, which belongs to its virtual schema |

The fixture builds all three with a Lua adapter that answers with one fixed
table, because Lua runs inside the engine and needs no language container and
no second database.

`UserMappings` is a connection granted to a principal. A connection holds an
address and the credentials to use it, which is what a mapping records: who
can reach a source, and as whom. Its server is the connection, which is not
the virtual schema `ForeignServers` reports, and a virtual schema names the
connection it uses in its `CONNECTION_NAME` property.

`Aggregates` is a set UDF that returns one value. Exasol's own documentation
calls that an aggregate. A set UDF that emits rows is a function returning a
set.

`Indexes` reports the indices the engine made. Exasol has no `CREATE INDEX`:
it builds an index for a key and others as joins need them, and drops the ones
that go unused. Nothing names one, so the object id is the name and `REMARKS`,
the engine's own description such as `GLOBAL INDEX (COUNTRY,AREA)`, is the
comment.

`IndexColumns` reads the column list out of `REMARKS`, which is the only place
it is kept. The names in it are not quoted, and a column can be called `a,b`,
so the list is matched against the table's real columns rather than split on
its commas. A name that is itself two other column names joined by a comma
matches twice. Nothing short of that can be misread.

### What it cannot answer

31 kinds. Most are absent from the product.

There is no sequence, trigger, domain, enumerated type, collation, tablespace,
access method, conversion, cast, large object, event trigger, operator,
operator class or family, extension, extended statistic, publication,
subscription, text search object or default privilege. There is no unique or
check constraint: both are refused as not supported, on both releases. An
identity column is the only generator.

Three are present and unreachable, which is a different thing.

`RoutineParameters` has no answer because Exasol keeps the parameters of a
function or a script only inside its text. Parsing them out was considered and
left alone: a SQL function's text is stored as written, and a type such as
`DECIMAL(18, 0)` puts a comma inside a parameter, so a split is wrong exactly
where it matters. `Function.ArgTypes` and `Function.ResultType` are empty for
the same reason, and the text is in `Function.Definition`.

`ColumnStats` has no answer because Exasol exposes no planner statistic.
`EXA_ALL_COLUMN_SIZES` holds the memory and raw size of each column, which is
storage rather than a statistic about the values.

`RoleSettings` has no answer. A user or a role carries a consumer group, which
limits resources, and that is not a configuration setting a role applies.

### What a second opinion found

Gemini and DeepSeek were both asked about the thirty kinds, given the whole
list of system tables. They agreed on three leads.

| Lead | What the server says |
| --- | --- |
| `UserMappings` from `EXA_DBA_CONNECTIONS` and `EXA_DBA_CONNECTION_PRIVS` | both exist with the columns named. It is the source `UserMappings` reads |
| `IndexColumns` by parsing `REMARKS` | the list is there and unquoted. Splitting it is wrong for a column named `a,b`, so the model matches it against the real columns |
| `RoutineParameters` by parsing the text | the text is there. Rejected, above |

DeepSeek also offered `ColumnStats` computed from the data, with `COUNT(*)`
and `COUNT(DISTINCT)` per column. That is a scan that grows with every row in
the database, which D47 forbids.

Both invented a column. Gemini said `EXA_ALL_COLUMN_SIZES` has
`AVERAGE_COLUMN_SIZE` and `COMPRESSED_SIZE`. It has `RAW_OBJECT_SIZE` and
`MEM_OBJECT_SIZE`. DeepSeek said `EXA_ALL_INDICES` has `ROOT_SCHEMA` and
`ROOT_NAME`. It has `INDEX_SCHEMA` and `INDEX_TABLE`. A query built on either answer does not run.
See D43.

### What the fixture cannot build

No index, because there is no `CREATE INDEX`, so the core object
`book_published` does not exist here and the fixture is not in the root
module's fixture test. No unique or check constraint on `book`. No user,
because a user outlives the schema, so parity makes its own.

A view comment is written in the `COMMENT IS` clause of `CREATE VIEW`, because
`COMMENT ON VIEW` is refused.

### What conformance says

Exasol agrees with the relational databases on 22 of their 23 lines. The one
it lacks is the unique constraint on `book.title`, which cannot be built, so
it is in `agreementExcluded` with that reason. It also reports each NOT NULL
as a named constraint of its own, which is how Exasol keeps them, and a view
column as nullable.

### Which answers depend on who is asking

Measured with SYS, the owner of the fixture schema and a grantee on it.

`RoleGrants` and `UserMappings` are refused to both, as above.

`Privileges` returns fewer rows to the grantee, because `EXA_ALL_OBJ_PRIVS`
lists the grants a user made, received, or owns the object of.

`ForeignServers` returns the same row with its options absent, because a
virtual schema's properties are shown to its owner and not to a user who can
only see the schema.

`CurrentSchema` returns no row for a new session, which has opened no schema.
That is the session rather than the principal.

On 2025.2.1 `ForeignDataWrappers` returns fewer rows as well. The Community
Edition ships eleven adapter scripts in `VS_ADAPTERS`, which SYS owns and a
principal that owns another schema cannot see. The nano image ships none, so
the difference is in what the machine holds and not in the query.
`test/testdata/parity.txt` has an `exasol@2025` section for it.

## Vertica

`models/vertica` answers 26 of the 56 on 25.1 and 24 on the three older
releases, which have no triggers and no per user settings to read. It was run
against 7.2.1, 9.1.0, 10.1.1 and 25.1.0, all four community images, and D88
records why those. The images are copies in `docker.io/usql/vertica`, which
D100 records.

### It reads v_catalog

Vertica began as a fork of PostgreSQL and its catalog is the `v_catalog`
schema, with the engine's running state in `v_monitor`. Every query here reads
those two. Much of it will look familiar to anybody who knows PostgreSQL:
`vsql` is `psql`, names fold to lower case, the empty string and NULL are
different values, and `standard_conforming_strings` decides how a literal is
escaped.

The catalog grows with the product: `v_catalog` has 56 tables on 7.2, 73 on
9.1, 89 on 10.1 and more on 25.1, where namespaces arrived. The base names,
a table's schema and name, are the same on every release, so most queries
need no fragment, and the gates are few:

| From | What |
| --- | --- |
| 9.1 | CHECK constraints, SET USING columns, a function's owner, a user's connection limit |
| 10.1 | `LISTAGG`, and a comment on a table column |
| 25.1 | PL/vSQL, a procedure's language, owner and security, triggers, per user settings |

The 25.1 row is measured present there and absent on 10.1, and nothing
between is measured.

Four things about writing SQL for it, all measured.

A CASE whose branches are literals of different lengths is typed CHAR of the
longest, so `unique` arrived padded to the width of `primary key`. Every CASE
that produces text is cast to VARCHAR.

`CURRENT_SCHEMA()` is allowed only in the outermost select list, not in a
WHERE, a join or a derived table, so CurrentSchema cannot join the schema's
row and leaves the owner and comment to Schemas.

A subquery beside GROUP BY is refused, so the access policies Privileges folds
in are aggregated first and joined.

`vertica-sql-go`, the driver `usql` uses, splits a statement at every
semicolon before it sends it, so a SQL function, whose body needs one, cannot
be created through it at all. `vsql` creates it without complaint.

### What it answers

Tables, schemas, columns, views, indexes, index columns, constraints,
constraint columns, sequences, partitioned tables, comments, functions,
aggregates, types, triggers, roles, role settings, role grants, privileges,
databases, tablespaces, settings, foreign servers, foreign tables, the
current schema and the current user.

Six answer with an analogue:

| Kind | Vertica | Why it is a fair answer |
| --- | --- | --- |
| `Indexes` | projections | Vertica has no index. A projection is a stored, sorted and segmented copy of some or all of a table's columns, and the optimizer chooses between projections the way another database chooses between indexes. `IndexColumns` is the projection's columns, in the order it stores them |
| `Tablespaces` | `storage_locations` | a storage location is a directory on a node that holds data or temporary files, and a label lets a storage policy send a table's data to it |
| `ForeignServers` | `hcatalog_schemata` | an HCatalog schema reaches the tables of a Hive metastore through the HCatalog connector: one source and the settings to reach it |
| `ForeignTables` | external tables | an external table reads its rows from files through the COPY statement it was created with, and names them directly, so there is no server object |
| `RoleSettings` | `user_configuration_parameters` | a user's own value for a parameter, set with ALTER USER ... SET. A role carries none. 25.1 alone |
| `Triggers` | `stored_proc_triggers` | Vertica's own word, and it runs a stored procedure on a schedule rather than on a change to a table, so it names no table. 25.1 alone |

A NOT NULL is not reported as a constraint. Vertica records one per column in
`constraint_columns`, all under the name `C_NOTNULL`, and not in
`table_constraints`, which is the way PostgreSQL treats one too. A key is
recorded whether or not it is enabled, and Vertica checks one only when it
is. An identity column refuses an explicit value, so its identity kind is
always.

Before 10.1 there is no string aggregate, so Privileges returns a row per
object and grantee there, and a row per object from 10.1.

### What it cannot answer

30 kinds. Most are absent from the product: domains, enumerated types, casts,
operators, operator classes and families, collation objects, conversions,
large objects, event triggers, extensions, extended statistics, publications,
subscriptions, text search objects, access methods, user mappings and foreign
data wrappers. HCatalog is one connector rather than a catalog of them.

Four are present in some form and rejected, which is a different thing.

`RoutineParameters` has no answer. A routine's arguments are one comma
separated list of types on its own row, and the named parameters a library
function declares in `user_function_parameters` are `USING PARAMETERS`
options rather than arguments. A report of those as parameters answers a
different question.

`DefaultACLs` has no answer. A schema can make new objects inherit its grants,
and `inherited_privileges` and `inheriting_objects` record that from 10.1, but
that is a flag on the schema rather than a default privilege a principal
sets.

`Languages` has no answer. Nothing lists them. A library records its SDK
version and not the language it was written in.

`ColumnStats` has no answer. `table_statistics` holds row counts, and nothing
exposes a column's null fraction, distinct count or most common values.

### What a second opinion found

Gemini and DeepSeek were both asked about the thirty one kinds, given the
table lists. Gemini's answer was cut off after the first rows, which agreed
with the table above. DeepSeek named three leads.

| Lead | What the server says |
| --- | --- |
| `RoleSettings` from `user_configuration_parameters` | it exists on 25.1 with user_name, parameter_name and current_value. It is the source `RoleSettings` reads |
| `Languages` from a `language` column of `user_libraries` | there is no such column |
| `ColumnStats` from `table_statistics` | it holds row counts, not column statistics |

Running the second one was the only way to tell it was invented. `ForeignServers`
from `hcatalog_schemata` was found here rather than offered, while checking
DeepSeek's answer that foreign servers are absent.

### What the fixture cannot build

No SQL function, because of the driver's semicolon split above. Functions
reads the functions the packages Vertica installs at startup provide, and a
PL/vSQL procedure on 25.1.

No HCatalog schema, because the images carry no HCatalog connector and CREATE
HCATALOG SCHEMA is refused, so `ForeignServers` is verified to run and return
nothing.

On 7.2 and 9.1 the view is created before `book_published`, because a table
with a projection somebody made gets no superprojection until it holds rows,
and a view over a column no projection carries is refused. On 7.2 there is no
CHECK constraint.

### What conformance says

Vertica agrees with the relational databases on all their lines, including
the unique and check constraints on `book`. 7.2 has no CHECK constraint, so
it has a section of its own, `vertica@7`, one line shorter. It is the first
product to need one in `conformance.txt`, and D88 records the change.

### Which answers depend on who is asking

Measured with `dbmeta`, which holds PSEUDOSUPERUSER because `dbadmin` has no
password, the owner of the fixture schema and a grantee on it.

`RoleSettings` is refused to both on 25.1, because a lesser principal cannot
read `user_configuration_parameters`.

`Roles`, `RoleGrants` and `Schemas` return fewer rows to both, because each
view shows a principal what it can reach. The grantee also sees fewer
comments, functions and sequences.

`Settings` returns the same rows with different values, because
`configuration_parameters` hides the values a superuser alone can see.

On 10.1 `Privileges` is refused to both, because a lesser principal cannot read
`access_policy`, which the query joins for its policies. 25.1 serves it. Before
10.1 the query does not read that view.

## Couchbase

`models/couchbase` answers 12 of the 56 on 7.6 and 8.0. It was run against
7.6.12 and 8.0.3 through `github.com/xo/dbimp/couchbase`, the driver `usql`
uses. 7.2.9 reports that it is too old, and D104 says why.

### It reads the system keyspaces

Couchbase has no `information_schema`. Its catalog is the `system` namespace,
which SQL++ reads like any other keyspace. Every query is one statement over
`system:buckets`, `system:all_scopes`, `system:all_keyspaces`,
`system:indexes`, `system:functions`, `system:sequences`, `system:user_info`
or `system:applicable_roles`.

A bucket is the database, a scope is the schema and a collection is the
table. So a table's catalog is its bucket and its schema is its scope. The
default collection of a bucket is reported as the scope `_default` and the
collection `_default`, which is how Couchbase addresses it.

Four things about writing SQL++ for it, all measured:

1. A field that is absent from a document is MISSING, and SQL++ leaves it out
   of the row. It does not write it as null. A driver takes its columns from
   the row, so a row that drops a key drops a column. Every expression that can
   be MISSING is written `IFMISSING(x, NULL)`. `docs/NULLS.md` has the rule.
2. `REGEXP_LIKE` matches the whole string. `REGEXP_CONTAINS` finds a
   substring. The descending flag of an index key uses the second.
3. `REGEXP_REPLACE` has no backreference such as `$1`. An index key loses its
   backticks through `TRIM(x, '`')`.
4. The reserved words differ between releases. `role` is reserved on 7.6 and
   8.0, and 8.0 also reserves `roles`. Both are quoted with backticks.

### What it answers

Databases, schemas, tables, indexes, index columns, functions, routine
parameters, sequences, roles, role grants, privileges and the current user.

Five answer with an analogue:

| Kind | Couchbase | Why it is a fair answer |
| --- | --- | --- |
| `Roles` | users | a Couchbase user is the principal that logs in and holds grants. A Couchbase role, such as `query_select` on a bucket, is a privilege, and `Privileges` returns those |
| `RoleGrants` | groups | a user is a member of groups and has their roles. SQL++ has no catalog of groups, so a group appears only as what a user is a member of |
| `Privileges` | `system:applicable_roles` | one row for each object, with each role held on it. The object is a bucket, a scope or a collection, and nothing for a role on the whole cluster |
| `Functions` | SQL++ user defined functions | a function is inline, with one expression as its body, or JavaScript, whose body is in a library. Its parameters have names and no types |
| `Sequences` | `system:sequences` | 7.6 added sequences. The catalog names a sequence by its scope and loses its bucket, so the bucket is not reported |

A variadic function has no `parameters` field at all, and a function with no
parameters has the empty list. So a missing list is reported as the one
parameter `...`, which is how CREATE FUNCTION writes it.

### What it cannot answer

44 kinds. Most are absent from the product: views, constraints of any kind,
domains, enumerated types, casts, operators, operator classes and families,
collations, conversions, large objects, triggers, event triggers, extensions,
extended statistics, publications, subscriptions, text search objects, access
methods, tablespaces, partitioned tables, foreign data wrappers, foreign
servers, foreign tables, user mappings, default privileges, languages,
aggregates, comments and the current schema.

Five are present in some form and rejected, which is a different thing.

`Columns` has no answer. A document has no fixed shape, so a collection has no
columns. INFER samples documents and guesses a schema from them. That is a
guess about the data and not a fact of the catalog. It is also one statement
per collection, and on 7.6 it cannot be a subquery at all, so it fails the
cost test of D47.

`ColumnStats` has no answer. UPDATE STATISTICS keeps its results in the
internal collection `_system._query` of each bucket, with each histogram as
base64 JSON. A statement names the bucket it reads in FROM, so no one
statement covers every bucket. The records of a dropped collection also stay
behind. `system:dictionary`, which is the view of that data, fails with a
server panic on 7.6.12 and 8.0.3, before and after UPDATE STATISTICS.

`Settings` has no answer. 8.0 adds `system:bucket_info`, which holds the
configuration of each bucket, and `system:aus`, which holds the configuration
of automatic statistics. Neither is the configuration of the server. The
query service keeps its own on a REST endpoint that SQL++ cannot read.

`RoleSettings` has no answer. A user carries its roles and no setting.

`Triggers` has no answer. The Eventing service runs a function when a document
changes, and SQL++ has no catalog of Eventing functions.

### What a second opinion found

Gemini and DeepSeek were both asked about the 43 kinds, with the system
keyspaces named. Both answers were short, and neither gave a lead that the
table above did not already reject. The leads they named:

| Lead | What the server says |
| --- | --- |
| `Columns` from INFER | it works, and it is a sample. On 7.6 it cannot be a subquery, and on 8.0 it takes only a fixed keyspace, so one statement cannot read every collection |
| `ColumnStats` from `system:dictionary` | the keyspace exists and every read of it fails with a server panic |
| `CurrentSchema` from `CURRENT_SCOPE()` | there is no such function on either release |
| `Settings` from `system:settings` | there is no such keyspace |
| `Settings` from `system:bucket_info` | it exists on 8.0 only, and it holds bucket configuration |
| `RoleSettings` from `system:user_info` | it holds the user's roles, which `Privileges` already reads |
| `Views` from `system:views` | there is no such keyspace |

The statistics in `_system._query` were found here, while checking the
`system:dictionary` lead.

### What the fixture builds, and what it cannot build

The fixture builds a scope, `dbmeta_fixture`, in the bucket `dbmeta` that the
`dbrun` setup makes. SQL++ can create a scope and cannot create a bucket. The
scope holds the four core collections, a primary index, `book_published`, an
index with a descending key and an index over an array, an inline function
with two parameters, a variadic function, a sequence, and a grant to the
ordinary user on one collection.

A collection that CREATE COLLECTION made is not visible to the next statement
for a moment, so the test tries such a step again until it succeeds.

It cannot build a group or a user, because SQL++ has no statement for either,
so `RoleGrants` is verified to run and returns no fixture row. It cannot
build a JavaScript function, because the library that holds its body is
managed through REST.

### What the conformance test says

The `couchbase` section holds the four core collections and nothing else.
There is no column catalog and no constraint, so the relational lines have no
Couchbase answer, and `agreementExcluded` says so.

### Which answers depend on who is asking

Couchbase has no containment and no object owner. A user belongs to the
cluster and holds roles, so there is one lesser kind of principal. It is
`dbmeta_user`, which the `dbrun` setup makes with select, insert, update and
delete on the bucket, and `query_system_catalog`.

`Roles`, `RoleGrants` and `Privileges` are refused to the user, because each
reads `system:user_info` or `system:applicable_roles`. The error names a
different role on each release, so 8.0 has a section of its own,
`couchbase@8`.

`Functions` and `RoutineParameters` return no rows to the user.
`system:functions` shows only the functions a user can run or manage, and the
user holds no function role. Measured with a scope function and a global one,
both of which the administrator sees and the user does not.

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
| ScyllaDB 2025.1, 2026.1, 2026.2, 2026.3 | granted role | the same as Cassandra, plus `role_settings` |
| ClickHouse 26.9 | granted user | `constraints`, `databases`, `foreign_servers`, `index_columns`, `indexes`, `privileges`, `role_grants`, `roles`, `tablespaces` |
| MySQL 8.4 | grantee | `foreign_servers`, `functions`, `role_grants`, `roles`, `user_mappings` |
| MariaDB 13.0 | grantee | `aggregates`, `column_stats`, `foreign_servers`, `role_grants`, `roles`, `user_mappings` |
| MariaDB 10.6, 10.11 | grantee | the same, plus `functions` |
| PostgreSQL 10, 11, 12, 13 | both | each has a section of its own, and `subscriptions` is refused on some. One release answers differently, below, says why |
| Firebird 3.0, 4.0, 5.0 | grantee | `roles`, `settings` |
| Apache Hive 4.2 | other principal | none. The image configures no authorization, so every principal is allowed everything |
| SAP HANA 2.0 SPS 08 | grantee | `collations`, `databases`, `foreign_data_wrappers`, `functions`, `privileges`, `role_grants`, `roles`, `settings`, `triggers`, `views` |
| Exasol 2026.2.0 | schema owner | `current_schema`, `foreign_servers`, `role_grants`, `user_mappings` |
| Exasol 2026.2.0 | grantee | `current_schema`, `foreign_servers`, `privileges`, `role_grants`, `user_mappings` |
| Exasol 2025.2.1 | both | the same, plus `foreign_data_wrappers`, because the Community Edition ships adapter scripts that SYS owns |
| Vertica 25.1 | schema owner | `role_grants`, `role_settings`, `roles`, `schemas`, `settings` |
| Vertica 25.1 | grantee | the same, plus `comments`, `functions`, `privileges` and `sequences` |
| Vertica 10.1 | both | the same as 25.1 without `role_settings` and `schemas`, a grantee sees every function, and `privileges` is refused, because a lesser principal cannot read `access_policy` |
| Vertica 7.2, 9.1 | both | the same as 25.1 without `role_settings`, a grantee sees every function, and a grantee sees fewer rows in `privileges` rather than different values |
| Trino 476, 483 | other principal | none. The image configures no authenticator |
| Presto 0.299 | other principal | none, for the same reason |
| Couchbase 7.6, 8.0 | ordinary user | `functions`, `privileges`, `role_grants`, `roles`, `routine_parameters` |

`current_user` and `current_schema` are left out of the table and are in the
file. They answer a question about the connection, so a run where they agree
is the fault.

The releases on Windows machines are measured too. They are Verified rather
than Tested, so `dbrun test sqlserver-2008R2` and its siblings run it rather
than CI. A contained database arrived in SQL Server 2012, so 2008 R2 skips that
scene and says so instead of failing.

Every dialect has a target or a recorded reason for having none, and
`TestEveryDialectIsMeasuredForParity` fails when one has neither. SQLite and
DuckDB have no user. Snowflake and Redshift have not run (D144). Impala and
Vitess run with no authentication, so every user is the same principal.
`parityExempt` in `test/parity_test.go` holds each reason.

### One release answers differently

`test/testdata/parity.txt` has a section per product, and fourteen releases
have one of their own: `postgres@10`, `postgres@11`, `postgres@12`,
`postgres@13`, `mariadb@10`, `exasol@2025`, `vertica@7`, `vertica@9`,
`vertica@10`, `couchbase@8`, `cockroachdb@24.3`, `cockroachdb@26.2`,
`questdb@9.4` and `tidb@8.5`. A section named for a release wins over the shared one, and
a name carrying a minor wins over one carrying only the major.

PostgreSQL restricts a column of `pg_subscription` from an ordinary role. The
`Subscriptions` query reads `subsynccommit` as `synchronous`, so the whole
query is refused where that column is restricted. Measured directly, with a
role granted nothing but LOGIN:

	13.23   SELECT count(*) FROM pg_subscription    ok
	13.23   SELECT count(subsynccommit) ...         permission denied
	14.24   SELECT count(subsynccommit) ...         ok

So an ordinary role is refused on 10, 11, 12 and 13, and served from 14. A
superuser reads it on every release, and `pg_subscription` does not exist
before 10, where `Subscriptions` reports `TooOld` instead.

An earlier version of this section said the grant widened in 13 and that the
same role is served from 13 on. That was wrong by one release and it went
unnoticed because only 12 had a section: 10, 11 and 13 were never measured
until D69 made the nightly matrix read the release list, and all three failed
the first night it did. The four sections now carry the measurement.

PostgreSQL 10 words the refusal differently from the rest, "permission denied
for relation" where 11 and later say "for table", which is why its section
cannot be shared with theirs.

The query is not gated for this. A superuser on 12 can read the column, and
padding it withholds a fact from the caller who is allowed it, which rule
13 forbids. What was wrong was the file claiming one answer covers every
release of a product.

It was found by CI rather than by the cross-release check that was supposed to
catch it. That check compared 9.6 against 18, and 9.6 has no `pg_subscription`
at all, so the query was never asked and the difference never showed.

ClickHouse had a sixth section, `clickhouse@25.3`, and it is gone because it
was never a difference in the product. It recorded that 25.3 did not refuse
`foreign_servers` when every later release did, and the truth was that the
query did not run on 25.3 at all: it read `n.create_query`, which arrived in
25.6, so the server was not able to resolve the identifier and there was no answer
to compare. A query that fails for everybody looks like a query that treats
everybody alike.

With the column gated the query runs on 25.3, a granted user is refused there
exactly as on every later release, and the shared section covers it. D83 has
the measurement.

A section can name a minor, and four do: the CockroachDB, QuestDB and TiDB sections above. The mechanism
was added for this section and it was right for a reason that outlives it:
the first attempt called it `clickhouse@25`, which is what every other
product's section does, and that broke 25.8, because ClickHouse versions by
calendar and 25.3 and 25.8 are both major 25. Any product that versions that
way will need it again. SAP HANA is the nearest, since every one of its
releases is major 2. Nothing moves for PostgreSQL, Oracle or SQL Server,
where the major does identify a release line.

MariaDB 10.6 is the second, and it is `Functions`.
`information_schema.ROUTINES` reports `routine_definition` as NULL to a user
that cannot read the routine's source, and the query selects that column as
`source`, so a grantee gets the same rows with the definition absent. MariaDB
11.3 made `SHOW CREATE ROUTINE` a grantable privilege, and
`GRANT ALL PRIVILEGES` on a database carries it, so 13.0 serves the definition
to the same principal. 10.6 has no such privilege to grant, and only the
definer or a user that can read `mysql.proc` sees it there.

Nothing is gated for this either, and for the same reason: an administrator on
10.6 reads the column, so padding it withholds a fact from a caller who is
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
`ALL_` views under-report. They do, but only for a caller asking about
a schema it has no privilege on, which is a different question, and that
measurement is what closed D60 with no change to the model.

## What every database agrees on

`TestConformance` builds the same core schema everywhere and compares the
canonical answer against `test/testdata/conformance.txt`, which is checked in.
23 of the canonical lines are identical across every section that
`agreementExcluded` in `test/conform_test.go` does not name. The table below
holds the differences between the first five, PostgreSQL, MariaDB, MySQL,
SQLite and DuckDB. Every difference is real and none is a fault.

| Difference | Why |
| --- | --- |
| SQLite reports a primary key column as nullable | An `INTEGER PRIMARY KEY` in SQLite genuinely accepts NULL unless the column is declared NOT NULL. It is the oldest surprise in SQLite and it is not a fault. |
| PostgreSQL reports a default on a primary key | `serial` is implemented as a `nextval` default. `AUTO_INCREMENT` and DuckDB's plain key are not defaults. |
| MariaDB reports the literal `NULL` as the default of a nullable column with no default | MariaDB is saying the column defaults to NULL, which is true. MySQL reports no default, as PostgreSQL, SQLite and DuckDB do. `TestMySQLNullDefault` pins both. |
| MariaDB reports a view's columns as not nullable | The others say nullable. Each is inferring from the view body differently. |
| Of those five, only PostgreSQL and DuckDB report the columns of a check constraint | `KEY_COLUMN_USAGE` does not cover a check, and SQLite publishes nothing about one. |

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

The table has a column for each of the first seven products, and the section of each later model
says which of the seven it answers. The SQL standard defines
`key_column_usage`, `parameters`, `views` and `schemata`, so a database close
to the standard answers those four. That was not the expectation: the two the
consumers wanted most turned out to be the two the standard already had.

`CurrentUser` is answered by every model but SQLite, which has no users, and
Cassandra, which has no CQL expression for it. It is
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
needs to track quoting, because a label can contain a comma or an escaped
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

DuckDB answers 20 of the 56. Its catalog is
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
parameters, collations, settings, extensions, the current schema and the
current user.

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
statistics the planner wrote. DuckDB computes them on demand and stores
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
PostgreSQL never reports a NOT NULL constraint. If DuckDB reports one, the
same schema answers differently by database. `Column.Nullable` carries the fact
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

SQL Server answers 32 of the 56, as many as SAP HANA and fewer than only
PostgreSQL and CockroachDB, which shares the PostgreSQL model. It is the only
one of its own model with roles, privileges, tablespaces and DDL triggers, and
the only one that keeps comments in a catalog of their own rather than on the
object.

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
three faults that no container can find.

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
between them can show it.

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
mappings, foreign tables, column statistics, extended statistics, settings, the
current schema and the current user.

Four are worth naming. `Comments` reads `sys.extended_properties` for the
`MS_Description` property, which is the convention every SQL Server tool uses
and the closest thing the product has to `COMMENT ON`. `ForeignServers` and
`UserMappings` read `sys.servers` and `sys.linked_logins`, because a linked
server is what SQL Server has instead of a foreign server. `Tablespaces` reads
the filegroups, which is a genuine match rather than an analogue: a filegroup
is where a table's pages live and that is what the question asks.

### Visibility rather than refusal

A SQL Server catalog view shows the caller what the caller is allowed to see and returns
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

One thing the model avoids on purpose. `STRING_AGG` arrived in 2017 and is
safe at this floor, and the model uses `STUFF(... FOR XML PATH(''))`
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

Hard rule 14 says to ask several models and then run the answer against a real
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

Oracle answers 26 of the 56. Every one is verified on six releases.

It needs no Windows and no virtual machine, which is the opposite of SQL
Server. The free Express images reach back to 11g Release 2 from 2010, so every
release runs in an ordinary container. See D57 for the SQL Server contrast and
`container/oracle.go` for the list.

### What it answers

Schemas, tables, columns, indexes, index columns, constraints, constraint
columns, sequences, views, the current schema and the current user. Then
comments, triggers, event triggers, functions, aggregates, routine parameters,
types, domains, operators, privileges, column statistics, extended
statistics, partitioned tables, foreign servers and foreign tables.

Verified on 11g, 18c, 19c, 21c, 23ai and 26ai. Every release runs 25 of them
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
connected user what it can see and needs no special role, `DBA_` needs
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

### 11g reads its whole catalog slowly

11g XE reads four queries slowly when the system objects are included:
tables, types, privileges and column_stats. The catalog holds 4,818 tables
with the system objects, and 18c reads the same queries in seconds. The plans
were read on 2026-09-30, on a fresh server with nothing else running (D150).

Two views are the cost. `all_objects` takes 196 seconds to count and
`all_types` 126, alone, while `dba_objects` takes 0.1 and every other view
the queries read takes about a second. Each row of the two views runs a
chain of privilege checks against the fixed tables `X$KZSPR` and `X$KZSRO`,
and an ordinary user with only `CREATE SESSION` waits as long as SYSTEM.
The dictionary statistics in the image date from 2011, and fixed objects
had none, but gathering both changed neither time. tables reads
`all_objects` and types reads `all_types`, so both stay slow on 11g.

The other two were slow because of what they joined, and both are fixed.
privileges looked up the type of each object in `all_objects`. From 12c
`all_tab_privs` names the type itself, so the lookup runs only where it says
UNKNOWN, and 11g, which has no such column, keeps it. column_stats joined
`all_tables` for the row count, which took 304 seconds over every schema. A
lookup for each row takes 41 seconds on 11g, 0.8 on 26ai, and 0.3 for one
schema, where the join took 0.4.

The scan test in the `test` module still reads 11g without the system
objects, because tables and types alone take more than five minutes with
them. Every query then answers, and filtered to the user's schemas the
privilege checks run for few rows.

### The D43 pass, and what it found

Eleven of 55 was too few for a dictionary this rich, so both models were asked
to sort the other 44. They disagreed in a way that proves the rule: DeepSeek
marked `ForeignServers` and `ForeignTables` absent, and Gemini named
`ALL_DB_LINKS` and `ALL_EXTERNAL_TABLES` for them. A database link is exactly
what a foreign server is, and `usql`'s own Oracle reader already queries it.

The pass produced nineteen leads. D43 says to run each against a real server
before believing it, and running them removed five. Fourteen shipped, which
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

`ExtendedStats` from `ALL_STAT_EXTENSIONS` was left out at first, because
the view holds an extension's expression and ExtendedStat had no field for
one. `ExtendedStat.Definition` holds it now (D147), and the lead was measured
again on 2026-09-30 and shipped (D149). An extension is a column group or an
expression that `DBMS_STATS.CREATE_EXTENDED_STATS` makes, and its statistics
are those of a hidden column named for it. The optimizer always counts its
distinct values, which is ndistinct, and a frequency or top frequency
histogram on the hidden column is a list of the most common values, which is
mcv. Oracle has no functional dependency statistic. Whether a histogram
exists depends on how the statistics were gathered, where PostgreSQL declares
the kind when it makes the object. The fixture makes a column group on
author.

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

## CockroachDB

`models/cockroachdb` answers 54 of the 56, on 24.3.36, 26.2.7 and 26.3.2. It
was measured on 2026-09-29, with pgx, which is the driver dburl opens for
`cockroachdb://`. CockroachDB imitates PostgreSQL's catalog, so 47 of its
statements are the postgres model's, shared with `Query.Share`. The version
set's main version is the PostgreSQL release that CockroachDB claims, 13.0.0
on 24.3 and 26.2 and 18.0.0 on 26.3, so a shared statement takes the fragments
of that release. CockroachDB's own release is under the key `cockroachdb`.
See D123.

Seven statements are its own, because the postgres model's call a function or
a type that CockroachDB lacks:

| Query | What differs |
| --- | --- |
| databases | `size` is NULL before 26.3, which has no `pg_database_size` or `pg_size_pretty` |
| tablespaces | `location` and `size` are always NULL. CockroachDB keeps no tablespace on a path, and `pg_tablespace` holds `pg_default` and `pg_global` for compatibility |
| triggers | `definition` is NULL before 26.2, which has no `pg_get_triggerdef`. 24.3 accepts `CREATE TRIGGER` and records nothing in `pg_trigger` or `information_schema.triggers`, so it answers no rows |
| index_columns | `descending` reads bit 1 of `indoption`, because there is no `pg_index_column_has_property` |
| extension_objects | `description` is always NULL, because there is no `pg_describe_object`. `pg_extension` is empty, and `CREATE EXTENSION` does nothing |
| operator_family_operators | the operator is written from `pg_operator`, because there is no `regoperator` type. `pg_amop` is empty |
| extended_stats | `definition` names the columns from `stxkeys`, because there is no `pg_get_statisticsobjdef_columns`. CockroachDB has no statistics on an expression, so the columns are the whole definition (D147) |

`column_stats` is not answered. CockroachDB keeps `pg_stats` empty even after
ANALYZE, so the shared statement says that no column has statistics,
which is false. Its statistics are in `SHOW STATISTICS FOR TABLE`, which names
the table in the statement rather than taking it as a bind parameter.
`system.table_statistics` holds them too, and 26.3 refuses to read it:
"Access to crdb_internal and system is restricted".

`text_search_config_maps` is not answered. CockroachDB has no
`ts_token_type`, which names the kind of token a mapping is for, and it keeps
`pg_ts_config` empty, so there is no configuration to map (D147).

A parameter declared `integer` reads as `bigint`, because CockroachDB makes
`integer` 64 bits. The fixture builds 29 of the PostgreSQL fixture's 36 steps
on 26.3, and 26 on 24.3 and 26.2, which have no domain and cannot comment on
a sequence or a function. It leaves out the revoke, the two partitioning steps,
the publication, the statistics on an expression, the text search
configuration and the collation with rules, and `models/cockroachdb/fixture`
says why. The
conformance report is line for line PostgreSQL's on all three releases.

Parity finds what PostgreSQL finds, with one difference by release. From 26.3
a principal that is not allowed to connect to a database reads "no access" as its size.

Gemini and DeepSeek were asked about each gap, as hard rule 14 requires, on
2026-09-29. Both called tablespace paths, extensions and operator families
absent from CockroachDB. DeepSeek named `crdb_internal.column_statistics`,
`crdb_internal.table_sizes`, `crdb_internal.create_statements` and a flag for
system schemas. None of them exists, or 26.3 refuses to read it, as measured.
Gemini's trigger statement from `information_schema.triggers` has nothing to
read on 24.3.

With `with_system` off, the shared statements hide `pg_*` and
`information_schema`, as `psql` does, and not `crdb_internal`. So `tables`
lists the 110 to 117 virtual tables of `crdb_internal`. No column of
`pg_namespace` or `pg_class` marks a schema that CockroachDB keeps for itself,
and only its name does. That is what `psql` shows on CockroachDB, which hard
rule 2 follows, and D128 keeps it.

## CrateDB

`models/cratedb` answers 26 of the 56 on 6.4.5 and 25 on 6.3.7, which has no
`information_schema.collations`. It was measured on 2026-09-29 with pgx, on
the PostgreSQL port, which is what dburl opens for `cratedb://`. The main
version is the PostgreSQL release that CrateDB claims, 14.0 on both, and
CrateDB's own release is under the key `cratedb`. See D123.

CrateDB speaks PostgreSQL's protocol and keeps a catalog of its own. Its
`pg_catalog` holds part of PostgreSQL's, and many of the functions that the
postgres model calls are absent. So only 3 statements are shared: settings,
role grants and the current user. The other 23 read `information_schema`,
`pg_catalog` and `sys` directly.

### What each answer lacks

| Query | What differs |
| --- | --- |
| schemas, current_schema | `owner` is empty. CrateDB keeps no owner for a schema, and `pg_namespace` says "unknown (OID=0)" |
| tables, views, privileges | a foreign table has the type `foreign table`. There is no comment on anything, because CrateDB has no COMMENT statement |
| columns | a generated column reads `s`, as a stored one does on PostgreSQL. `identity` is always empty. The column of a foreign table has the ordinal -1, which is what CrateDB reports |
| indexes, index_columns | the only index is the one behind each primary key, and CrateDB reports it as not unique. A full text index is in no catalog, only in SHOW CREATE TABLE. `type` is empty, because `pg_am` is empty |
| constraints | `definition` is NULL on 6.3, which has no `pg_get_constraintdef` and keeps a check expression in no catalog. There is no foreign key and no unique constraint on any release |
| constraint_columns | a primary key only. A check constraint has no row in `key_column_usage` |
| partitioned_tables | the strategy is `LIST`, because a partition holds one value of each partition column |
| functions | a JavaScript function, from `information_schema.routines`. `id` is `specific_name`, which holds the argument types. `volatility` is `immutable` for a deterministic function and `volatile` for the rest. `owner` is NULL, and `security` and `parallel` are empty |
| types | every type is built in and in `pg_catalog`, so the query answers only with `with_system` |
| collations | from 6.4. CrateDB has one collation, in `pg_catalog`, so it too answers only with `with_system` |
| roles | from `pg_roles`, because a user who is not a superuser is refused `sys.users` and `sys.roles`. `create_db` and `bypass_rls` are false, because CrateDB has no database to create and no row security |
| privileges | from `sys.privileges`, a grant on a table as `grantee=type/grantor`, with `denied` after a DENY. A grant on a schema or on the cluster is not a privilege on the table and is not shown |
| databases | CrateDB has one database, `crate`. `owner` is empty, and `size` is NULL, because a size is in `sys.shards`, which a user who is not a superuser cannot read |
| foreign_servers, foreign_tables, user_mappings | the options are aggregated from the three `*_options` views. The only wrapper is `jdbc` |
| publications | `truncate` and `via_root` are false, because CrateDB replicates neither |
| subscriptions | the fixture builds none. A subscription connects to another cluster, and the tests start only one |

### What it does not answer

CrateDB has none of these: tablespaces, access methods, conversions, casts,
large objects, event triggers, domains, operators, role settings, default
privileges, extensions and the objects in them, extended statistics,
comments, triggers, sequences, enum values, and the four kinds of operator
class and family. `pg_tablespace`, `pg_am`, `pg_enum`, `pg_description` and
`pg_event_trigger` are there and empty.

These are left unanswered, although CrateDB holds something like them:

- Aggregates. `pg_proc` holds 178 of them, with `prokind` a, and each names a
  `pronamespace` that is in no row of `pg_namespace`. An answer must invent
  the schema.
- Languages and foreign data wrappers. JavaScript and `jdbc` are the only
  ones, and no view lists them. A distinct `routine_body` or
  `foreign_data_wrapper_name` lists only the ones in use.
- Routine parameters. There is no `information_schema.parameters`, and a
  JavaScript function is not in `pg_proc`. Only `specific_name` holds the
  argument types, with no names.
- The five text search kinds. `information_schema.routines` lists analyzers,
  tokenizers, token filters and char filters, which do the work of a text
  search configuration, a parser and a dictionary. An analyzer has no schema,
  and a built-in one records no tokenizer, so a configuration has no
  parser. D131 leaves them out.

### What the fixture builds

`models/cratedb/fixture` builds the five core tables and the view, a
partitioned table with a generated column, a JavaScript function, a role, a
user, a grant and a denial, a publication, a foreign server with a user
mapping and a foreign table, and rows with statistics over them. It builds
every step on both releases. The conformance report has no foreign key, no
unique constraint and no check, for the reasons above, and D53's agreement
count leaves CrateDB out with that reason.

### Which answers depend on who is asking

Parity asks as `crate` and as two users, one with every privilege on the
fixture schema and one with DQL only. Both answer the same way on both
releases. `privileges` is refused, because a user who is not a superuser is
refused the `sys` schema: "Schema 'sys' unknown". `schemas` lists only the
schemas the user has a privilege in. `foreign_servers`, `user_mappings`,
`publications` and `publication_tables` list fewer rows.
`information_schema.role_table_grants` answers for any user, and it lists only
the grants to the user who asks, so it is not a substitute.

### What a second opinion found

DeepSeek and Gemini were asked about each gap, as hard rule 14 requires, on
2026-09-29. Both called the kinds in the first list absent. Both named
`pg_proc` for aggregates, `information_schema.routines` for the text search
kinds, and the distinct wrapper names in `information_schema.foreign_servers`
for foreign data wrappers. Gemini also named `routine_body` for languages.
Each lead was run against 6.4.5, and each is left unanswered for the reason
written above.

## QuestDB

`models/questdb` answers 11 of the 56 on 9.4.3 and 10.0.1. It was measured on
2026-09-29 with pgx, on the PostgreSQL interface on 8812, which is what
dburl opens for `questdb://` and what usql uses.

### Which catalog it reads

QuestDB's own catalog is a set of functions: `tables()`, `views()`,
`materialized_views()`, `functions()` and `table_columns()`, which takes one
table name as a constant. Its `information_schema` and `pg_catalog` imitate
PostgreSQL's for the clients that read them, and hold a part of the same
facts. The model reads whichever answers the question in one statement, and
shares nothing with the postgres model, whose statements read catalogs and
functions QuestDB does not have.

The version is read from `SELECT build()`. `SHOW server_version` and
`version()` answer PostgreSQL 12.3 on both releases, and only `build()` names
QuestDB's.

### What each answer lacks

| Query | What differs |
| --- | --- |
| schemas, current_schema | QuestDB has no schemas. `pg_namespace` holds `public` and `pg_catalog`, and every table is in `public`. `owner` is empty |
| tables | from `tables()`, where a table, a view and a materialized view each have a kind of one letter. QuestDB lists no system table there |
| columns | from `information_schema.columns`, because `table_columns()` reads one table at a time. The type is the PostgreSQL type the wire protocol sends, so a SYMBOL reads character varying and a DECIMAL numeric. QuestDB counts the position from 0, and the model adds 1. Every column is nullable, and none has a default or is a key, because QuestDB has none of the three |
| views | a view from `views()` and a materialized view from `materialized_views()`, with the text each was created with. Neither is updatable or insertable |
| partitioned_tables | a table or a materialized view partitioned by its designated timestamp. The strategy is RANGE, and the expression names the interval and the column, as YEAR (published) |
| databases | `qdb`, the one database. The encoding is UTF8, which is the only one QuestDB stores text in. There is no owner and no size |
| settings | from `(SHOW PARAMETERS)`, which QuestDB lets a query select from. There is no type, and the context says whether a setting takes effect without a restart |
| functions, aggregates | from `functions()`, one row per signature. Every function is built in, because QuestDB has no CREATE FUNCTION, so both answer only with `with_system`. A signature can be both an aggregate and a window function, so the id carries the kind |

### What it does not answer

QuestDB has none of these: tablespaces, access methods, languages,
conversions, casts, collations, large objects, event triggers, domains,
operators, roles, role settings, role grants, privileges, default privileges,
foreign data wrappers, foreign servers, user mappings, foreign tables,
publications, publication tables, subscriptions, the five text search kinds,
the four kinds of operator class and family, extended statistics, extension
objects, comments, constraints,
constraint columns, triggers, sequences, enum values and column statistics.
The open source edition has no roles or grants, and `pg_roles` is empty.

These are left unanswered, although QuestDB holds something like them:

- Indexes and index columns. A SYMBOL column can have an index, and only
  `table_columns()` says so, one table at a time. `pg_index` is empty, and
  `table_columns()` refuses a column as its argument, so no one statement
  lists them.
- Types. `pg_type` holds the 17 PostgreSQL types the wire protocol maps
  QuestDB's to, and not QuestDB's own, such as SYMBOL and GEOHASH.
- Extensions. `pg_extension` holds one row, which is QuestDB itself.
- Routine parameters. A built in function's arguments are one text in
  `functions()`, with no names. `split_part` takes only a constant index, so
  one statement cannot split them into rows.
- A deduplication key, which DEDUP UPSERT KEYS declares and which keeps rows
  unique by it. It is not a constraint, and only `table_columns()` names its
  columns, one table at a time.

### What the fixture builds

`models/questdb/fixture` builds the five core tables and the view, with book
partitioned by year on its designated timestamp as a WAL table, a
materialized view on book, and an indexed SYMBOL column on author. It builds
every step on both releases. The conformance report has every column
nullable, none a key and no constraint line, and D53's agreement count leaves
QuestDB out with that reason.

### Which answers depend on who is asking

Parity asks as `admin` and as the user of the PostgreSQL interface that can
only read. Every query answers the same way, except that 10.0.1 reports the
reader's own name as the current user. 9.4.3 reports `admin` for the reader
too, which is the one difference between the releases, and it has a parity
section of its own.

### What a second opinion found

Gemini Pro and DeepSeek were asked about each gap, as hard rule 14 requires,
on 2026-09-29. Both called the kinds in the first list absent. Both named
`pg_type` for types and `pg_extension` for extensions, which hold something
else, as above. Gemini named `table_columns()` for indexes, index columns,
constraints and constraint columns, which reads one table at a time, and
string parsing of `functions()` for routine parameters, which one statement
cannot do. Each lead was run against 10.0.1.

## TiDB

`models/tidb` answers 19 of the 56 on 8.5.8, and 18 on 7.5.8 and 8.1.2,
where privileges is too old. It was measured on 2026-09-29 with the mysql
driver, which is what dburl opens for `tidb://` and what usql uses. TiDB
imitates MySQL's information_schema, so 16 of its statements are the mysql
model's, shared with `Query.Share`. The version set's main version is the
MySQL release TiDB claims, 8.0.11 on every release, set under the `mysql`
key too, and TiDB's own release is under the key `tidb`. See D133.

### What differs from MySQL

| Query | What differs |
| --- | --- |
| every query that filters by schema | TiDB spells INFORMATION_SCHEMA and PERFORMANCE_SCHEMA in capitals, compares a schema name with its case, and has METRICS_SCHEMA of its own, with 637 tables. The mysql model's filter carries an alternative for TiDB that names all five as TiDB spells them |
| roles | `conn_limit` is 0 before 8.5, where `mysql.user` has no `max_user_connections` and TiDB has no limit per user |
| settings | from `information_schema.variables_info`, TiDB's own, because TiDB has no `performance_schema.global_variables`. The context is the scope, such as session,global |
| sequences | from `information_schema.sequences`, TiDB's own. A TiDB sequence is always a bigint |
| privileges | from 8.5. Before it `information_schema.TABLE_PRIVILEGES` is empty although `mysql.tables_priv` holds the grant, so the query is too old there |
| constraints | a CHECK constraint is not listed. TiDB accepts one and enforces it only with `tidb_enable_check_constraint`, and the mysql model lists a check only from MySQL 8.0.16, which TiDB does not claim |

### What it does not answer

TiDB has no stored function, procedure or trigger, and no foreign server, so
functions, aggregates, routine parameters, triggers, foreign servers, user
mappings and foreign tables are not answered. `information_schema.ENGINES`
lists only InnoDB, which TiDB writes for compatibility, so access methods is
not answered either, and neither is extensions. The rest are absent, as they
are on MySQL.

Two are left unanswered although TiDB holds something like them:

- Column statistics. `mysql.stats_histograms` holds a distinct count and a
  null count for each column, keyed by a column id that no catalog view
  names. A match by position is wrong after a column is dropped, and
  only `SHOW STATS_HISTOGRAMS` gives the names, which a statement cannot read.
- Extended statistics. `mysql.stats_extended` exists and is empty, because
  extended statistics are experimental and off by default.

### What the fixture builds

`models/tidb/fixture` is the MySQL fixture less the function, the two
procedures, the trigger and the foreign server, with a sequence, a role, a
user and the grants between them added. It builds every step on the three
releases. The conformance report is line for line what MySQL reports, less
the check constraint, which TiDB does not list.

### Which answers depend on who is asking

Parity asks as root, as a user with every privilege on the fixture schema,
and as one that can only read it. Both lesser users are refused roles and
role grants, which read the mysql schema, as they are on MySQL, and see fewer
schemas, databases and privileges. 8.5 refuses a column of `mysql.user` and
`mysql.role_edges`, and 7.5 and 8.1 refuse the whole table, so 8.5 has a
parity section of its own. The current schema differs because the DSN of root
names the database dbmeta, which the lesser users are not allowed to use, so they connect
to the fixture schema.

### What a second opinion found

Gemini and DeepSeek were asked about each gap, as hard rule 14 requires, on
2026-09-29. Both called functions, triggers, routine parameters, foreign
tables and extensions absent, and named `mysql.stats_extended` for extended
statistics and `mysql.stats_histograms` for column statistics, which are
above. Both called enum values derivable from `COLUMN_TYPE`, which the mysql
model does not answer on MySQL either. DeepSeek named `ENGINES` for access
methods, which is a list kept for compatibility.

## Vitess

`models/vitess` answers 20 of the 56 on 23.0.6 and 24.0.3. It was measured on
2026-09-30 on vttestserver, with the mysql driver, which is what dburl opens
for `vitess://` and what usql uses. vtgate passes a query of
information_schema to the MySQL of one tablet, so 19 of its statements are
the mysql model's, shared with `Query.Share`. The version set's main version
is the MySQL release Vitess claims, 8.4.6 on both releases, set under the
`mysql` key too. Vitess's own release is under the key `vitess`, read from
`@@version_comment`. See D135.

### A schema is a keyspace

A keyspace is stored in one MySQL database for each shard, named
`vt_<keyspace>_<shard>`. information_schema names that database and never the
keyspace, and vtgate refuses that name in a query: a SELECT from
`vt_dbmeta_q_0.t` failed with VT05003, unknown database, and a SELECT from
`dbmeta_q.t` succeeded. So every statement reports the keyspace, which
`mysql.Keyspace` reads from the name of the database, and a filter matches the
keyspace. The current schema is the keyspace too. A test queries every table
of the fixture under the name the model reports. See D135.

### What differs from MySQL

| Query | What differs |
| --- | --- |
| every query that filters by schema | `_vt` holds the state of the tablet. The mysql model's filter carries an alternative for Vitess that adds it to the system schemas |
| every query that names a schema | the keyspace, read from the name of the shard's database, which is what information_schema holds |
| sequences | Vitess's own. A sequence is a table with the comment `vitess_sequence` and the columns id, next_id and cache, which one statement over information_schema lists. The start, the bounds and whether it cycles are not recorded, so they are NULL. The increment is 1, measured with NEXT VALUE. The VSchema names the column a sequence fills, and information_schema does not, so the owner is empty |
| settings, access methods and extensions | the variables, engines and plugins of the tablet's MySQL, which is the MySQL that stores the rows |

### What it does not answer

Roles, role grants and privileges are not answered, although the mysql
model's statements run. They list the accounts of the tablet's MySQL, such as
`vt_dba`, `vt_app` and `vt_repl`, which Vitess uses itself and no client of
vtgate logs in as. vttestserver starts vtcombo with no authentication, so a
Vitess user cannot be measured either. vtgate refuses CREATE TRIGGER,
CREATE FUNCTION and CREATE SERVER, so triggers, aggregates, foreign servers,
user mappings and foreign tables are not answered. The rest are absent, as
they are on MySQL.

### What the fixture builds

`models/vitess/fixture` is the MySQL fixture less the function, the trigger
and the foreign server, with a sequence added. Both procedures are built. It
builds every step on both releases. The conformance report is line for line
what MySQL reports.

### Which answers depend on who is asking

Nothing. vttestserver starts vtcombo with no authentication and no table
rules, so every user is the same principal, and Vitess is exempt from parity
with that reason.

### What a second opinion found

Gemini was asked about each gap, as hard rule 14 requires, on 2026-09-30, and
Gemini Pro about the ones it named. DeepSeek returned nothing twice, because
its reasoning used every token it was given. Gemini named a table with the
comment `vitess_sequence` for sequences, which is above.
`information_schema.INNODB_TABLESPACES` for tablespaces,
`COLUMN_STATISTICS` for column statistics, `mysql.func` for aggregates and
`COLUMN_TYPE` for enum values are what the mysql model leaves unanswered on
MySQL, for the reasons in the MySQL section. It named
`information_schema.TRIGGERS`, which vtgate passes and which is empty,
because vtgate refuses to create a trigger. It named `mysql.user`,
`mysql.role_edges` and the privilege views for roles and privileges, which
are the tablet's accounts, above. It named `_vt.vreplication` for
subscriptions and said that vtgate does not pass the SELECT.

## Databend

`models/databend` answers 20 of the 56 on 1.2.881 and 1.2.948. It was
measured on 2026-09-30 with dbimp's driver, which is what usql's databend
scheme opens. It reads the system database, as the ClickHouse model does,
and the table function show_sequences(). See D140.

### Why not information_schema

Databend's information_schema reports an ordinal position of 1 for every
column, on both releases, and has no view of indexes, constraints,
functions, sequences or roles. system has all of them. system.columns has no
position either, so the model numbers a table's columns in the order
system.columns lists them, which is the order they were declared in. The
fixture test checks that order.

### What differs

| Query | What differs |
| --- | --- |
| schemas and databases | a database is the only namespace, so both read system.databases. A database Databend made has no owner |
| tables | the type is read from the engine: table for FUSE, view, materialized view, system table, and the engine's name and table for any other |
| columns | no primary key, identity or generated column is reported. Databend has no primary key, and system.columns does not say that a column is AUTOINCREMENT or computed |
| indexes and index columns | an inverted, ngram or vector index belongs to a table and enforces nothing. The columns are read from the definition, such as book(a, b). An aggregating index belongs to a query and has no columns |
| constraints and constraint columns | a CHECK is the only constraint, and system.constraints lists its columns |
| functions and aggregates | a function or procedure a user made belongs to no database, so its schema is empty. The functions built into the server are listed with the system objects, in the schema system |
| sequences | from show_sequences(), because system has no table of sequences. A sequence belongs to no database. The start, the bounds and the type are not recorded |
| roles and role grants | a user and a role are both listed. A user or a role that holds account_admin is a superuser. The roles granted to each are one list joined by commas, which the role grants query splits |

### What it does not answer

Privileges are not answered, because show_grants lists the grants of one
role or user at a time, and a statement cannot call it once for each. Routine
parameters are not answered, because a procedure records its arguments as
one text, such as addup(Int32,Int32) RETURN (Int32), and a function records
them as a variant. Databend has no trigger, type, domain, collation,
extension or partitioned table, and the rest are PostgreSQL's alone.

A computed column needs an Enterprise licence on 1.2.881. 1.2.948 accepts
one, and system.columns does not mark it as computed on either release.

### What the fixture builds

`models/databend/fixture` builds the core tables, with no key, a CHECK on
book.title, an inverted and an ngram index, a view, a function, a procedure,
a sequence, two roles and a user with the grants between them, and rows in
author with statistics taken by ANALYZE TABLE. The function, the procedure,
the sequence, the roles and the user belong to no database, so the teardown
drops each by name.

### Which answers depend on who is asking

Parity asks as root, as a user whose role holds every privilege on the
fixture's database, and as one whose role can only read it. Both lesser users
are refused system.settings, system.engines, system.statistics and
system.constraints, so settings, access methods, column statistics,
constraints and constraint columns are refused to them, and they see fewer
schemas and databases.

### What a second opinion found

Gemini and Gemini Pro were asked about each gap, as hard rule 14 requires, on
2026-09-30. Both named the column list of system.constraints for constraint
columns and the definition of system.indexes for index columns, which are
above. The rest are stretches and are left out: a stage is a place files are
read from, not a tablespace, a catalog is a source of tables rather than a
server with options, a task runs on a schedule and not on a change, and a
cluster key orders the rows of a table rather than partitioning it.

## SingleStore

`models/singlestore` answers 23 of the 56 on 9.0 and 9.1. It was measured on
2026-09-30 on the development image, which runs with no licence on a machine
with at most 8 cores and 64 GB, through the mysql driver, which is what
dburl's memsql scheme opens and what usql uses. SingleStore imitates MySQL's
information_schema, so 16 of its statements are the mysql model's, shared
with `Query.Share`. The version set's main version is the MySQL release
SingleStore claims, 5.7.32 on both releases, set under the `mysql` key too,
and SingleStore's own release is under the key `memsql`, read from
`@@memsql_version`. See D141.

### What differs from MySQL

| Query | What differs |
| --- | --- |
| every query that filters by schema | memsql and cluster are SingleStore's own schemas. The mysql model's filter carries an alternative for SingleStore that names them |
| constraints | SingleStore reports the primary key of a columnstore table as UNIQUE, with the name PRIMARY. The mysql model's statement carries an alternative that reports it as the primary key |
| index columns | a shard key is listed as an index of the type SHARD, and a key that is also the shard key is listed twice. The mysql model's statement carries an alternative that leaves out the second row |
| settings | from GLOBAL_VARIABLES, because SingleStore has no performance_schema. The context is whether a variable can be set while the server runs |
| roles | from USERS, ROLES and GROUPS, because SingleStore has no mysql database. A user can log in, and a role and a group cannot. A group's member_of is the roles it holds |
| role grants | from GROUPS_ROLES, a role granted to a group. No view says which groups a user is in |
| privileges | from ROLE_PRIVILEGES, because TABLE_PRIVILEGES and SCHEMA_PRIVILEGES are empty although a role holds a grant. A grant to a user is in no view |
| aggregates | from AGGREGATE_FUNCTIONS, a user defined aggregate and the four functions that make it |
| column statistics | from OPTIMIZER_STATISTICS, which ANALYZE TABLE fills. The bounds are only inside the histogram |
| extended statistics | from CORRELATED_COLUMN_STATISTICS, which `ANALYZE TABLE ... CORRELATE COLUMN` fills. A correlation says how strongly one column follows another, which is PostgreSQL's functional dependency, so the kind is f. It has no name (D149) |

### What it does not answer

SingleStore refuses a foreign key, a trigger, a foreign server and PARTITION
BY, and has no sequence, so those queries are not answered. PLUGINS is
empty, so extensions is not answered. The rest are absent, as they are on
MySQL.

### What the fixture builds

`models/singlestore/fixture` builds the core tables in SingleStore's own
syntax, with no foreign key. book is a reference table, because a unique key
of a sharded table must hold the shard key, and book_title_unique is on title
alone. It adds a function, a procedure, an aggregate and the four functions
behind it, a role granted to a group and the group to a user, a privilege,
rows with statistics, and a correlation between two columns of author.
`CORRELATE COLUMN` refuses to run with no database selected, even on a
qualified table, so the fixture selects one with USE, and the test runs the
setup on one connection and discards it afterwards.

### Which answers depend on who is asking

Parity asks as root, as a user with every privilege on the fixture schema,
and as one that can only read it, as for MySQL. Both lesser users see fewer
roles, role grants and privileges.

### What a second opinion found

Gemini and Gemini Pro were asked about each gap, as hard rule 14 requires, on
2026-09-30. Every lead was a stretch or a view that exists for MySQL's
clients and is empty: LINKS as foreign servers, EXTERNAL_TABLES as foreign
tables, PIPELINES as subscriptions, RESOURCE_POOLS as role settings,
DISTRIBUTED_PARTITIONS as partitioned tables, and an AUTO_INCREMENT column as
a sequence. TRIGGERS and TABLESPACES are empty. Both named the histograms for
extended statistics, and the histograms are over one column each. The view
of correlations covers two, and D149 shipped it.

## Snowflake and Amazon Redshift

`models/snowflake` answers 13 of the 56 and `models/redshift` answers 11.
Neither has run. Ken chose on 2026-09-30 to write both from the vendors'
documentation before an account or a cluster was provisioned, and hard rule
9 says a query that has never run is not finished. Their tests skip until
dbrun resolves a connection string for each (D117), and running them is what
finishes the models. See D144.

Snowflake reads the INFORMATION_SCHEMA of the database the connection is in.
It has no KEY_COLUMN_USAGE, so no column is reported as a key, and no
constraint's columns are listed. It has no index. The fold is upper case, as
Snowflake documents.

Redshift reads the pg_catalog tables that PostgreSQL 8.0 already had, from
which Redshift was built, because the postgres model's statements need 9.6.
Its roles, grants, and the collation of a column are in SVV views, which the
model does not read yet.

## Apache Impala

`models/impala` answers 11 of the 56 on 4.4.1 and 4.5.2, measured on
2026-09-30 in the one container dbrun builds (D145), with sclgo/impala-go,
which is what usql uses.

### A walk of SHOW statements

Impala has no catalog a SELECT can read. Schemas, databases, tables, views,
columns, column statistics, functions, aggregates and settings are each a
walk (D146): SHOW DATABASES, and then SHOW TABLES IN and SHOW VIEWS IN each
database, DESCRIBE FORMATTED each table for its type and comment, DESCRIBE
and SHOW COLUMN STATS each table for its columns, SHOW CREATE VIEW each view,
and SHOW FUNCTIONS IN and SHOW AGGREGATE FUNCTIONS IN each database. The metastore makes a new table
external, so a table the fixture makes reads as an external table. A walk costs one statement for
each database it reaches, and one for each table where it goes deeper. The
caller's patterns are matched in Go, because a SHOW statement takes none.
The current schema and the current user are one SELECT each.

### What it does not answer

A primary key and a foreign key are information Impala keeps and no SHOW
statement lists, so constraints and constraint columns are not answered.
Impala has no index, trigger or sequence, and authorization is off in the
image, so roles and privileges are not answered. A function of a user's own
needs a library file, and SHOW FUNCTIONS records no parameter names, so
routine parameters are not answered.

### Which answers depend on who is asking

Nothing. The image configures no authentication, so every user is the same
principal, and Impala is exempt from parity with that reason.

## rqlite

`models/rqlite` answers 14 of the 56 on 9.4.5 and 10.3.6, measured on
2026-09-30 through the `database/sql` driver of `github.com/rqlite/gorqlite`.
dbimp has no rqlite driver yet, and the tests move to it when it exists
(D148). rqlite runs SQLite 3.53 behind an HTTP API, so every statement is the
sqlite3 model's, shared with `Query.Share`. It answers what SQLite answers,
and the SQLite section above says why the rest is not answered. Every check
the SQLite tests make runs on rqlite too, and the conformance report is line
for line SQLite's.

gorqlite decodes every number as a `float64`, and `database/sql` does not
read a `float64` into a `bool`. So the sqlite3 model scans its boolean
fields through `dbmeta.NumberAsBool`, which reads a bool or a number.

### What rqlite adds, and why none of it is an answer

rqlite reads its users and their permissions from the file that `-auth`
names, and no SQL statement reaches that file. Its SQLite has the user
authentication extension compiled in: `auth_enabled()`, `auth_user_add()`,
`auth_user_change()`, `auth_user_delete()` and `authenticate()` are in
`pragma_function_list`. `auth_enabled()` returns 0 and there is no
`sqlite_user` table, so the extension holds no user. `Roles`, `Privileges`
and `CurrentUser` stay unanswered.

The version is the release of SQLite that the server runs. No SQL statement
names the rqlite release, and only the HTTP API reports it.

Gemini and DeepSeek were asked about the 42 unanswered kinds on 2026-09-30,
as hard rule 14 requires. Both said rqlite adds nothing that SQL can read.
Their SQLite leads were `sqlite_sequence`, `sqlite_stat1`, `sqlite_stat4`,
`pragma_module_list`, and the type `a` of `pragma_function_list`. The SQLite
section rejects each of them but `sqlite_stat4`, and rqlite's SQLite is built
without `SQLITE_ENABLE_STAT4`, so it has no `sqlite_stat4` to read. Its
compile options name `ENABLE_DBSTAT_VTAB`, and `dbstat` reports the pages of
each table and index, which is a size and not one of the 56.

### Parity

The entry declares `dbmeta_user`, who can query and execute and nothing else.
rqlite has no grant on a table, so the user reads every answer the
administrator reads, and the section in `test/testdata/parity.txt` is empty.

## Releases that need a licence file

Stardog, GraphDB and Volt Active Data do not start without a licence file
that a person downloads, and `dbrun` lists them only while it finds the file.
dbmeta has no model for any of them, so every release of the three is Staged,
and CI never runs one. See D118 and D119.

| Product | Releases | Measured |
| --- | --- | --- |
| Stardog | 12.0.4, 12.1.4 | Not yet. No licence file is provisioned |
| GraphDB | 11.4.3, 11.5.1 | Not yet. No licence file is provisioned |
| Volt Active Data | 14.1.0, 15.2.0 | Not yet. No licence file is provisioned |

Put each file at `$XDG_CONFIG_HOME/dbmeta/licenses/<product>`, where the
product is `stardog`, `graphdb` or `voltdb`, or name its path in
`DBMETA_<PRODUCT>_LICENSE`. `docs/DBRUN.md` says the same under Licence files.
