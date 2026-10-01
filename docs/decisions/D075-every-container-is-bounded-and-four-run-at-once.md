# D75. Every container is bounded, and four run at once

Status: Amended by D98 and D108.

`container.MemoryLimit` is `4g` and every container this project starts is
given it. `dbrun` starts a fifth server by stopping the longest running one,
and says which.

Neither existed before and the machine showed why. A session that starts a
server per question ends with a dozen up, all idle. Thirteen were running at
once on the development machine, including two query engines and a Windows
machine, and a database given the whole host will take it: SAP HANA alone had
taken 3.1g and Oracle 2.3g while neither was being used.

## The limit is one value, the way the password is

`MemoryLimit` is a constant rather than a field with a default, for the same
reason `Password` is. These are throwaway servers holding fixture data, none
of them is doing real work, and a per product number is a thing to tune rather
than a thing to read.

One product does not fit and it carries the exception on `Server.Memory`. An
exception is allowed only where the product was measured and the number is
written down beside it. SAP HANA is the only one, at `8g`, and
`container/hana.go` holds the measurement.

## Why the eviction rather than a refusal

The alternative was to refuse the fifth start and name what to stop. It is
rejected because `dbrun start all` and `dbrun test all` both walk the list,
and a refusal turns either of those into an error on the fifth server rather
than into a run.

Evicting the longest running is the right one to lose. The server somebody is
using is the one most recently started, rebuilding a container is a minute,
and the line says what happened rather than leaving a server mysteriously
down. A Windows machine is never evicted: D57 keeps one because rebuilding it
is an hour.

## What it cost to get right

Two fields on `container.Server` that no product needed before SAP HANA, and
both are data rather than behavior. `RunFlags` goes before the image and
`Args` after it, because HANA's entrypoint takes the initial password and the
license agreement as command line arguments and reads no environment variable
for either. `Startup` is a third, because HANA answers in 108 seconds where
every other product here answers within 90.

The first attempt set `--ulimit nofile=1048576:1048576`, which rootless podman
refuses outright: the host's hard limit is 524288 and a container already gets
it. The flag was both impossible and unnecessary, which is the sort of thing
only running it finds.
