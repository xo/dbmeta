# What dbtpl Reads, and What dbmeta Added

`dbtpl` generates Go code from a database schema. It reads metadata with
queries of its own, in `models/*.dbtpl.go`, dispatched through a `Loader` struct
of function fields in `loader/loader.go`.

This measures what `dbtpl` reads, how much of it `dbmeta` already answers, and
what `dbmeta` had to add before `dbtpl` can stop carrying its own SQL.

Measured against `dbtpl` at commit `8366044`.

## What dbtpl reads

The `Loader` has 17 fields. Eleven read metadata, four write or drop a view,
and two are helpers.

| Loader field | What it returns | postgres | mysql | sqlite3 | sqlserver | oracle |
| --- | --- | --- | --- | --- | --- | --- |
| `Schema` | the name of the current schema | yes | yes | yes | yes | yes |
| `Tables` | name, type, and the definition of a view | yes | yes | yes | yes | yes |
| `TableColumns` | ordinal, name, type, not null, default, is primary key, comment | yes | yes | yes | yes | yes |
| `TableSequences` | which columns the database fills itself | yes | yes | yes | yes | yes |
| `TableForeignKeys` | key name, column, referenced table, referenced column, key id | yes | yes | yes | yes | yes |
| `TableIndexes` | name, unique, primary | yes | yes | yes | yes | yes |
| `IndexColumns` | position, column id, column name | yes | yes | yes | yes | yes |
| `Procs` | id, name, kind, return type, return name, body | yes | yes | no | yes | yes |
| `ProcParams` | parameter name and type | yes | yes | no | yes | yes |
| `Enums` | the enum types of a schema | yes | yes | no | no | no |
| `EnumValues` | each label and its sort order | yes | yes | no | no | no |

The four view operations, `ViewCreate`, `ViewSchema`, `ViewTruncate` and
`ViewDrop`, write rather than read. They stay in `dbtpl`. `dbmeta` only reads,
which is D5 and does not change.

## What dbmeta already answers

Nine of the eleven now map onto a `dbmeta` query. Six always did, and three
arrived with the kinds D47 added.

| dbtpl | dbmeta | Note |
| --- | --- | --- |
| `Tables` | `dbmeta.Tables` | the view definition is `dbmeta.Views`, a kind of its own |
| `TableColumns` | `dbmeta.Columns` | exact: `Column.PrimaryKey` is in the same row |
| `TableSequences` | `dbmeta.Columns` | `Column.Identity` says which column the database fills, which is what this asks |
| `TableIndexes` | `dbmeta.Indexes` | exact: name, unique and primary are all there |
| `IndexColumns` | `dbmeta.IndexColumns` | exact, and `dbmeta` adds the expression and the direction |
| `Procs` | `dbmeta.Functions` | exact: `Function.ID` is the oid `dbtpl` joins on |
| `ProcParams` | `dbmeta.RoutineParameters` | exact, except on SQLite, which has no named parameters |
| `TableForeignKeys` | `dbmeta.ConstraintColumns` | exact, including the referenced column and the position in a composite key |
| `Schema` | `dbmeta.CurrentSchema` | one row, read with `dbmeta.First` |

`dbmeta` also answers these for MariaDB and SQLite, which `dbtpl` supports, and
`dbtpl` gains nothing new there, because it already has them. What it gains is
not having to maintain five dialects of the same query.

### Which databases answer everything dbtpl needs

The nine above are what `dbtpl` reads to generate code. This is how many of
them each model answers, counted from the queries the models register rather
than from memory.

