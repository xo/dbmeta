# D194. No models for hosted services beyond Redshift and Snowflake

Status: Amends D188, amended by D216 and D218.

## The decision

Ken decided on 2026-10-08 that dbmeta builds no model for a hosted service
other than Amazon Redshift and Snowflake. This is no priority at present, and
Ken can revisit it.

The hosted services without a model are Athena, BigQuery, Cosmos, Databricks,
MaxCompute, Tablestore and Spanner. D188 listed them as the ones that usql
supports and that dbmeta does not cover.

## What does not change

The entries of BigQuery, Cosmos and Spanner stay Staged in `container/`, for
the drivers of dbimp and usql, and `dbrun` still starts their emulators. No
model reads them.
