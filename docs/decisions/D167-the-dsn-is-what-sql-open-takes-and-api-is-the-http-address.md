# D167. The DSN is what sql.Open takes, and api is the HTTP address

Status: Amends D160.

## The decision

A server in `container/` has two connection strings, and on 2026-10-02 Ken
confirmed what each one is for:

- The DSN is the raw string that `sql.Open` takes, for the driver that the
  tests use. `dbrun` connects with it and gives it to the tests.
- The URL is what `dburl.Open` takes, which is what a person types into
  usql.

Five products broke the first rule. libSQL, Neo4j, ArangoDB, SurrealDB and
InfluxDB 1 and 2 had the `http://` address of their HTTP API as the DSN,
because dbimp's tools read it, and dbimp's drivers take only the URL. D160
added a flag, `ConnectURL`, so that `dbrun` connected with the URL instead.
Ken chose to remove the flag and make the DSN mean one thing.

So the DSN of those five is now the form that their driver takes, which is
the same string as the URL. The `http://` address moves to a new field,
`api`, on the server and on each principal, which `dbrun dsn --json` prints.
A server whose DSN is already an `http://` address gives that as its `api`,
so every product with an HTTP interface has one, as dbimp asked.

dbimp asked on 2026-10-07 for the same for six products whose driver has its
own scheme: Druid, Drill, Solr, Elasticsearch, OpenSearch and DynamoDB, which
Alternator shares. Their DSN and URL are now `druid://`, `drill://`,
`solr://`, `elasticsearch://`, `opensearch://` and `dynamodb://`, with the
user and the key, at the port on the host, and their `api` is the `http://`
address they had. The DynamoDB DSN is
`dynamodb://key:secret@host:port?region=us-east-1`, with the endpoint as the
host, which replaces the godynamo form. Its `api` is `http://host:port` with
no credentials, because a request there is signed with the key.

dbimp agreed to the name on 2026-10-02. Nothing in dbimp's code read the DSN,
and its docs/DRIVER.md reads `api` from this change.

Ken asked on 2026-10-07 that the products whose DSN is already the driver URL
also have an `api`, where they have an HTTP interface the entry publishes.
rqlite, Couchbase, Databend, the BigQuery emulator and InfluxDB 3 now set
one. The BigQuery emulator's `api` carries no credentials, because it takes
none. ClickHouse, Spanner, YDB, Hive, SAP HANA, Exasol, MongoDB and the rest
publish a native or gRPC port and no HTTP one, so they have no `api`.
