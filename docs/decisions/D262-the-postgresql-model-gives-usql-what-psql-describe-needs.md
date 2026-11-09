# D262. The PostgreSQL model gives usql what psql describe needs

Status: Amends D47 and D201.

usql compared 184 describe commands, and `\d` and `\d+` of 97 relations, with
psql 18.6 on PostgreSQL, and asked for what dbmeta lacked. The maintainers decided on
2026-11-04 that the describe output of usql is the same as psql. dbmeta
supplies the facts and usql decides what to show (rule 13), so each change
here is a fact and none is the text of psql.

## Decisions

1. `Visible` is a `sql.Null[bool]` on `Table`, `Index`, `Sequence`,
   `Function`, `Type`, `Domain`, `Comment`, `Privilege` and
   `PartitionedTable`. PostgreSQL fills it with `pg_table_is_visible`,
   `pg_function_is_visible` and `pg_type_is_visible`. It is absent on a product
   with no search path. usql keeps a row whose value is true or absent when a
   pattern has no schema part, as psql does. An argument that filtered the rows
   in the query was rejected, because it puts the rule of one client into the
   statement.
2. `Comments` returns the comments on table constraints, domain constraints,
   operator classes, operator families, rules and triggers, as psql lists with
   `\dd`, and keeps the relations. The `Type` column says which kind a row is.
3. `Setting.Source` is `pg_settings.source` and `Setting.Boot` is
   `pg_settings.boot_val`. `\dconfig` with no pattern lists the settings whose
   source is not `default` and whose value is not their boot value, so a setting
   that the configuration file sets to its boot value is not listed. An earlier
   version of this decision said that psql also leaves out the source
   `override`, and it does not. The name of a setting is matched without case,
   as psql does.
4. `Languages` and `RoleGrants` take `with_system`. Without it, and with no
   name, `Languages` leaves out the languages that have no call handler and
   `RoleGrants` leaves out the memberships of the roles named `pg_...`. The
   default leaves the answer of `Languages` as it was.
5. `Sequence.Identity` says that an identity column owns the sequence.
6. `Index.Partitioned` says that the index is a partitioned index. `Partitions`
   and `Inherits` take `indexes`, which includes the indexes that they left out,
   so that a partitioned index has partitions and an index partition has a
   parent. `Partition.Partitioned` is true for a partitioned index too, and a
   relation type of `index` and `partitioned index` is added.
7. `IndexColumn` has `ColumnName`, `ColumnType`, `Storage` and `StatsTarget`,
   the attributes of the index itself. For `lower(name)` the name is `lower` and
   the type is `text`, and for a `gin_trgm_ops` column the type is the stored
   one.
8. The default of a column and the expression of a generated column come from
   `pg_get_expr` with the pretty flag, as psql passes it.
9. `TextSearchParserTokenTypes` is a new kind, from `ts_token_type`.
10. The functions and aggregates are ordered by schema, name and argument types,
    as psql orders them. The third key was the result type.

## What was not done, and why

- The access method type stays `index` and `table`, and usql writes `Index` and
  `Table`. The case is how a client shows the word.
- An aggregate with no argument has an empty `ArgTypes`, which is what
  `pronargs = 0` is. usql writes `*`.
- The sequence numbers stay `sql.Null[string]`, because Oracle needs 28 digits
  (D201). usql converts them.
