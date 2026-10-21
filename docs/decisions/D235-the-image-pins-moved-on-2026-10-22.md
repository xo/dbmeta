# D235. The image pins moved on 2026-10-22

Status: Decided.

## The decision

Ken asked for a check of every image registry and for the pins to move where the
coverage rules say so. Four read only checks ran on 2026-10-22. Each product
followed the rule that its own file in `container/` records, which is step 2 of
`docs/EVALUATION.md` or the rule for an image that is never rebuilt (D112). The
tier or the cadence of every release stayed as it was. Where a list holds two
releases and one moves, the tier moves with the place of the release, so the
ceiling stays Tested.

## What moved

| Product | Old | New |
| --- | --- | --- |
| CockroachDB | 26.2.7, 26.3.2 | 26.2.8, 26.3.2 |
| Vitess | 23.0.7, 24.0.4 | 23.0.8, 24.0.5 |
| SurrealDB | 2.7.0, 3.3.0 and 3.1.6, 3.2.4 | 2.7.0, 3.3.4 and 3.1.6, 3.2.6 |
| CrateDB | 6.3.7, 6.4.5 | 6.3.7, 6.4.6 |
| Databend | 1.2.881, 1.2.951 | 1.2.881, 1.2.954 |
| Qdrant | 1.18.3, 1.19.2 | 1.19.2, 1.20.0 |
| Dgraph | 25.3.8, 25.4.1 | 25.4.1, 25.5.0 |
| Cosmos DB emulator | EN20260907 | EN20261019 |
| PostgREST | 14.18, 16.4 | 14.18, 16.5 |
| Neo4j | 5.26.31, 2026.09.0 | 5.26.32, 2026.10.0 |
| rqlite | 9.4.5, 10.5.2 | 9.4.5, 10.6.3 |
| Meilisearch | 1.53.3, 1.54.3 | 1.54.3, 1.55.0 |
| GizmoSQL | 1.40.0, 1.41.0 | 1.41.0, 1.42.0 |
| Milvus | 2.6.24, 3.0.2 | 2.6.25, 3.0.3 |
| Stardog | 12.0.4, 12.1.4 | 12.1.5, 12.2.0 |
| MongoDB | 8.0.32, 8.3.11, 9.0.2 | 8.0.34, 8.3.13, 9.0.4 |
| OpenSearch | 2.19.6, 3.9.0 | 2.19.7, 3.9.0 |
| GraphDB | 11.4.3, 11.5.1 | 11.4.3, 11.5.2 |
| Weaviate | 1.39.10, 1.40.0 | 1.39.13, 1.40.2 |
| Trino | 476, 483 | 476, 484 |
| CouchDB | 3.4.3, 3.5.2 | 3.4.3, 3.5.3 |
| Elasticsearch | 8.19.22, 9.4.6, 9.5.3 | 8.19.23, 9.4.8, 9.5.5 |
| InfluxDB 3 | 3.10.6, 3.11.6, 3.12.0 | 3.10.6, 3.11.7, 3.12.1 |
| TerminusDB | 11.1.17, 12.0.7 | 11.1.17, 12.0.8 |
| DynamoDB Local | 3.2.0, 3.3.1 | 3.2.0, 3.3.2 |
| Apache Impala | 4.4.1, 4.5.2 | 4.4.1, 4.5.3 |
| Spanner emulator | 1.5.58 | 1.5.59 |

InfluxDB 1 and 2 did not move, and neither did the CockroachDB release 24.3.36.

## What was not run

No release in the table was started before its pin moved. CI runs the Tested
tier on the next push, and it is the first run of every new release in that tier.
Some moves carry a caution.

- Dgraph 25.5.0 is a new minor line. The setup was measured on 25.3 and 25.4.
- Stardog 12.2.0 is a new minor line. The commands in `container/stardog.go` were never
  measured, because no license file is provisioned.
- The notes on what the Cosmos DB emulator answers were measured on EN20260907. The
  registry gives no date for a tag, so the date comes from the name of the tag.
- Ken accepted the license of DynamoDB Local on 2026-09-28. Nobody checked whether
  the terms of 3.3.2 changed.
- Apache Impala 4.5.3 is built here. The `RELEASE` defaults in
  `test/cmd/dbrun/image/impala.Containerfile` moved with the entry.
- PostgREST 16.5 is built here from its tag, and the next build uses the new tag.

## What stayed

- Solr stayed. The release 9.11.0 is a new line, rebuilt on 2026-10-17, and the rule in
  `container/solr.go` does not say whether it replaces 9.10.1. Ken decides.
- SingleStore stayed on image 0.2.85. The image 0.2.86 exists, and its engine is not
  known until `dbrun` measures it.
- MariaDB stayed. The vendor ended 10.6 in 2026-07, and the image still got a patch on
  2026-10-17, so the written rule keeps it.
- Cassandra stayed. The image of 3.11 was not rebuilt since 2025-11-14, which the file
  already recorded.
- ClickHouse stayed. The release 25.3 was not rebuilt for eight months and is the first
  pin to look at next time.

The goldens in `test/testdata` did not change, because each section is named for a
product and a major release, and no major changed.

`docs/BACKLOG.md` holds the decisions that need Ken.
