# D269. dbrun starts a Kubernetes cluster for dbfwd

Status: Decided.

dbfwd forwards a port to a database that runs inside Kubernetes, and its tests
need a temporary cluster. Ken decided on 2026-11-07, in dbfwd D14, that dbrun
starts and removes it. The first rule of dbrun says that nothing else starts a
container on a shared machine, and a kind cluster is a container. dbfwd asked,
and Ken answered the three open questions on the same day.

## Decision

A cluster is a fifth kind of target, beside a container, a machine, an embedded
database and a hosted service. It is a bare cluster that kind runs as one node
container, and dbrun installs nothing in it. dbfwd installs its own operators.

The list is in `container/cluster.go`, as Go data, one entry for each of the
four newest Kubernetes lines, from the notes of kind v0.33.0. Each node image
is pinned by tag and digest. A cluster is not in `container.All` and not in a
tier. A selector such as `all`, `tested` or `staged` never reaches one, so
neither does the CI matrix of dbmeta. The selectors are `k8s-1.37`, `k8s` for
the newest, and `clusters` for every release. dbfwd reads the releases with
`dbrun list --json --names clusters`.

| Command | What it does |
| --- | --- |
| `start` | Creates the cluster with `kind create cluster`, or starts the node of a stopped one and writes the kubeconfig again. It returns when the node is Ready and the API answers. |
| `stop` | Stops the node and keeps it. |
| `remove` | Deletes the cluster and its directory, only for its owner. |
| `status` | Shows each cluster of the release with its owner. |
| `dsn --json` | Gives the `cluster`, the `context`, the `kubeconfig` and the `image`. |

The commands that connect to a database, `usql`, `cli`, `version`, `test` and
`fixture`, do not apply to a cluster.

The kubeconfig is its own file with mode 0600, in
`~/.local/share/dbmeta/clusters/<cluster>/`. kind writes only to the file that
the command names, and dbrun never touches `~/.kube/config`.

## Owners

Nobody shares a cluster. A test run installs operators and their CRDs across
the whole cluster, so two owners in one cluster clash on operator versions.
The name of a cluster is `k8s-<release>-<owner>`, where the owner is the first
eight lower case letters and digits of the owner, after the kind of owner. The
node container is named `<cluster>-control-plane`, as kind names it.

kind cannot put a label on its node container, so the owner and the friendly
name are in `state.json` beside the kubeconfig. A stopped cluster of another
owner can be removed, as a stopped container can (D108). A running one cannot,
unless `--force` says so.

## The limit

At most two clusters run at once. The limit is its own, and the eight servers
of D108 do not count a cluster, because one cluster is several processes and
one to two GB of memory. A start that makes a third run is refused and names
the owners. `--force` removes the oldest. A cluster is never stopped to make
room, because it holds the operators and fixtures of a test run and takes
minutes to build again. Ken confirmed the limit on 2026-11-07.

## The host

`start` does not check the host before it runs kind. If kind fails, the error
adds what dbfwd found on 2026-11-07: the four kernel modules that rootless kind
asks for, and the log driver of podman. Loading the modules needs root.

Rejected: a check of the host before every start, because a kernel can have a
module built in, and a refusal on a host that works is worse than a hint on a
host that does not.
