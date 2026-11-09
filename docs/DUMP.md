# Dump and copy

Status: decided in D274 and D275 on 2026-11-08, and built for PostgreSQL, SQLite,
DuckDB and MySQL. The section "As built" says where the code differs from the
design below. This document is the design of the `dump` package. usql asked for a `\dump` command,
a `--dump` flag, a target dialect and a rewritten `\copy`, and this is the
dbmeta side. Where a claim comes from a measurement it says so. Where it comes
from memory it says that too, and the claim must be measured before it is built.

## As built

The names that the code has, where they differ from the design below:

- `dump.MakePlan(snapshot, target, policy)` is the plan, and it returns a `Plan`
  or a `*PlanError` that lists every column with no allowed level.
- `dump.Policy` has `Default`, `Kinds` and `Columns`, each an `Allow{Lossy, Text}`,
  and `Casts`. A `Cast` matches a column pattern and a source type text and names a
  kind.
- `dump.Support(dialect)` returns a `Capability` with `Source`, `Target` and the
  reason that each is not true. It replaces the `Dialect.Dump()` of the design,
  because a method cannot be added to a type of another package.
- `dump.NewLoader(snapshot, schema, table, target, policy)` returns a `Loader` with
  `Columns`, `SelectAll`, `Insert`, `InsertArgs`, `Args`, `BatchSize` and `Ordered`.
  `dump.Convert(value, from, to)` reads a value by its kind. The loader is the
  `dump.Loader` of "One API for dump and copy".
- A model registers `dump.RegisterTypes` and `dump.RegisterStyle` from `init`. The
  style holds the quoting, the literals and the syntax that differ by product.
- `dbmeta.ViewDependencies` is the reader of D278, for PostgreSQL.
- The MySQL model reads the rules of a foreign key, the expression of a generated
  column and whether a default is a value (D276).
- The published ports of `dbrun` start at 20000 (D279).

What a dump leaves out is in `docs/BACKLOG.md`, under Dump.

## What was decided

D274 and D275 hold the decisions. The rest of this document follows from them.

1. dbmeta builds the statements of a dump and a copy and runs nothing. It takes
   no connection for anything it builds. The client reads the rows, writes the
   file and runs the statements. This amends D5, D56 and D273.
2. The code is a package named `dump` in the root module. It uses the standard
   library and nothing else.
3. The first targets are PostgreSQL, SQLite, DuckDB and MySQL. SQL Server and
   Oracle follow. MariaDB follows MySQL and CockroachDB follows PostgreSQL.
4. dbmeta does no SQL translation, and neither does usql. A view, a function, a
   procedure or a trigger is produced as the text of the source database, marked
   as not translated, for every target. The client warns or stops. Producing the
   data has no such barrier.
5. A type with no equivalent in the target fails by default. The export stays
   possible, with flags that the client sets, and dbmeta reports what it did.
6. The idioms of a database serve the export only. A reader does not change, and
   usql prints a column as it does today.
7. `dump` takes no dependency. It names the Go types of the kinds as strings, and
   it has its own small ISO 8601 parser (D275).
8. The `Snapshot` is the format that dbtpl moves to later.
9. The idioms and the cast rules of "Conventions that are not types" are approved.
10. A lossy mapping never happens unless the client asks for it with an explicit
    option of `\dump`.
11. dbmeta adds the readers that the dump needs, such as `Dependencies`.
12. The snapshot reads get a cost test, as D47 requires, against a catalog with
    thousands of tables.

## What dbmeta has today

Readers, all streaming and all taking an `Args` filter:

- `Schemas`, `Tables`, `Columns`, `Views`, `Sequences` and `Triggers`.
- `Constraints` with `ConstraintColumns`, which give the position of each key
  column and the table and column that a foreign key points at.
- `Indexes` with `IndexColumns`, and `Index.Predicate` for a partial index.
- `Functions` with `RoutineParameters`. For PostgreSQL `Function.Definition` is
  the whole CREATE statement.
- `Types`, `EnumValues`, `Domains` and `Comments`.

