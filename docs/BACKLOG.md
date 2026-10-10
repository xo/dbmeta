# Backlog

This document lists work that is known and not done. Each item names the
decision or the measurement that found it.

A decision is not a backlog item. It goes in [`decisions/`](decisions/README.md).
A question for Ken goes at the end of [`PLAN.md`](PLAN.md), under Open questions
for Ken. When an item here is done, delete it, and record in `decisions/`
anything that was decided on the way (D110).

## Servers

### Find why the Hive setup fails on a slow machine

The Hive setup fails on a GitHub runner, soon after HiveServer2 first answers,
with a SerDeException: "proto.class has to be set". It comes from one of the
`PROTO_` tables at the end of the schema script, which dbmeta does not read.
It never failed on a development machine. D105 runs the setup again after a
failure, and D107 made the script safe to run twice, so a retry can get past
it. The cause is not known.

### Read the log when Oracle 21c does not answer

`oracle-21c` did not answer in five minutes on one CI run, and it answers in
under a minute on every other run. The cause was not recorded. D105 now
prints the last 20 lines of the container's log when a server never answers.
When it happens again, read that log and decide whether the entry needs a
change.

### Find why oracle-26ai and QuestDB failed in CI

On 2026-10-08 the first CI run on 90d0261 failed two jobs, and both passed when
rerun, so neither touches that change. `oracle-26ai` failed in the image's
setup, when it reset the SYSTEM password, with ORA-65048 on `ALTER USER SYSTEM`.
It failed again on 76f266b the same day, at the same statement, with ORA-04021,
a timeout while it waited to lock an object. So the setup of the 26ai image
races a job that the database starts itself, and two runs in about thirty
lost. `questdb-9.4.3` failed once, in `TestQuestDBFixtureObjects` at
`questdb_test.go:216`, with the view `recent` listed and the view `book`
missing. The cause of neither is known. For Oracle, read what the image runs
at start and whether `dbrun` can wait for it, or start the server again when
that statement fails.

`questdb-9.4.3` and `questdb-10.0.1` each failed once on 2026-10-10, in
`TestPrivilegeParity` for the read-only principal: the `tables` query read "the
same rows with different values". The cause was `Table.Rows` (D207), which reads
`table_row_count`: a WAL table applies its writes later, so the count differed
between two reads, and the setup waited only for the table `book`. The setup now
waits until `wal_tables()` shows no table with `sequencerTxn` above `writerTxn`.
If the failure returns, read the log first.

### Find why Databend's first start can fail

The first start of `databend-1.2.948` after the pull of the image did
not answer. The image's bootstrap script starts the query server one second
after the metadata server, and the metadata server was still waiting to become
the leader, so the query server gave up with "cannot connect to
http://0.0.0.0:9191". It did not happen again in four fresh starts (D112). If
it comes back in CI, the entry can start the two servers itself and wait for
the metadata server between them.

### Move the ceilings that move

Three ceilings move faster than the others. Databend publishes a weekly
release almost every day, ArangoDB's release is the newest 3.12 patch, and
Neo4j's is the newest monthly release (D106, D112). When a newer release is
published, replace it in the file in `container/` and in the row in
`EVALUATION.md`.

### Consider a SQL layer for what a product does not answer

This is not a priority (D166). Apache Pinot keeps its catalog only in the
Controller's REST API, and its Broker's SQL has no information_schema, no
SHOW and no DESCRIBE, measured on 1.4 and 1.5 on 2026-10-01. InfluxDB names
its release only in `GET /ping`. A generic SQL layer, in a driver or beside
one, can answer such statements, for example SHOW TABLES, DESCRIBE and
`SELECT version()`, from what the product does expose. If Ken takes it up,
a Pinot model can walk those statements, as Impala's does (D146), and the
InfluxDB models can read the release with a statement.

### Finish what D204 left open for Redshift

Spectrum external tables are not measured, because the namespace has no
default IAM role and `CREATE EXTERNAL SCHEMA` is refused with "Cannot find
default IAM role on this cluster". Ken must associate a role that reads a Glue
catalog or an S3 bucket before the external tables can be measured. A grant of
a table to a role is not in `Privileges`, because only SVV_RELATION_PRIVILEGES
lists it and that view shows a user only the rows of that user. The routine
parameters, the types, the casts, the aggregates and the operators have a
source in pg_catalog and are not read.

