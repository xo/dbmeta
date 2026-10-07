# D187. GizmoSQL shares the DuckDB model and the tests open the session

Status: Decided.

## The decision

Ken asked on 2026-10-08 for a GizmoSQL dialect. `models/gizmosql` answers 20
of the 56 on GizmoSQL 1.40.0 and 1.41.0, through the Arrow Flight SQL driver in
`github.com/apache/arrow-go/v18`, which dburl v0.46.0 names for the scheme
`gizmosql` and its aliases `gz` and `gizmo`. The dialect is `gizmosql`. Ken
reviews the mapping below.

GizmoSQL is a server for Arrow Flight SQL. Its engine is DuckDB, or SQLite when
the server starts with `--backend sqlite`. The entry in `container/` starts
the default, which is DuckDB 1.5.6 on both releases. So the catalog is DuckDB's
own: `duckdb_tables()`, `duckdb_columns()` and the other table functions, and
every statement of `models/duckdb` runs through the Flight SQL driver.

## How the code is shared

The model imports `models/duckdb` and shares each of its 20 bindings with
`Query.Share`, the way `models/libsql` and `models/rqlite` share SQLite's. No
statement is copied. The fixture is the DuckDB fixture, as a value in
`models/gizmosql/fixture`. A Go package that two models import adds
nothing that `Share` does not already do.

Two changes went into `models/duckdb` for this.

1. GizmoSQL attaches a database of its own, `_gizmosql_system`, which holds
   two views for the Flight SQL metadata calls: `gizmosql_index_info` and
   `gizmosql_view_definition`. DuckDB does not flag it internal. So the helper
   that excludes system objects returns a `Choice`, and its fragment for the
   version key `duckdb.GizmoSQL` also excludes that database by name, unless
   the caller asks for the system objects. The `gizmosql` model records the
   key with an unknown version, as `libsql` does.
2. The `types` filter of Tables reads `',' || ? || ','`. DuckDB cannot infer
   the type of a bare parameter in a `||` chain when it prepares the
   statement. GizmoSQL then reports every parameter of the statement as a
   string, and the driver refuses the boolean one with "invalid value type
   bool for builder *array.StringBuilder". The statement now casts the
   parameter to VARCHAR. That is valid DuckDB and changes no answer.

## The version

`SELECT version()` returns the release of DuckDB, such as `v1.5.6`. No SQL
statement names the GizmoSQL release. The Flight SQL call `GetSqlInfo` names
the server `gizmosql` and the engine `duckdb v1.5.6`, and not the release of
GizmoSQL either. The model reports the DuckDB release, as `libsql` reports
SQLite's, and the display line says "GizmoSQL, which runs DuckDB v1.5.6".
usql has no `Version` for its gizmosql driver, so it runs the generic
`SELECT version();` and the answer is the same, when its driver works.

## The tests open the session, and that is a fault in the driver

GizmoSQL gives a session at the Flight SQL handshake and refuses any call that
does not carry it, with "No session ID in request context". The driver that
dburl names never makes the handshake. It sends the user and the password on
every call, so a connection opens and every statement fails. This is the fault
D118 found in usql's driver, and usql keeps its gizmosql driver in the bad
group because of it. It is in `arrow-go`, and the fix belongs there.

The driver does accept a token in its DSN. So `test/internal/gizmosql` makes
the handshake with the Flight client of the same module and puts the token it
gets in the DSN. The tests and `dbrun version` use it. Nothing else about the
driver changes, and D154 still holds: the driver is the one dburl names. A
caller that has a working session needs none of this, because the model takes
only a `Queryer`.

`google.golang.org/grpc` is a direct requirement of the test module for this,
and so is `arrow-go`, at the versions that were already there.

## What the driver cannot read

Measured on 2026-10-08. The driver returns a typed Go value for every column
whose Arrow type it knows: strings, booleans, signed integers, floats, timestamps
and times as `time.Time`, and a blob as bytes. It
returns `ColumnTypeDatabaseTypeName` empty for every column. It refuses these
types with "type ...: not supported": `Date32`, `Uint64`, a list, a map, a
struct, a dictionary, which is an enum, and an interval. It reads `HUGEINT` and
`DECIMAL` as a `float64`. No statement of the model returns one of the
refused types, and `scanEveryQuery` checks that. A trailing semicolon is
taken, so the syntax flags are DuckDB's and the terminator is kept.

## The principals

The core has one user, `admin`, and DuckDB has none. The other roles need an
identity provider or an enterprise license, and nobody here signs up for one.
So `parityExempt` names GizmoSQL with that reason, and the question whether an
ordinary user sees another catalog is not measured. It stays open until
somebody provisions a second principal. `gizmosql_metrics()` is refused to the
administrator with "requires the 'metrics' Enterprise license feature".

## Flight SQL has metadata calls that SQL does not reach

`GetCatalogs`, `GetDbSchemas`, `GetTables`, `GetPrimaryKeys`, `GetImportedKeys`
and `GetSqlInfo` are calls of the protocol, and a SQL statement cannot make
them. `GetTables` answers the tables of the connection from the same DuckDB
catalog. The model does not use them. It has only a `Queryer`, and the SQL
catalog answers every one of these questions with more fields.

## Mapping

| GizmoSQL | Kind | Why |
| --- | --- | --- |
| a DuckDB schema | schema | `duckdb_schemas()` |
| an attached database | database | `duckdb_databases()`. `_gizmosql_system` is system |
| a table or a view | table, view | `duckdb_tables()`, `duckdb_views()` |
| the other 15 kinds | the same as DuckDB | the statements of `models/duckdb` |

The other 36 kinds are unsupported, for the reasons in the DuckDB
section of `docs/COVERAGE.md`. Gemini and DeepSeek named no source that holds.
See the GizmoSQL section of that file.
