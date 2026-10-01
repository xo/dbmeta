# D55. The current user moves here. Changing a password does not

Status: Decided.

`usql` carries database specific SQL outside its metadata readers. All of it
was audited, and it is three hooks on `drivers.Driver` and nothing else.

| Hook | What it runs | Reads or writes | Moves |
| --- | --- | --- | --- |
| `Version` | `SHOW server_version`, `SELECT sqlite_version()`, five `SERVERPROPERTY` calls | reads | already here, D38 |
| `User` | `SELECT current_user`, and `SELECT user FROM dual` on Oracle | reads | yes, as `CurrentUser` |
| `ChangePassword` | `ALTER USER`, `ALTER ROLE`, `ALTER LOGIN` | writes | no |

Everything else on that struct is Go behavior rather than SQL: `Process`
rewrites a statement, `ColumnTypes`, `RowsAffected`, `Err` and the `Convert`
family read a result, and `Copy` generates inserts, which is data movement and
not metadata.

Two statements the audit turned up outside those hooks are already in scope.
The `Catalogs` readers for DuckDB and Trino run
`SELECT database_name FROM duckdb_databases() WHERE NOT internal` and
`SHOW catalogs`, and both are metadata, answered here by `Databases`.

## CurrentUser, and why it reports two names

`CurrentSchema` already exists and describes the connection rather than the
database. The current user is the same shape of question, so it is the same
shape of answer: one row, read with `First`.

It reports two names because most products have two. The effective user is who
the session acts as now and the session user is who it authenticated as. They
differ after `SET ROLE` on PostgreSQL, they differ on MariaDB when the
connection matched a wildcard host, and on SQL Server they always differ,
because a connection authenticates as a server login and acts as a database
user. Connecting as `sa` answers `dbo` and `sa`.

Both come from one statement on every product that has them, so D47 says
report both. `usql` reports one, which is the login on SQL Server, because
`ALTER LOGIN` is what its password change needs.

SQLite answers neither and the query is not registered there, so it reports
`ErrNotSupported` rather than inventing a name. DuckDB answers the fixed string
`duckdb` and has no session user, so that column is NULL under the padding
rule rather than a copy of the name.

The shared model gains it too, which takes it from 11 kinds to 12. It is the
one binding there that reads no view at all, because `CURRENT_USER` and
`SESSION_USER` are standard SQL expressions rather than `information_schema`
tables. Two lines of standard SQL for a whole kind is the cheapest addition
this model has had.

## Why changing a password stays in usql

D5 is the first reason and it is enough on its own. `dbmeta` reads. A
consumer can hand it a read only connection and reason about what it can do,
and one write ends that.

The second reason is the one that settles it even for somebody willing to
reopen D5. A password statement cannot bind a parameter. Both were tried on a
real server:

```
postgres=# PREPARE t AS ALTER USER postgres PASSWORD $1;
ERROR:  syntax error at or near "ALTER"

1> EXEC sp_executesql N'ALTER LOGIN sa WITH PASSWORD = @p', N'@p nvarchar(50)', @p=N'X'
Msg 102, Level 15, State 1: Incorrect syntax near '@p'.
```

Every statement here binds its arguments. Moving this one means `dbmeta`
interpolating a secret into SQL text, and carrying a quoting and escaping rule
per product to do it safely. That is a new class of risk in a library that has
none today, in exchange for moving one line per driver.

`RequirePreviousPassword` is a flag on the same struct and points the same way:
this is a client concern, and the client already knows how to prompt for it.

## The rule this sets

A statement moves here when it reads. A statement that writes stays with the
client, whatever it is about. That is D5 restated, and the audit found no case
that argues against it.
