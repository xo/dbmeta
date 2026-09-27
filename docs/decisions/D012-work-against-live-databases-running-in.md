# D12. Work against live databases running in containers

Status: Amended by D70 and D71.

What this decision got right is that the work needs a running server, and that
is still true for a different reason than it gave. D71 has it: nothing is
generated, so there is no generation step needing a connection. A query is
written against a live server and checked there, and a fixture is built there,
so every model still needs one.

D70 replaced the harness this decision chose. Read it first: the databases are
started by `dbrun` and by nothing else, and the rest of this section is the
record of what was decided in 2025 and why. What does not stand is
`usql/contrib`, and neither does the paragraph below about generation time.

`dbtpl query` introspects a real connection. It creates a temporary view from
the statement, reads the column types, and drops the view. There is no offline
mode. Every driver therefore needs a running database at generation time.

`usql/contrib` already solves this and `dbmeta` must reuse the pattern rather
than invent one. It holds a directory per database, each with a `podman-config`
file of four lines, and the scripts `podman-run.sh` and `podman-stop.sh` that
start and stop them. The postgres config reads:

```
NAME=postgres
IMAGE=docker.io/usql/postgres
PUBLISH=5432:5432
ENV="POSTGRES_PASSWORD=P4ssw0rd"
```

`usql/contrib/config.yaml` records the connection URL for each running
container, such as `postgres://postgres:P4ssw0rd@localhost`.

`contrib` covers every database in the three phases, and 27 databases in total.
Note one naming detail before you look for something that is not missing. Phase
2 uses MariaDB, and there is no `mariadb` directory. The `mysql` directory is
the MariaDB one. Its config reads `IMAGE=docker.io/library/mariadb`.

The directories are adodb, cassandra, charts, clickhouse, cockroach, couchbase,
db2, duckdb, exasol, firebird, flightsql, godror, h2, hive, ignite, mymysql,
mysql, oracle, oracle-enterprise, pgx, postgres, presto, sqlite3, sqlserver,
trino, vertica, and ydb.

D8 adds one requirement that `contrib` does not yet meet. Most configs name an
image without a tag, as in `IMAGE=docker.io/usql/postgres`, so each database
runs at one version. `dbmeta` generates one model per database version, so the
harness must start a named version, such as postgres 15 and postgres 18, and
must record which version produced which model. That is an extension of the
existing configs, not a replacement for them.
