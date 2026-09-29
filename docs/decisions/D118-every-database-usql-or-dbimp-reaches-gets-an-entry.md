# D118. Every database that usql or dbimp reaches gets an entry

Status: Amended by D123, D141 and D145.

Ken asked on 2026-09-28 for a `dbrun` entry for every database that `usql`
supports and has none yet, such as CockroachDB, and for every target in
dbimp's TARGETS table that has none. A hosted service gets the D117 form
instead. dbmeta has no model for most of these products, and none of them
has a dialect yet, because dbimp settles the name with each driver, as D112
says.

This decision holds the entries as they arrive. Each product has the doc
comment of its file in `container/`, which holds the range and the reasons,
and a row in `docs/EVALUATION.md`.

## The rules the entries follow

Every entry follows the rules that D112 set:

1. The range comes from `docs/EVALUATION.md`. An image that the publisher
   rebuilds takes the oldest release still rebuilt and the newest. An image
   that is built once per release takes the newest release of each of the
   last two lines.
2. Every release is Staged, because no model reads it yet, so CI does not
   run it. D119 made that the rule for every release no model reads.
3. The administrator has `container.Password`. An ordinary user is named
   `dbmeta_user`, and a product that has no users gets a key that may only
   read, which goes by that name in the DSN.
4. The setup is safe to run twice. Each entry was started fresh, measured,
   stopped, started again, measured again and removed.
5. Every usage report to a vendor is turned off where the product has a
   setting for it.

## The first entries

| Product | Releases | Administrator | Ordinary principal |
| --- | --- | --- | --- |
| Qdrant | 1.18.3, 1.19.1 | the API key | a second key that may only read |
| Chroma | 1.4.1, 1.5.9 | none | none |
| Weaviate | 1.38.17, 1.39.7 | the key of `admin` | the key of `dbmeta_user`, a read only user in the admin list |
| CouchDB | 3.4.3, 3.5.2 | `admin` | `dbmeta_user`, a member of `dbmeta` |
| QuestDB | 9.4.3, 10.0.1 | `admin` | none |
| Meilisearch | 1.53.2, 1.54.0 | the master key | a key that may search and read `dbmeta` |
| Typesense | 29.1, 30.2 | the bootstrap key | a key that may search `dbmeta` |
| TerminusDB | 11.1.17, 12.0.7 | `admin` | `dbmeta_user`, with a role that reads `admin/dbmeta` |
| CockroachDB | 26.2.7, 26.3.2, 24.3.36 | `root` | `dbmeta_owner`, who owns `dbmeta`, and `dbmeta_user`, who may connect to it |
| TiDB | 7.5.8, 8.5.8, 8.1.2 | `root` | `dbmeta_owner`, with every right on `dbmeta`, and `dbmeta_user`, who may select |
| MongoDB | 7.0.43, 8.3.11, 8.0.32 | `admin` | `dbmeta_user`, with the role `read` on `dbmeta` |
| Elasticsearch | 8.19.22, 9.5.3, 9.4.6 | `elastic` | `dbmeta_user`, who may read the indices named `dbmeta*` |
| Dgraph | 25.3.8, 25.4.1 | `groot` | `dbmeta_user`, in a group that may read `dgraph.type` |
| YDB | 26.2.1.14, 26.3.1.17 | `root` | `dbmetauser`, who may read and describe `/local/dbmeta` |
| Spanner emulator | 1.5.58 | none | none |
| BigQuery emulator | 0.7.2, 0.8.1 | none | none |
| GizmoSQL | 1.38.5, 1.39.0 | `admin` | none. The core has one user |
| Virtuoso | 7.2.17 | `dba` | `dbmeta_user`, who may run SPARQL queries and not updates |
| Alternator | 2025.1, 2026.3 | `cassandra` | `dbmeta_user`, with SELECT on every keyspace |
| Vitess | 23.0.6, 24.0.3 | none | none. vttestserver accepts any user |
| Milvus | 2.6.24, 3.0.2 | `root` | `dbmeta_user`, with a role that reads `dbmeta` |
| OpenSearch | 2.19.6, 3.8.0 | `admin` | `dbmeta_user`, who may read the indices named `dbmeta*` |
| DynamoDB Local | 3.2.0, 3.3.1 | none | none. It checks no key |
| Cosmos DB emulator | EN20260907 | the account key | none |
| Solr | 9.9.0, 10.0.0, 9.10.1 | `admin` | `dbmeta_user`, with the role search, which reads and runs SQL |
| Drill | 1.21.2, 1.22.0 | `admin` | `dbmeta_user`, who may query and not change an option |
| H2 | 2.4.240, 2.5.252 | `sa` | `dbmeta_user`, who may read the schema PUBLIC |
| Fuseki | 6.1.0, 6.2.0 | `admin` | `dbmeta_user`, who may query and not update |
| PostgREST | 14.18, 16.4 | the token of `dbmeta_admin` | the token of `dbmeta_user`, who may read the schema `dbmeta` |
| ksqlDB | 8.2.4, 8.3.2 | `admin` | none. Every user open source ksqlDB lets in has the same rights |
| Stardog | 12.0.4, 12.1.4 | `admin` | `dbmeta_user`. Not yet measured |
| GraphDB | 11.4.3, 11.5.1 | `admin` | `dbmeta_user`. Not yet measured |
| Volt Active Data | 14.1.0, 15.2.0 | `admin` | `dbmeta_user`. Not yet measured |

