# D149. Oracle and SingleStore answer extended statistics

Status: Decided.

## The decision

Two backlog items asked whether Oracle and SingleStore keep statistics over
several columns. Both do, and both models now answer `ExtendedStats`. They
were measured on 2026-09-30, Oracle on 11g and 26ai and SingleStore on 9.1.

## Oracle

`ALL_STAT_EXTENSIONS` lists each extension, which is a column group or an
expression that `DBMS_STATS.CREATE_EXTENDED_STATS` makes. D43's first pass
left it out, because the view holds an expression and `ExtendedStat` had no
field for one. D147 added `ExtendedStat.Definition`, which holds it.

An extension's statistics are those of a hidden column that has the
extension's name, so the statement joins `ALL_TAB_COL_STATISTICS` on that
name. The optimizer always counts the distinct values of an extension, so
`Ndistinct` is always true and `Kinds` starts with `d`. A frequency or top
frequency histogram on the hidden column is a list of the most common values
with their frequencies, which is what PostgreSQL's mcv kind holds, so `MCV`
is true and `Kinds` adds `m` when one exists. A hybrid or height balanced
histogram holds buckets and is not a list, so it does not count. Oracle has
no functional dependency statistic, so `Dependencies` is always false.

PostgreSQL declares the kinds when it makes the object, and Oracle decides
the histogram when it gathers the statistics. So on Oracle `MCV` says what
the last gathering made. The field's description says so.

The fixture makes a column group on author before it gathers the statistics.

## SingleStore

`ANALYZE TABLE ... CORRELATE COLUMN a WITH COLUMN b USING COEFFICIENT x`
declares how strongly a follows b, so that the optimizer does not treat a
filter on both as independent. That is the job of PostgreSQL's functional
dependency statistic. `CORRELATED_COLUMN_STATISTICS` holds each one, so the
model reads it with `Kinds` set to `f` and `Dependencies` true.

A correlation has no name, and `ExtendedStat.Name` is absent for it. A name
is not invented, because a caller cannot tell an invented name from a real
one. The name parameter matches only the empty pattern.

`CORRELATE COLUMN` refuses to run with no database selected, even on a
qualified table. So the fixture selects its database with USE before that
step, and the test runs the setup on one connection, which it discards at
the end, so that no other query inherits the database.