| Database | Of the nine | What is missing, and why |
| --- | --- | --- |
| PostgreSQL | 9 | nothing |
| MariaDB and MySQL | 9 | nothing |
| SQL Server | 9 | nothing |
| Oracle | 9 | nothing |
| DuckDB | 8 | `IndexColumns`: DuckDB names an index and does not list the columns of it |
| GizmoSQL | 8 | the same as DuckDB, whose statements it shares |
| SQLite | 8 | `RoutineParameters`: a SQLite function has no named parameters |
| rqlite | 8 | the same as SQLite, whose statements it shares |
| libSQL | 8 | the same as SQLite, whose statements it shares |
| InfluxDB 3 | 6 | `TableIndexes`, `IndexColumns` and `TableForeignKeys`: InfluxDB 3 has no index and no key. `Procs` lists DataFusion's built in functions, and only with the system objects |
| YDB | 1 | every one but `Tables`: the columns, indexes and keys of a table are in its schema, which only a gRPC call per table reads, YQL has no function list, and a session has no current directory |
| ArangoDB | 5 | `TableIndexes`, `IndexColumns`, `ProcParams` and `TableForeignKeys`: AQL lists no index, a function's parameters are in its JavaScript source, and ArangoDB has no foreign key. `Schema` is the database of the connection (D168). `TableColumns` reads a collection's schema rule, and a collection with no rule has no column |
| Apache Druid | 5 | `TableIndexes`, `IndexColumns`, `ProcParams` and `TableForeignKeys`: Druid has no index and no key, and ROUTINES lists a function's signatures as text. `Procs` lists the built in functions, and only with the system objects (D171) |
| Apache Drill | 5 | `TableIndexes`, `IndexColumns`, `ProcParams` and `TableForeignKeys`: Drill has no index and no key, and sys.functions lists a function's argument types as text. `Procs` lists the built in functions, and only with the system objects. `Schema` has a row only after the session names a default schema (D178) |
| Elasticsearch | 4 | `TableIndexes`, `IndexColumns`, `ProcParams`, `TableForeignKeys` and `Schema`: Elasticsearch has no index its SQL lists, no key and no schema. `Procs` lists the built in functions, and only with the system objects (D177) |
| OpenSearch | 3 | `TableIndexes`, `IndexColumns`, `Procs`, `ProcParams`, `TableForeignKeys` and `Schema`: OpenSearch has no index its SQL lists, no key, no function list and no schema. `TableColumns` and `TableSequences` answer on 3.9.0 alone, because dbimp's driver cannot read DESCRIBE on 2.19.6 (D181) |
| Apache Avatica | 9 | nothing. The standalone server runs HSQLDB, which has keys, indexes, routines and a current schema (D186) |
| Apache Solr | 4 | `TableIndexes`, `IndexColumns`, `TableForeignKeys`, `Procs` and `ProcParams`: Solr has no index, no key and no function that SQL lists (D179) |
| InfluxQL | 3 | everything but `Tables`, `TableColumns` and `TableSequences`: InfluxDB has no index, no key, no function list and no current database a statement returns |
| ClickHouse | 7 | `ConstraintColumns` and `RoutineParameters`: a CHECK holds an expression rather than columns, and a function is overloaded across types with no signature recorded |
| Cassandra | 7 | `CurrentSchema` and `RoutineParameters`: CQL has no expression for the current keyspace, and arguments are two parallel lists on the function's own row |
| ScyllaDB | 7 | the same two as Cassandra, for the same reasons |
| Trino | 4 | `Indexes`, `IndexColumns`, `Functions`, `RoutineParameters` and `ConstraintColumns`: Trino is a query engine and has no index, no constraint of any kind, and no table valued source for its function list |
| Presto | 3 | the same five as Trino, and `Schema`: neither `current_catalog` nor `current_schema` resolves |
| SAP HANA | 9 | nothing |
| Apache Hive | 6 | `Indexes`, `IndexColumns` and `RoutineParameters`: Hive removed indexes in 3.0, and a function is a Java class whose parameters are in the class rather than in the metastore |
| Firebird | 8 | `Schema`: Firebird has no schemas before 6.0, so there is no current one to read and none is invented |
| Exasol | 8 | `RoutineParameters`: Exasol keeps the parameters of a function or a script only inside its text, and no catalog view lists them |
| Vertica | 8 | `RoutineParameters`: a routine's arguments are one comma separated list of types on its own row, and the named parameters a library function declares are options rather than arguments |
| Couchbase | 5 | `TableColumns`, `TableSequences`, `TableForeignKeys` and `Schema`: a document has no fixed shape, so there is no column and no key, and SQL++ has no expression for the current scope |
| Neo4j | 7 | `TableColumns` and `TableSequences`: the only list of the properties of a label reads every node, which D47 forbids. `TableForeignKeys` answers and holds no foreign key, because Neo4j has none. On 5.26 `IndexColumns`, `Procs`, `ProcParams` and `TableForeignKeys` need 2026.05 (D162) |
| CockroachDB | 9 | nothing. It shares the postgres model's statements for all nine |
| CrateDB | 8 | `RoutineParameters`: a JavaScript function is not in `pg_proc`, and `information_schema` has no `parameters` view. Only `specific_name` holds the argument types, with no names |
| QuestDB | 5 | `TableIndexes`, `IndexColumns`, `ProcParams` and `TableForeignKeys`: a symbol index is listed only one table at a time, a built in function's arguments are one text, and QuestDB has no key of any kind. `Procs` lists the built in functions, and only with the system objects |
| TiDB | 7 | `Procs` and `ProcParams`: TiDB has no stored function or procedure. Everything else is the mysql model's statement |
| Vitess | 9 | nothing. Every one is the mysql model's statement |
| Databend | 8 | `ProcParams`: a procedure's arguments are one text, such as addup(Int32,Int32) RETURN (Int32), and a function's are a variant |
| SingleStore | 9 | nothing, although no foreign key is ever listed, because SingleStore has none |
| Snowflake | 6 | measured on 2026-10-08 and 2026-10-09 (D190, D203). `TableIndexes` and `IndexColumns`: Snowflake has no index outside a hybrid table, which a trial account refuses. `ProcParams`: information_schema has no parameters view, and the argument signature is one text. `TableForeignKeys` is answered by SHOW IMPORTED KEYS, read through the pipe operator |
| Amazon Redshift | 6 | `TableIndexes` and `IndexColumns`: Redshift has no index. `ProcParams` is not read (D204) |
| Apache Impala | 5 | `TableIndexes`, `IndexColumns`, `ProcParams` and `TableForeignKeys`: Impala has no index, and SHOW lists no key and no parameter names |
| Amazon Athena | 3 | `TableIndexes`, `IndexColumns`, `ProcParams`, `Procs`, `TableSequences` and `TableForeignKeys`: Athena has no index, no sequence and no key, and a function is behind SHOW FUNCTIONS (D222) |
| Databricks | 6 | `TableIndexes`, `IndexColumns` and `TableSequences`: Databricks has no index and no sequence. An identity column exists and INFORMATION_SCHEMA does not report it. A primary key and a foreign key are declared and never enforced (D224) |
| Spanner | 9 | nothing. A foreign key points at a primary key or at a unique index, and both are read. Measured on Spanner Omni 2026.r4-lts (D216) |
| Google BigQuery | 9 | nothing. The schema is the dataset of the connection, and a dataset with no table or view has no row. A primary key and a foreign key are declared and never enforced (D220, D226) |
| SurrealDB | 9 | nothing on 3.x, although no foreign key is ever listed, because SurrealDB has none. 2.7 answers `Schema` alone, because a 2.x statement cannot read INFO as a value (D164) |
| any `information_schema` | 7 | `Indexes` and `IndexColumns`: the standard has no index at all |

