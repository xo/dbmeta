# D114. A server can answer more than one dialect

Status: Amends D112.

Ken accepted dbimp's D78 on 2026-09-28. dbimp's InfluxDB driver has two
dialects: `influxdb`, which is SQL on InfluxDB 3 and later, and `influxql`,
which is InfluxQL through `/query` on InfluxDB 1, 2 and 3. A dbrun server had
one dialect, and its test variable was named for it. InfluxDB 3 answers both,
so Ken chose on 2026-09-28 that a server can answer more than one dialect.

## How it works

`container.Server` has `Also`, the dialects the server answers beside its
`Dialect`. dbrun sets the test variable of each to the same connection string,
so one container serves both. `dbrun start` prints each variable, and
`list --json` and `dsn --json` print `also` and `alsoEnv`. `ForDialect`
returns a server for any dialect it answers.

The alternatives were two entries for each InfluxDB 3 release, which runs the
same server twice with two ports and twice the memory, and one dialect per
product, which never tests InfluxQL on InfluxDB 3.

## The InfluxDB entries

Ken chose the releases in dbimp's D79. InfluxDB 1 and 2 are in maintenance,
so the newest of each is Tested and the oldest is Nightly. InfluxDB 3 changes
fast, so its floor and its ceiling are Tested, as D112 made them.

| Line | Releases | Dialects | Ordinary user |
| --- | --- | --- | --- |
| InfluxDB 1 | 1.13.1, and 1.11.8 nightly | `influxql` | `dbmeta_user`, READ on `dbmeta` |
| InfluxDB 2 | 2.9.1, and 2.8.0 nightly | `influxql` | `dbmeta_user`, a v1 user who may read the bucket `dbmeta` |
| InfluxDB 3 Core | 3.9.13, 3.11.5, and 3.10.6 nightly | `influxdb`, and also `influxql` | none |

This amends D112, which gave InfluxDB 3 no dialect because the names were not
settled.

## What was measured

On each release, fresh, then stopped and started again, on 2026-09-28:

- InfluxDB 1: the image makes the database and both users on the first start,
  and Init sets the user's password and grants READ again on every start. The
  user reads, and it is refused a write and CREATE DATABASE. A wrong password
  or none is refused.
- InfluxDB 2: the image makes the admin, the organization, the bucket and the
  admin token on the first start. `/query` needs a mapping of a database to a
  bucket, and InfluxDB 2 makes one for each bucket by itself, as a virtual
  mapping, so Init makes none. The first setup made a mapping only if none was
  listed, and the listing of the virtual ones is never empty, so that step
  never ran and was removed. Init makes the v1 user with read on the bucket.
  It reads through `/query`, and a write is refused with "insufficient
  permissions for write". The admin token works as the password.
- InfluxDB 3: `/query` answers InfluxQL with the admin token as a password and
  as a bearer token.

A v2 token that may only read the bucket is possible on InfluxDB 2. The server
gives it a random value, so no connection string here could name it, and the
ordinary user is the v1 user instead.
