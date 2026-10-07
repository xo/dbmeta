# D173. The ODBC fallback belongs to the client

Status: Decided.

## The decision

`xo/odbc` is a pure Go `database/sql` driver for ODBC. On 2026-10-07 it
gained a way to read the catalog of any database that has an ODBC driver:
`GetInfo*` methods that wrap `SQLGetInfo`, and `Tables`, `Columns` and
`PrimaryKeys` that wrap the ODBC catalog functions. They sit on the driver
connection, behind `sql.Conn.Raw`. The odbc session suggested a dbmeta model
for a plain ODBC database, built on a small interface that dbmeta defines and
that the odbc connection satisfies.

Ken decided on 2026-10-07 that dbmeta builds no such model. The fallback for
a database that dbmeta has no model for belongs to the client that knows the
driver, which is usql, or another package downstream of the driver.

The reasons are in the design. dbmeta reaches a database only through its
queryer, which runs SQL, and these methods are not SQL. Such a model needs a
second, optional interface that hands over the raw connection, which changes
the one method that every model rests on. The gain is three of the 56 kinds,
for databases that have neither a model nor a standard information_schema, and
the client can use the odbc methods directly without dbmeta.

If a database later needs more than those three kinds, a model of its own is
the answer, as for every other product.
