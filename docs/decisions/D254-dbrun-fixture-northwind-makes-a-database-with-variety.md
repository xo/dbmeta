# D254. dbrun fixture northwind makes a database with variety

Status: Amends D244, amended by D256, D257, D259 and D290.

usql asked for a database with many kinds of objects, so that it can test its
describe commands and other meta commands against more than two tables. Ken
decided on 2026-11-03 that dbmeta holds it as a fixture, for PostgreSQL and
SQL Server only.

## The decision

`dbrun fixture northwind <selector>` drops the database `northwind` and builds
it again on a PostgreSQL or a SQL Server server. Any other product is refused.
The role `northwind_reader` is part of it. On PostgreSQL a role belongs to the
whole cluster, so the SQL creates it only when it is missing, and a rebuild
keeps it.

The fixture has two files for each product, and they load in this order:

1. Northwind. It is Microsoft's sample database, with 13 tables, 16 views and 7
   procedures on SQL Server. The PostgreSQL file is a conversion of it: snake
   case names, the type names of PostgreSQL, the dates in the ISO form, and the
   foreign keys after the rows. The script that converted it is in
   `test/cmd/dbrun/northwind/convert/`, and dbrun does not run it.
2. A schema named `hr` with what Northwind lacks: Unicode, nulls and long
   text, computed and JSON columns, UUIDs, a sequence, triggers, a partitioned
   table, a role with grants, comments and several kinds of index. PostgreSQL
   also has an enum, a domain, a composite type, arrays, a materialized view
   and two extensions. SQL Server also has an alias type, a table type, a
   temporal table, a synonym, an indexed view and extended properties.

usql made both files by hand on PostgreSQL 18 and SQL Server 2025. The source
file of Microsoft had three faults, which usql fixed: two inserts into
`Orders` with a stray semicolon, and one insert with no semicolon after it.

## How it loads

The files are written for a client that splits at each semicolon. dbrun does
the same with a small splitter that keeps what is in a quote, a name between
brackets or double quotes, a comment, a dollar quoted string, and the body of a
CREATE or ALTER between BEGIN and END. Each statement goes to the server as
one batch, because SQL Server needs a view or a procedure to be alone in its
batch.

All statements run on one connection. The T-SQL file sets identity insert on
for one table before each insert, and SQL Server allows one table at a time, so
dbrun turns it off for the previous table first.

## What was measured

On PostgreSQL 18 the fixture loads 3433 statements and on SQL Server 2025 it
loads 3428. Each gives 91 customers and 830 orders, and a second run gives the
same result. The fixture needs the extensions `pg_trgm` and `citext`, which the
PostgreSQL image holds, and a release that has generated columns, which is 12
or later.
