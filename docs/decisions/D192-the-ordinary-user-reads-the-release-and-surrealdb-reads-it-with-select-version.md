# D192. The ordinary user reads the release, and SurrealDB reads it with SELECT version()

Status: Amends D191, D176 and D164.

## The decision

Ken decided two things on 2026-10-08.

First, the ordinary user of the Elasticsearch, OpenSearch and Solr entries can
read the release, so `Dialect.Version` works for every user. D191 left this
open and listed five options. Ken took the second: grant the right in the
entry. The refusal that D191 recorded is gone.

Second, the SurrealDB model reads the release with `SELECT version()`. dbimp
v0.15.0 answers that statement for SurrealDB, and this change measured it. It
replaces the probe of the major release that D164 chose.

## What each user gets

The administrator and the ordinary user read the same release on every release
of the three products. Parity does not change, because no query other than the
version query needs the new right, and `testdata/parity.txt` is the same.

| Product | What the entry grants the ordinary user | Releases |
| --- | --- | --- |
| Elasticsearch | the cluster privilege `cluster:monitor/main` in the role `dbmeta_role` | 8.19.22, 9.4.6 and 9.5.3 |
| OpenSearch | the cluster permission `cluster:monitor/main` in the role `dbmeta_role`, beside `cluster:monitor/health` | 2.19.6 needed it. 3.9.0 gave the release to every user already |
| Solr | a rule in security.json that lets the roles `search` and `admin` read `/admin/info/system` | 9.9.0, 9.10.1 and 10.0.0 |

Elasticsearch accepts the action name `cluster:monitor/main` in the `cluster`
list of a role. It is the action behind `GET /`, and it is the smallest right
that works. The named privilege `monitor` holds it and many more actions,
which the user does not need.

Solr has no predefined permission for the path, so the rule is a path rule:
`{"name": "system-info", "collection": null, "path": "/admin/info/system",
"role": ["search", "admin"]}`. Two details matter. A rule with no `collection`
key applies only to a request that names a collection, and
`/admin/info/system` names none, so the rule needs `"collection": null`. Solr
also takes the first rule that matches, so the rule must come before the rule
`all`. Measured on 9.9.0, the ordinary user gets HTTP 200 for
`/solr/admin/info/system` and still gets HTTP 403 for the collections API, the
metrics, the cores, the thread dump and the properties.

The right is wider than the release alone. `GET /` on Elasticsearch and
OpenSearch also returns the cluster name and the cluster identifier. The Solr
path also returns facts about the JVM, the host and the memory. The entry
grants that one action or path and nothing else.

The tests assert it. `TestElasticsearchVersionForAnOrdinaryUser`,
`TestOpenSearchVersionForAnOrdinaryUser` and `TestSolrVersionForAnOrdinaryUser`
read the release as both principals and compare the two. The Solr test also
checks that the collections API is still refused with HTTP 403. The two
refusal tests of D191 for Elasticsearch and Solr are renamed and rewritten
into these.

## SurrealDB

The server refuses `SELECT version()`, because SurrealQL has no such function.
The driver of dbimp v0.15.0 answers it with the RPC method `version`, as one
row with one column named `version`. This change ran it as the root user and as
the ordinary user on every release in `container/surrealdb.go`. Both principals
got the same answer on each release.

| Release | Answer |
| --- | --- |
| 2.7.0 | `surrealdb-2.7.0` |
| 3.1.6 | `surrealdb-3.1.6+20260813.cfbaec4` |
| 3.2.4 | `surrealdb-3.2.4+20260803.93ab219` |
| 3.3.0 | `surrealdb-3.3.0` |

`ParseVersion` already read these answers, so the model needed only the new
`VersionQuery`. The probe `RETURN IF (<set>[2, 1])[0] = 1 THEN '3' ELSE '2'
END` is gone from the model, with its two cases in the unit test and the
comparison of the major release in the integration test. The helper that
reads the RPC method stays in the test module. `TestSurrealDBVersion` uses it
to check that `SELECT version()` gives what `usql` prints.
`TestSurrealDBVersionForAnOrdinaryUser` is new.

The version is now the full release. D164 said that a fragment that gates
within 3.x needs the full release, which the probe did not give. It does now.

## What changes in earlier decisions

D191 left the ordinary user without the release on Elasticsearch, Solr and
OpenSearch 2.19.6, and kept the probe for SurrealDB. This decision replaces
both. D176 gave the OpenSearch role its wider permissions, and this adds one
permission to it. D164 chose the probe, and `SELECT version()` replaces it.
The rest of each decision stands.