Nine answer all nine: PostgreSQL, the MySQL dialect, SQL Server, Oracle, SAP
HANA, CockroachDB, Vitess, SingleStore and SurrealDB from 3.0. A `dbtpl` built
on `dbmeta` can generate from the first seven with nothing missing.
SingleStore and SurrealDB answer all nine and list no foreign key, because
neither has one.

Every gap above is the product rather than the model, except on Redshift. The
Redshift model does not read `ProcParams`. `dbtpl` supports
PostgreSQL, MySQL, SQL Server, Oracle and SQLite today, so the only one of its
own databases that is short is SQLite, by one query, for a reason `dbtpl`
already knows: it writes no parameter names for SQLite either.

### Whether dbtpl can generate for each database

The count above is how many queries answer. Whether `dbtpl` can generate
from a database is a different question, and it is the one to answer when a
dialect is added.

| Database | `dbtpl` supports it today | Can generate from `dbmeta` |
| --- | --- | --- |
| PostgreSQL | yes | yes, all nine |
| MySQL and MariaDB | yes | yes, all nine |
| SQL Server | yes | yes, all nine |
| Oracle | yes | yes, all nine |
| SQLite | yes | yes, without parameter names |
| rqlite | no | yes, without parameter names, the same as SQLite |
| libSQL | no | yes, without parameter names, the same as SQLite. A vector column reads its declared type, such as `F32_BLOB(3)`, which a generator has to map to a type of its own |
| InfluxDB 3 | no | no. It has no key and no foreign key, so there is nothing to relate one measurement to another, and every column but time is nullable |
| YDB | no | no. It answers the tables and none of their columns, so there is nothing to generate a type from |
| ArangoDB | no | no. A collection has columns only where a schema rule names them, no column is a key, and there is no foreign key to follow. A graph's edge definitions are not enforced and no kind reads them |
| Apache Druid | no | no. A datasource has no key and no foreign key, so there is nothing to relate one datasource to another, and every column but `__time` is nullable |
| Apache Drill | no | no. A table has no key and no foreign key, so there is nothing to relate one table to another. A file table is listed only when the Metastore is on, and the columns of a view are all type ANY and nullable |
| Elasticsearch | no | no. An index has no key and no foreign key, so there is nothing to relate one index to another, every field is nullable, and a field of an object is a column named with a dot |
| OpenSearch | no | no. An index has no key and no foreign key, so there is nothing to relate one index to another, every field is nullable, and a field of an object is a column named with a dot |
| Apache Avatica | no | yes, all nine. The version is the release of HSQLDB. A NOT NULL is a check named SYS_CT and a number, a type reads as CHARACTER VARYING(n) with its length, and an index that a key made has a name with a number that changes between runs (D186) |
| Apache Solr | no | no. A collection has no key and no foreign key in SQL, so there is nothing to relate one collection to another, and every column reads nullable, the unique key included |
| InfluxQL | no | no, for the same reason as InfluxDB 3. A measurement has tags and fields and no key, and `Schema` has no answer, because no statement returns the database of the request |
| DuckDB | no | yes, without index columns |
| GizmoSQL | no | yes, without index columns, the same as DuckDB. A generator needs a driver that opens a session, and the one dburl names does not (D187) |
| ClickHouse | no | partly: no foreign key to follow and no parameter names |
| Cassandra | no | partly: no current keyspace expression and no parameter names |
| ScyllaDB | no | partly: the same as Cassandra |
| Trino | no | no |
| Presto | no | no |
| SAP HANA | no | yes, all nine |
| Apache Hive | no | partly: the foreign keys are there to follow, and they are declarations Hive does not enforce, so a generator trusts something the database never checks |
| Firebird | no | yes, once it is told there is no schema to qualify by |
| Vertica | no | yes, without parameter names. Every table, key and foreign key is there, and an index is a projection, which a generator can emit or leave out |
| Exasol | no | partly: every table, key and foreign key is there to follow, and a routine has no parameters to read. The indexes are the engine's own, built and dropped as queries need them and named by object id, so a generator that emits an index emits a different set on another day |
| Couchbase | no | no. A collection has no columns, so there is no field to generate |
| Neo4j | no | no. A label has no column catalog, so there is no field to generate, and a relationship joins two nodes rather than a foreign key joining two tables |
| CockroachDB | no | yes, all nine. A parameter declared integer reads as bigint, because CockroachDB makes integer 64 bits |
| CrateDB | no | partly: no foreign key to follow and no parameter names. Every table, column and primary key is there |
| QuestDB | no | no. It has no key and no foreign key, so there is nothing to relate one table to another, and every column is nullable |
| TiDB | no | yes, without routines. Every table, key and foreign key is there, and TiDB enforces its foreign keys |
| Vitess | no | yes, all nine. A schema is the keyspace, which is the name vtgate accepts in a query (D135) |
| Databend | no | partly: no key and no foreign key to follow, because Databend has neither. Every table and column is there, and a CHECK and its columns |
| SingleStore | no | partly: no foreign key to follow, because SingleStore refuses one. Every table, key, index and routine is there |
| Snowflake | no | partly (D203): every table, column, primary key and foreign key is there, and the key columns are filtered in Go, because the statement takes no filter. A key is declared and not checked, so `Constraint.Enforced` reads false |
| Amazon Redshift | no | partly: no parameter names. Every table, column, primary key and foreign key is there. Redshift declares a key and does not enforce it |
| SurrealDB | no | partly, on 3.x: no foreign key to follow, because a link is a field of the type `record<t>` that the server does not check, and a schemaless table has no columns, because only a DEFINE FIELD is one. Every SCHEMAFULL table, field, index and function is there, and the record id is the key of every table. 2.7 has nothing to generate from (D164) |
| Google BigQuery | no | yes, all nine. The current schema is the dataset of the connection, and a dataset with no table has none. Every table, column, primary key and foreign key is there, and BigQuery declares a key and never enforces it, so a generator trusts something the database never checks. A table has no unique constraint. A column type is a BigQuery type such as `ARRAY<FLOAT64>`, which a generator has to map to a type of its own. A constraint name begins with the table and a dot, as `book.book_author_fk` (D220) |
| Amazon Athena | no | no. A Glue table has no key and no foreign key, so there is no relation to follow, and every column is nullable. A table is a prefix in S3, and a generator that wants a type has to map the type of Athena, such as `varchar` or `timestamp(6)`, to one of its own (D222) |
| Databricks | no | yes, without sequences and indexes. Every table, column, primary key and foreign key is there, and Databricks declares a key and never enforces it, so a generator trusts something the database never checks. A table has no unique constraint, and a CHECK constraint, a default and an identity column are not in INFORMATION_SCHEMA, so a generated column is not marked. A column type is a Databricks type such as `bigint` or `array<string>`, which a generator has to map to a type of its own (D224) |
| Spanner | no | yes, all nine. A column type is a Spanner type such as `STRING(100)`, which a generator has to map to a type of its own. A table interleaved in a parent has the key of the parent as the start of its own, and no foreign key says so, so a generator reads the parent from `Table.Options` (D216) |
| Apache Impala | no | no. No key and no foreign key is listed, so there is nothing to relate one table to another, and every column is nullable |

