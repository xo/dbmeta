# D213. Snowflake tests and dbrun use the driver of dbimp

Status: Amends D190, D193 and D203, amended by D214.

## The decision

dburl v0.49.0 moved the `snowflake` scheme from gosnowflake to
`github.com/xo/dbimp/snowflake`, in dbimp v0.16.0. The dialect is still
`snowflake`. Hard rule 10 and D154 say that the driver of the `test` module is
the one that dburl names, so the tests and `dbrun` moved to it on 2026-10-10.
The `test` module pins dburl v0.49.0 and dbimp v0.16.0, and it no longer
requires gosnowflake. D190 named gosnowflake as the driver of the first
measurements, and D193 and D203 measured on it. Their facts about Snowflake
hold. Only the driver changed.

## The connection string

The form is `snowflake://user:key@host/database/schema?role=r&warehouse=w`.
The password of the URL is the private key, as the base64url text, with no
padding, of its PKCS8 DER bytes. The query takes only `role`, `warehouse`,
`timeout` and `timezone`, and the driver refuses any other key. The made users
of the tests build this form: they take the host from the connection string of
the administrator and replace the user, the key, the role, the warehouse and
the path. `dbrun` passes `u.DSN` of dburl to `sql.Open` (D203). Nothing prints
a key.

## What the new driver does differently

1. The SQL API runs each statement in a session of its own. `USE` is refused
   ("391911 (0A000): Command not supported by SQL API: USE"), and a temporary
   table is gone when the next statement runs. The model needs the database and
   the schema from the URL path now. The two key statements of D203 say
   `IN DATABASE` and `IN TABLE` with the catalog, so they never depended on the
   session. The test of D203 that moved the session to another schema tested a session
   that no longer exists, so it is deleted. Persistence "temporary" cannot be read
   across statements through this driver, so the check of the table SCRATCH is
   one named skip with that reason.
2. The SQL API sends a boolean of a piped statement as the text 0 or 1. The
   driver of dbimp reads only true and false, so `Columns` failed with
   `reading "0" as a boolean`. A plain SELECT sends true and false, and no
   other statement of the model failed. `Columns` is the only piped statement
   that selects a boolean. `ConstraintColumns` and `Settings` select none. So
   `nullable` and `primary_key` are `IFF(..., 1, 0)` in the statement, and
   database/sql converts the number for the bool field. The cast can go when
   dbimp reads 0 and 1 for a boolean. A request for that fix goes to dbimp.
3. The pipe operator is accepted by the new driver. D203 measured it on
   gosnowflake, and it holds on this one.
4. The session has only what the URL holds. The administrator has a database
   and no schema, so `CURRENT_SCHEMA()` answers NULL, where gosnowflake gave
   the default schema of the user. The fixture tests name the schema, so they
   are unchanged.
5. A password cannot log in. The driver reads the password of the URL only as a
   private key. `TestChangePasswordSnowflake` runs the statement that
   `Dialect.ChangePassword` builds for each hostile password, against a user the
   test made, and checks that Snowflake accepts it and that the user holds a
   password afterward, read from the column `has_password` of `SHOW USERS`.
   The login with each password was verified live on 2026-10-08 with
   gosnowflake (D193: seven hostile passwords, with the 13 character prefix
   that the password policy of the account needs). It is not repeated. The
   escaping of `ChangePassword` stays covered by the root tests.
6. The version query `SELECT CURRENT_VERSION()` works unchanged.

## Parity

Only the Snowflake sections of `test/testdata/parity.txt` were recorded again.
Two things changed.

1. The stranger has no database in the URL path, and the SQL API refuses the
   statement before it runs. The refusal text was `090105 (22000): Cannot
   perform SELECT. This session does not have a current database. Call 'USE
   DATABASE', or use a qualified name.` It is now `snowflake: 391918 (22000):
   Unable to run the SELECT command. You must specify the database to use by
   either setting the database field in the body of the request or by setting
   the DEFAULT_NAMESPACE property for the current user.` The set of refused
   queries is the same. The wrapper "Uncaught exception of type
   'STATEMENT_ERROR'" still shows on `columns`.
2. `current_schema` no longer differs for the grantee and the visitor, because
   the administrator has no current schema either (item 4 above).