### Read what Snowflake answers and no kind holds

Measured by D203 and left out. `Roles` is answerable by SHOW ROLES, and Role has
bools that Snowflake has no source for. `Indexes` and `IndexColumns` are
answerable by INFORMATION_SCHEMA.INDEXES and INDEX_COLUMNS for a hybrid table,
which a trial account refuses to make, so no row was read. The same account
refuses masking policies, row access policies and materialized views. A run on
an Enterprise account with a hybrid table and a row access policy finishes
them.

### Settle the size and the rows of CrateDB and QuestDB

D207 measured it. The sources for the size and the row count of a CrateDB table
(`sys.shards`, and `pg_class.reltuples`) and of a QuestDB table
(`table_storage()`) cost a scan of the whole catalog for each read.
`Table.Size` and `Table.Rows` are left NULL for CrateDB, and `Table.Size` for
QuestDB. If either product makes the join follow the filter, the fields can be
added. Redshift is done (D212).

### Read the settings of a CrateDB user

`sys.users.session_settings` is the analogue of `RoleSettings`. One statement
returns the object as JSON text and not as `name=value` lines, because CrateDB
cannot read the value of a key that the statement does not name. A later
release can change that.

## PostgreSQL model

### Read the sections of `\d` that no kind answers

D201 left four things out. psql's Referenced by section lists the constraints of
other tables that name this one, and `Constraints` filters by the table that owns
the constraint, so a kind or a filter on `confrelid` is needed. The token types
that `\dFp+` prints come from `ts_token_type` of the parser. The index footer of
`\d NAME` prints the tablespace of each index. `Subscriptions` lists the
subscriptions of every database, and psql narrows to the current one.

### Make the owner of a sequence cost what the rows cost

The `owned_by` column of `Sequences` reads `pg_depend` by `objid` alone, and the
index of that catalog starts with `classid`. Each sequence scans the table, so
5005 sequences took 5.9 seconds on release 15, and one sequence by name takes
0.2 ms. D201 measured it. The fix is to add the class of the sequence and of the
table to the subselect. Measure it before and after, on a catalog of thousands
of sequences.

## Consumers

### Move the rules of the products with no model out of usql

ODBC, csvq, Athena and Cosmos keep rules of their own in usql's drivers,
such as ODBC's usql_trim, csvq's SHOW routing and the trailing semicolon
Athena and Cosmos strip, and have no model here, so D143's Info fields do
not reach them. Ken decided on 2026-09-30 that this waits for a larger
restructuring of usql, whose aim is to strip every database specific helper
out of usql/drivers. dbmeta then needs a form for a dialect with rules and
no queries.

### usql reads metadata through dbmeta

usql's W21 moved its describe commands, its version and \password onto
dbmeta, and its W31 wires up every psql describe command in
[`COMMANDS.md`](COMMANDS.md). Both are staged in usql and not committed.
When W31 lands, update the column "usql today" in COMMANDS.md, and what
[`USQL.md`](USQL.md) says usql answers.

### Move usql's \copy statements here

usql's \copy builds an INSERT with the dialect's placeholder, and probes the
columns of a table with SELECT * FROM t WHERE 1=0. They are the last SQL in
usql's drivers. usql's W29 records moving them to dbmeta, and Ken asked on
2026-09-30 for it to wait for a design. It is not a read, so it needs a
decision beside D56 before any of it lands here.

### dbtpl reads metadata through dbmeta

[`DBTPL.md`](DBTPL.md) holds which of the nine reads `dbtpl` needs each model
answers, and whether `dbtpl` can generate from each database. `dbtpl` does
not read `dbmeta` yet.

## Models

### Read the size and the clustered flag of an index

D205 left `Index.Size` and `Index.Clustered` NULL on MariaDB and MySQL. The
sources are `mysql.innodb_index_stats` (the size in pages, a table of the mysql
schema) and `INNODB_INDEXES` or `INNODB_SYS_INDEXES` (the type, where 1 is the
clustered index). Both refuse a user without the PROCESS privilege or SELECT on
the mysql schema, as measured on MariaDB 12.3. They can come back as a
fragment that a parity scene marks as refused for the grantee, if Ken wants the
fields more than the statement that answers for every principal.

