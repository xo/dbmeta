# D217. The Cloud Spanner emulator returns beside Spanner Omni

Status: Amends D215.

## The decision

Ken decided on 2026-10-10 that dbrun starts the Cloud Spanner emulator again,
and that Spanner Omni stays. The emulator is the product `spanneremulator`,
release 1.5.58, with the image built from
`test/cmd/dbrun/image/spanneremulator.Containerfile`. It is Staged, because no
model reads it. Spanner Omni stays the product `spanner`, and the model reads
it (D216).

## Why

dbimp measured Spanner Omni on 2026-10-10 and found that no port serves the
REST API of Spanner. Omni listens on 15000 to 15025 and speaks gRPC only. One
port, 15012, is an HTTP status page. The emulator serves REST on 9020 through
`gateway_main`, so a driver that speaks REST can test against it only there.
dbimp D186 dropped the REST driver for that reason, and Ken hopes that a later
Omni serves REST.

## What changes

The emulator publishes 9020 as its second port and prints it as `api` (D167).
Its setup is the one that D215 removed. D215 said that Omni replaces the
emulator, and that is no longer true.

## What does not change

The model, its fixture and its tests read Spanner Omni, and they use the
product name `spanner`. The hosted Cloud Spanner entry names the product
`spanner` as its container.
