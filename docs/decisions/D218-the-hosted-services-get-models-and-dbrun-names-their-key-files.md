# D218. The hosted services get models and dbrun names their key files

Status: Amends D194.

## The decision

Ken decided on 2026-10-10 that dbmeta builds a model for each of BigQuery,
Cloud Spanner, Cosmos DB, Athena and Databricks, and that the dbsetup session
provides the accounts. D194 said that no model is built for them. It stays
true for MaxCompute and Tablestore, which have no account. D216 did the
Spanner model first, against Spanner Omni.

## The accounts

dbsetup made each account and wrote the connection strings to the credential
files of D117, in `$XDG_CONFIG_HOME/dbmeta/credentials`. Each service has an
administrator in the file `<name>` and a lesser principal in the file
`<name>-reader`. The reader is the second principal of hard rule 16, and the
parity test connects as it. The name `<name>-reader` is the convention for a
second hosted principal.

Each administrator owns one dataset, schema or database named `dbmeta` and
cannot create or drop the container of it, so a fixture builds and drops
objects inside it and never the container.

## The key files

BigQuery and Cloud Spanner read a key file that
`GOOGLE_APPLICATION_CREDENTIALS` names, and each principal has its own key.
dbrun sets the variable to `$XDG_CONFIG_HOME/dbmeta/gcp/<credential file
name>.json` when it runs the tests or usql on a hosted service, for example
`gcp/spanner.json` for `spanner` and `gcp/spanner-reader.json` for the reader.
If the variable is set already, or the file is not there, dbrun changes
nothing. The credential files of these two hold no secret.

## What does not change

A hosted service is Verified and never Tested, so CI never runs one (D119).
The tokens and keys expire, and dbsetup renews them. A hosted model needs the
driver that dburl names, which is a driver of dbimp for BigQuery, Cosmos DB,
Athena and Databricks once they are tagged.
