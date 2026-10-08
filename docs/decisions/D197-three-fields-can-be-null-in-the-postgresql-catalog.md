# D197. Three fields can be NULL in the PostgreSQL catalog

Status: Decided.

## The decision

usql compared its describe commands with psql 18 on 2026-10-08 and found three
statements that fail whole on a real schema. Each reads a column that the
catalog leaves NULL in a case the fixture did not hold.

- `IndexColumn.Descending` is NULL for an INCLUDE column, because a column that
  is stored in the index and not searched has no sort order.
  `pg_index_column_has_property(..., 'desc')` returns NULL for it.
- `DefaultACL.Schema` is NULL for a default privilege without `IN SCHEMA`,
  which holds for every schema.
- `RoleSetting.Database` is NULL for a setting without `IN DATABASE`, which
  holds for every database. `RoleSetting.Role` is NULL for the same reason in
  the other direction: a setting made with `ALTER DATABASE` names no role.

The fields are `sql.Null[bool]` and `sql.Null[string]` now. `docs/NULLS.md` and
D51 give the rule: a field that the catalog can leave absent has a type that
says so. The statements did not change. A caller that read `Descending` as a
bool reads `Descending.V` and `Descending.Valid`.

## What changed

- The type of the four fields in `object.go`. Nine models scan `Descending`, and
  `sql.Null[bool]` is a scanner, so no scan site changed. The tests that read
  the fields use `.V`.
- The PostgreSQL fixture builds an INCLUDE index (from 11), a partial index, a
  role, a setting for the role in every database, and a default privilege for
  the role in every schema. The teardown drops the role and what it owns. The
  CockroachDB fixture leaves the steps out, because they were not run there.
- `TestRolesAndColumnsWithNoValueAreNull` reads all three back.

usql and dbtpl are told, because the types of the four fields changed.
