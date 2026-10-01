# D168. Ken reviews the six dialects of 2026-10-01

Status: Amends D163 and D164.

## The decision

On 2026-10-02 Ken reviewed the mappings that D160 to D165 proposed for
libSQL, YDB, Neo4j, ArangoDB, SurrealDB and InfluxQL, and chose the
following.

A database that holds tables directly is a schema, and the catalog is empty.
MySQL and ClickHouse already do this, and so do Neo4j and InfluxQL. ArangoDB
changes to match. D163 had made its database the catalog, with no schema
level, as on Firebird (D74). So ArangoDB now answers Schemas and
CurrentSchema, from `CURRENT_DATABASE()`, and answers 7 kinds rather than 5.

Schemas lists only the schemas whose objects the other kinds read on the
connection. Neo4j already does this. SurrealDB changes to match: its other
kinds read only the database of the connection, so Schemas lists that one
alone, and Databases still lists every namespace. A schema whose tables are
never returned is a wrong answer.

The series index of InfluxDB is a catalog. SHOW TAG KEYS reads it and not
the points, so Columns on InfluxQL passes D47's cost test. On InfluxDB 1 it
grows with the number of series, 0.122 seconds at one million, and the cost
is written beside the query (D165). Neo4j stays as Ken chose on 2026-10-01,
because its properties are known only by reading every node (D162).

The version of InfluxQL stays as D165 has it. No statement names the
release, so a caller passes the release it reads from `GET /ping`, as usql
does (D166). Without it, Roles, Privileges and Settings report Supported on
InfluxDB 2, and running one returns the server's own error. The field
descriptions and COVERAGE.md say so.

Three choices stand as proposed:

- The JSON schema rule of an ArangoDB collection is a check constraint,
  because the server enforces it (D163).
- YDB gives a second partial answer under D45, as D161 proposed: Tables
  leaves out views, and Schemas leaves out empty directories.
- libSQL's two fixes are fragments in the sqlite3 model, behind the libSQL
  key, rather than statements of its own (D160).

`Info.Named` stays, and `ConnectURL` is replaced by the `api` field (D167).
