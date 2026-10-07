# D176. The search dialects follow Ken's answers to the survey

Status: Decided.

## The decision

Ken answered the open questions of the surveys of Elasticsearch, OpenSearch,
Drill and Solr on 2026-10-07. The surveys are the record of what each product
answers. This decision records the choices. No model is built until dbimp tags
the drivers and a dburl release names them. dburl v0.45.0 names the four.

1. Elasticsearch is built first, because it needs no walk and its fixture also
   serves OpenSearch.
2. Drill is built with the Drill Metastore price. If the Metastore is off,
   Tables returns views and system tables and no file table, and gives no
   error. The fixture turns the Metastore on. COVERAGE.md records the limit.
3. The role `dbmeta_role` of the OpenSearch entry holds
   `indices:admin/get` and `indices:data/read/search` on every index, and the
   cluster permission `cluster:monitor/health`. The ordinary user can then run
   the walk of D175 and page a plain SELECT. The user can also see the name of
   every index. Parity records that.
4. Elasticsearch, OpenSearch and Solr have no SQL source for the release. The
   model reads it over HTTP as the administrator, as D171 does for Druid. The
   ordinary user gets no release.
5. The cluster is the catalog, there is no schema, and an alias is a view.
6. Solr uses a fixed schema name, because the name that Solr gives is the
   address of ZooKeeper, which changes with the machine. Before the model is
   built, the cost check of D47 runs against a catalog with thousands of
   collections, because `metadata.COLUMNS` does not prune a filter.

Ken also asked on 2026-10-07 that the DSN of every DynamoDB entry ends with
`tls=false`, because every endpoint is plain HTTP and the driver turns TLS on
unless the DSN says otherwise.
