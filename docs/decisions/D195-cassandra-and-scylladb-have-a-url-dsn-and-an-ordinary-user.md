# D195. Cassandra and ScyllaDB have a URL DSN and an ordinary user

Status: Decided, amended by D196.

## The decision

The Cassandra driver moves from `github.com/xo/cql` to
`github.com/xo/cassandra`. The new driver reads a URL only. Its scheme is
`cassandra`, it has no host list form, and it refuses every other scheme,
including `cql`. This change makes the entries ready for it. The old driver,
`github.com/xo/cql` v0.1.0, reads the URL form too, so the tests keep passing
with it.

### The DSN

The DSN of every Cassandra and ScyllaDB release is now:

```
cassandra://cassandra:cassandra@127.0.0.1:<port>/?connectTimeout=30s&timeout=30s
```

Both options keep the names and the values that the host list form had. The
new driver reads the same two keys.

ScyllaDB uses the scheme `cassandra` in its DSN, because the driver reads no
other. The `url` field, which a person pastes into usql, is not the DSN. It
has no options. On Cassandra it is
`cassandra://cassandra:cassandra@127.0.0.1:<port>/`. On ScyllaDB it keeps
`scylla://`, which dburl reads as another name for the `cassandra` scheme.
D167 says what each field is for.

### The ordinary user

Both entries now have `container.CassandraUser`, which is `dbmeta_user`, with
the shared test password. Its `Init` runs on every start with one statement:
`CREATE ROLE IF NOT EXISTS`, with `LOGIN = true` and `SUPERUSER = false`. It is
safe to run twice. `Init` runs after the ready command, which logs in as the
superuser. That login works only after `system_auth` can answer, so the role
can be made. `dbrun` also runs `Init` again when it fails.

The role holds no permission, which is the smallest set. Every role reads
`system` and `system_schema` with no grant, and those two hold everything the
catalog queries need. A first version granted SELECT on all keyspaces, so that
the role can read the fixture keyspace, which does not exist when `Init`
runs. Parity on ScyllaDB showed the cost. There the grant reaches
`system.roles`, `system.role_members`, `system.role_permissions`,
`system.role_attributes` and `system.config`, so the user got the same answers
as the superuser, with the password hashes included. The grant was removed.
The catalog queries do not read the data in the fixture keyspace, so the role
loses nothing.

The role cannot read the tables of roles and permissions. On Cassandra they are
in `system_auth`. `Settings` reads `system_views` on Cassandra 4.0 and later,
and the role cannot read that either. A grant on `system_views` is refused on
3.11, where the keyspace does not exist, so the decision accepts the
difference rather than give `Init` a statement that depends on the release.

### Parity

`test/parity_test.go` has the principal `user` beside `grantee`, and
`makeCassandraUser` becomes it by changing the credentials of the URL. The
helper `cqlUser` is gone, because the DSN is a URL and `replaceUser` handles
it. The golden file has two new sections. `[cql/same/user]` lists `privileges`,
`role_grants`, `roles` and `settings` as refused. `[scylla/same/user]` lists
`privileges`, `role_grants`, `role_settings`, `roles` and `settings` as refused.
These are the same queries that the section for `grantee` refuses, so the user
adds no new difference.

## What does not change

The dialect is still `cql`, the driver name in the tests is still `cql`, and
the tests still import `github.com/xo/cql`. The coordinator changes those when
the new driver has a tag, together with dburl.
