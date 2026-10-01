# D162. Neo4j maps a database to a schema and a label to a table

Status: Decided.

## The decision

Ken asked on 2026-10-01 for a Neo4j dialect. `models/neo4j` answers 17 of the
56 on 2026.09.0 and 13 on 5.26.31, through dbimp's neo4j driver, which dburl
names. The dialect is `neo4j`, which D109 already named. Ken reviews the
mapping below. Ken chose one part of it on 2026-10-01: Columns is not
answered, for the reason under Columns.

Neo4j has no relational catalog. A server holds databases. A database holds
nodes and relationships. A node carries labels, a relationship has one type,
and both carry properties. The catalog is a set of SHOW commands and a set of
procedures. A SHOW command takes YIELD, WHERE and RETURN, so it filters and
projects like a SELECT, and every kind is one statement. No kind is a walk
(D146).

## The mapping

| Kind | Neo4j | Source |
| --- | --- | --- |
| `Databases` | every database on the server, with `system` | SHOW DATABASES |
| `Schemas`, `CurrentSchema` | the database the connection is in, and only that one | `db.info()` |
| `Tables` | each node label and each relationship type, with the type `node label` or `relationship type` | `db.labels()`, `db.relationshipTypes()` |
| `Indexes`, `IndexColumns` | each index and its properties | SHOW INDEXES |
| `Constraints`, `ConstraintColumns` | each constraint and its properties | SHOW CONSTRAINTS |
| `Functions` | each function and each procedure | SHOW FUNCTIONS, SHOW PROCEDURES |
| `Aggregates` | each aggregating function | SHOW FUNCTIONS |
| `RoutineParameters` | the arguments of each, the return value of a function, and the columns a procedure yields | SHOW FUNCTIONS, SHOW PROCEDURES |
| `Roles` | each user | SHOW USERS |
| `RoleGrants` | each role a user holds | SHOW ROLES WITH USERS |
| `RoleSettings` | the home database of a user | SHOW USERS |
| `Privileges` | each graph, segment and resource, with every privilege on it | SHOW PRIVILEGES |
| `CurrentUser` | the user | SHOW CURRENT USER |
| `Settings` | each setting of the server | SHOW SETTINGS |

### A database is the schema, and only the connected one

A database has no namespace inside it. So the database is the schema, as on
ClickHouse, and the catalog is empty. A label and an index belong to one
database, and `db.labels()` and SHOW INDEXES read only the database of the
connection, which is the path of the URL. So Schemas returns that one
database, and every object names it as its schema. A caller that asks for
the tables of another database gets no rows, which is a wrong answer rather
than an empty one, so Schemas does not list the others. Databases lists them
all.

### A label and a relationship type are two kinds of table

A label groups nodes and a relationship type groups relationships, and an
index or a constraint is on one of them. `db.labels()` lists a label that some
node carries, from the count Neo4j keeps for each label, and not from the
data. On 5.26.31 the two procedures took 0.015 seconds together on an empty
database and 0.024 seconds with 8 million nodes and 4 million relationships.
So the cost does not grow with the data. A label that no node carries is not
listed, even when an index or a constraint names it.

A full text index can be on several labels. It is one row, with the labels
joined by `|`, as Cypher writes them, and a pattern on its table matches
either one. A token lookup index is on every node or every relationship, and
its table is empty.

A node key and a relationship key are unique and present on every node or
relationship, so their type is `primary key`. A uniqueness constraint is
`unique` and an existence constraint is `not null`. Every other kind keeps the
Neo4j name in lower case, such as `node property type`.

### A user is the role

PostgreSQL has one kind of principal, and Neo4j has two. A user logs in, holds
roles and cannot hold a privilege. A role holds privileges and cannot log in.
Roles returns the users, and a role appears in RoleGrants as what a user is a
member of and in Privileges as the grantee. The union of the two is not
possible: SHOW USERS and SHOW ROLES cannot be joined in one statement on any
release, measured on 5.26.31 and 2026.09.0. Couchbase made the same choice
for the same reason.

### The home database is a setting of a user

A user can have a home database, set with ALTER USER SET HOME DATABASE. A
session that names no database runs there. It is the one setting Neo4j keeps
for a user, and RoleSettings returns it as `home=<database>`. This is the
nearest thing to a role's `search_path` on PostgreSQL, and Ken can reverse it
if it is a stretch.

