# D221. Redshift reads the Spectrum external tables

Status: Amends D212.

## The decision

The Redshift model reads an external table, which Redshift Spectrum reads from
files in S3 that a Glue data catalog describes. Tables lists it with the type
`external table`, and Columns lists its columns. The fixture makes an external
schema over a Glue table, and the parity scene gives the grantee USAGE on it.

## What was measured

On 2026-10-10, on Redshift Serverless 1.0.477953, with the IAM role
dbmeta-redshift-spectrum as the default role of the namespace. The role reads
the Glue database `dbmeta_spectrum` and one S3 prefix, and it cannot write to Glue.

1. `CREATE EXTERNAL SCHEMA dbmeta_spectrum FROM DATA CATALOG DATABASE 'dbmeta_spectrum'
   IAM_ROLE default REGION 'us-east-1'` works for the administrator. The fixture
   cannot make a Glue table, so dbsetup makes `spectrum_t` with the columns id
   (int) and v (string) and three rows. On a namespace with no default role the
   statement fails with "Cannot find default IAM role", and the fixture skips
   the step.
2. The external schema is a row of pg_namespace, so Schemas and Privileges read
   it already. The table is not in pg_class, pg_table_def or
   SVV_RELATION_PRIVILEGES. It is in SVV_EXTERNAL_TABLES, SVV_EXTERNAL_COLUMNS,
   SVV_TABLES and SVV_COLUMNS. SVV_EXTERNAL_PARTITIONS has no row for a table
   with no partition.
3. SVV_EXTERNAL_TABLES holds the location, the input and output formats, the
   serde library and its parameters. Its tabletype column is empty.
   SVV_EXTERNAL_COLUMNS holds the Glue type, as int or string, and an empty
   is_nullable.

## What the model reads

Tables adds a branch that reads SVV_EXTERNAL_TABLES. The type is `external
table`, the owner is the owner of the external schema, and the size and the
rows are absent, because no view counts them. Options holds the location and
the serde library, as `location=s3://bucket/path/, serde=...`. One statement
gives both, so D47 allows them. Columns adds a branch that reads
SVV_EXTERNAL_COLUMNS. The data type is the Glue type as it is. Nullable is true,
because a Glue column has no NOT NULL and the view leaves the flag empty.

Three things in Redshift cost the statements their shape.

1. A UNION with a query that reads pg_user is refused with "Specified types or
   functions are not supported on Redshift tables". The cause is pg_shadow, whose
   valuntil column has the type abstime. A derived table that names the two
   columns the statement needs avoids it.
2. Comparing a varchar column with `current_database()`, which is a name, calls
   a function that the compute nodes lack. The statement casts it to text.
3. Beside a UNION the type of a CASE takes the width of its ELSE. The identity
   column has the ELSE `''`, so the a and the d were refused as too long, and the
   statement casts it to varchar(1). The primary key column is a join to
   pg_constraint, because a correlated subquery cannot sit under a UNION.

## What is not read

SVV_EXTERNAL_PARTITIONS and the partition keys of an external table, because no
measured table has a partition. The serde parameters and the input and output
formats, which the serde library names well enough. The privilege on an external
table, which no system view lists. The external databases in
SVV_EXTERNAL_DATABASES and the data shares.

## Test support

A fixture step can name text in an error that skips it, as `SkipWhen`. The test
files read `DBMETA_UPDATE` as the default of `-update`, because dbrun starts
the test with the connection string and a person cannot pass a flag through it.
