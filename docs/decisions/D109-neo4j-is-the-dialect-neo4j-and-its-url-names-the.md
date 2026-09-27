# D109. Neo4j is the dialect neo4j, and its URL names the database

Status: Amends D106, amended by D112.

Ken accepted dbimp's D60 and D61 on 2026-09-27, which D106 waited for. The
dialect and the dburl scheme are `neo4j`, so `dbmeta.Neo4j` is `neo4j`, as
`dbmeta.SurrealDB` was added for dbimp's SurrealDB driver. dbmeta has no
Neo4j model.

The URL is `neo4j://user:password@host:port/<database>`. The port is the HTTP
port that `dbrun` maps from 7474, and the path names the database. `dbrun`
prints `neo4j://neo4j:<password>@127.0.0.1:<port>/dbmeta` as the `url` of the
administrator, and the same with `dbmeta_user` for the ordinary user. dbimp's
CI reads the `url` field and the `url` of the principal whose role is `user`.
The `dsn` stays the plain `http://` address.

The test variable is still `DBMETA_NEO4J`, and it now comes from the dialect.
D106 named it for the product, because the server had no dialect, and that
fallback in `dbrun` is gone, because no server needs it.
