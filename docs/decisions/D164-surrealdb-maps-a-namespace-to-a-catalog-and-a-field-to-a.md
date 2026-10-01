# D164. SurrealDB maps a namespace to a catalog and a defined field to a column

Status: Amended by D168.

## The decision

Ken asked on 2026-10-01 for a SurrealDB dialect. `models/surrealdb` answers 18
of the 56 on 3.1.6, 3.2.4 and 3.3.0, and 1 on 2.7.0, through dbimp's
surrealdb driver at v0.10.0, which dburl names. The dialect is `surrealdb`,
which `dialect.go` already held. Ken reviews the mapping below. Ken chose
three parts of it on 2026-10-01: no kind is a walk, a field is a column only
when a DEFINE FIELD names it, and the version is a probe of the major release
with the RPC answer parsed beside it.

SurrealDB has no relational catalog, and it records a schema all the same.
INFO FOR ROOT, INFO FOR NS, INFO FOR DB and INFO FOR TABLE return the
namespaces, the databases, the tables, the functions, the sequences, the
users, the fields, the indexes and the events. With STRUCTURE, each comes
back as an object with its parts, such as the `kind` and the `default` of a
field.

## INFO is a value from 3.0, and not on 2.x

On 3.x an INFO statement is a value that a SELECT reads, so `SELECT name FROM
(INFO FOR DB STRUCTURE).tables` lists the tables. `INFO FOR TABLE $t.name`
takes the table from a variable, so `array::map` over the tables reads INFO
FOR TABLE once for each table inside one statement. That is how every kind
below a table is one statement.

On 2.7.0 INFO is a statement and never a value. `RETURN (INFO FOR DB)`, `LET
$i = INFO FOR DB`, `SELECT * FROM (INFO FOR DB)` and `SELECT * FROM {INFO FOR
DB}` are each a parse error. The driver turns the one object INFO returns
into one row whose columns are its keys, so a bare INFO is not a row for each
table either. So 2.7.0 answers only the current schema, which needs no INFO,
and reports `TooOld` for every other kind. Gemini said the same, and said
that 3.0 made INFO a value.

## No kind is a walk

Ken ruled on 2026-10-01 that a walk (D146) is not allowed for SurrealDB. None
is needed, because `array::map` and INFO FOR TABLE do inside one statement
what a walk does with many. The cost was measured on 3.3.0 with 2000 tables
of 5 fields each: Tables took 0.02 seconds, Columns took 0.34 seconds for
the 10000 fields, and Columns took 0.08 seconds for one table, because a
kind below a table reads INFO FOR TABLE only for the tables that match its
table pattern. Constraints and ConstraintColumns read every table and match
the pattern after, because the server refuses a statement whose expressions
nest too deep, and those two reached that limit with the pattern inside.

## The mapping

| Kind | SurrealDB | Source |
| --- | --- | --- |
| `Databases` | every namespace on the server | INFO FOR ROOT |
| `Schemas` | every database of the namespace of the connection | INFO FOR NS |
| `CurrentSchema` | the database of the connection, in its namespace | `session::ns()`, `session::db()` |
| `Tables` | every table, with the type `table`, `view` or `relation` | INFO FOR DB |
| `Views` | a table defined AS SELECT | INFO FOR DB |
| `Columns` | a DEFINE FIELD | INFO FOR TABLE |
| `Indexes`, `IndexColumns` | a DEFINE INDEX and its fields | INFO FOR TABLE |
| `Constraints`, `ConstraintColumns` | a UNIQUE index, and the ASSERT of a field as a check | INFO FOR TABLE |
| `Triggers` | a DEFINE EVENT | INFO FOR TABLE |
| `Functions`, `RoutineParameters` | a DEFINE FUNCTION and its parameters | INFO FOR DB |
| `Sequences` | a DEFINE SEQUENCE, from 3.0 | INFO FOR DB |
| `Roles` | every system user on the root, on the namespace and on the database | INFO FOR ROOT, NS and DB |
| `RoleGrants` | the roles OWNER, EDITOR and VIEWER that each user holds | the same |
| `Privileges` | the PERMISSIONS of each table and of its fields | INFO FOR DB and TABLE |
| `Comments` | every table, field, index, event, function, param, analyzer and database user with a COMMENT | INFO FOR DB and TABLE |

