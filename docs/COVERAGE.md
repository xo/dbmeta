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
`test/cassandra/Containerfile`. Run `test/cassandra/build.sh` before the tests,
and CI builds its own from the same file.

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
| PostgreSQL 18 | schema owner | `settings`, `tablespaces` |
| PostgreSQL 18 | grantee | `settings`, `tablespaces` |
| Cassandra 5.0 | granted role | `privileges`, `role_grants`, `roles`, `settings` |
| MySQL 8.4 | grantee | `foreign_servers`, `functions`, `role_grants`, `roles`, `user_mappings` |
| MariaDB 13.0 | grantee | `aggregates`, `column_stats`, `foreign_servers`, `role_grants`, `roles`, `user_mappings` |

`current_user` and `current_schema` are left out of the table and are in the
file. They answer a question about the connection, so a run where they agreed
would be the fault.

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
a schema it has no privilege on, which is a different question.

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

`Tablespaces` is in `USER_TABLESPACES` and `DBA_TABLESPACES` only, so it waits
on D60. `Collations` was said to have arrived in 12.2, and it did not: 26ai has
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
`SELECT_CATALOG_ROLE`. D60 sets the rule that allows it and neither query is
written yet.

`Databases` is the same shape of problem. Oracle has one database per
instance, and the nearest list is the pluggable databases, which no `ALL_`
view carries: `ALL_PDBS` does not exist on any release here. `DBA_PDBS` and
`V$DATABASE` do, and both are behind D60.
