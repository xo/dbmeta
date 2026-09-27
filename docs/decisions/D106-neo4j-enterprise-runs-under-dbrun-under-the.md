# D106. Neo4j Enterprise runs under dbrun, under the evaluation agreement

Status: Amended by D109.

dbimp asked on 2026-09-27 for a `dbrun` entry for Neo4j, which Ken named as
its third driver that day. dbimp's D59 chose the Enterprise Edition. Ken
agreed to the product, to the Enterprise Edition and to its licence on the
same day. dbmeta has no Neo4j model. The entry is for dbimp's tests, as the
SurrealDB entry is (D103).

## Ken accepted the evaluation agreement

The Enterprise image starts only when `NEO4J_ACCEPT_LICENSE_AGREEMENT` is
set. The value `eval` accepts the Neo4j Software Evaluation Agreement: 30
days, for internal development only. D90 counts an evaluation edition as
free for development and testing, so Neo4j qualifies. `EVALUATION.md` asks
for an accepted licence to be recorded, as D76 records the SAP licence.

The agreement describes a usage report, which is on by default. The entry
sets `dbms.usage_report.enabled` to false. With it on, the log of 5.26.31 said
"Anonymous Usage Data is being sent to Neo4j". With it off, that line is gone
on 5.26.31 and 2026.09.0, and `SHOW SETTINGS` reports false on both.

The Community Edition needs no agreement. It has one database and no roles,
so it has no ordinary user with a role such as `publisher`. That is why
dbimp chose the Enterprise Edition.

## The releases

5.26.31, the LTS line, is the floor, and 2026.09.0, the newest monthly
release, is the ceiling. Both are Tested, because dbimp needs both on every
push. 4.4.48 is still rebuilt, and its Enterprise image accepts only `yes`,
the commercial licence, so nobody can run it without paying. A monthly
release stops being rebuilt when the next one arrives, so the ceiling moves
each month.

## The setup

`Init` runs `cypher-shell` against the system database as `neo4j`. It makes
the database `dbmeta` with `IF NOT EXISTS WAIT`, makes `dbmeta_user` with
`CREATE OR REPLACE USER`, and grants it the role `publisher`. It ends by
signing in as `dbmeta_user` on `dbmeta`, so that a server that `dbrun`
reports as up serves the ordinary user.

An `ALTER USER` that sets the password it already has fails with "Old
password and new password cannot be the same", so it cannot reset the
password on every start. `CREATE OR REPLACE USER` resets the user and its
password every time, and the GRANT that follows gives the role back. A
second GRANT of the same role does not fail.

The readiness check asks the system database for its databases. `RETURN 1`
fails there, because the system database runs only system commands.

## What was measured

On 5.26.31 and 2026.09.0, each started fresh, then stopped and started again
so that `Init` ran twice:

- `Init` gives the same users, roles and databases on the second run.
- `dbmeta_user` reads and writes on `dbmeta` over HTTP, and it is refused
  CREATE INDEX with `Neo.ClientError.Security.Forbidden`.
- The JVM took a quarter of the 4 GB limit for the heap, and the image set
  the page cache to 512 MiB. The entry fixes both at those values, and the
  server used 1.7 GB after the setup.
- 5.26.31 answered in 32 seconds, so the default budget holds.

2026.09.0 marks the HTTP transaction endpoint, `/db/<name>/tx/commit`, as
deprecated, with "HTTP API is deprecated. It is replaced by Query API." The
Query API, `/db/<name>/query/v2`, answers on both releases. dbimp was told.

## What waits for dbimp

No `Dialect` is named yet, because dburl has no Neo4j scheme and dbimp
settles the URL in its own step 9. The server's `Dialect` is empty, so
`dbrun` names the test variable for the product, `DBMETA_NEO4J`. The `dsn`
is the plain `http://` address of the HTTP API, and there is no `url` field.
Both change when dbimp settles the name.
