# D188. No dialect for H2, VoltDB, chai or csvq

Status: Amends D158, amended by D194.

## The decision

Ken decided on 2026-10-08 which databases that usql supports get no dialect.

1. H2 gets none. usql will drop H2, and the one driver that dburl names for
   it, h2go, fails against both releases (D158).
2. VoltDB gets none. Its catalog has no source that a statement can reach
   (D180).
3. chai and csvq get none. They are embedded databases with no server.

Ken asked for dialects for DynamoDB, Apache Pinot, Apache Avatica and
GizmoSQL. With those, every database that usql supports has a dialect, except
the four above, the hosted services without a model, and ODBC, whose fallback
belongs to the client (D173).

The hosted services without a model are Athena, BigQuery, Cosmos, Databricks,
MaxCompute, Tablestore and Spanner. Snowflake has a model that has not run.

## What changes

The backlog items for H2 and VoltDB are gone. The entries of H2 and VoltDB in
`container/` stay Staged until Ken says to remove them.
