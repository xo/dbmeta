# What Each Database Can Answer

`dbmeta` asks every database the same 48 questions. PostgreSQL answers all of
them, because PostgreSQL is the model. No other database answers all of them,
and this document says which ones each database answers, which ones it cannot,
and why.

A question that a database cannot answer returns `dbmeta.ErrNotSupported`. It
never returns an empty result. Those are different facts and a caller has to be
able to tell them apart: an empty result means the server holds none of that
object, and `ErrNotSupported` means the server has no such object at all.

Read `COMMANDS.md` for the `psql` command that each Go value answers. Read
`PLAN.md` D43 for the rule about finding these analogues in the first place.

## The count

| Model | Answers | Of | Tested against |
| --- | --- | --- | --- |
| `models/postgres` | 48 | 48 | PostgreSQL 9.6 through 18 |
| `models/mysql` | 23 | 48 | MariaDB 11.8.9 and 10.6.28 |
| `models/informationschema` | 7 | 48 | any database with a standard `information_schema` |

The shared `information_schema` model answers seven: tables, schemas, columns,
functions, privileges, constraints and sequences. It is the floor. A native
model exists to beat it, and `models/mysql` beats it by sixteen.

## MariaDB

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
