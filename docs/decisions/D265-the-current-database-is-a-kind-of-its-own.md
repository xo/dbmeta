# D265. The current database is a kind of its own

Status: Decided.

The prompt escape `%/` of usql is the name of the current database in psql.
usql took it from the URL path, so on SQL Server with `?database=northwind` it
was empty, and after `USE master` on SQL Server or MySQL it kept the old name
(xo/usql#467). The maintainers decided on 2026-11-04 that dbmeta answers it.

## Decision

`CurrentDatabase` is a kind of its own, beside `CurrentSchema` and
`CurrentUser`. It answers one row, a `DatabaseName` with a name that is absent
when the connection has chosen no database. A product with no such notion has
no model for it, and `usql` falls back to the URL.

`CurrentSchema` was not enough. Its `Catalog` is the database for PostgreSQL
and SQL Server, and for the others it is `def` or empty.

| Product | Statement |
| --- | --- |
| PostgreSQL, CockroachDB | `current_database()` |
| MySQL, MariaDB, TiDB, Vitess, SingleStore | `DATABASE()`, which is NULL when none is chosen |
| SQL Server | `DB_NAME()` |
| Oracle | `SYS_CONTEXT('USERENV', 'CON_NAME')` from 12c, and `DB_NAME` before |
| SQLite, libSQL | the name of the first entry of `pragma_database_list`, which is `main` |
| DuckDB | `current_database()` |
| ClickHouse | `currentDatabase()` |

The file of a SQLite database is in the same pragma, and this kind does not
return it.

## Not modeled

Trino, Presto, Vertica, Snowflake and the others have a current catalog or
database, and their statements were not run here. They are left out until they
are measured.
