# D257. The northwind values agree on the three servers

Status: Amends D254 and D256, amended by D259.

The maintainers found that the PostgreSQL fixture differed from the other two. The table
`hr.staff` had one column `started` of the type `tstzrange`, which prints as
`["1992-05-01 00:00:00+00",)`, where SQL Server and Oracle had the columns
`started` and `ended`. The maintainers asked on 2026-11-04 to compare every value of the
fixture on the three servers, directly and as usql prints it.

## What changed

- `hr.staff` on PostgreSQL has the columns `started` and `ended`, both
  `timestamp(0)`, like SQL Server. Oracle had `started_on` and `ended_on`, and
  now has the same two columns with the same type.
- The money views on PostgreSQL gave whole dollars. The T-SQL script divides a
  price by 100, converts it to money and multiplies by 100, and the money type
  of PostgreSQL has cents only, so the division lost the rest. `nw2pg.py` now
  writes the product of the price, the quantity and the discount, which is
  money rounded to cents already. It also subtracts the discount in single
  precision, as SQL Server does.
- The same views on Oracle rounded to four places, where SQL Server rounds to
  cents. `nw2ora.py` now rounds to two places, and subtracts the discount in
  single precision before it makes the product a double.

## How the comparison works

`TestNorthwindServersAgree` in the `test` module reads every table and view of
the fixture from PostgreSQL, SQL Server and Oracle, writes each value as text so
that the same fact is the same text, and compares the servers in pairs. It
needs `DBMETA_POSTGRES`, `DBMETA_SQLSERVER` and `DBMETA_ORACLE`, and skips
otherwise. It compares these as text in the same way: a CHAR with its padding
gone, a number with no sign for money, no separator and no trailing zero, a
real as a single precision number, and JSON as a parsed value, because
PostgreSQL sorts the keys and SQL Server drops the spaces.

Every table and view has the same rows on the three servers, with these
exceptions that the fixture makes on purpose: the extra columns of `hr`, the
tables that only one product has, and the generated values of a key, a time or
a row version.

The columns that the views compute from a price, a quantity and a discount are
compared with a tolerance of 2 cents or 0.02 percent. Where the exact value is a
half cent, such as 921.375, SQL Server goes down because of its float
arithmetic, and PostgreSQL and Oracle go up. The servers cannot be made to agree
without copying the noise of one.

## What usql prints

usql prints these differences, which are in the drivers and the products, and
not in the data:

- Money is `$62,000.00` on PostgreSQL, `62000.0000` on SQL Server and `62000`
  on Oracle.
- A NUMBER of Oracle loses its trailing zeros, so a `number(14, 2)` prints
  `1312500`, where the other two print `1312500.00`.
- A `real` of SQL Server prints as a double, `4.900000095367432`, where the
  other two print `4.9`.
- A flag is `true` on PostgreSQL and SQL Server and `1` on Oracle.
- `discontinued` is a `smallint` on PostgreSQL and a `bit` on SQL Server, so
  usql prints `0` and `false`.
- A RAW, a bytea and a varbinary print with Go escapes. D258 gives them
  one form.
- A JSON column has spaces on PostgreSQL and Oracle and none on SQL Server.

## A fault found on the way

On Oracle 19c, reading a CLOB column that has an IS JSON check through
xo/oracle fails with `TTC error: received code 0 during response reading`.
`hr.departments.settings` and `hr.events.detail` are the two cases, and an
ordinary CLOB column reads. The test reads the settings as text and leaves
the events out.
