# D203. Snowflake reads the columns of a key through the pipe operator

Status: Amends D190 and D193.

## The decision

D193 left five things open for Snowflake. Ken said on 2026-10-09 to finish the
dialect. All of it ran on 2026-10-09 against the trial account, release
10.36.101, as the role DBMETA_ROLE.

1. A made role gets USAGE on the warehouse DBMETA_WH, and a made user runs on
   it.
2. `Column.PrimaryKey` stays a plain bool, and Snowflake answers it truthfully.
3. `Constraint` has a new field, `Enforced`, and Snowflake fills it.
4. The model reads the columns of every key and the table a foreign key points
   at. It also reads the session settings, and it names a dynamic table.
5. The cost was measured on a schema of 2000 tables.

## The warehouse

Ken granted USAGE on DBMETA_WH to DBMETA_ROLE with the grant option. The role
also owns the warehouse. The test grants USAGE to each role that it makes, and
revokes it before it drops the role. It never alters, suspends, resizes or
drops the warehouse, and it grants nothing to a role that it did not make,
because dbimp uses the same warehouse through DBMETA_IMP_ROLE. A made user
connects with the warehouse DBMETA_WH, and the workaround on
SYSTEM$STREAMLIT_NOTEBOOK_WH is gone. The password test makes a role of its own
for the same reason, and the user logs in as that role and not as PUBLIC.

## The columns of a key

INFORMATION_SCHEMA has no KEY_COLUMN_USAGE. SHOW PRIMARY KEYS, SHOW UNIQUE KEYS
and SHOW IMPORTED KEYS list the columns, and a SHOW is not a table. D193 took
that to mean that no statement can read them. Gemini and DeepSeek agreed that
one plain select cannot, and GET_DDL returns one text per schema with the key
written inside, which is prose and does not fit in one value for thousands of
tables.

The pipe operator makes one statement of a SHOW and a select:

```sql
SHOW PRIMARY KEYS IN DATABASE ->> SELECT ... FROM information_schema.columns c
  LEFT JOIN $1 k ON k."schema_name" = c.table_schema AND ...
```

`$1` is the result of the latest stage and `$2` the one before. The constraint
columns statement chains three SHOW stages and reads `$3`, `$2` and `$1` with
UNION ALL. SHOW IMPORTED KEYS carries the referenced database, schema, table
and column, so a foreign key row has its target. So `primary_key` is a fact:
true when SHOW lists the column. It stays `bool` and no model, test or consumer
changes for it. `ConstraintColumns` is a new kind for Snowflake, and the
fixture reads back with 9 key columns.

Three things about the pipe came out of the measurement.

- A SHOW with no scope reads the current schema, not the database. A session
  can change that with USE SCHEMA, and the first version of the statement read
  the right rows only because the fixture had been created on that connection.
  The statements say IN DATABASE, which with no name is the current database.
  `TestSnowflakeKeysIgnoreTheCurrentSchema` moves the session and reads again.
- A bind parameter is refused after the pipe. The server answers "invalid
  identifier '1'". So the first version took no filter in SQL, and its
  bindings set `Keep` (D200) to filter in Go. That read the whole database for
  one table (see Cost). Ken decided on 2026-10-09 to write the values into the
  statement, which is the section below.
- A session with no current database makes SHOW IN DATABASE read every database
  that the role can see. The stranger of parity read more rows than the
  administrator. The constraint columns statement keeps the rows of
  CURRENT_DATABASE(), and the stranger now reads fewer rows.

No cap of 10000 rows applied. One SHOW PRIMARY KEYS answered 10803 rows.

## Values are written into the statement, and the SHOW is scoped

Ken decided on 2026-10-09 to use `Info.Literal` (D78) for Snowflake and to scope
the SHOW. Both followed.

`Info.Literal` is a dialect switch, so every statement of the Snowflake model
now has its values written into it, and `Placeholder` panics as Hive's does.
`literal` in `models/snowflake` renders a string as a single quoted literal
with the backslash doubled and the quote doubled, which is the rule that
`ChangePassword` uses. It renders a bool as TRUE or FALSE and refuses every
other type and a string with a NUL. `models/snowflake/snowflake_test.go` checks
a quote, a backslash, a quote after a backslash and an injection string. The
other Snowflake statements ran unchanged against the server, so no answer
moved.