### A namespace is the catalog, and a database is the schema

A namespace holds databases and a database holds tables, which is the shape
of a PostgreSQL database and its schemas. So Databases lists the namespaces,
Schemas lists the databases of the namespace the connection is in, and every
object names the namespace as its catalog and the database as its schema.

Every kind below a schema reads the database the connection is in, which
the path of the URL names, because INFO FOR DB reads the database of the
session and no statement reads another one. So a pattern that names another
database matches nothing. That is the case D162 met on Neo4j, where Schemas
returns the database of the connection alone. Here Schemas lists every
database of the namespace, because INFO FOR NS lists them and hard rule 13
does not withhold a fact. Ken can choose the Neo4j answer instead.

### A table is a table, and a view is a table defined AS SELECT

A table defined `AS SELECT` is a view. The server keeps its records from
the source tables and refuses a write to it, measured on 3.3.0. A table
defined `TYPE RELATION` holds the edges of a graph and has the type
`relation`. Every other table has the type `table`.

### Only a DEFINE FIELD is a column

Ken chose on 2026-10-01, for Neo4j first, that D47's cost test is strict. A
schemaless table holds whatever fields its records hold, and only the
records say what those are, so a field that no DEFINE FIELD names is not a
column. A field of a schemaless table that a DEFINE FIELD names is one.

A field of the items of an array, such as `tags.*`, is a DEFINE FIELD of its
own, which 3.x makes for `array<string>`, and it is a column. The record id
`id` is the key of every table. It is a column only when a DEFINE FIELD
names it, and then `primary_key` is true.

The type of a field is its TYPE as INFO writes it, and `any` for a field
defined with no TYPE, because such a field takes any value. A field is
nullable when it has no TYPE or a TYPE that names `none` or `null`, which 3.x
writes `none | string` for `option<string>`. `generated` is `s` for a VALUE
clause, which the server computes on each write, and `v` for a COMPUTED
field, which it computes on each read.

SurrealDB records no position for a field. INFO lists the fields by name, so
`ordinal` is the place in that order. The conformance report shows it.

### A UNIQUE index and an ASSERT are the constraints

A UNIQUE index is a unique constraint with the fields of the index. The
ASSERT of a field is a check with that one field, and it takes the name of
the field, because it has no name of its own. There is no foreign key. A
field of the type `record<book>` holds a link and the server does not check
that the record exists. 3.x has REFERENCE, which says what a delete does to
the records that link to the deleted one. It is close to a foreign key, and
it is left out because the server still takes a link to a record that does
not exist: a field with REFERENCE ON DELETE REJECT took `rbook:missing` on
3.3.0. Ken can choose otherwise.

### An event is a trigger

A DEFINE EVENT runs when a record of its table changes, with a WHEN and a
THEN. Its definition is the DEFINE EVENT statement, which INFO FOR TABLE
returns without STRUCTURE. SurrealQL has no clause that turns an event off,
so `enabled` is always `enabled`.

### A system user is the role

SurrealDB defines a system user on the root, on a namespace or on a
database, with one of three roles there: OWNER, EDITOR or VIEWER. Roles lists
the users of the three levels that reach the database of the connection, and
RoleGrants lists their roles. A user of one name on two levels is two rows.
An OWNER on the root is the superuser. A record user signs in through a
DEFINE ACCESS, is a record of a table, and is not a role.

### The PERMISSIONS of a table are its privileges

The PERMISSIONS of a table say what a record user can do with its records,
for select, create, update and delete, as FULL, NONE or WHERE and an
expression. Those of a field say the same for the field. Privileges returns
the first as `access` and the second as `column_access`, one line for each
field. A system user is held to its role and not to these, which is why
`bypass_rls` is true on every role. Ken can call this a stretch, because the
grantee is every record user and is not named.

## The columns arrive in the order of their names

The server sorts the keys of every object it returns, and dbimp's driver takes
the columns of a row from the keys (dbimp D52). No setting keeps the order of
a projection. So every query declares its fields in the order of their names,
and Scan reads them in that order. Couchbase 7.2 has the same fault, and D104
set a floor above it. Here every release has it, so the order of the names is
the order of the fields.

## Parameters are bound by name

