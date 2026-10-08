# D182. Redshift ran against Redshift Serverless

Status: Amends D144, amended by D204.

## The decision

The Redshift model ran for the first time on 2026-10-08, against Redshift
Serverless in us-east-1, release 1.0.434008. A person provisioned the
workgroup and a connection string that dbrun resolves (D117). Redshift now has
parity targets and is no longer exempt from parity. Snowflake is unchanged and
has not run. D144 said to finish both when a connection string existed, and
this finishes one of them.

## What failed

One statement failed. The roles query cast `valuntil` to text, and Redshift
refuses with `cannot cast type abstime to character varying`. The statement
now casts it to a timestamp first and then to a string. A user with no expiry
reads as absent, and the administrator reads as `infinity`.

The test failed before any statement ran. dburl names the scheme `redshift`
and pgx reads `postgres`, so the tests rewrite the scheme before they open a
connection. Nothing else in the model needed the rewrite, because the model
never opens a connection.

The fixture, the version query and every other statement ran as written. The
version banner is `PostgreSQL 8.0.2 on i686-pc-linux-gnu, ..., Redshift
1.0.434008`, and `parseVersion` reads the number after the word Redshift.

## What changed

Three changes came from what the server answered.

1. Three schemas that Redshift keeps for itself, `pg_auto_copy`, `pg_mv` and
   `pg_s3`, are hidden by default beside the ones the model already hid.
   They hold tables in pg_class that a person never made.
2. The identity kind of a column is now answered. Redshift writes an IDENTITY
   column's default as the call `"identity"(<oid>, 0, '1,1'::text)`, so a
   column with that default reports `a`, which means always, because an
   INSERT cannot give it a value. Every other column reports an empty kind.
   The earlier note said that pg_attribute has no identity kind, which is
   true, and the default was still a source.
3. The roles cast above.

## What parity found

Three principals were measured: a user that owns the fixture schema and its
relations, a grantee with USAGE on the schema and SELECT on its tables, and a
stranger with no grant. All three read the same rows as the administrator on
every query. Only `current_user` differs, as it does everywhere. The model
reads pg_catalog, and Redshift filters nothing there by user. The SVV views do
filter by user, and the model reads none of them. If a later change reads an
SVV view, run parity again.

Redshift has no DO block and no REASSIGN OWNED, so the owner principal hands
its tables back by name before the user is dropped. The first version of that
cleanup ran with a canceled context and left both users behind. It now drops
the cancellation, as `cleanup` does.

## What stays open

- The privileges and roles in the SVV views, the collation of a column, the
  external tables of Spectrum and datashares are not read. A role of Redshift
  is in SVV_ROLES, and a user of IAM identity such as `IAM:RootIdentity`
  appears in pg_user beside the database users.
- No conformance target exists for Redshift, so none ran.
- The ALTER USER statement of D56 was not run against the server. The
  documentation forbids a quote, a backslash, a slash, an at sign and a space
  in a password, so the hostile passwords of D127 cannot be set.
- Each test pass ran once, to keep the cost of the trial credit small.
