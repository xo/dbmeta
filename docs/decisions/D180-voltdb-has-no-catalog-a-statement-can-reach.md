# D180. VoltDB has no catalog that a statement can reach

Status: Decided.

## The decision

Ken decided on 2026-10-08 that every VoltDB kind is unanswered. There is no
VoltDB model, no `Binding` and no change to the root package. The VoltDB
releases stay Staged with the cadence Verified (D119, D120). The model waits
for a way to run a procedure or a way to filter a result, which
`docs/BACKLOG.md` records.

## Why

VoltDB has no `INFORMATION_SCHEMA` and no relation that a SELECT reads. Its
catalog is a set of system procedures. Three facts together stop a `Stmt`.

1. `@SystemCatalog` takes one argument, the name of a component, and nothing
   else. It has no WHERE and no pattern. A caller cannot ask for one schema,
   one table or one name. The only filter is one written in Go after every row
   has arrived.
2. `bind` in `query.go` reads every at sign in a statement as the start of a
   parameter name. The statement `@SystemCatalog` fails with
   `ErrUnknownParam`, and `@@` is not an escape.
3. A walk is allowed for Impala, InfluxQL, Elasticsearch and OpenSearch alone
   (D146, D159, D175). A walk of one statement for each kind answers, and
   it needs Ken to widen those decisions. He chose not to.

The Go driver is not the cause. `github.com/VoltDB/voltdb-client-go` v1.0.18 is
the package dburl names for the scheme voltdb. It is not a dbimp driver. It
reads the text it is given as the name of a procedure and the arguments as the
procedure's parameters. So `db.QueryContext(ctx, "@SystemCatalog", "TABLES")`
returns rows, and so does `db.QueryContext(ctx, "@AdHoc", "CREATE TABLE ...")`
for the fixture. Values arrive as `[]byte` for text and as integers for
numbers. A failed statement reaches the caller as an error from `rows.Err`
with the text "No valid table" or nothing, and the real message is in the
server log. The driver is not pinned in `test/go.mod`, because nothing uses it
yet. D154 applies when the model arrives.

## What was measured

Servers: voltdb-14.1.0 and voltdb-15.2.0, one at a time, with the entry in
`container/voltdb.go`. The administrator is `admin` and the ordinary user is
`dbmeta_user`, which holds the role `dbmeta_reader`, the permissions
`sqlread` and `defaultprocread`.

### Components of @SystemCatalog

The names come from the strings of `JdbcDatabaseMetaDataGenerator` in the
server jar, and each was run. An unknown name returns no rows and the error
"No valid table".

| Component | Rows are | Columns that matter |
| --- | --- | --- |
| `TABLES` | tables, views and streams | `TABLE_NAME`, `TABLE_TYPE` (TABLE, VIEW or EXPORT), `REMARKS` holds JSON such as `partitionColumn`, `sourceTable` and `drEnabled` |
| `COLUMNS` | one per column, not in ordinal order | name, `DATA_TYPE`, `TYPE_NAME`, `COLUMN_SIZE`, `NULLABLE`, `COLUMN_DEF`, `ORDINAL_POSITION`, `IS_NULLABLE` |
| `INDEXINFO` | one per index column | `NON_UNIQUE`, `INDEX_NAME`, `TYPE` (3 tree, 2 hash), `ORDINAL_POSITION`, `COLUMN_NAME` |
| `PRIMARYKEYS` | one per key column | `KEY_SEQ`, `PK_NAME` |
| `PROCEDURES` | user procedures and the generated ones for each table | `PROCEDURE_NAME`, `REMARKS` holds JSON with `readOnly` and `singlePartition` |
| `PROCEDURECOLUMNS` | one per parameter | name, type, `ORDINAL_POSITION` |
| `CLASSES`, `FUNCTIONS` | loaded classes and user defined functions | none in the fixture |
| `ROLES` | one per role | `ROLE`, `PERMISSIONS` as a comma list |
| `TASKS` | one per task | scheduler and schedule classes and their parameters |
| `TOPICS` | one per topic | stream and procedure names |
| `TYPEINFO` | the built in types | JDBC type information |
| `USERS` | users and their roles | not run, see below |