Helpers: `QuoteIdentifier`, `QuoteLiteral`, `Info.Fold`, `Info.Placeholder`,
`Info.Syntax` and `Query.Support`, which says whether a model answers a kind.

What is missing:

1. A parsed type. `Column.DataType` is the text of the product, such as
   `character varying(10)` or `decimal(10,2) unsigned`. There is no length,
   precision, scale, array or unsigned flag.
2. A reader of rows. dbmeta reads metadata and no user data.
3. Dependencies between views, functions and tables. Foreign keys give the
   order of tables, and nothing gives the order of views.
4. Literals for data. `QuoteLiteral` handles a string and nothing else.
5. CREATE syntax. The statements built today are the password statement and the
   two statements behind `\gdesc`.
6. A default and a constraint that are portable. `Column.Default` and
   `Constraint.Definition` are the product text, such as `nextval('s'::regclass)`
   or `CHECK ((a > 0))`. They are right for the same dialect and wrong for any
   other.
7. A statement of what a product can dump, and what it can be dumped into.

## The package

`dump` has two halves.

The first half reads. `dump.Read(ctx, m, db, filter)` collects a `Snapshot` of
the schema into plain Go values, by calling the queries above. A schema is
small, so the snapshot is in memory. Row data is never in it.

The second half builds text. It takes a snapshot and a target dialect and yields
statements in order:

```go
type Statement struct {
	Stage        Stage  // Pre, Data or Post, as in pg_dump
	Kind         string // "table", "index", "sequence value", and so on
	Name         string // a stable name, such as schema.table.column
	SQL          string
	Note         string // a warning, such as a type that was not exact
	Untranslated bool   // the SQL is in the source dialect and was not translated
}

func Script(s *Snapshot, target dbmeta.Dialect, p Policy) iter.Seq2[Statement, error]
```

An iterator keeps the rule that a caller can tell the end of a result from a
failure part way through it.

## The snapshot and its format

The `Snapshot` is also the intermediate format that Ken asked for, so that a
schema can be written to a file and read back. The rules:

1. The `json` tags define the format. Every encoder writes the same names, so a
   JSON file, a YAML file from usql and a YAML file from dbtpl hold the same keys
   for the same schema. An encoder can be swapped without a change to the format.
   A field has one tag, and a tag never changes without a new `version`.
2. The `version` field is a whole number. A reader refuses a version that it does
   not know and says which one it found.
3. The snapshot is schema only. It never holds data, and nothing in it is
   specific to a target or a policy. The plan depends on both, so it is computed
   from a snapshot and is not in it.
4. The snapshot holds what an exact dump to the same dialect needs, because a
   file read back must give the same script as a live dump. It holds:
   - The source dialect and the version of the server.
   - For each type, the `Raw` text next to the kind.
   - Defaults, checks, generated expressions and the definitions of views and
     routines, as the text of the source.
   - Comments.
   - The collation of each column, so that the plan reports a dropped collation
     from a file as it does from a live source.
5. The order is stable. Tables, columns, constraints, indexes and the rest are in
   a fixed order, by schema and name and by position where a position exists. The
   same database written twice gives the same file, and a diff of two files is a
   diff of two schemas.

dbmeta writes the format as JSON, because `encoding/json` is the standard
library. YAML stays in usql and dbtpl, because the root module has no
dependency. usql writes the format with `\dump -format sql|json|yaml`, where sql
is the default, and the two others refuse a data option. Reading a file back
works only for a schema only dump, and it belongs with the phase for a target,
where it gives the tests a fixture that needs no server.

### Partitioned tables and materialized views

A partitioned table and a materialized view are objects of the snapshot (D285).

- `Table.Partitioning` says how a table is split, by strategy and key. For
  PostgreSQL each partition is a table of its own, and it has `Table.PartitionOf`,
  which names the parent and holds the bound. For MySQL and MariaDB the
  partitions live inside the table and are listed in `Partitioning.Parts`. The
  subpartitions of MySQL are not written, and `Skipped` says so.
