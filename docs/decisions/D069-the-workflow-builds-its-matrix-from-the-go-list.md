# D69. The workflow builds its matrix from the Go list

Status: Amends D42.

The CI workflow names no release, no image and no port. It asks the runner
for the list as JSON and expands it with `fromJSON`, and each job runs the
runner against one server.

## What it replaces

D42 put the release matrix in `container/container.go` and had the workflow
repeat it in YAML, with a test failing when the two disagreed. That worked and it was a second copy: twelve jobs, one per product,
each with its own service block, its own image, its own health command and its
own environment. 690 lines.

Every one of those images was unqualified, which is its own fault. `mariadb:13.0`
resolves against whatever the runner's search list holds, so the same YAML
means one thing locally and another in CI, and `container/container.go` has
written every image in full for exactly that reason since it was created.

## What it is now

Six jobs and 276 lines. A `releases` job reads the list and hands it on, a
`server` job runs the Tested tier, a `nightly` job runs the rest, and `unit`,
`embedded` and `compare` are unchanged. Each server job is three steps and the
last one is `go run ./cmd/dbrun test "${{ matrix.server }}"`, which is
the same entry point a person uses, which is what D68 asks for.

## What the drift test became

There is nothing left to drift, so the test that compared two lists is gone.
Three take its place and they hold the property rather than the agreement: the
workflow must read both tiers from `dbrun list --json --names`, every image it
still names must carry its registry, and every image it still names must be one
`container.All` knows.

Only the comparison job names any, because it needs MariaDB and MySQL running
at once and the runner starts one server at a time.

## What this does not change

The list is still `container/container.go` and it is still the only copy. D42
decided that and it stands. What changed is that the workflow reads it instead
of repeating it.
