# D258. A binary value prints as psql prints bytea

Status: Amends D143.

usql printed a binary column with Go escapes, such as `\x15\x1C/\x00`. usql
first asked for the form of the own client of each product. Ken decided on
2026-11-04 that this is wrong. The output of usql must be the same on every
database and the same as psql, so one column of the northwind fixture prints
the same way on PostgreSQL, SQL Server and Oracle.

## The decision

Every dialect writes a binary value in one form: `\x` and the bytes in lower
case hex, as psql writes bytea with `bytea_output = hex`. An empty value is
`\x`. The form is not a setting of a model.

`Dialect.FormatBinary(ct, b)` writes that form when the column is binary and
returns false otherwise, so that a caller leaves the value alone. The caller
does not call it for a NULL, because the function cannot tell a NULL from an
empty value.

What differs by product is which columns are binary. `Info.IsBinary` holds a
function that says it from the type name that the driver reports, and
`BinaryNames(names...)` builds it. The name is needed because a driver scans
more than binary types into bytes. lib/pq scans `uuid` and `bit` as bytes, the
MySQL driver scans every text column as bytes, and go-mssqldb scans a
`uniqueidentifier` as bytes. A model with no `IsBinary` records nothing.

## What was measured

`TestFormatBinaryIsMeasured` in the `test` module reads a table with one row
through each driver that dburl names. It holds each binary type to the one form
and each other type to false.

| Dialect | Binary types, as the driver reports them |
| --- | --- |
| PostgreSQL, CockroachDB | BYTEA |
| MySQL, TiDB | BINARY, VARBINARY, TINYBLOB, BLOB, MEDIUMBLOB, LONGBLOB |
| SQL Server | BINARY, VARBINARY, IMAGE, TIMESTAMP, ROWVERSION |
| Oracle | RAW, LONG RAW, BLOB |
| SAP HANA | BINARY, VARBINARY, BLOB |
| Cassandra | BLOB |
| Trino, Presto | VARBINARY |
| Vertica | BINARY, VARBINARY, LONG VARBINARY |

Some details of what was seen:

- The MySQL driver reports TINYBLOB, BLOB, MEDIUMBLOB and LONGBLOB all as
  BLOB, and a JSON column as TEXT.
- go-mssqldb reports a rowversion column as BINARY. A `uniqueidentifier` is
  not binary here, and usql already prints it as a lower case UUID.
- xo/oracle reports the name of every type. go-ora reports none, so the model
  treats a column that scans into bytes as binary when the name is empty. That
  case was not run against go-ora, which the tests no longer use (D253).
- An Oracle RAW(16) key prints as `\x` and 32 lower case hex digits, the same
  as a bytea.
- The hdb driver scans a varchar into bytes. The name decides.

## The forms of the clients

An earlier version of this decision gave each model the form of its own client.
They are kept here as a record of what the clients do, and no code uses them.

| Product | Client | Form |
| --- | --- | --- |
| PostgreSQL, CockroachDB | psql, cockroach sql | `\x` and lower case hex |
| MySQL, TiDB, SQL Server, SAP HANA | mysql, sqlcmd, hdbsql | `0x` and upper case hex |
| Oracle | SQL*Plus | upper case hex, no prefix |
| Cassandra | cqlsh | `0x` and lower case hex |
| Trino, Presto | trino, presto-cli | lower case hex pairs with a space between them |
| Vertica | vsql | a byte that prints as itself, a backslash as two, any other byte as `\` and three octal digits |

The mysql client of MySQL 26.7 shows hex only in a terminal. The mariadb client
of 13.0 writes the raw bytes unless it has `--binary-as-hex`.

## What was not measured

These have no model for binary columns:

- SQLite. The driver reports no binary type name for a blob.
- ClickHouse. The String type has no binary name.
- Firebird. The driver reports a CHAR or VARYING with the character set OCTETS
  as text, and a BLOB is an identifier in isql, not its bytes.
- Databend, QuestDB, Hive, Impala, Avatica, SurrealDB, Drill, Druid, Exasol,
  CrateDB. They were not run.

Vitess and SingleStore copy the MySQL model, and ScyllaDB copies Cassandra. They
were not run.