- `Table.IsPartition` is true for a partition that is a table. The parent holds
  its rows, so a client that reads the rows of every table leaves a partition out.
- `View.Materialized` is true for a materialized view, and `View.Indexes` holds
  its indexes.
- A script for the same product writes the parent with `PARTITION BY`, and each
  partition with `PARTITION OF`, parents first. It does not write a key, an index
  or a trigger on a partition that the parent already makes there.
- A materialized view is written in the Post stage with `WITH DATA`, after the rows
  and after the views that it reads, and its indexes follow it. The snapshot does
  not say whether the source view had its rows, so a view that was empty on
  purpose is written with its rows.
- A script for another product writes each partitioned table as one table, and
  each materialized view as a plain view that reads its tables each time. A note
  on the statement says so.
- SQL Server and Oracle keep partitions as a storage choice with a function and a
  scheme of their own, so they read a partitioned table as a plain table.

### What dbtpl can reuse

Measured against dbtpl at commit `8366044`. The YAML template of dbtpl marshals
one value, `xo.Set`, with `goccy/go-yaml`. The `Set` holds queries, which are
specific to dbtpl, and schemas. A schema holds enums, procs, tables and views. A
table holds its columns, primary keys, indexes and foreign keys as nested lists.
dbtpl does not use dbmeta yet. It reads with five loaders of its own that run one
query for each table.

Reusable:

1. The readers. `docs/DBTPL.md` maps nine of dbtpl's eleven loader reads to
   dbmeta queries, and one query for a whole schema replaces the query for each
   table.
2. The snapshot. dbmeta keeps a child of an object as flat rows (rule 13). The
   nested shape of dbtpl comes from grouping columns by table and foreign key
   columns by constraint. That grouping is in the `Snapshot`, so dbtpl and the
   dump share the code that dbtpl now writes in `cmd/schema.go`.
3. The type model. `xo.ParseType` and the `Type` of the dump do the same job, and
   the dump version has more: a kind, the enum values, the zone, `Raw` and the Go
   type.

Not reusable: the YAML encoder, the queries half of the `Set`, the template data
that reads `xo.Schema`, and the fields that exist only to generate Go. dbtpl can
build an `xo.Schema` from a snapshot and keep its templates as they are. dbtpl
also has a `createdb` template of 641 lines that writes CREATE statements from an
`xo.Schema`. It overlaps the DDL builders of the dump. dbtpl can keep it until
the builders exist and are measured. Ken decided that the snapshot is the format that dbtpl
moves to later, and the move is dbtpl's work.

## The type model

dbmeta owns it, because it is database knowledge. A source type is parsed into

```go
type Type struct {
	Kind             Kind     // the kind names of dbimp, see below
	Bits             int      // 8, 16, 32 or 64 for an integer or a float
	Length           int      // of a string or a binary value, 0 for no limit
	Fixed            bool     // char and not varchar, binary and not varbinary
	Precision, Scale int
	Unsigned         bool
	Zone             bool     // a timestamp or a time that has a zone
	Elem             *Type    // for an array
	Values           []string // for an enum
	Raw              string   // the source text, kept
	Go               GoType
}
```

Each model gets two functions, `ParseType(text) Type` and `RenderType(Type)
(string, error)`. Every path goes through the neutral kind, so a pair of
products needs no table of its own.

### The kinds are the kinds of dbimp

The `Kind` list is the kind list of `docs/TYPES.md` in dbimp (read at commit
`0e48f09a`), restricted to what a relational database has: boolean, integer,
unsigned integer, float, decimal, big integer, string, binary, date, time of day,
time of day with offset, local timestamp, timestamp, interval, uuid, ip address,
json, array, set, map, tuple, vector, geometry, range, other. dbmeta does not make
a second list. dbimp already holds a rule for the Go type of each kind, and this
gives dbmeta and dbimp one answer for a type that both name. A char and a varchar
are the kind string with `Fixed` and `Length`, and a smallint and a bigint are the
kind integer with `Bits`.

