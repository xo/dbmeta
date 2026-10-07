# D159. InfluxQL can walk SHOW statements

Status: Amends D146, amended by D175.

## The decision

D146 allowed a query to walk several SHOW statements for Impala alone.
InfluxQL, the language of InfluxDB 1 and 2, has the same shape. It reads
metadata only through SHOW statements, such as SHOW MEASUREMENTS, SHOW FIELD
KEYS and SHOW TAG KEYS, and each one reads a single database, which ON names.
No statement reads every database at once. Ken chose on 2026-10-01 to allow a
walk for InfluxQL too.

Everything else in D146 holds. A walk is for a kind that no single statement
answers, and for nothing else. Its cost is written beside it: one statement
for each database, and one for each measurement where the walk goes deeper.
A kind that one SHOW statement answers is one statement.

InfluxDB 1 reads the grants of one user at a time, with SHOW GRANTS FOR, and
each grant is READ, WRITE or ALL on one database. Ken allowed on 2026-10-01
that Privileges walks one statement for each user as well, so InfluxQL
answers it on InfluxDB 1. InfluxDB 2 has no SHOW GRANTS.

Neo4j does not need this. Its SHOW commands take YIELD and WHERE, so each
kind is one statement.
