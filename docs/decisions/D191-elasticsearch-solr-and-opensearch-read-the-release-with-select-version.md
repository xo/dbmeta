# D191. Elasticsearch, Solr and OpenSearch read the release with SELECT version()

Status: Amends D177, D179, D181 and D183, amended by D192.

## The decision

D183 said that the models of Elasticsearch, Solr and OpenSearch report an
unknown version until dbimp answers `SELECT version()`. dbimp v0.15.0 does. It
returns one row with one column named version, a string such as 9.5.3. The test
module now pins dbimp at v0.15.0, and each of the three models has a
`VersionQuery` of `SELECT version()` and one column. Each already had a
`ParseVersion`, which reads that column, so only the statement is new.

The unknown version is gone. The comments of the three models that described
it are gone. The tests no longer read the release over HTTP: the helpers
`esRelease`, `osRelease` and `solrRelease` are deleted, and the setup of each
fixture calls `Dialect.Version`. The HTTP helpers that the fixtures use stay.

The model has no query that depends on the release, so nothing else changes.

## Who gets the release

The driver reads `GET /` for Elasticsearch and OpenSearch, and
`GET /solr/admin/info/system` for Solr. Each needs a permission that the role
of the ordinary user does not hold, so the product refuses the user and
`Dialect.Version` returns the error of the product. This is the rule that D171
chose for Druid. It is a clear answer and not an unknown version.

| Release | Administrator | Ordinary user |
| --- | --- | --- |
| Elasticsearch 8.19.22, 9.4.6, 9.5.3 | the release | HTTP 403, `security_exception`, the action `cluster:monitor/main` |
| Solr 9.9.0, 9.10.1, 10.0.0 | the release | HTTP 403 |
| OpenSearch 2.19.6 | the release | HTTP 403, `security_exception`, the action `cluster:monitor/main` |
| OpenSearch 3.9.0 | the release | the release, from the header `X-OpenSearch-Version` |

dbimp named Elasticsearch 9.5.3, Solr 9.10.1 and OpenSearch 2.19.6 and 3.9.0.
This change measured the other five releases, Elasticsearch 8.19.22 and
9.4.6 and Solr 9.9.0 and 10.0.0, and they answer as their neighbors do. None
refuses the administrator and none gives a different answer to the ordinary
user.

The tests assert each row. The refusal tests of Elasticsearch and Solr read the refusal through
`Dialect.Version`, and D192 renamed them. The refusal test of OpenSearch is now
`TestOpenSearchVersionForAnOrdinaryUser`, because the answer depends on the
release: it asserts the refusal on 2.x and the same release as the
administrator on 3.x. Parity runs with the metadata that the administrator
built, as it does for Druid, so it does not read the version as a lesser
principal. The comments of the three parity targets say so, and
`testdata/parity.txt` does not change.

## What stays open

The ordinary user cannot read the release on Elasticsearch, Solr and OpenSearch
2.19.6. This change does not solve that, and Ken has not chosen. The options
are these:

1. Leave it as it is. The caller gets the refusal and can hand the release to
   `New` by hand, as D36 allows.
2. Grant the user the privilege `cluster:monitor/main` in the entry. That
   changes the principal that parity measures, so it hides what an ordinary
   user can do.
3. Use a probe of the major release, as D164 did for SurrealDB. No statement
   that a lesser principal can run is known to tell the releases apart, and
   this change did not look for one.
4. Ask for the release another way on OpenSearch 2.19.6. It sends no header,
   so no such way is known.
5. Let a model call HTTP. D183 declined it.

## SurrealDB and the Presto flavor of Trino

dbimp v0.15.0 also answers `SELECT version()` for SurrealDB and for the Presto
flavor of Trino. Neither model needs a change.

The SurrealDB model keeps its probe of the major release. D164 chose it on
purpose because it works on 2.x, where nothing else reads INFO as a value, and
it is correct for every principal. `ParseVersion` already reads a full
release, so the model can take `SELECT version()` too. Whether the major
release alone is still the better answer is Ken's choice, and this change does
not make it.

The Presto model keeps `SELECT node_version FROM system.runtime.nodes WHERE
coordinator = true LIMIT 1`, which works. `SELECT version()` can replace it,
but no measurement here shows that it is better.