### The Go type of a kind

`dump` does not import the Go types (D275). A `Type` carries a descriptor, and a
caller imports the package:

```go
type GoType struct {
	Import  string // "", "time", "github.com/cockroachdb/apd/v3"
	Package string // "", "time", "apd"
	Name    string // "int64", "Time", "*Decimal", "[]byte"
}
```

`Import` is empty for a type that Go predeclares, such as `bool`, `string`,
`int64` and `[]byte`. The name `builtin` is not used, because it is the name of a
documentation package that cannot be imported. A type of the standard library has
its real path, such as `time` for `time.Time`. `Package` is the name that the code
writes, and it is not always the last element of the path, because
`github.com/cockroachdb/apd/v3` is the package `apd`. `Name` carries a leading `*`
or `[]`, and a caller writes the type as that prefix, then `Package` and a dot,
then the rest of the name.

The table has one row for each kind and is data, in one place. These are the rows
that Go has no type for in the standard library:

| Kind | Go type | What it holds |
| --- | --- | --- |
| date | `dbimp.Date` | a day, with no time and no zone |
| time of day | `dbimp.LocalTime` | a time from 0 to 23 hours, no date, no zone |
| time of day with offset | `dbimp.OffsetTime` | a time of day and an offset in seconds |
| local timestamp | `dbimp.LocalDateTime` | a date and a time, with no zone |
| timestamp | `time.Time` | an instant |
| interval | `dbimp.Interval` | months, days and nanoseconds, kept apart |
| uuid | `uuid.UUID` | a UUID |
| decimal | `*apd.Decimal` | an exact number |

The other rows follow dbimp: boolean is `bool`, binary is `[]byte`, json is the
decoded value, and a float is `float64`, with `float32` for a 32 bit float. The
packages are `github.com/xo/dbimp`, `github.com/google/uuid` and
`github.com/cockroachdb/apd/v3`. usql confirmed on 2026-11-08 that its go.mod
holds the last two already, indirectly.

Each dbimp civil type writes ISO 8601 with `String`, reads it with a `Parse`
function, and is a `driver.Valuer` of that text.

Mappings by product, from memory and to be measured:

- PostgreSQL `timestamp` and MySQL `datetime` are a local timestamp. PostgreSQL
  `timestamptz`, MySQL `timestamp` and SQL Server `datetimeoffset` are an
  instant. SQL Server `datetime2` is a local timestamp, `time` is a time of day
  and `date` is a date.
- Oracle `DATE` holds a time, so it is a local timestamp. Oracle
  `INTERVAL YEAR TO MONTH` has months only, and `INTERVAL DAY TO SECOND` has days
  and nanoseconds only, so both fit `Interval`.
- A MySQL `TIME` of more than 24 hours is an `Interval` with nanoseconds and no
  days, because a `LocalTime` stops at 23 hours.
- A zero date such as MySQL `0000-00-00` is not a `Date`, because it is not a day
  of the calendar. It is a policy case: an error by default, and a null or text
  on request.

### Conventions that are not types

Some products have no type for a thing and use a convention. The catalog reports
the convention and cannot say what the author meant. `ParseType` reads text, so
it cannot decide these alone. Each one needs a rule that dbmeta holds and a
caller can change.

- MySQL and MariaDB: BOOL and BOOLEAN are `tinyint(1)`, and the catalog says
  `tinyint(1)` for both. A column that really holds a small number looks the
  same. `bit(1)` is also used as a boolean.
- MariaDB: JSON is `longtext` with a `json_valid` check. The snapshot reads the
  check and makes the column a JSON column (D284).
- MySQL: `char(36)` and `binary(16)` often hold a UUID. `year` is a 4 digit type.
  `enum` and `set` are column types and not named types.
- SQLite: a declared type is free text with an affinity. `BOOLEAN` is stored as a
  number, and a column can hold any kind of value.
- Oracle: `NUMBER(1)` and `CHAR(1)` with Y and N are used as booleans, and
  `NUMBER(p,0)` is the integer.
