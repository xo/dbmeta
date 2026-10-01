# D128. CockroachDB lists crdb_internal

Status: Decided.

Ken decided on 2026-09-29 that the CockroachDB model does not hide
`crdb_internal` when `with_system` is off.

The statements the model shares with the postgres model hide `pg_*` and
`information_schema`, as `psql` does. So `tables` lists the 110 to 117 virtual
tables of `crdb_internal` beside a user's tables, and that is what `psql`
shows on CockroachDB, which hard rule 2 follows.

Hiding it was rejected. No column of `pg_namespace` or `pg_class` marks a
schema that CockroachDB keeps for itself, and only its name does. So hiding it
needs a CockroachDB statement of the model's own for every query that filters
by schema, about thirty, in place of the shared ones (D123).
