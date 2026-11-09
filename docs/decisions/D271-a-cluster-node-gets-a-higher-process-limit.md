# D271. A cluster node gets a higher process limit

Status: Amends D269.

D269 starts a kind cluster for dbfwd. Podman gives every container a limit of
2048 processes and threads, and the node of a cluster is one container that
holds the whole cluster. dbfwd ran every operator suite in one cluster and used
the limit up. ClickHouse stopped for want of threads, and Couchbase probes
failed with "resource temporarily unavailable" at 1934 of 2048. dbfwd raised
the limit by hand with `podman update --pids-limit 16384`, and asked on
2026-11-08 that dbrun do it. The maintainers agreed.

## Decision

When the runner is podman, `dbrun` runs `podman update --pids-limit 16384` on
the node of a cluster. It does this when `start` creates the cluster, when
`start` brings a stopped cluster back, and when `start` finds the cluster
already running. The last case repairs a cluster that was created before this
decision. Docker has no such default, so `dbrun` does nothing for it.

kind has no setting for the limit under podman, so the update after the node
exists is the only way. A server container keeps the default, because one
database does not need more.

## Rejected

A kind configuration file. It cannot set a process limit for the podman
provider.

Leaving it to the test. Every consumer of a cluster must then repeat the
command, and each one finds the fault first by a failed test.