A SHOW cannot take a pattern. It takes a scope, which is a name and not a
value, so a literal does not make one. The scope depends on two arguments, the
schema and the parent, so no one `@name` holds it. The root package has a new
type, `dbmeta.Derived`, and `Binding.Derived` lists values that the binding
computes from the arguments and that a caller cannot pass. A statement reads one
as `@scope`, and `bind` asks the dialect's `Literal` to write it. Only Snowflake
uses it. This is an addition to the root API and is not in any decision before
this one, so Ken must say if it is the shape he wants. The alternative was
`Binding.Walk`, which makes `Build` fail for two kinds.

`keyScope` computes the scope, and the Snowflake `Literal` writes a scope as it
is, because only `keyScope` makes one and it quotes every name with
`QuoteIdentifier` (D127). The scope is:

- `IN TABLE "schema"."table"` when the schema and parent patterns are both exact
  names. With an exact catalog pattern it has three parts.
- `IN SCHEMA "catalog"."schema"` when the catalog and schema are exact and the
  parent is not.
- `IN DATABASE` otherwise, or `IN DATABASE "catalog"` when only the catalog is
  exact.

The two key statements take a new parameter, `catalog`, a name pattern for the
database. It is there because SHOW refuses a schema without its database with
"Must specify the full search path starting from database", and the statement
cannot name the current database. So the schema scope of Ken's rule needs the
catalog, and a caller that gives an exact schema and no catalog gets the
database scope. A table scope needs no catalog, because a name of two parts
starts at the current database. A consumer that never sets the catalog reads
the table scope and the database scope only. `Args.Catalog` already exists, and
`Each` passes it to these two statements.

A pattern is an exact name when it is not empty and holds no `%`, no `_` and no
backslash. The model treats a backslash as not exact. Ken's rule counts an
escaped underscore as exact, but whether the server reads a backslash in a LIKE
as an escape depends on the statement, and a wider scope loses no row. So a
name with an underscore, such as `DBMETA_FIXTURE` or `ORDER_LINES`, reads the
wide scope. That is the common case for a table name, and the cost of it is the
cost of the database scope below.

The names are never folded. Every other statement of the model matches with LIKE,
which is case sensitive on the upper case names that Snowflake stores, and the
dialect fold of upper case is the consumer's. So the exact name is quoted as the
caller wrote it, and a pattern that the consumer folded to AUTHOR scopes to
"AUTHOR". The same patterns filter the rows in SQL, so the scope only decides
how many rows the server reads, and the two cannot disagree. The case that is
not covered is a table that was created with a quoted lower case name, when the
consumer folds the name the user typed to upper case. The scope then names the
upper case table, which does not exist, and the table scope is a server error.
The LIKE of every other statement fails to match that table in the same way, so
the model is consistent and the answer is wrong for that table only. Nothing
ignores case anywhere, so `LikeFold` is not used. The `Keep`
functions of the first version are gone, and the filters are SQL again.

A hostile name is quoted and stays inside its quotes. `TestSnowflakeKeyScopes`
creates the schema `DMKEYS"; DROP TABLE T; --` on the server and reads its keys
at the table scope, the schema scope and a wildcard. Every row came back, the
table T was not dropped, and the schema was dropped by the test at the end. The
unit test builds the statement for the pair of names `X"; DROP TABLE T; --` and
`O'B` and checks that the double quote is doubled in the scope and the single
quote is doubled in the literal.

One thing changed that a consumer can see. A table scope on a table that does
not exist, or that the role cannot see, is an error from the server, "does not
exist or not authorized", where the wide scope answers no rows. A view and a
table of INFORMATION_SCHEMA answer no key rows and no error. The error is
only for an exact schema and an exact table that the server does not have, so a
describe of a missing table surfaces the server's message. Parity does not see
it, because its patterns hold an underscore.

## ENFORCED

`Constraint` had no field for it. It has `Enforced sql.Null[bool]`, and the
constraints statement reads it from INFORMATION_SCHEMA.TABLE_CONSTRAINTS. Every
Snowflake key reads false. `deferrable` of false and `deferred` of true stay as
the server says them (rule 13), and their descriptions now say to read
`enforced` first. No other model returns the field, which is the rule that D198
set for a new field: the model that has a source fills it and the rest leave it
NULL. MySQL has ENFORCED from 8.0.16 and MariaDB records a check constraint from
10.2. Neither was measured, so the item is in docs/BACKLOG.md.

A consumer that reads `Constraint` must know that the field can be NULL, which
is every model but Snowflake. usql and dbtpl need no change for it.

## What else changed

- The functions statement read an empty volatility and said that
  INFORMATION_SCHEMA does not record it. It has a VOLATILITY column. The
  fixture function reads volatile and a procedure reads empty.
