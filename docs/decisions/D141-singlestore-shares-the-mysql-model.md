# D141. SingleStore shares the mysql model, and runs with no licence

Status: Amends D118.

## The decision

Ken asked on 2026-09-30 for dbmeta to answer for SingleStore, because usql
deletes its own readers in W21 and read SingleStore through its mysql reader
until then. D118 had given SingleStore no entry. Ken asked for it to be tried
without a licence key, and the development image,
ghcr.io/singlestore-labs/singlestoredb-dev, starts with none on a machine
with at most 8 cores and 64 GB, as its README says. So SingleStore has a
container entry, and 9.0 and 9.1 are Tested.

`models/singlestore` shares the mysql model's statements where they answer,
as `models/tidb` does (D133), under the dialect memsql, which is dburl's
name. SingleStore answers `VERSION()` with 5.7.32, the MySQL release it
claims, and names its own release only in `@@memsql_version`. The version
set's main version is the MySQL release, set under the `mysql` key too, and
SingleStore's own is under the key `memsql`.

## The image names no engine

The image is tagged by its own version, such as 0.2.85, and not by the
engine's. 0.2.85 ships 9.1.1, measured on 2026-09-30. 9.0 is the line before
it, and SINGLESTORE_VERSION makes the container download it when it starts.
Each release pins the image tag.

## The shared statements carry alternatives for SingleStore

- The schema filter. memsql and cluster are SingleStore's own schemas.
- Constraints. SingleStore reports the primary key of a columnstore table as
  UNIQUE, with the name PRIMARY, and the alternative reports it as the
  primary key it is.
- Index columns. SingleStore lists a key that is also the shard key twice,
  once under the type SHARD, and the alternative leaves out the second row.

A SingleStore alternative must never sit in a choice beside one on the
`mysql` key, because a SingleStore version set reports both keys.

## What is its own

SingleStore has no mysql database and no performance_schema. Settings read
GLOBAL_VARIABLES, roles read USERS, ROLES and GROUPS, role grants read
GROUPS_ROLES, privileges read ROLE_PRIVILEGES, aggregates read
AGGREGATE_FUNCTIONS, and column statistics read OPTIMIZER_STATISTICS.
docs/COVERAGE.md has the rest.
