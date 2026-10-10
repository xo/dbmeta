# D215. Spanner Omni replaces the Cloud Spanner emulator

Status: Amends D118, amended by D217.

## The decision

Ken decided on 2026-10-10 that dbrun starts Spanner Omni and no longer starts
the Cloud Spanner emulator. The emulator entry, its release 1.5.58 and
`test/cmd/dbrun/image/spanner.Containerfile` are gone. Ken will also have the
`dbsetup` session make a Cloud Spanner instance on GCP, for tests when they
need one.

## Why

The emulator is a mock of the API. Omni is the engine of the service, and it
answers `information_schema` the way the service does. Google ships it as a
container image, `us-docker.pkg.dev/spanner-omni/images/spanner-omni`, and
the first pull needed no account.

## What was measured

On 2026-10-10, release 2026.r4-lts started with `start-single-server` and the
address 0.0.0.0, because the default address is localhost. The image has a
shell, so the setup is one `spanner databases create dbmeta`. The project and
the instance are both `default`. The Go driver connects with the DSN
`host:port/projects/default/instances/default/databases/dbmeta;usePlainText=true`
and reads `information_schema`, which holds the schemas INFORMATION_SCHEMA and
SPANNER_SYS.

## What does not change

The entry stays Staged, because D194 says dbmeta builds no Spanner model. The
hosted entry of Cloud Spanner stays too.

## The license

Omni is free for development and testing. It prints a warning that its license
expires in 90 days, and a person extends it with the form that the warning
names. A commercial edition needs a license from Google.
