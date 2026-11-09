# D290. The northwind fixture has an hr schema on four more products

Status: Amends D254 and D259.

D254 gave the northwind fixture a schema named `hr` on PostgreSQL, SQL Server
and Oracle. MySQL, MariaDB, DuckDB and SQLite had only the Northwind tables.
The maintainers asked on 2026-11-10 for as much of `hr` as each product can hold. Where a
feature is missing, the file says so in a comment and uses the nearest object
that the product has.

## Files

`dbrun fixture northwind` loads these files after the Northwind file:

- MySQL and MariaDB: `mysql-hr.sql`. MySQL then loads `mysql-only-hr.sql` and
  MariaDB loads `mariadb-only-hr.sql`, because the two differ. A source names
  those files in `more`, by the product, so one dialect can apply a file that
  its other flavor refuses.
- DuckDB: `duckdb-hr.sql`.
- SQLite: `sqlite3-hr.sql`.

The schema of MySQL and MariaDB is a database named `hr`, so `dbrun` drops it
and the role `northwind_reader` before it loads the fixture.

## What each product holds

MySQL and MariaDB hold the tables, the generated column, the check
constraints, the foreign keys, an enum column, a partitioned table, comments,
three triggers, a procedure, a function, a view, a role with grants and a
full text index. MySQL adds an index on an expression and a multi valued index
on a JSON array. MariaDB adds a sequence, an aggregate function and an
expression index that a virtual column holds.

DuckDB holds the schema, an enum, a type alias, sequences, a struct column, a
list column, an interval column, a virtual generated column, a macro, a view,
comments and an index on an expression.

SQLite holds the tables, the generated column, check constraints, a partial
index, an index on an expression, four triggers and a view. It has no schema
that a foreign key can cross, so the names start with `hr_`.

## What no product here can hold

- A domain, because none of the four has one. A check constraint holds the rule
  of `hr.email`, and DuckDB has a type alias with no rule.
- A composite type. The address is four columns, or a struct in DuckDB.
- A materialized view. A table that holds the rows of the query takes its place.
- A partial index, in MySQL, MariaDB and DuckDB.
- A trigger, a procedure or a stored function in DuckDB and SQLite. DuckDB
  writes the log rows that the triggers of the other products write, and its
  macro does the work of the function.
- Partitions, in DuckDB and SQLite.
- A foreign key across schemas in DuckDB, so `hr.staff.employee_id` has none.
- A comment, in SQLite.
- A role, in DuckDB and SQLite.
