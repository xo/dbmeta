# D163. ArangoDB maps a collection onto a table

Status: Amended by D168.

## The decision

Ken asked on 2026-10-01 for a dialect for ArangoDB. `models/arangodb` answers
5 of the 56 under the dialect `arangodb`, which dburl names, through
`github.com/xo/dbimp/arangodb` at v0.10.0, which dburl's scheme opens. The
release 3.12.12 takes the cadence it recorded while it was Staged, which is
Tested (D120). dbimp's driver takes only the `arangodb://` form, so the
entry sets `connectURL` (D160), and dbrun gives the tests the url. The dsn
stays the `http://` address for the administrator and for `dbmeta_user`.

D167 later removed the flag. The DSN of the entry is now the URL that the
driver takes, and the `http://` address is its `api`.

ArangoDB has no relational catalog. This decision proposes how its objects
map onto the kinds of dbmeta, and Ken reviews it. Each part was measured on
3.12.12 on 2026-10-01, as root and as `dbmeta_user`.

## What AQL can read

Every query is one AQL statement for the whole database. AQL reaches these
catalog sources and no others:

| Source | What it holds |
| --- | --- |
| `COLLECTIONS()` | the name and the id of each collection the user can see. No type |
| `SCHEMA_GET(name)` | the JSON schema rule of a collection, its level and its message, or null |
| `_aqlfunctions` | each user defined function: its name, its source and whether it is deterministic |
| `_graphs` | each named graph and its edge definitions |
| `_analyzers` | each analyzer this database defines, with its type and properties. The 13 built in analyzers are not in it |
| `CURRENT_DATABASE()`, `CURRENT_USER()`, `VERSION()` | the connection and the release |

AQL has no function that lists indexes, views or databases.
`INDEXES()`, `VIEWS()`, `DATABASES()` and `COLLECTION_TYPE()` are each error
1540, an unknown function. `_users` is in `_system` alone, and the ordinary
user cannot reach `_system`. The HTTP API describes all of these, one call
for each collection where it goes deeper. Ken chose on 2026-10-01 that a
walk (D146) is not allowed for ArangoDB, so a kind that only the HTTP API
answers is not answered.

## The mapping

| ArangoDB | dbmeta | Why |
| --- | --- | --- |
| database | catalog | A connection names one database in its path, and AQL cannot reach another one, which is how a PostgreSQL database behaves. `Table.Catalog` is `CURRENT_DATABASE()` |
| nothing | schema | AQL names a collection with no qualifier, so no level sits between the database and the collection. None is invented, as on Firebird (D74). `Schemas` and `CurrentSchema` are not answered |
| collection | table, of the type `collection` | `COLLECTIONS()`. It does not say whether a collection holds documents or edges, so every one is `collection` |
| property of a JSON schema rule | column | `SCHEMA_GET`, read from the collection's properties and not from its documents |
| JSON schema rule | check constraint | the one constraint ArangoDB checks on a write. A document that breaks it is refused with error 1620 |
| user defined AQL function | function | `_aqlfunctions` |
| `CURRENT_USER()` | current user | |

The alternative for a database was the schema, with an empty catalog, as
ClickHouse does. Then `Schemas` and `CurrentSchema` answer, but `Schemas`
names only the database of the connection, because AQL lists no others, and
no statement qualifies a collection by its database. The catalog is the
closer fit, and Ken can choose the other.

## A document attribute is a column only where a rule names it

ArangoDB is schemaless and keeps no catalog of the attributes of the
documents in a collection. Ken chose on 2026-10-01 that D47's cost test is
strict here, as it is for Neo4j: a kind whose only source reads the documents
is not answered. So a column is a top-level property of the collection's JSON
schema rule, in the order the rule lists it, and a collection with no rule
has no column.

`data_type` is the JSON schema type, or the names joined by a comma where the
rule lists several, and empty where the property names no type. `nullable` is
false where the rule requires the attribute and its type does not take null.
The rule checks a document as its level says, `none`, `new`, `moderate` or
`strict`, and no rule checks a document that was stored before it was set,
so a field description says that a stored document can still lack the
attribute. `primary_key` is true for `_key`, where a rule names it.

The cost is bounded by the catalog. On a database of 2000 collections, each
with a rule of three properties, the columns query took 11 milliseconds and
returned 6000 rows, and the tables query took 4 milliseconds. The plan of the
columns query reads no collection: it is an enumeration over a list and a
calculation for each collection.

## The rule as a check constraint

A collection with a rule has one constraint of the type `check`, with an
empty name, because a rule has none. Its definition is the whole schema as
JSON: the rule, the level, the message of a refused write and the type. A
rule is a check on the whole document rather than on named columns, so
`ConstraintColumns` is not answered, as on ClickHouse. If Ken holds this to
be a stretch, `Constraints` is left unanswered and the model answers 4.

## What is not answered, and why

- `Indexes`, `IndexColumns`, `Views` and `Databases`: only the HTTP API lists
  them, and a walk is not allowed.
- `Roles`, `RoleGrants` and `Privileges`: the users and their grants are in
  `_system`, which the ordinary user cannot reach.
- The type of a collection: `COLLECTIONS()` has none. `_graphs` names the
  edge collections that a graph uses, and an edge collection in no graph is
  missing from it, so it is not used.
- A graph: no kind fits it. Its edge definitions look like foreign keys, and
  ArangoDB does not enforce them on a write in AQL. An edge from `book` to
  `author` in a graph defined from `author` to `book` was accepted.
- An analyzer: CrateDB's analyzers were left unanswered for the same reason
  (D131). An analyzer of the type `text` splits, folds and stems, so it is a
  text search configuration, a parser and a dictionary at once, and fits no
  one of them. `_analyzers` also leaves out the built in analyzers.
- `RoutineParameters`: the parameters of a function are only in its
  JavaScript source.
- `EnumValues`: an `enum` in a rule is a check on one property, not a type.
- `Extensions`: a Foxx service in `_apps` is an application mounted at a
  path, and it adds no object to the database.

## Who is asking

The parity test has two principals besides root: `dbmeta_user`, with read
and write on the database, and `dbmeta_reader`, which the test makes with
read only access to the database and no access to the collection `note`.
`COLLECTIONS()` leaves out a collection the user cannot access, so the reader
gets fewer tables, columns and constraints than the administrator. The user
gets the administrator's answer to every query but the current user.
