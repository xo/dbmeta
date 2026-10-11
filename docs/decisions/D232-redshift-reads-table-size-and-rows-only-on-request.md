# D232. Redshift reads table size and rows only on request

Status: Amends D207 and D212.

## The decision

Ken decided on 2026-10-11 that the Tables statement of Redshift does not read
SVV_TABLE_INFO by default. The caller opts in to the size, the rows and the
options.

D212 made the model read the view and said that the administrator grants
SELECT on it. A user who is not a superuser and holds no grant gets the error
"permission denied for relation svv_table_info" (SQLSTATE 42501) from the whole
kind. Measured through usql with an ordinary user, `\dt`, `\dt+`, `\d t` and
`\dv` all failed, because each one reads Tables. The cost was every table, for the sake of the
size of one.

## The change

1. `Args.WithTableInfo` is new. It is the parameter `with_table_info`, false by
   default. Only the Redshift Tables takes it. `Query.Each` leaves it out for
   every other query, and `Query.All` refuses it there with ErrUnknownParam,
   as it refuses any parameter a query does not take.
2. The default statement does not name the view. `Size`, `Rows` and `Options`
   are NULL for a table and a view, selected as `CAST(NULL AS ...)` under the
   same names, so the column set does not change (rule 3 and D51). An external
   table keeps its `Options`, the location and the serde, because they come from
   SVV_EXTERNAL_TABLES and not from the view.
3. With the parameter set, the statement is the one of D212: the derived table
   joined by table id. `tablesStmt` builds both from one function, so they
   differ in four pieces and no more.
4. The root package has a new type, `dbmeta.Variant`, and a new field,
   `Binding.Variants`. A statement is fixed text per release, and no argument
   changed it before. A variant is a second statement for a binding, and
   `Query.Build` uses it when its boolean parameter is true. A statement that
   names the view cannot hide it behind a false condition, because Redshift
   checks the right to read the view when it plans the statement. Only the
   Redshift Tables has a variant.
5. With the parameter set and no right to the view, the error stays 42501. The
   test `TestRedshiftTableInfoIsReadOnRequest` reads the default as an ordinary
   user, then asks for the table info and expects 42501, then grants SELECT and
   reads the sizes.

## Other statements

Every other Redshift statement was read as an ordinary user. The parity test
makes an owner, a grantee and a stranger, and none of them holds a grant on
SVV_TABLE_INFO any more. No query is refused for any of them, so no other
statement reads a view that only a superuser can read. The parity golden did not
change: the three principals still differ from the administrator only where an
SVV view shows a user its own rows (D204). The conformance golden did not
change.

## Rejected

Reading the options from `pg_class` and `pg_attribute`, which every user can
read, was not measured. It is in `docs/BACKLOG.md`.

A fallback that catches the error 42501 and runs the plain statement was
rejected. It needs a second statement for each failure, and it hides from the
caller which of the two answers it got.

## Releases run

Redshift Serverless 1.0.477953.
