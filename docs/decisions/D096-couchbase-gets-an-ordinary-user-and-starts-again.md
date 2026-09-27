# D96. Couchbase gets an ordinary user, and starts again after a stop

Status: Amends D94, amended by D104.

dbimp asked on 2026-09-27 for an ordinary Couchbase user, so that its driver
is tested as more than the administrator. Ken accepted that for dbimp in its
D31. dbmeta's parity work needs the same principal (D61), so `Init` makes one
for both.

## The ordinary user

`container.CouchbaseUser` is `dbmeta_user`, and its password is
`container.Password`. It holds `query_select`, `query_insert`, `query_update`
and `query_delete` on the `dbmeta` bucket, and `query_system_catalog`.
`user-manage --set` makes the user or resets it, so it is safe on every start.

Measured on 7.2.9, 7.6.12 and 8.0.3: the user writes and reads the bucket and
reads `system:keyspaces`. It is refused `system:user_info`, `CREATE INDEX` and
the creation of a bucket. That is a grantee in D61's terms. An owner, with
`bucket_admin` on the bucket, is the second lesser principal a parity target
will want, and a test makes it when the model exists.

`Init` also makes a primary index on the bucket. Without one, a SELECT over
the bucket is refused on every release: 7.2 has no sequential scan at all, and
8.0 allows one only through `query_use_sequential_scans`, a role the user does
not hold. A primary index is the smaller grant.

## A start after a stop never became ready

D94's `Ready` asked `/pools` for a 200. A new node answers 200, and a node
whose cluster exists answers 401 to a request with no credentials. So a
stopped server that was started again never became ready, and `dbrun` gave up
after 90 seconds. D94 measured only a first start. `Ready` now takes 200 or
401, which says that the cluster manager answers.

On that start the query service answers before the bucket has warmed up, and
7.2.9 refused an INSERT in that window. `Init` now waits until a count over
the bucket succeeds. Each release was stopped and started again, and each
became ready and served the ordinary user.

## A fact for a driver's tests

The index is updated after a write, not with it. On 7.6.12 a SELECT straight
after an UPSERT returned no rows. A test that writes and then reads asks for
`scan_consistency=request_plus`.
