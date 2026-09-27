# D36. The client drives. dbmeta decides nothing about the connection

Status: Decided.

The client opens the connection, chooses the dialect, and chooses the version
used to resolve queries. `dbmeta` never detects any of the three and never
guesses.

```go
func New(dialect Dialect, ver VersionSet) (*Meta, error)
```

`dbmeta` reads a version from a server only in the sense that it hands the
client a query to run. It does not run it. See D38.

## Why an override is not a luxury

Both reviews gave the same reasons, and each is a real deployment.

A proxy hides the server. PgBouncer and ProxySQL report themselves rather than
the database behind them.

A compatible product lies on purpose. CockroachDB answers a PostgreSQL version
query, and the answer describes neither its real catalog nor a PostgreSQL
release that behaves like it. This is the D14 flavor axis arriving at run time.

A code generator has no server. `dbtpl` generates against a target release the
developer names, with nothing to connect to.

A person is debugging. Forcing an older query set is how you find out whether a
fault is a version gate.

## Out of range

Follow D21, which already settled the behavior and now gets an API.

Above the newest version `dbmeta` knows, use the newest query set and proceed.
Do not fail. People upgrade a server faster than a library.

Below the oldest, return `ErrVersionTooOld`. The client can pass an explicit
override to try anyway, which is the opt-in escape D21 requires.

An unknown version is treated as newest, not as oldest. A serverless database
is continuously released, so newest is the truthful reading, and it agrees with
the rule above the ceiling.