- SQL Server: `bit` is a real boolean, and `datetime` has a coarse precision.
  `rowversion` and the old name `timestamp` are an 8 byte binary value and not a
  time. The kind is binary, and it is read only.
- PostgreSQL `inet` and `cidr` are the kind ip address. `money` has a locale and
  is not mapped, so it is Text or an error.
- SQL Server `money` is `decimal(19,4)` and `smallmoney` is `decimal(10,4)`.
  Oracle `NUMBER` with no precision is a decimal with no limit.
- Auto numbering: PostgreSQL `serial` and `bigserial` are an integer with a
  sequence default, and the others are MySQL `AUTO_INCREMENT`, SQL Server
  `IDENTITY`, SQLite `INTEGER PRIMARY KEY` and Oracle identity columns. The kind
  is integer and `Column.Identity` says how it numbers, so the target writes its
  own idiom.
- Defaults: `CURRENT_TIMESTAMP`, `now()`, `GETDATE()`, `sysdate` and
  `SYSTIMESTAMP` are one neutral default, the current time. The same holds for the
  current date, `true` and `false`, a number and a string. Any other default is
  an expression and moves only to the same dialect, with a note. MySQL
  `ON UPDATE CURRENT_TIMESTAMP` has no equivalent and is a note.

A native type needs no idiom and maps by its name: PostgreSQL `uuid`, SQL Server
`uniqueidentifier`, DuckDB `uuid` and MariaDB `uuid` from 10.7.

The rules are a list of cast rules, in the way that pgloader has them. A rule
matches a source type, and optionally a table and column, and names the kind.
Each model ships defaults, and `Policy.Casts` replaces or adds to them. Two
defaults need saying:

- `tinyint(1)` to another dialect is a boolean, because the MySQL manual says
  that BOOL is `tinyint(1)` and that is the convention pgloader follows. To the
  same dialect or the same family there is no rule, and the type is written back
  as it was, so the round trip is exact.
- A `char(36)`, a `binary(16)` and an Oracle `RAW(16)` are a UUID only through a
  cast rule or a data check where every value matches the UUID form. A column name
  is never evidence. The Oracle rule of `Dialect.FormatBinary` that prints a
  `RAW(16)` with `id` in its name as a UUID is for display only (D258) and stays
  out of the export, because a guess from a name loses data.

The data side checks a rule that depends on the data. A conversion from a
`tinyint(1)` to a boolean accepts 0, 1 and NULL and returns an error for any other
value, so a column that held 2 stops the dump and does not turn into true. An
option runs one query for each candidate column, `SELECT MIN(c), MAX(c)`, before
the dump, so that the rule is dropped for a column that fails it. That query
reads user rows, so usql runs it from a statement that dbmeta builds.

Every rule that fires is in the plan, and in `Statement.Note`.

The idioms serve the export and nothing else. `ParseType` gives the kind boolean
for `tinyint(1)` and keeps the source text in `Raw`. `Column.DataType` still holds
the text of the product, so usql prints a `tinyint(1)` column as it does today, and
a caller that does not use `dump` never sees an idiom.

### What usql already knows

usql has very little type recognition, all of it for display and for scanning
without errors. It has no type model and no table to keep in step, so this
document holds the only table. Its rules, as it reported them on 2026-11-08:
FLOAT4 as a 32 bit float, OID, XID and CID as `uint32`, TIME and TIMETZ as strings
that print as psql does, UNIQUEIDENTIFIER as a type that prints in lower case,
text that parses as a time on SQLite, and a scan destination that takes NULL for
every driver. A uuid from PostgreSQL has no rule and prints as the driver's text.

The byte order of a SQL Server UUID is a trap for the data side. The 16 bytes that
go-mssqldb gives are not in the order of the text form. The conversion for the
kind uuid reads each driver's value by the type it has and writes the text form,
so a UUID from SQL Server and one from PostgreSQL are the same string. This is
measured by loading one value from each product and comparing the text.

## Types with no equivalent: the policy and the plan