SurrealQL has named parameters only, and `$1` is a parse error. dbimp's driver
refuses a positional argument for that reason (dbimp D50). So `Info.Named`
was added to the root package on 2026-10-01. Under it, a parameter is written
`$p1`, `$p2` and so on, and its value is passed as `sql.Named("p1", v)`, so
`Query.Build` still returns the values in order. `Info.Literal` was the other
choice. It was not taken, because the product can bind, and a value written
into a statement is a place where a quote can go wrong.

SurrealQL has no LIKE. A pattern is turned into a regular expression inside
the statement with `array::fold`, one character at a time, so that `%`, `_`
and a backslash mean what `dbmeta.Like` says. `TestSurrealDBPatterns` checks
it against `dbmeta.Like`.

## The version is a probe and the RPC answer

No SurrealQL statement returns the version of the server. `RETURN version()`
and `RETURN surrealdb::version()` are parse errors, measured by dbimp. The
driver reads it through the RPC method `version` with `surrealdb.Version`,
and usql prints it that way.

Ken chose on 2026-10-01 to do two things. The version query is a probe that
tells 2.x from 3.x and nothing finer: `RETURN IF (<set>[2, 1])[0] = 1 THEN '3'
ELSE '2' END`. 3.x sorts the members of a set and 2.x keeps their order, which
dbimp recorded on all four releases, and the probe parses on both lines, where
a probe that reads INFO as a value fails to parse on 2.x. So
`Dialect.Version` is never wrong on the major release. `ParseVersion` also
reads the RPC answer, such as `surrealdb-3.3.0` or
`surrealdb-3.1.6+20260813.cfbaec4`, so a caller that reads it gets the full
release. The tests read the RPC answer and check that the probe gives the
same major release. No fragment gates within 3.x today. One that does gates on
the full release, and the probe alone cannot select it.

## What is left unsupported, and why

Each of these has something close in SurrealDB, and each is a stretch:

| Kind | What SurrealDB has | Why it is not an answer |
| --- | --- | --- |
| `Settings` | DEFINE PARAM, a value that a query reads as `$name`, and DEFINE CONFIG, which sets up GraphQL and the HTTP API | a param is a value of the database and not a setting of the server, and a config is the shape of an interface |
| `TextSearchConfigs` | DEFINE ANALYZER, with tokenizers and filters | an analyzer has no parser or dictionary to report. D162 rejected the Neo4j analyzers for the same reason |
| `Extensions` | the modules and the models that INFO FOR DB lists | a model is a machine learning model that a query calls as `ml::name`, and neither is a package that adds objects to the catalog. The fixture builds neither, so neither is measured |
| `EnumValues` | a TYPE that is a union of literals, such as `"a" \| "b"` | it is the type of one field and not a named type |
| `AccessMethods` | the kinds of index, such as HNSW or FULLTEXT | they are a fixed set that no catalog lists |
| `ForeignServers`, `ForeignTables` | DEFINE API and DEFINE BUCKET | an API is an HTTP endpoint the server serves, and a bucket is a store for files |

`CurrentUser` is absent. No function names the system user a session signed
in as, `$auth` is NONE for a system user, and `$session` names no user,
measured on 3.3.0. DeepSeek offered `session::user()`, which is a parse error.

The rest are absent from SurrealDB: types, domains, collations, casts,
conversions, operators and their classes and families, languages, large
objects, event triggers, tablespaces, partitioned tables, publications,
subscriptions, foreign data wrappers, user mappings, default privileges,
extended statistics, column statistics, role settings, aggregates and the
other text search kinds. docs/COVERAGE.md holds what Gemini and DeepSeek
found.

## The dsn stays the HTTP address

dbimp's driver takes only the `surrealdb://` URL. The entry sets
`connectURL`, which D160 added for libSQL, so dbrun gives the tests the `url`
and connects `dbrun version` with it. The `dsn` stays the plain `http://`
address, as D109 says. Each principal keeps its `url`, which dbimp's CI reads.

D167 later removed the flag. The DSN of the entry is now the URL that the
driver takes, and the `http://` address is its `api`.

The four releases move from Staged to the cadence each recorded (D120):
2.7.0 and 3.3.0 to Tested, and 3.1.6 and 3.2.4 to Nightly.