A component the documents and two models suggested does not exist: `VIEWS`,
`SEQUENCES`, `PERMISSIONS`, `SCHEMAS`, `CONFIG` and `CONNECTORS` all returned
"No valid table".

### As both principals

`@SystemCatalog` (every component run) and `@SystemInformation` (`OVERVIEW` and
`DEPLOYMENT`) answered the same rows for `admin` and for `dbmeta_user`. The
catalog is not filtered by who asks, so no parity difference was found. The survey did
not run `USERS` and `LICENSE`. The setting `users` of `DEPLOYMENT` already
lists each user and its roles for both principals.

### The version

`@SystemInformation OVERVIEW` returns `HOST_ID`, `KEY` and `VALUE` rows. The
row with the key `VERSION` is the release, 14.1.0 or 15.2.0, and `BUILDSTRING`
holds the same text. `EDITION` is Developer Edition. Both principals read it,
so the version is not an administrator only fact here. A model must send the
procedure and then keep one row, which again needs a filter.

### What the catalog lacks

- No schema, catalog or database name. `TABLE_CAT` and `TABLE_SCHEM` are NULL
  on every row.
- No foreign key. CHECK is parsed and ignored with the warning "The database
  software does not enforce check constraints". The catalog holds neither.
- No view definition text, no comment, no trigger, no sequence and no
  user defined type.
- No current user. `OVERVIEW` has no user row.
- A partial index shows no `FILTER_CONDITION`.
- A default is stored in an internal form, such as `CURRENT_TIMESTAMP:43`.
- A materialized view is a table of type VIEW with an index named
  `MATVIEW_PK_INDEX` whose column is the number 0.

### The cost check

3300 tables of five columns and one index each were made, with the cost test of
D47 in mind. One call of `TABLES` took 0.11 seconds and returned 3306 rows,
`COLUMNS` took 0.24 seconds for 16 522 rows, and `INDEXINFO` took 0.13 seconds
for 6610 rows. The cost grows with the catalog and never with a filter,
because there is none. The server is the limit before the call is. At 306
tables it logged that 3444 MB of Java heap was needed and 2048 MB was
available, and at 3307 tables the resident memory passed 80 percent and the
server went read only.

### The container entry

Three faults in `container/voltdb.go` were found and fixed. 14.1.0 refused to
start with "Command logging is not supported in the Developer Edition", so the
deployment file turns command logging off. 15.2.0 warned that `dbmeta_user` had
a role that did not exist, because the role came after the user, so the first
`voltdb init` now loads a schema that creates the role, and Init only checks
that it is there. Both releases then start with no warning or error in the log,
and the ordinary user reads and is refused a write.

dbimp asked for the HTTP and JSON interface on port 8080 as the second port and
the API address. Neither release has one. A deployment file that enables
`httpd` is dropped when init converts it to YAML, nothing listens on 8080, the
jar of 15.2.0 holds no class of the listener, and `OVERVIEW` lists no HTTP port.
The ports are the client port 21212, the admin port 21211, the internal port
3021, ZooKeeper 7181, metrics 11781, DR 5555 and topics 9092. So the entry has
no second port and no API address.

### The fixture

DDL works through `@AdHoc` with the same driver: tables, indexes, a unique
constraint, `PARTITION TABLE`, a view with an aggregate, a procedure, a
partitioned procedure, a role, a stream, and a task. A function, an aggregate
and a procedure from a class need a jar, and the image has no compiler, so a
fixture cannot build them. A topic did not parse as `CREATE TOPIC` on 14.1.0.
DECIMAL takes no precision and a TTL clause goes after the column list.

### What two models said

Gemini sorted the kinds correctly. DeepSeek flash ran out of tokens three
times, and DeepSeek Pro named four sources that do not exist (`VIEWS`,
`SEQUENCES`, `PERMISSIONS` and `USERNAME`). Each of those was run and refused.
Rule 14 asked for two models, and these are the two. No lead held beyond what
the measurements above found.

## What changes this

A way to run a procedure through a `Stmt`, such as a form of the statement
that `bind` leaves alone, together with a way to filter in Go. Then the model
can answer tables, columns, indexes, index columns, primary keys, routines,
routine parameters, roles, role grants, settings, types and partitioned
tables, from the sources above. `docs/BACKLOG.md` holds the item.