A column has one of three levels, and dbmeta reports the level and never chooses
it on its own:

1. Exact. The target type holds every value of the source type.
2. Lossy. The target has a type of the same kind that holds less, such as a
   float64 that the target has only as a 32 bit float. The client must allow it.
   The note names the source type and the target type.
3. Text. The target has no type of the kind, so the string type of the target is
   used, such as `text` or `varchar(n)`. The client must ask for it, and the data
   side writes the value as its text form.

A `Policy` has three scopes: a default level, a level for each kind and a level for
each column. The most specific scope wins. usql first exposes one level for all
columns and one for each column, and the level for each kind stays in the API.
Two rules hold:

1. A level allows only itself. Allowing Text does not allow Lossy. Someone can
   want an array written as text and still refuse a float that loses digits.
2. For each column dbmeta picks the best level that is allowed, in the order
   Exact, Lossy, Text. It fails if none is allowed.

A planning step runs before the first statement:

```go
func MakePlan(s *Snapshot, target dbmeta.Dialect, p Policy) (Plan, error)
```

The plan holds:

- Every column, with its source type, its target type, its level, and the reason
  for a level that is not Exact.
- Every object that is not translated, which is a view, a function, a procedure
  or a trigger, with its kind and name, so that usql can warn or stop.
- Every column whose collation was left out, with the same weight as a level that
  is not Exact, because a missing collation can change uniqueness and sort order.

When a column has no allowed level, `Plan` returns an error that lists all of those
columns at once, so that one run tells the user every flag they need. usql then
writes the file whole or not at all, and a dry run prints the plan alone. Each
entry has a stable name, such as `schema.table.column`, so that usql can write it
into the script as a comment next to the statement it concerns. The same plan
drives `\copy` with a create option and the value conversion of each column, so
that `\dump` and `\copy` pick the same target type for the same column.

A dump to the same dialect has no level. Each type is written back as it was read,
which is Exact by definition. The dialect is the test, and dbmeta does not check
the version. PostgreSQL 18 to PostgreSQL 13 is the same dialect, and a type that
the old release lacks fails when the client runs the statement.

An enum has two exact targets. PostgreSQL gets `CREATE TYPE`, and any other product
gets a varchar with a CHECK constraint.

## DDL and order

dbmeta writes CREATE TABLE, CREATE INDEX, the constraints, the sequences and the
views from what it reads, once the type model exists. The order is the one pg_dump
uses, in three stages:

1. Pre: schemas, types, sequences, tables with their columns, defaults and primary
   keys.
2. Data: the rows, which usql supplies.
3. Post: the sequence values, unique constraints, indexes, foreign keys, check
   constraints that need data, triggers and views.

Foreign keys go last, so tables need no order and the load needs no switch for
foreign keys. A dump never turns checks off, except for SQLite, below.

Views have no edges today. The dump adds one reader, `Dependencies`, for the
products that record them: PostgreSQL has `pg_depend`, SQL Server has
`sys.sql_expression_dependencies` and Oracle has `all_dependencies`. Those three
are named from memory and must be measured. For a product that records none, views
come in name order and `Statement.Note` says that the order is not checked. usql
can run the failed ones again at the end.

SQLite needs its own order. It cannot add a foreign key to a table that exists, so
for a SQLite target the foreign keys go inside CREATE TABLE and not in the Post
stage. usql measured on SQLite 3.54.1 that a CREATE TABLE with a foreign key to a
table that does not exist yet succeeds, with foreign keys on or off. So the order
of the tables does not matter when they are created. Only the load needs an order,
or foreign key checks off. The plan for a SQLite target turns the checks off and
checks at the end:

1. `PRAGMA foreign_keys = OFF` at the start of the data stage. Children can then
   load before parents.
2. `PRAGMA foreign_keys = ON` and `PRAGMA foreign_key_check` in the Post stage.
3. The dump fails if `foreign_key_check` returns rows, because the data is not
   whole.

