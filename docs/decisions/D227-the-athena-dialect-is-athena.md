# D227. The Athena dialect is athena

Status: Amends D222.

## The decision

The value of `dbmeta.Athena` is `athena`. It was `awsathena`, which is the name
that dburl v0.49.0 gave the scheme. The usql session asked for the rename on
2026-10-11, because the dburl that names the driver of dbimp has the scheme
`athena`, with the driver name and the dialect `athena`, and keeps `awsathena`,
`s3` and `aws` as aliases. A dialect is the dburl dialect, so this follows it as
the rename of Cassandra did (D196).

## What changes

The constant, the name of the parity section in `test/testdata/parity.txt`
and the documents that named the old value change. The name `awsathena` stays
where it is the name of a Go driver, which the driver of Uber registers, and in
the scheme of the hosted form until dburl releases its change.

## What does not change

The model, its fixture and its tests are the same. D222 said that the value stays
`awsathena` until dburl tags the rename, and this amends that.
