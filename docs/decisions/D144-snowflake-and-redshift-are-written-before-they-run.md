# D144. Snowflake and Redshift are written before they run

Status: Decided.

Ken asked on 2026-09-30 for dbmeta to answer for Snowflake and Amazon
Redshift, because usql deletes its own readers in W21. Both are hosted, and
neither has an account or a cluster provisioned, so nothing here can run
them. Hard rule 9 says a query that has never run is not finished. Ken chose
to write both models from the vendors' documentation anyway, and to finish
them when a person provisions a connection string that dbrun resolves
(D117).

So `models/snowflake` and `models/redshift` exist, each with a fixture and
tests that skip until their connection string is set. README.md says
"Written, not run" beside each, docs/COVERAGE.md says the same, and each is
exempt from parity with that reason, which is to be replaced by a target
when the service is reached. The hosted entries are Verified, because a
model reads each (D119).

Snowflake reads INFORMATION_SCHEMA. Redshift reads the pg_catalog tables of
PostgreSQL 8.0, from which it was built. Every statement is expected to need
changes the first time it runs.
