# D175. Elasticsearch and OpenSearch can walk SHOW statements

Status: Amends D146 and D159.

## The decision

D146 allowed a query to walk several SHOW statements for Impala alone, and
D159 allowed it for InfluxQL as well. Elasticsearch and OpenSearch read their
catalog the same way. Their SQL has `SHOW TABLES` and `DESCRIBE`, and no table
that a SELECT reads. Ken allowed on 2026-10-07 that both can walk.

Everything else in D146 holds. A walk is for a kind that no single statement
answers, and for nothing else. Its cost is written beside it: one statement
for the list of indexes, and one for each index where the walk goes deeper,
such as the columns of an index. A kind that one statement answers is one
statement. Solr and Drill are not covered here. Drill has an
INFORMATION_SCHEMA, and Solr has no catalog in SQL.

The dialects wait for dbimp to release the drivers and for dburl to name them
(D154). The survey of each product comes first, and it is recorded in the
decision of that dialect.
