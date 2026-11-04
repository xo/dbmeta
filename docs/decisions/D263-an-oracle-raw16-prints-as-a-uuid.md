# D263. An Oracle RAW(16) prints as a UUID

Status: Amends D258.

Oracle has no UUID type. It keeps a GUID, such as `SYS_GUID()` returns, in a
RAW(16). In the northwind fixture the column `staff_id` is a `uuid` on
PostgreSQL, a `uniqueidentifier` on SQL Server, which usql prints as a lower
case UUID, and a RAW(16) on Oracle. Ken decided on 2026-11-04 that the column
displays the same on every database. This is an exception to D258, and it is
the only one.

## Decision

`Info.IsGUID` is a function that says a column holds a GUID, from the type that
the driver reports. `Dialect.FormatBinary` writes a value of 16 bytes in such
a column as a lower case UUID, which is 8, 4, 4, 4 and 12 hex digits with
dashes, such as `5f6ce336-2b56-0fbd-e063-0501a8c0af0e`. Every other binary
value keeps the form of D258.

The Oracle model sets `IsGUID` for a column with these properties:

- The type name is RAW.
- The driver reports a length of 16, from `ColumnType.Length`.

The rules for the value:

- The bytes print in the order Oracle stores them, with no swapping. This
  differs from the `uniqueidentifier` of SQL Server.
- `SYS_GUID()` is not an RFC 4122 UUID, so the version bits are not checked.
- If the driver reports no length or another length, the value keeps `\x` and
  hex. A RAW of any other length keeps `\x` and hex.
- A value of another length in a RAW(16) column, which Oracle allows, keeps
  `\x` and hex.
- A RAW(16) that is not a GUID, such as an MD5 hash, also prints as a UUID. Ken
  accepts that, because the hex digits are the same.

## Driver

xo/oracle v0.2.0 reported no length for any RAW. v0.3.0 reports one through
`RowsColumnTypeLength`: 16 for `SYS_GUID()` and for a RAW(16) column, and 1 for
`HEXTORAW('AB')`. `TestFormatBinaryIsMeasured` in the `test` module reads a
RAW(8) and a RAW(16) from Oracle 26ai through that driver and holds the first
to `\x` and hex and the second to a UUID. A unit test in `models/oracle` builds
the same cases with a fake driver and needs no server.
