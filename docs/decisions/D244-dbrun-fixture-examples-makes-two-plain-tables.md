# D244. dbrun fixture examples makes two plain tables

Status: Decided.

## The decision

`dbrun fixture examples <selector>` drops and makes two tables on each server it
names, so that a README can print the same example on every base database. usql asked
for it, and the name does not tie it to usql.

The tables are the same on every product:

- `authors (author_id INTEGER PRIMARY KEY, name VARCHAR(100) NOT NULL)` with the rows
  (1, 'Isaac Asimov') and (2, 'Stephen King').
- `books (book_id INTEGER PRIMARY KEY, author_id INTEGER, title VARCHAR(200) NOT NULL)`
  with a foreign key to `authors`, and the rows (1, 1, 'Foundation'),
  (2, 1, 'I, Robot'), (3, 2, 'Carrie') and (4, 2, 'Cujo').

The fixture drops the books first and then the authors, so two runs leave the same
rows. It works on PostgreSQL, MySQL, MariaDB, SQL Server, Oracle, SQLite and DuckDB. Any
other product, and any hosted service, gets a refusal.

## Why it is not the fixture of D53

The fixture of D53 exists for the metadata readers. It makes one of every object, and it
lives in a schema of its own. An example in a README needs two tables in the default
schema of the user in the URL, so that `select * from authors` runs with no schema.

## What differs by product

The drop is the only statement that differs. Oracle has `DROP TABLE IF EXISTS` only from
23ai, so its drop is a block that ignores ORA-00942 and works on every release. The
foreign key is a table constraint, because MySQL and MariaDB read a column constraint and
ignore it.

The URL of a MySQL or MariaDB server names no database, so it has no default schema. The
fixture makes the database `examples` and puts the tables there. A caller connects to
`.../examples` to read them unqualified. The URL of `dbrun` is not changed, because the
tests read it (D68).

The tables of SQL Server are in `dbo` of the database `master`, and the tables of Oracle
are in the schema `SYSTEM`, because those are the default schemas of the users in the
URLs.

## Embedded databases

`dbrun` links no cgo driver, so it runs the statements of SQLite and DuckDB with
`usql --file`, and `usql` must be on the path, as it must for `dbrun usql`.

## What was measured

The command ran twice on postgres-18, mysql-26.7, mariadb-13.0, sqlserver-2025,
oracle-26ai, sqlite3 and duckdb on 2026-10-31. Each run reported 2 authors and 4 books,
and a join of the two tables printed the same four rows on PostgreSQL, MySQL, SQL Server
and Oracle.