- A dynamic table, an Iceberg table and a hybrid table were base tables to the
  Tables type. They read dynamic table, iceberg table and hybrid table, from
  IS_DYNAMIC, IS_ICEBERG and IS_HYBRID. Only the dynamic table was created.
- Settings read SHOW PARAMETERS through the pipe, with the key as the name and
  the level as the context. The session read 161 parameters. The name filter
  is in the statement.
- The model answers 15 of the 65 questions.

## The unread objects

docs/COVERAGE.md holds the table. Tags, stages, file formats, pipes, streams
and tasks have no kind in `dbmeta` and no other product has one, so they are
unsupported. A task and a stream are not triggers. A dynamic table is a Tables
row. The trial account is Standard edition, so it refuses masking policies, row
access policies, materialized views and hybrid tables, and nothing of those was
measured with a row. INFORMATION_SCHEMA has INDEXES and INDEX_COLUMNS, which
answer no rows here. Roles are answerable by SHOW ROLES and were left out,
because Role has bools that Snowflake cannot fill.

## Parity and conformance

The grantee and the visitor read fewer rows of constraint columns than the
administrator, as they do for constraints. The stranger reads fewer, which is
none. The error text of the stranger for columns changed, and it now has the
wrapper "Uncaught exception of type 'STATEMENT_ERROR'", because the statement
runs inside the pipe. The conformance file has the primary keys and the seven
constraint lines for Snowflake, and they agree with PostgreSQL. Snowflake stays
in `agreementExcluded`, for the default of an identity column and the
nullability of a view column. The other golden sections did not change.

## Cost

D47 asks for a catalog of thousands of tables. The scratch schema had 2000
tables, 14000 columns, and on each table a primary key of four columns, a unique
key and a foreign key of four columns, so 17996 key columns. It was the only
schema in the database, so a schema scope and a database scope read the same
rows. The runs are one pass each on an XSMALL warehouse, and the warehouse
queue makes a run vary by a few seconds.

The first version, with `Keep` and no scope:

| Statement | Rows | Whole schema | One table |
| --- | --- | --- | --- |
| tables | 2000 | 1.2 s | not asked |
| columns | 14000 | 9.8 s | 10.1 s |
| constraints | 5999 | 24.6 s | 0.8 s |
| constraint columns | 17996 | 24.6 s | 24.4 s |

The second version, with the scope and the filters in SQL, on a schema whose
names have no underscore:

| Arguments | Columns | Constraint columns |
| --- | --- | --- |
| one table, schema and table exact | 7 rows, 1.4 s | 9 rows, 1.4 s |
| one table, database exact too | 7 rows, 1.3 s | 9 rows, 1.0 s |
| one schema, database and schema exact | 14000 rows, 19.3 s | 17996 rows, 18.4 s |
| schema exact, no database | 14000 rows, 9.7 s | 17996 rows, 22.4 s |
| a wildcard table, TBL17% | 777 rows, 6.4 s | 999 rows, 32.2 s |
| the whole database | 14000 rows, 8.0 s | 17996 rows, 21.0 s |

One table costs 1.4 s where it cost 10 s and 24 s. A plain read of one table's
columns before the keys was 1.5 s, so the key flag costs about nothing there.
A wildcard and the whole database read every key of the database, and the cost
grows with the database, because SHOW has no filter. D47 refuses that for a
read that a person runs for one table, and this read no longer does it for one
exact table. A person who runs `\d TBL17` with an exact name pays 1.4 s. A person who
asks for `TBL17%` or for a table with an underscore pays for the database.
The numbers for a wildcard are noisy, and the 32 s of the constraint columns
is the same statement as the 21 s of the whole database.

## What was left out

- Roles, indexes, row access policies, masking policies and materialized views,
  for the reasons above.
- The ENFORCED column of MySQL and MariaDB.
- The schema scope for a caller that gives no catalog. The statement cannot
  name the current database.
- A scope for a pattern with an escaped underscore.
- The statement of D56 for a user that changes its own password, which the
  server refuses for any statement.

## Cost of the run

About 25 dbrun commands ran for the first version and about 10 more for the
second. The warehouse did most of the waiting, and 2000 tables took 8 minutes
24 seconds and then 8 minutes to create. Every user, role, grant on the
warehouse and scratch schema was dropped, and SHOW USERS, SHOW ROLES and SHOW
SCHEMAS IN DATABASE DBMETA showed nothing left.
