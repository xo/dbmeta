# D183. Aliases are tables where SQL cannot tell them, and dbimp supplies the version

Status: Amends D176, amended by D191.

## The decision

Ken answered two questions on 2026-10-08 that D176 had settled the other way.

1. D176 said an alias is a view. Only Elasticsearch can tell an alias from an
   index in SQL, so only its model answers Views (D177). Solr lists an alias as
   a table with the same columns as its collection, and OpenSearch lists an
   alias on 2.19.6 as a base table and none on 3.9.0. Their models answer no
   Views, and an alias reads as a table where the product lists it (D179,
   D181). A model has only a `Queryer`, so an HTTP call that tells them apart
   is not available to it.
2. D176 said the model reads the release over HTTP. A model has only a
   `Queryer`, so it cannot. The models of Elasticsearch, Solr and OpenSearch
   report an unknown version, and the caller passes the number it read over
   HTTP to `ParseVersion`. Ken asked dbimp to support `SELECT version()` for
   any database that cannot return its release as a query. When dbimp does,
   each of the three models gets a `VersionQuery` of `SELECT version()`, and
   the test no longer reads the release over HTTP. Until then, the models
   keep the behavior that D177, D179 and D181 record.

## Why

The alternative for aliases was a root change that lets a model call HTTP,
and Ken did not choose it. The alternative for the version was to leave the
work to every caller. A statement that every driver answers is what a model
can use, and it keeps the knowledge of how to read a release in dbimp, which
knows the product.