### MariaDB puts the compression comment in the data type

`Column.DataType` is `COLUMN_TYPE`, and MariaDB writes `/*M!100301 COMPRESSED*/`
into it for a compressed column, so the type reads `text /*M!100301 COMPRESSED*/`
on 12.3. D205 reads `Column.Compression` from that comment and did not change
`DataType`. A caller that compares types sees the comment.

### Vitess reads the size of one tablet

`Table.Size` and `Rows` on Vitess are the numbers of the tablet that vtgate
picks, and a keyspace with several shards holds the rest elsewhere. A sum over
the shards needs vtgate to run one statement on every shard, which it does not
do for `information_schema`.
### A Validated field on Constraint

D206 fills `Constraint.Enforced` for SQL Server and Oracle as "the server checks
a new row". Both products also say whether the rows that were already there are
known to satisfy the constraint: `is_not_trusted` on SQL Server and `VALIDATED`
on Oracle. PostgreSQL has `convalidated`. No field carries it on `Constraint`.
Add `Validated sql.Null[bool]` to the root type and fill it in the three models.
`NotNull.Validated` already exists.

### Oracle sizes of partitioned tables and subpartitions

`PartitionedTable.DirectSize` and `TotalSize` are NULL on Oracle, and
`Partitions` has no rows for subpartitions (D206). A size needs the connected
user and a sum over `USER_SEGMENTS`. A subpartition needs a kind that is not a
LONG in a UNION.

### Settle the size and the rows of a Vertica table and the encoding of its columns

D208 measured it. The size and the rows of a table are in
`v_monitor.projection_storage` and the encoding of a column is in
`v_catalog.projection_columns`. A subquery on either for every table or column
made a read of one table 3 to 22 times slower, and the cost grew with the
catalog. `Table.Size`, `Table.Rows` and `Column.Compression` are NULL. A
projection of a replicated table is also repeated on every node, so a sum of
rows is not the rows. If a later release gives a view that follows the filter,
the fields can be added.
### Options and size of a SQLite table
D208 leaves `Table.Options` NULL on SQLite. `pragma_table_list` has the strict
and the WITHOUT ROWID flags, and joining it for every table grows with the
square of the catalog. A shadow table is listed as a `table`, and
`pragma_table_list` calls it `shadow`. Ken decides whether the type changes.
`Table.Size` needs `dbstat`, which scans the file and is not in the mattn
driver.
### Row security for Vertica, ClickHouse and Databend
`Table.RowSecurity` is NULL for all three. The Vertica and ClickHouse sources
(`access_policy` and `row_policies`) are refused to or filtered for a user who
is not an administrator, and a flag that reads `false` for that user is a wrong
answer. Databend 1.2.951 has no catalog for its row access policies. A later
release can add one.
### The SETTINGS clause of a ClickHouse table
The settings of a MergeTree table, such as `index_granularity`, are in
`engine_full` as text after SETTINGS. A statement cannot split them, so
`Table.Options` has the clauses that have a column of their own.
### SAP HANA size and row count
`Table.Size`, `Table.Rows` and `Index.Size` are NULL on SAP HANA (D209). The
sources are `SYS.M_TABLES`, `SYS.M_RS_INDEXES` and `SYS.M_CS_INDEXES`, and any
user reads them for the objects it can see. The planner reads one row when the
filter is a literal, a plain `= ?` or a plain `LIKE ?`, and reads every table
when the filter is the `(? = '' OR x LIKE ?)` form that all the models use. Fill
the three fields when one statement can pass a pattern that the planner pushes
down, and measure with 3000 tables.
### Exasol left join to the object sizes
`Table.Size` on Exasol joins `EXA_ALL_OBJECT_SIZES`, which costs 85 ms at 3000
tables for one table, and 41 ms with the fixture only (D209). An inner join
costs 10 ms and can drop a table that a user sees in `EXA_ALL_TABLES` and not
in `EXA_ALL_OBJECT_SIZES`. Run the parity targets with an inner join, and keep
it if the rows do not change.
