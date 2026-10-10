# D212. Firebird and MariaDB enforce every constraint, ArangoDB counts, and Redshift reads SVV_TABLE_INFO

Status: Amends D205, D207, D209 and D210.

## The decision

Ken decided four things on 2026-10-10. Each fills a field that an earlier
decision left NULL and listed as open in `docs/BACKLOG.md`.

1. Firebird fills `Constraint.Enforced` with true for every constraint. D209
   left it NULL.
2. MariaDB fills `Constraint.Enforced` with true, from the MariaDB version key
   (D44). D205 left it NULL.
3. ArangoDB fills `Table.Rows` from `COLLECTION_COUNT`. D210 left it out.
4. Redshift fills `Table.Size`, `Table.Rows` and `Table.Options` from
   SVV_TABLE_INFO. D207 left them NULL.

## Enforced is a fact of the product

Firebird cannot disable a constraint that it records, so every one is
enforced. No catalog column says so. MariaDB from 10.6 has no ENFORCED column
and checks every CHECK, key and foreign key that it records. In both products
the value is a genuine true and not an unknown one (`docs/NULLS.md`), so the
statement selects the literal `TRUE`, and a comment says why.

On MariaDB the fragment gates on `Gate{Key: MariaDB}`. MySQL keeps its
ENFORCED column from 8.0.16 and NULL below that. TiDB, SingleStore and Vitess
did not change. The MariaDB against MySQL compare no longer lists `enforced`
as product specific, because both products answer true on the fixture. It
passed on MariaDB 13.0 against MySQL 26.7.

## ArangoDB counts

`Tables` adds `rows: COLLECTION_COUNT(c.name)` to the statement that lists the
collections, so the count comes in the same statement and there is no round
trip for each collection. The value is exact. Measured on one server with
1500 collections, as the server timed it:

| Read | Without | With |
| --- | --- | --- |
| all 1500 collections | 1.9 to 2.1 ms | 10.6 to 12.0 ms |
| one collection by name | not measured | 0.85 ms |

A cluster counts with a round trip for each shard. That cost was not
measured. The field description says so. The cost check stays at 1500
collections.

## Redshift reads SVV_TABLE_INFO

D207 left the view out for four reasons. The first is that Redshift refuses
it to every user who is not a superuser. Ken decided that the model reads it
and that the administrator grants SELECT on it. The measurement came first:
`GRANT SELECT ON svv_table_info TO` a made user works, and the user then reads
the view. Redshift Serverless 1.0.477953 accepted it.

The statement now reads the view as a derived table, joined to `pg_class` by
table id. Five things were measured, and the first three are the defects that
D207 found.

1. The leader node functions `pg_get_userbyid` and `obj_description` are
   refused beside the view. The owner is a join to `pg_user` and the comment
   is a join to `pg_description` with the class oid 1259, which is `pg_class`.
2. A name comes back padded to its type beside the view, and `LIKE 's5'` with
   no wildcard then matches nothing. The statement trims the schema and the
   name before it filters and before it returns them.
3. The view lists a table only when the table holds a row. An empty table has
   no row, so its size, rows and options are NULL. They are unknown and not
   zero. The fixture inserts one row into `author` and one into `events` so
   that a test reads values, and `book` stays empty.
4. A bare boolean bind parameter is refused beside the view, and so is a cast
   of `relkind` to text. The system filter reads `@with_system = TRUE`, and
   the type expression has no ELSE branch. The WHERE clause leaves only tables
   and views.
5. The size is in blocks of 1 MB. `Table.Size` is the blocks times 1048576, a
   bound. `Table.Rows` is `tbl_rows`, which counts rows marked for deletion
   until a vacuum. `Table.Options` is `diststyle=KEY(event_id),
   sortkey=happened`, with the sort key part left out when there is none. A
   view has none of the three.

### The cost of the decision

A user without SELECT on SVV_TABLE_INFO gets an error from the whole `tables`
kind: "permission denied for relation svv_table_info". It does not get NULL
fields. A consumer must grant `SELECT ON svv_table_info` to every user that
reads tables, and a superuser needs nothing. A role or a group can hold the
grant. The test `TestRedshiftTablesNeedSelectOnTableInfo` shows the error and
then the same user reading rows after the grant.

The read costs a fixed amount, which does not follow the rows returned.
Measured with six tables in the schema: 0.27 s without the view, and 0.77 s
for all six tables or for one table by name. D207 measured 0.79 s for 202
tables.

## Parity and conformance

The parity test grants SELECT on the view to the owner, the grantee and the
stranger where it makes them, and revokes it before it drops them. Redshift
refuses to drop a user that holds a grant. The parity golden did not change
for Redshift, because every principal reads the same rows and values as the
administrator once it holds the grant. The conformance golden did not change.

## Releases run

Firebird 3.0, 4.0 and 5.0. MariaDB 10.6, 12.3 and 13.0, and MySQL 8.4 and
26.7. ArangoDB 3.12.12. Redshift Serverless 1.0.477953. MariaDB 10.11, 11.4 and
11.8 and MySQL 9.7 were not run. Their statements are the same as on the
releases that ran.
