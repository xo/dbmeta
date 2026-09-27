# D94. Couchbase runs under dbrun, and its model waits for the n1ql rewrite

Status: Amends D66, amended by D95, D96 and D104.

Ken asked on 2026-09-27 whether a Couchbase dialect can be built, or at least
whether `dbrun` can start Couchbase for the tests of the n1ql rewrite. The
second is done. The first is possible on the catalog and not yet on the
driver.

## dbrun starts it with no change to dbrun

`container/couchbase.go` names 7.2.9 and 8.0.3 as Tested and 7.6.12 as
Nightly, on the official image. `EVALUATION.md` has the floor. A bare tag is
the Enterprise edition, which is free for development and testing, and D90
says that qualifies.

A new node belongs to no cluster, which `EVALUATION.md` had recorded as a
person in the web console. It is not. `couchbase-cli`, which ships in the
image, makes the cluster with the data, index and query services and a bucket
named `dbmeta`, and `Init` runs it. It checks for each first, so it is safe on
every start, and it waits until the query service answers. `Ready` asks the
cluster manager on 8091 inside the container, which answers before any
cluster exists. Each release was up in about ten seconds.

Only 8093, the query service, is published, and the DSN names it. The driver
tries a DSN as a cluster address first, and a cluster address hands back the
container's own address for the query service, which the host cannot reach.

The releases are Tested and Nightly although no model reads them yet. A CI
job then starts each release and runs `Init` on a GitHub runner, and the n1ql
CI depends on exactly that. Verified was the other choice, and it needs a
section in `COVERAGE.md` that has nothing to describe yet.

## The catalog can answer

SQL++ reads a real catalog: `system:buckets`, `system:scopes`,
`system:keyspaces`, `system:indexes`, `system:functions` and the user and
role views, with `ORDER BY`, `CASE` and `UNNEST`. A bucket, a scope and a
collection map onto a database, a schema and a table. There is no column
catalog, because a document has no fixed shape, and `INFER` is a statement
rather than a relation.

## The driver cannot, today

`github.com/couchbase/go_n1ql`, the driver `usql` uses, was run against 8.0.3.
It has three faults, and any one of them breaks a model:

1. It returns the columns in name order rather than in the order the
   statement selects them. A query selecting name, namespace, bucket and scope
   returned bucket, name, namespace and scope. Every Scan here reads a column
   by its position.
2. It returns each value as JSON text, so a name arrives as `"dbmeta"` with
   its quotes.
3. A result of one column arrives as the whole object, such as
   `{"v":"8.0.3-5933-enterprise"}`.

`xo/n1ql` is rewriting the driver, and hard rule 10 requires the package that
`usql` uses. So the model waits for the rewrite, is written against it, and
moves `usql` with it, the way D93 moved the cql tests. The three faults went
to the `n1ql` session as requirements.