The pragma has no effect inside an open transaction. If the script wraps the load
in BEGIN and COMMIT, the first pragma must come before BEGIN.

## Data

dbmeta owns the literal for each kind in each target dialect. The input is a Go
value from `database/sql` and the `Type` of the column, because the value alone is
not enough: a `[]byte` is a binary value or the text of a number.

- Binary: hex, as `X'..'` for MySQL and SQLite, `0x..` for SQL Server, `'\x..'` for
  PostgreSQL and DuckDB, and `HEXTORAW` for Oracle.
- Dates and times: the target format, with the precision of the column and the
  zone when the type has one. A value arrives as a `time.Time`, as text, or as any
  `driver.Valuer` whose text is ISO 8601, which every dbimp civil type is. `dump`
  has its own small parser for the forms that databases write, and tests it against
  the text that each dbimp civil type writes. A year before 0001 or after 9999 is
  outside the standard library `time` forms, so the data side returns an error for
  it and the plan says so, unless the parser takes the signed forms that dbimp
  writes.
- Numbers: floats go through `strconv` with the shortest exact form. NaN and
  infinity are an error for a target that has no literal for them.
- Decimals arrive as text and stay as text, so nothing is lost.
- JSON, XML and UUID: a string literal with a cast where the target needs one.
- Interval: the target types differ most here. PostgreSQL keeps months, days and
  nanoseconds. A target with no such type gets Lossy or Text.
- NULL and booleans: the target spelling. SQL Server and Oracle write a boolean as
  1 and 0.

Two builders share the rules. `Insert(table, columns, rows)` writes literals, for
`\dump`, and `InsertArgs(table, columns, n)` writes the target placeholders, for
`\copy`. Both write a multi-row INSERT up to a batch size. The size also has a cap
for each target, such as 1000 rows in a SQL Server VALUES list, and the cap wins.
The batch size and the cap are measured for each product, which is work.

For order, `SelectAll(table, order)` builds the SELECT with the quoted names and an
ORDER BY the primary key, so a dump is the same twice. A table with no key has the
physical order and the statement list says it is not reproducible. dbmeta builds
the SELECT and usql runs it, because dbmeta reads no user rows.

Values after the data (Post stage): `setval` for PostgreSQL, a restart value for a
DuckDB and Oracle sequence, an update of `sqlite_sequence` for SQLite, an
`AUTO_INCREMENT` for MySQL, and for SQL Server `SET IDENTITY_INSERT` around each
table that has an identity column in the Data stage.

## One API for dump and copy

The neutral `Type`, the value conversion and the two INSERT builders are shared.
`\dump` calls `Insert` and writes the text to a file. `\copy` calls `InsertArgs`
and binds each row. A conversion between dialects, such as a boolean read as 0 or 1
from MySQL and written to PostgreSQL, is `Convert(v any, from, to Type) (any,
error)`. It is in dbmeta, because it is the same knowledge as the literal. The
builder type is `dump.Loader`, and not `Writer`, because AGENTS.md keeps that name
in usql.

## What cannot be dumped

`dump.Support(dialect)` has two answers: whether the product can be a source,
and whether it can be a target. A source needs the `Tables` and `Columns` readers.
A target needs the type functions, the builders and a statement of the syntax. The
reason is a constant, so usql prints it:

- No catalog reader: the model does not answer `Tables`.
- Not relational: Cassandra answers tables and columns, but the other document,
  graph, key value and search stores do not, such as Cosmos DB, DynamoDB,
  Elasticsearch, Neo4j, ArangoDB and SurrealDB.
- Read only: Cosmos DB, Druid, Drill, Athena and similar, which can be a source and
  never a target.
- Hosted API: BigQuery and Snowflake can be a source, and a target only when the
  DDL is built and measured.

`Query.Support` already answers the first reason for a model that is built. The rest
is new data on `Info`. The targets are the six products of the third decision, and
every other product is a source for its own dialect only.

## Scale and filters

Every reader returns an iterator and nothing in dbmeta holds a whole result, except
the snapshot, which is metadata. Rows are never collected: usql reads a row, calls
the builder and writes. The memory of a dump is the snapshot and one batch.

