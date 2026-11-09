# D283. Hive and Impala are reached over HTTP only

Status: Amends D282.

## Decision

The binary entries of Hive and Impala are gone. `hive` and `impala` are now the
entries that D282 called `hivehttp` and `impalahttp`. HiveServer2 runs in the
HTTP transport, Hive on 10001 and Impala on 28000, and nothing answers on the
binary ports 10000 and 21050.

Ken chose this on 2026-11-09. dburl v0.58.0 names `github.com/xo/dbimp/hive` for
both products, and that driver speaks HTTP and nothing else. Rule 10 says that
the `test` module uses the driver that dburl names, so `beltran/gohive` and
`sclgo/impala-go` left the `test` module.

The release names are `hive-4.2.1`, `hive-4.0.1`, `impala-4.5.3` and
`impala-4.4.1`, as they were before D282. Both products stay Verified (D281).

## What changed

- `container.HiveHTTP` and `container.ImpalaHTTP` are removed.
- The DSN is the `hive://` URL that the dbimp driver takes, for Impala too. The
  `api` is the `http://` address. The server checks neither the user nor the
  password.
- The Hive entry keeps its Init, which creates the `sys` database over the
  metastore, because `models/hive` reads it. beeline runs it over HTTP.
- The URL of Impala that `dbrun` prints is `impala://`, which dburl turns into the
  DSN of the driver, so that a client opens Impala with the impala dialect. The
  DSN stays `hive://`.
- beeline connects with `transportMode=http` and impala-shell with
  `--protocol hs2-http`.
- The `test` module opens Hive and Impala with the one driver name `hive`.
