# D256. The northwind fixture is on Oracle too

Status: Amends D254, amended by D257 and D259.

D254 made `dbrun fixture northwind` for PostgreSQL and SQL Server only. The maintainers
decided on 2026-11-04 to add Oracle, because Oracle is the most like those two
and has the same kinds of object that the Northwind database shows.

## The decision

`dbrun fixture northwind <selector>` also works on an Oracle server. Oracle
calls a schema a user, so the fixture drops and makes two users, `northwind`
and `hr`, and the role `northwind_reader`. They have the password of the
containers. It runs as the administrator of the server and sets its current
schema, so it never connects as the new users.

The files are `oracle.sql` and `oracle-hr.sql` in `test/cmd/dbrun/northwind/`.
Each statement ends with a semicolon, and a PL/SQL unit ends with the semicolon
after its last END. usql applies the files and ends a statement at a semicolon
only, as psql does. The first version of the files ended each statement with a
line that holds only a slash, as SQL*Plus does, and dbrun split them itself.
The maintainers decided on 2026-11-04 to drop that form.

## How the files were made

`convert/nw2ora.py` converts the PostgreSQL file, and `convert/ora-tail.sql`
holds the seven procedures, written as PL/SQL functions that return a cursor.
dbrun does not run either. The conversion changes these things:

- The types: `VARCHAR2`, `CLOB` and `BLOB`, `NUMBER` for money and for
  `SMALLINT`, and `BINARY_FLOAT`.
- DEFAULT goes before NULL and NOT NULL. The check that uses `now()` is gone,
  because Oracle takes no function that changes in a check.
- A second index on the same columns is left out, because Oracle refuses it.
- The pictures are binary strings of up to 22 kilobytes, and a statement of
  Oracle holds no literal that long. Each insert with a picture is a PL/SQL
  block that adds the bytes in pieces of 2,000.
- The identity columns go on from the largest key with `START WITH LIMIT
  VALUE`, which needs Oracle 12.2 or later.
- A view and a function cannot have the same name in Oracle, so the function
  `sales_by_category` is `sales_by_category_for_year`.

The schema `hr` is written by hand. It has an object type and a varray, a
virtual column, a column that checks for JSON, a RAW key with a default of
`SYS_GUID()`, a sequence, a table in two partitions, triggers, a package, a
function, a procedure, a view, a function based index and comments. In the
schema `northwind` it adds a materialized view, a synonym and the role with
grants. Oracle has no enum, domain or partial index before 23ai, so a check
constraint and a function based index stand in for them.

## What was measured

On Oracle 19c the fixture loads 3,435 statements and gives 91 customers and 830
orders, and every object is valid. A second run gives the same result.
