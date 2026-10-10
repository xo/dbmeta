# D225. The BigQuery and Cosmos DB entries follow the drivers of dbimp

Status: Amends D118, amended by D228.

## The decision

Ken approved on 2026-10-11 that dbmeta changes the entries of BigQuery and
Cosmos DB for the drivers that dbimp wrote, which it tags as v0.17.0.

## What changes

The BigQuery emulator release 0.7.2 is gone. The driver of dbimp cannot read the
result of a query job on it, so the list holds 0.8.1 and nothing else. The URL
of the entry is `bigquery://admin@dbmeta/dbmeta?endpoint=http://127.0.0.1:<port>`,
which is the form the driver reads for an emulator, with no credential.

The URL of the Cosmos DB entry has the key as the password and any word as the
user: `cosmos://x:<key>@127.0.0.1:<port>/?insecure=true`. The driver takes the
database and the container from its caller, so the path is empty. The DSN of
the entry stays the form of gocosmos, which usql uses.

## What does not change

Both entries stay Staged. The hosted forms in `hosted/hosted.go` and the
dialect names change when dburl releases the schemes that name the drivers of
dbimp.