Trino is the first that is a clear no, and it is not the same as answering few
of the nine. `dbtpl` generates typed access from a schema and follows a foreign
key to decide what relates to what. Trino has no foreign key, no unique
constraint and no index at any release, so the relationships are not there to
read and a generator produces a struct per table with nothing tying them
together.

That is the answer for a query engine rather than for Trino alone, and Presto
answers the same way for the same reason, which D66 predicted.

## What dbmeta added

Five things, three of them shared with `usql`. All five now exist, under D47.
What follows is what each one became and what it still cannot do.

Routine parameters became `dbmeta.RoutineParameters`, with a name,
position, mode and type per parameter. PostgreSQL, the MySQL dialect, SQL
Server, Oracle, DuckDB, Firebird, SAP HANA, Couchbase, CockroachDB, Vitess,
SingleStore, InfluxDB 3, Neo4j from 2026.05 and the shared model answer it. SQLite cannot: a function there is compiled C with no
named parameters.

Group by `Routine` and, where the database overloads a name, by `RoutineID`.
`Function.ID` carries the same value, so the join is one expression for every
database. On PostgreSQL it is the oid, which is what `dbtpl` uses today.

Constraint columns became `dbmeta.ConstraintColumns`, with the column, its
one based position within the constraint, and for a foreign key the catalog,
schema, table and column it points at. Every model but ClickHouse, Trino,
Presto, Couchbase, QuestDB, Redshift, Impala, InfluxDB 3 and YDB
answers it, SQLite included.