### Functions and procedures are routines of the server

A function and a procedure belong to the server and not to a database, so the
catalog and the schema are empty. Their namespace is part of the name, as in
`db.labels`. SHOW FUNCTIONS says which functions are built in, so the system
objects filter leaves them out. SHOW PROCEDURES does not say, so a procedure
is always listed.

## Two grammars

On 5.26, a SHOW command cannot be joined with anything. It cannot be in a
subquery or a UNION, and no UNWIND can follow it. Each was tried on 5.26.31,
and each failed with a syntax error. From 2026.05, in Cypher 25, SHOW INDEXES,
SHOW CONSTRAINTS, SHOW FUNCTIONS and SHOW PROCEDURES can be joined with other
clauses. The Cypher manual records it under 2026.05. SHOW USERS, SHOW ROLES
and SHOW PRIVILEGES still cannot.

Four kinds need it: IndexColumns and ConstraintColumns, which are one row for
each item of a list, and Functions and RoutineParameters, which join two SHOW
commands. Each gates on 2026.05 and starts with CYPHER 25, so that it runs in a
database whose default language is Cypher 5. 5.26 reports that it is too old
for those four. Every other kind is one statement on both releases.

A SHOW command on 5.26 takes a subquery expression in its WHERE and its
RETURN, so `COLLECT { CALL db.info() ... }` names the database in each row on
both releases.

## Patterns

Cypher has no LIKE. A pattern is turned into a Java regular expression inside
the statement, one character at a time, so that `%`, `_` and a backslash mean
what `dbmeta.Like` says they mean, and every other character is literal.
`TestNeo4jPatterns` checks it against `dbmeta.Like` on both releases.

## Columns

Neo4j keeps no catalog of the properties of a label or a type. The only source
is `db.schema.nodeTypeProperties()` and `db.schema.relTypeProperties()`, which
read every node and every relationship to find them. On 2026.09.0,
`db.schema.nodeTypeProperties()` took 0.37 seconds for 1 million nodes and
0.95 seconds for 4 million. Its cost grows with the data, and D47 forbids
that. Ken chose on 2026-10-01 to leave Columns unsupported for that reason.
The same test was applied to every other kind, and none of the others reads
the data.

The two procedures also report what the data holds and not what the schema
requires. A property that every node has today reads as mandatory, and the
type of each value is listed as it was found.

## What is left unsupported, and why

Each of these has something close in Neo4j, and each is a stretch:

| Kind | What Neo4j has | Why it is not an answer |
| --- | --- | --- |
| `AccessMethods` | the index provider of each index, such as `range-1.0` | SHOW INDEXES lists only the providers of indexes that exist, and no statement lists the rest |
| `TextSearchConfigs` | the full text analyzers, from `db.index.fulltext.listAvailableAnalyzers()` | an analyzer is a Lucene tokenizer with its filters, and it has no parser or dictionary to report |
| `Types`, `Domains` | the Cypher property types, and property type constraints | no catalog lists the types, and a property type constraint is a constraint, which Constraints returns |
| `Casts` | conversion functions such as `toInteger` | they are functions, which Functions returns, and no catalog lists a cast from one type to another |
| `ForeignServers`, `UserMappings` | a remote database alias, which names a database on another server and a user to reach it | an alias is a name for a database, and its user is a credential, not a mapping of a local role |
| `ColumnStats`, `EnumValues` | the values the properties hold | each reads the data, which D47 forbids |

The rest are absent from Neo4j: views, sequences, triggers, comments,
collations, conversions, languages, large objects, event triggers, operators
and their classes and families, extensions, extended statistics,
publications, subscriptions, tablespaces, partitioned tables, foreign data
wrappers, foreign tables, default privileges and the other text search kinds.
docs/COVERAGE.md holds what Gemini and DeepSeek found.

## The dsn stays the HTTP address

The model reads Neo4j through dbimp's driver, which takes only the `neo4j://`
form. The entry sets `connectURL`, which D160 added for libSQL, so dbrun gives
the tests the `url` and connects `dbrun version` with it. The `dsn` stays the
plain `http://` address, as D109 says, so what other projects read from
`dbrun dsn` does not change.

The two releases move from Staged to Tested, the cadence each recorded (D120).
