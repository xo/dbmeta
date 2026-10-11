# D231. Hosted Spanner tests use the driver of dbimp and Omni has its own binary

Status: Amends D229 and D154.

## The decision

The tests of Cloud Spanner open `github.com/xo/dbimp/spanner`, which dburl v0.51.0
names for the spanner scheme. D154 is met for Spanner, and the exception that D229 kept
is over. dbimp v0.19.0 added the key `database_role` and DDL batches, which were two of
the three reasons for the exception.

The tests of Spanner Omni open `github.com/googleapis/go-sql-spanner` by hand, because
Omni speaks gRPC only and the driver of dbimp speaks REST. Both drivers register the
name `spanner`, so one binary cannot import both. The Omni tests run in a binary of
their own, which the Go build tag `omni` builds. The tag is not an operating system or
an architecture, so hard rule 11 does not forbid it.

Cloud Spanner stays Verified, and Spanner Omni stays Tested in CI.

## How the two binaries share the tests

The tests, the fixture, the conformance expectations and the parity expectations are
one set of files. Two small files hold what differs, and the tag chooses one of them:

- `test/spanner_hosted_test.go` has the tag `!omni`. It imports the driver of dbimp. It
  runs the fixture as one `Exec` of several DDL statements, which the driver sends as
  one request. It names a role with `&database_role=<role>` in the URL, and it reads the
  URL of the reader from the file `spanner-reader`.
- `test/spanner_omni_test.go` has the tag `omni`. It imports go-sql-spanner. It runs the
  fixture with `START BATCH DDL` and `RUN BATCH`. It names a role with
  `;database_role=<role>`. The reader principal skips, because Omni has no
  authentication.

The golden files do not drift, because both binaries read the same files. The section
for each product has its own name in `parity.txt`, and `conformance.txt` is shared.

`dbrun` follows the same rule. A default `dbrun` links the driver of dbimp. A `dbrun`
built with `-tags omni` links go-sql-spanner, and only it can read the version of
Omni. `dbrun test` does not need that version, so the default `dbrun` starts Omni and
runs its tests. `dbrun version` on a target of the other build prints a message that
says which build it needs.

## How dbrun picks the binary

`dbrun test` runs `go test -tags omni` for the container that answers Spanner, and
`go test` for the hosted Cloud Spanner. If a prebuilt binary is wanted, the variable
`DBMETA_TEST_BINARY` names the default one and `DBMETA_TEST_BINARY_OMNI` names the Omni
one. The name of a tag in capitals follows `DBMETA_TEST_BINARY_`. A target with a tag
never runs the default binary, because that binary cannot reach its server.

The CI workflow builds both test binaries in the build job and uploads both, and every
matrix job sets both variables. The workflow still reads the release list from
`dbrun list --json`, so it names no release of its own (D69, D82).

## What changed

The test module no longer has `test/internal/spannerdsn`. The credential file of Spanner
holds the URL that the driver of dbimp reads, and `dbrun` hands it to the tests as it
is. The URL of Omni and its DSN stay in `container/spanner.go`.

The fixture of Cloud Spanner took 461 seconds statement by statement. As one batch of 34
statements it took 91 seconds, which is one operation that Spanner runs.

Conformance and parity of Cloud Spanner did not change.