A filter is an include list and an exclude list of name patterns for schemas and
tables. usql exposes them as long options only, because psql already uses `-t`,
`-T` and `-N`. The `Args` patterns say what is included, and the exclude list has no
field today. The dump applies it in Go over the snapshot, because the table list is
small, and passes one include pattern to the query when there is only one. Columns
for every table in a schema come from one query, not one query for each table. This
still needs the cost test of D47 against a catalog with thousands of tables.

## What to copy and what to avoid

This part is from memory of these tools and has not been checked against them.
Treat it as a lead.

Copy:

- pg_dump: the three sections, constraints and indexes after the data, schema only
  and data only, include and exclude patterns, one consistent snapshot with a single
  transaction, and `--no-owner` as the default. A dump that carries owners and grants
  only restores for the same roles.
- mysqldump: multi-row INSERT as the default, and `--complete-insert`, which names
  the columns, as an option.
- sqlite3 `.dump`: the whole file in one transaction. It is a plain script, and that
  is the target for usql.
- pgloader: its casting rules for MySQL to PostgreSQL, such as `tinyint(1)` as a
  boolean, are the best table of defaults that I know of, and the cast rules can
  start from them.
- Atlas: it reads a schema into a neutral model for each dialect and renders it. That
  is this design, and its type handling is worth reading before the types are
  written.

Avoid:

- Version comments such as `/*!40101 ... */` and the `SET` lines of mysqldump. The
  script is for usql and must read the same everywhere.
- pg_dump custom and directory formats, expdp binary files and DuckDB `EXPORT
  DATABASE` with Parquet. A text script is the goal. A data format option can come
  later.
- Owners, grants, tablespaces and collations by default. They do not translate.
- Liquibase and Flyway. They run migrations and do not dump a database. Their diff
  model is a separate and larger job.
- sqlglot and a parser for views. Translating SQL text needs a parser for each
  product, and dbmeta has none.
- mssql-scripter: it needs the SQL Server object model. The same result comes from
  the readers here.

## Order of work

Step 2 is built: `Snapshot`, `Read`, the JSON format with the version field, and the stable
order, for PostgreSQL, SQLite, DuckDB, MySQL and MariaDB. `Read` makes one query for each kind
of object and not one for each table, and `Skipped` lists what it saw and did not read. The
cost test of D47 read 3000 tables of PostgreSQL 18, each with 5 columns and 2 indexes, in 0.42
seconds. The `Dependencies` reader for views is part of step 3, where the order of views is
needed. DuckDB reports no index columns and keeps no length for a varchar, and the tests say
so.

Step 1 is built for PostgreSQL, SQLite, DuckDB and MySQL, and tested on real columns.
It found two things that the text of a type cannot say. MySQL writes `int unsigned`
with no brackets, which the parser now reads, and MariaDB writes a JSON column as
`longtext`, so the kind json comes from the check constraint, which the snapshot
reads, and not from the text.

usql accepted three phases on its side. Phase 1 is a dump to the same dialect for
PostgreSQL, SQLite, MySQL and DuckDB. Phase 2 is the new `\copy`, which commits after
each batch. Phase 3 is a target that is another of those four, and reading a file
back. The dbmeta steps are:

1. The type model, with `ParseType` and `RenderType` for PostgreSQL, SQLite, DuckDB
   and MySQL, and the kinds written as tests of real columns.
2. `Snapshot`, `Read` and the JSON format, with the version field and the stable
   order.
3. `Script` for a dump to the same dialect, for PostgreSQL first, tested by loading
   the script back and comparing the snapshots.
4. The ISO parser, the literals and the two INSERT builders, tested by a round trip of
   every type in the fixtures.
5. `Plan` and `Policy`, and a target other than the source, one pair at a time.
6. `Dump()` support data, and the report for what cannot be dumped.

Each step is tested against a real server, as rule 9 requires, and each target needs a
parity run, as rule 16 requires.