This was the largest gap, and it is closed exactly the way `dbtpl` needs: a
composite key is several rows sharing a constraint name, ordered by `Ordinal`,
each paired with the column it references.

Enum values as rows became `dbmeta.EnumValues`, and PostgreSQL, DuckDB
and CockroachDB answer it. A label there is a row with a one based ordinal, which is what a generated
Go constant needs.

MariaDB and MySQL cannot, and this is the one place `dbtpl` gains nothing. They
have an enum column rather than an enum type, and the labels exist only inside
the `enum('red','green','blue')` text of `COLUMN_TYPE`. Splitting that
correctly means tracking quoting, because a label can hold a comma or an
escaped quote, and no portable SQL does that. `dbtpl` splits it in Go today and
has the same limitation, so nothing is lost by keeping that where it is.
`Column.DataType` returns the text verbatim.

The definition of a view became `dbmeta.Views`, a kind of its own rather
than a field on `Table`. Reaching the definition costs a join or a function
call per row, and a caller that lists tables does not pay it. Every model but
Couchbase, InfluxDB 3, Neo4j and YDB answers it.

The current schema became `dbmeta.CurrentSchema`, which answers one row and
is read with `dbmeta.First`. It is session dependent and says so. Every model
but Cassandra, Firebird, Presto, Couchbase and YDB answers it.

