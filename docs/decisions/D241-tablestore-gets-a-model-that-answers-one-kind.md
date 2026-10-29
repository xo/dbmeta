# D241. Tablestore gets a model that answers one kind

Status: Amends D194, amended by D242.

## The decision

Ken asked on 2026-10-29 for a model for Alibaba Cloud Tablestore, and D218 had
left it without one because it had no account. dbsetup made the account and wrote
the credential files `tablestore` and `tablestore-reader`. The model is
`models/tablestore`. It answers 1 of the 65 kinds: tables.

It reads the hosted service through its SQL feature and nothing else. Nothing else
runs it, so the tier of the entry is Verified and CI never runs it.

## What the service offers

The SQL feature of Tablestore is a front end that speaks the MySQL 5.7 dialect over
tables that a person maps with CREATE TABLE. It has no INFORMATION_SCHEMA and no
system table. Every statement below ran against the instance `dbmeta` in the region
ap-southeast-5, as the administrator and as the reader.

Six statements answer. SHOW TABLES and SHOW FULL TABLES list the mapped tables of the
instance, and the second names the type, which is BASE TABLE. DESCRIBE, SHOW COLUMNS,
SHOW CREATE TABLE and SHOW INDEX each need the name of one table. SHOW INDEX lists the
primary key, each secondary index as `GlobalIndex` and each search index as
`SearchIndex`. SELECT @@version answers `5.7.25-SQLFrontend-b30fece-dirty`.

These answer an error: SHOW DATABASES, SHOW VARIABLES, SHOW STATUS, SHOW GRANTS, SHOW
TABLE STATUS, SHOW FUNCTIONS, SHOW TRIGGERS, SHOW COLLATION, SHOW PRIVILEGES, SHOW
CREATE VIEW, CREATE VIEW, `information_schema.tables`, `mysql.user`, `select
version()`, `user()` and `database()`. A table in the instance that no CREATE TABLE
mapped is not visible to SQL.

## What the model reads

SHOW FULL TABLES is the one statement that lists tables and names none. The model reads
it for Tables. The statement takes a LIKE pattern, and it takes nothing that a
parameter binds, so the model sets `Keep` and matches the rows in Go, as the Cassandra
model does (D200). The instance is the only namespace, so Catalog and Schema of a table
are empty. A secondary index and a search index are not tables, and SHOW FULL TABLES
does not list one.

The version query is `SELECT @@version`. The service has no release number, and the
text is the one place that it says what it is. The number is the level of MySQL that
the front end claims, and the rest is its build. The model keeps the text whole and
gates nothing on the number. usql declares no version for ots, so it runs `SELECT
version();`, which the service refuses, and usql prints no version.

## What the model does not read, and the question for Ken

DESCRIBE, SHOW CREATE TABLE and SHOW INDEX read the columns, the definition and the
indexes of one table. A SELECT cannot filter or join them, and a read of every column
needs one statement for each table. That is a walk, and D146 allows a walk for Impala,
InfluxQL, Elasticsearch and OpenSearch only. So Columns, Indexes and IndexColumns are
not answered, and neither is anything built from them, such as the primary key of a
column.

Ken can decide whether to allow a walk for Tablestore. A walk reads one SHOW FULL TABLES
and then one DESCRIBE and one SHOW INDEX for each table. It adds Columns, Indexes and
IndexColumns, which is 3 more kinds. The cost is two statements for each table, and each
statement is one call to the service.

The rest is absent. A statement refuses every one of the other kinds, and the service has
no schema, role, view, sequence, trigger, function list or privilege that SQL reads. The
users and the policies belong to RAM, which only the Alibaba Cloud API reads. A search
index and a secondary index are made with the Tablestore API and not with SQL, and SHOW
INDEX is the one place that SQL names them.

## The leads that were wrong

Gemini and DeepSeek sorted the unanswered kinds. Both said that nothing lists every
column or every index at once, that no hidden table exists and that no function list
exists, and the statements above agree. Gemini named RAM for roles and privileges, and
the instance for databases and schemas, which is where those facts live, and SQL reads
none of it. Neither invented a relation. Every lead was run against the service, and
the tests assert that each refusal is still a refusal, so a change on the service shows.

## The driver

dburl v0.53.0 names `github.com/aliyun/aliyun-tablestore-go-sql-driver` for the ots
scheme, and the test module uses v0.0.0-20220418015234-4d337cb3eed9 of it, which is the
version that usql uses (D154). It registers ots. dburl writes its DSN with the scheme
`https`, which is how the driver reads the endpoint, so `dbrun` hands it the DSN that
dburl writes. The driver binds on the client and writes a backslash before a quote. It
ignores a context, so canceling one does not stop a statement that is running. The model
binds no parameter, so none of that reaches a statement.

The service resets a connection now and then, and it answers "user is disabled" for
about a minute after the account is made. The tests open the driver through a small
wrapper that tries a statement again, and the model has no retry.

## The fixture

SQL cannot create a Tablestore table. CREATE TABLE makes a mapping over a table that
exists already, and a mapping needs the table to be there. A non key string column maps
to MEDIUMTEXT, and VARCHAR is for a key only. So the fixture is two layers. The package
holds the four tables as data, and the test module makes them with
`github.com/aliyun/aliyun-tablestore-go-sdk`, with a secondary index on author and a
search index on book. Each table is pay as you go with no reserved throughput. Then
SQL maps each table, and the teardown drops the mappings and deletes the tables. Each
name starts with `dbmeta_fx_`, and the teardown deletes only a table with that prefix.

The SDK is a dependency of the test module only. The root module stays on the standard
library.

## Parity

Tablestore has no user in SQL, and access is by RAM. The principals are the administrator
and the reader, a RAM user that reads the tables and is refused when it creates one, which
was measured with the SDK. The two answered the same, so the section of the reader in
`testdata/parity.txt` is empty.

## The cost

A pay as you go table with no reserved throughput costs nothing while it holds no data,
and the fixture writes no row. The tests make four tables and delete them, and a run takes
about three seconds.
