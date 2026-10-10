# D219. Spanner is measured on Cloud Spanner and a database role is its principal

Status: Amends D216.

## The decision

Ken approved on 2026-10-10 that the Spanner model is measured on Cloud Spanner
(D218). dbsetup made the instance dbimp-spanner and the database dbmeta, and
two principals: an administrator and a reader that holds databaseReader. This
decision records what that measurement found and changes three things in D216.

## Cloud Spanner and Spanner Omni agree

Both have the same 47 views in INFORMATION_SCHEMA, the same optimizer versions,
and the same named schema rules. The fixture builds and drops in the schema
dbmeta_fixture on both, a vector index and a change stream are refused in a
named schema on both, and the hash join hint on NotNulls works on both.
TestConformance records one answer for the two.

Three things differ:

- SESSION_USER answers on Cloud Spanner, with the IAM principal such as
  name@project.iam.gserviceaccount.com. Spanner Omni, with no authentication,
  refuses it. `CurrentUser` is registered because of it, and it is an error on
  Omni and not an answer. The model has 22 of the 65.
- The default locality group reports the option storage=ssd on Cloud Spanner
  and no option on Omni.
- SPANNER_SYS.TABLE_SIZES_STATS_1HOUR holds one row, for unknown_table_name, on
  an instance that has no table older than the hour.

## D216 was wrong about roles

D216 said that Spanner Omni enforces no database role, and that Spanner was
exempt from the parity test for that reason. The measurement was wrong. The
property that names a role in the DSN of go-sql-spanner is `database_role`, and
the test wrote `role`, which the driver ignores without a word. With
`database_role` both Omni and Cloud Spanner refuse a role that does not exist,
refuse a table that the role holds no grant on, and filter INFORMATION_SCHEMA by
what the role holds. The test that claimed it is replaced by TestSpannerEnforcesRoles, and the
parityExempt entry is gone.

## The principals

Hard rule 16 holds for Spanner. A database role that the administrator names is
a principal on both servers: dbmeta_reader holds SELECT on a table, a column, a
view, a change stream and a function, and dbmeta_stranger holds nothing. The
service account of the reader holds databaseReader and no role permission, so it
reads as an IAM principal, cannot name a role (it lacks
spanner.databases.useRoleBasedAccess), and gets the administrator's answer to
every query but CurrentUser. It is a principal on Cloud Spanner only, and the
test names its key file in the DSN with the property `credentials`, which holds
a path and no secret.

A role reads fewer rows for 16 queries on Cloud Spanner and 15 on Omni, and the
stranger for 18 on both. No query is refused, because INFORMATION_SCHEMA filters
and does not refuse. The one difference is Schemas for dbmeta_reader: Cloud
Spanner leaves out the default schema and Omni lists it. The parity file keeps a
section for each, `spanner` and `spanneromni`, and parityName chooses by the
DSN, because the two report one version (D61).