Each ordinary principal was measured on both releases, except on the three
that need a licence file. It read, it was
refused a write with 403, and a request with no credential or a wrong one
was refused with 401.

## What was measured

These facts cost time and are recorded so that the next entry does not pay
for them again:

1. Qdrant, Chroma, Typesense and TerminusDB publish images with bash and no
   curl or wget. `container/http.go` sends a request through bash's
   `/dev/tcp` instead. Two such requests cannot be joined with `||` as text,
   because each is several lines and `||` joins only the lines next to it.
   The setup of Qdrant failed on its second start for that reason.
   `bashEither` and `bashAll` run each request in a subshell of its own.
2. Meilisearch computes the value of a key from its identifier and the master
   key, as the HMAC-SHA256 in hex. So the entry fixes the identifier, and the
   value is known before the server starts. The value computed in Go matched
   the one the server returned.
3. CouchDB refuses a member that writes the security object with 500 and the
   reason `no_majority`, and not with 403. The object stays as it was.
4. The server of Chroma 1 has no authentication at all. It is secured only by
   a proxy in front of it, so there is no principal to make.

5. CockroachDB lets every user make a table in the schema `public`, as
   PostgreSQL 14 and older do. Init gives the schema to the owner and takes
   that right from everybody else, so the grantee is refused.
6. The YDB image deploys its cluster without a user, so a setting that turns
   on a login before the cluster is up leaves it with no storage pools, and
   it cannot make a table. `POSTGRES_USER` and
   `YDB_ENFORCE_USER_TOKEN_REQUIREMENT` both do that. So the server starts
   with neither, and Init gives root its password afterwards. A connection
   with no user is still accepted and may do anything. YDB also allows no
   underscore in a user name.
7. TiDB reads no variable for the password of root, and its image has no
   MySQL client. The command starts the server with `--initialize-sql-file`,
   which runs the setup once, when the server makes its store.
8. On its first start the MongoDB image runs a server with no authentication
   and then restarts it. The check asks whether authorization is on, so it
   cannot pass against the first server.

9. The image of the Cloud Spanner emulator is distroless and has no shell.
   Moving the emulator onto Debian 12 fails, because it needs the C++
   library its own image ships. So the Containerfile adds a static busybox
   to Google's image. dbrun now builds a Containerfile for any product that
   has one in `test/cmd/dbrun/image`, and `TestEveryContainerfileHasItsImage`
   holds each file and its entry together.
10. The curl in the ScyllaDB image signs a DynamoDB request that 2025.1
    accepts and 2026.3 refuses with "wrong signature". godynamo's signature
    passes on both. So the Alternator check logs in through cqlsh and asks
    `GET /`, the health check of Alternator.
11. The entrypoint of the ScyllaDB image sets the address of Alternator to
    the one the node listens on outside, so a check inside cannot reach it.
    The entry sets it to `0.0.0.0`.
12. The usql driver for Flight SQL fails against GizmoSQL with "No session
    ID in request context". GizmoSQL gives a session at the handshake, and
    the driver sends only the password on each call. GizmoSQL's own client
    works. This is a fault between usql's driver and the server, and the
    entry stays so that the driver can be tested against it.

13. The installer in the OpenSearch image rates the first password of
    `admin` with a rule of its own, and refuses `P4ssw0rd!x`. No setting
    lowers that rule. `admin` is a reserved user, so the REST interface
    refuses to change its password, and `securityadmin.sh` speaks HTTPS,
    which the entry turns off. So the command runs the installer with a
    strong password, and writes the hash of `P4ssw0rd!x` into
    `internal_users.yml` before the server first starts and loads its users
    from the file. The SQL plugin also needs `indices:monitor/settings/get`
    for a user that reads.

