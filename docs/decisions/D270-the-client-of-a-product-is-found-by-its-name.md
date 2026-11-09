# D270. The client of a product is found by its name

Status: Amends D236.

D236 put the native client of a product on each server in the list, so a
caller found it by walking the releases. dbfwd runs a native client through a
Kubernetes forward, and it knows only the type of the database, such as
postgres, mariadb or sqlserver. It has no release and no container. The maintainers
approved on 2026-11-08 that dbfwd reuses this table, so that the family keeps
one copy.

## Decision

`container.ClientFor(name)` returns the [Client] of a product. The name is a
product, such as mariadb, or a dialect that dburl names, such as postgres. It
is compared in lower case with the spaces removed. A product wins over a
dialect, because MariaDB and MySQL share the dialect mysql and have a client
each. When no product has the name, the first server with that dialect answers.

A release does not change the client of a product, with one exception. InfluxDB
1 and 2 have `influx` and InfluxDB 3 has `influxdb3`. The answer is the client
of the newest release. The second result is false when no server has the name.
A product with no client comes back with an empty command and the reason in
`Why`, as D236 says.

The caller builds a `Connection` from the local host, port, user, password and
database, calls `Client.Call`, and reads `Call.Env`, `Call.Args` and
`Call.InArgs`. `InArgs` says that the password is in the arguments, where a
process list shows it. dbfwd refuses such a client, because its password is a
real one from a cluster Secret.
