# D233. Cosmos DB answers five more kinds from the new dbimp statements

Status: Amends D228.

## The decision

dbimp added five catalog statements to its Cosmos DB driver for this model: `$index_paths`,
`$unique_keys` with `$unique_key_paths`, `$routines`, `$partition_key_ranges` and
`$session`. The model now answers 15 of the 65 kinds. It answered 10 in D228. The new
kinds are index columns, constraints, constraint columns, partitions and the current schema.
Functions reads `$routines` in place of `$functions`.

The statements are in dbimp v0.20.0, and the test module pins it.

## The mapping

- Index columns. Every row of `$index_paths` is one path of the indexing policy. The index
  is the one `indexing_policy` index of D228, so every row names it. `IndexColumn` has no field
  for the kind, so `Expression` says which entry the path is in: `included`, `excluded`,
  `composite 0`, `spatial Point`, `vector flat 128` or `full_text`. A path of a composite
  index has the ordinal of its position from 1 and the order in `Descending`. Every other
  path has the ordinal 1 and no order.
- Indexes stay one row for each container. A row of `$index_paths` cannot make one row for each
  index entry, because a composite index has several rows and a binding returns one object for
  each row. A container with the indexing mode `none` and the value of `automatic` come only
  from `$containers`, so an index for each entry loses them. So `Indexes` still reads
  `$containers` and still names every container of the database. The index columns read the
  one container of the URL, as the functions do.
- Constraints. A unique key is a constraint of the type `unique`. It has no name, so the name is
  `unique_key_` and its position from 0. The constraint columns are its paths, in order, with
  the ordinal counted from 1.
- Functions. One statement reads the user defined functions and the stored procedures, so a
  stored procedure is a function of the kind `procedure`, as PostgreSQL, SQL Server and Oracle
  list it. No new kind exists. The id of a routine is its resource id.
- Partitions. A range of the partition key is a partition. Its name is the id of the range, its
  type is `partition` and its bound is `FROM ('min') TO ('max')` with the two hexadecimal ends.
  The status and the parents of a range have no field.
- The current schema is the database of the URL, from `$session`. The statement sends no request.
- The current user stays unanswered. The session has no user and no kind of key, and the
  account has two keys that no statement tells apart. A test asserts that no column of
  `$session` names a user.

## The cost

The new statements read the one container of the URL. `$index_paths`, `$unique_keys` and
`$unique_key_paths` read the container, `$routines` reads two feeds and `$partition_key_ranges`
reads one. No statement grows with the number of documents. `$session` costs nothing.

## What was measured

On 2026-10-11 on the hosted account, with dbimp v0.20.0. The fixture adds a
full text index to the container `author`, and the account accepted it. The service accepted
the composite, spatial and unique key policies of D228. It still has no vector search, so
the vector rows were not read. The hosted service lists all four geometry types for a spatial
path that names Point. Conformance and parity did not change.
