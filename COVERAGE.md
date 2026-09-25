# What Each Database Can Answer

`dbmeta` asks every database the same 54 questions. PostgreSQL answers all of
them, because PostgreSQL is the model. No other database answers all of them,
and this document says which ones each database answers, which ones it cannot,
and why.

Forty eight of the questions come from `psql`. The other six exist because a
consumer needs them and `psql` has no command for them: the columns of a
constraint, the parameters of a routine, the labels of an enumerated type, the
statement a view selects, the statistics over a column, and the schema an
unqualified name resolves in. D47 sets the rule for adding one.

A question that a database cannot answer returns `dbmeta.ErrNotSupported`. It
never returns an empty result. Those are different facts and a caller has to be
able to tell them apart: an empty result means the server holds none of that
object, and `ErrNotSupported` means the server has no such object at all.

Read `COMMANDS.md` for the `psql` command that each Go value answers. Read
`PLAN.md` D43 for the rule about finding these analogues in the first place.

## The count

| Model | Answers | Of | Tested against |
| --- | --- | --- | --- |
| `models/postgres` | 54 | 54 | PostgreSQL 9.6 through 18 |
| `models/mysql` | 28 on MariaDB, 25 on MySQL | 54 | MariaDB 10.6 to 13.0, MySQL 8.4 to 26.7 |
| `models/sqlite3` | 14 | 54 | both drivers: mattn/go-sqlite3 and modernc.org/sqlite |
| `models/informationschema` | 11 | 54 | any database with a standard `information_schema` |

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

MariaDB answers 28 of the 54 and MySQL answers 25. The three MySQL cannot
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

Those two are named in the test. Any other difference fails it, so a query
written for one product and run against the other is caught here rather than by
a user.

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

SQLite answers 14 of the 54. It is the smallest native model here and it still
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

## The six kinds psql has no command for

These exist because a consumer measured in D46 needs them. D47 allows them:
`psql` sets the object model and does not set the column set, and `psql`
renders a constraint and a routine signature as text because a person is
reading them.

| Question | PostgreSQL | MariaDB | MySQL | SQLite | information_schema |
| --- | --- | --- | --- | --- | --- |
| `ConstraintColumns` | yes | yes | yes | yes | yes |
| `RoutineParameters` | yes | yes | yes | no | yes |
| `Views` | yes | yes | yes | yes | yes |
| `CurrentSchema` | yes | yes | yes | yes | yes |
| `EnumValues` | yes | no | no | no | no |
| `ColumnStats` | yes | yes | no | no | no |

Four of the six are answered by every model, because the SQL standard defines
`key_column_usage`, `parameters`, `views` and `schemata` and every database
here has them. That was not the expectation: the two the consumers wanted most
turned out to be the two the standard already had.

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