14. `bashRequest` put the path of a request into the format of `printf`, so
    the `%7B` in a query string was read as a directive. Fuseki's check
    failed for that reason. The helper escapes each percent sign now.
15. The Java of Drill 1.22.0 is 17.0.2, which fails with a
    NullPointerException when it reads the cgroups of a current host.
    Turning its container support off fixes it, and the heap is set anyway.
    Drill answers a request with no user with 404, and a wrong password with
    307, a redirect to its login form.
16. h2go, the H2 driver usql ships, fails against H2 2.4.240 and 2.5.252
    with "Can't read all data needed". The server answers its own Shell, so
    the fault is between the driver and those releases. An H2 user that is
    not an administrator also may not set `DB_CLOSE_DELAY` in its address.
17. The Kafka start script writes a GC log under `/var/log/kafka`, which the
    ksqlDB image does not have, so the broker would not start. The command
    sets `LOG_DIR`. The preflight of ksqlDB gives up when Kafka is slow, so
    the command waits for the broker's port first.
18. ksqlDB 8 runs on Jetty 12, whose JAAS module is
    `org.eclipse.jetty.security.jaas.spi.PropertyFileLoginModule`, and not
    the `org.eclipse.jetty.jaas` name older documentation gives.

## What Ken decided on 2026-09-28

1. DynamoDB Local and the Cosmos DB emulator get entries, and Ken accepted
   their licences for this project.
2. Stardog, GraphDB 11 and VoltDB get entries, each with a licence file that
   a person downloads. The file is the licence's equivalent of a
   credential, and the next section says how an entry finds it.
3. SingleStore gets no entry.
4. Gel, Blazegraph, Stargate and Apache Impala get no entry. A product that
   is dead, retired or no longer supported is removed from every `xo`
   project when that is convenient. Netezza has no image at all and is left
   out for the same reason.
5. OpenSearch keeps `P4ssw0rd!x`, with the rating of passwords lowered,
   rather than a password of its own.
6. Elasticsearch keeps 8.19 as its floor, because 8.19 is still patched.

## A licence file is found the way a credential is

Stardog, GraphDB 11 and Volt Active Data do not start without a licence file
that a person gets by signing up. D117 already answers the shape of this: a
thing only a person can provision is found on the host at run time, and the
entry exists in `dbrun` only while it is found. So `container.Server.License`
names where the product reads its file inside the container, and holds no
file. `dbrun` looks for the file in `DBMETA_<PRODUCT>_LICENSE` and then in
`$XDG_CONFIG_HOME/dbmeta/licenses/<product>`, and mounts it read only.

Every release of the three is Staged, as every release no model reads is,
and CI has no file for any of them either. None has
started here yet, because no file is provisioned, so each entry says in its
doc comment that it is not yet measured. `docs/COVERAGE.md` says the same.
A licence file is not a secret the way a password is, so `dbrun` does not
ask that only its owner may read it.

## Wire compatible products wait for their models

CockroachDB speaks the PostgreSQL protocol, and TiDB and Vitess the MySQL
protocol, and usql reaches each with its own scheme. Neither entry names a dialect yet.
`models/postgres` reads the version with `SHOW server_version`, which
CockroachDB answers with a PostgreSQL compatibility number, and
`models/mysql` sets the version key of MySQL for the `VERSION()` of TiDB,
which is `8.0.11-TiDB-v8.5.8`, and of Vitess, which is `8.4.6-Vitess`. With a dialect, `dbrun test` would run each
model's tests and record a wrong answer. Each entry takes its dialect when
the model detects the product and sets its own version key, as D44 requires.

## A port inside a container is not on the host

`TestEveryMachineIsUsable` compared the port inside each container with the
ports of the Windows machines on the host. Typesense listens on 8108 inside
its container, and the viewer of `sqlserver-2014` is 8108 on the host. The
two cannot collide, because dbrun publishes a container on 55000 plus its
index. The test now checks that range, which is what its comment always said
it did.

## Rejected

Running these entries in CI. They were Tested when they were added, and that
put 57 releases in CI that test nothing in dbmeta, because no model reads
them. D119 moved them, and every other release no model reads, to the Staged
tier, which CI does not run.