### The two smaller ones

`Column.PrimaryKey` is now a field on `Column`. It was the example the whole
policy was written around, and the answer is that it is free on MySQL and
SQLite, which already select it, and one join on PostgreSQL. `dbtpl` reads it
per column and no longer needs a second query.

`Function.ID` is now a field on `Function`, and `RoutineParameters` carries the
same value. PostgreSQL returns the oid, which is what `dbtpl` joins on today.
Everything else returns the name or the specific name, because nothing else
here overloads a routine.

## How dbtpl can use dbmeta

Unlike `usql`, `dbtpl` reads per schema and per table rather than listing, and
it already passes a context and a `DB` everywhere. The shape fits.

A `Loader` field becomes a closure over a `Meta`:

```go
TableIndexes: func(ctx context.Context, db models.DB, schema, table string) ([]*models.Index, error) {
    args := dbmeta.Args{Schema: schema, Parent: table}.Map()
    var out []*models.Index
    for v, err := range dbmeta.Indexes.All(ctx, m, db, args) {
        if err != nil {
            return nil, err
        }
        out = append(out, &models.Index{
            IndexName: v.Name, IsUnique: v.Unique, IsPrimary: v.Primary,
        })
    }
    return out, nil
},
```

Three things `dbtpl` gets that it does not have today.

`Query.Support(m)` says whether a database has an object kind at all. `dbtpl`
today sets a `Loader` field to nil for that, which means the knowledge lives in
five separate files rather than in the model.

Version gates. `dbtpl`'s queries have none, so a query written against a recent
PostgreSQL either works on 9.6 or does not, and nothing says which. `dbmeta`
carries the gates and the padding rule, so a column absent on an old release
arrives as NULL with `Field.Min` saying why.

The fixtures. `models/<driver>/fixture` builds a schema with one of every object
the queries read. `dbtpl` generates against a live database and needs one, and
that was the reason the fixtures are exported rather than kept in the tests.
`dbtpl` can generate against `fixture.Everything` instead of a schema someone
has to build by hand.

### The dependency runs one way

`dbmeta` must not import `dbtpl`, which is hard rule 7. There is no dependency
in either direction today: `dbmeta` does not use `dbtpl` for anything, and
nothing here is generated, which is D71. If `dbtpl` starts reading `dbmeta`,
the rule still holds with one arrow: `dbtpl` imports `dbmeta`, and `dbmeta`
keeps its zero dependencies.

### The order that loses nothing

The five kinds exist, so the order is about risk rather than blocking.

Move `TableIndexes` and `IndexColumns` first: they map exactly and a mistake
shows immediately in generated code. Then `Tables` and `TableColumns`, which
gains `PrimaryKey` in the same row. Then `TableForeignKeys` onto
`ConstraintColumns`, which is the one that was not possible to approximate
and now can be read directly. Then `Procs` and `ProcParams` together, joined on
`Function.ID`.

Leave `Enums` and `EnumValues` where they are for MariaDB and MySQL. `dbmeta`
answers them only for PostgreSQL, DuckDB and CockroachDB, so moving them means
two code paths rather than one.
