# D158. H2 and VoltDB wait

Status: Decided, amended by D188.

## The decision

On 2026-10-01 Ken asked for dialects for eight products that dburl names and
that had no model. Two of them cannot be built now, and Ken chose to leave
both Staged.

H2 waits for a driver. dburl names `github.com/jmrobles/h2go` for the h2
scheme, and h2go fails against both releases that dbrun runs, 2.4.240 and
2.5.252, with "Can't read all data needed" (D118). The H2 Shell answers on
the same servers, so the fault is between the driver and those releases. A
model needs a driver that reaches the server (D154).

VoltDB waits for a license file. The developer edition does not start
without one, and dbrun lists the VoltDB releases only while it finds the
file (D118). No file is on this machine, and nothing here signs up for a
license or downloads one.

Both entries stay in `container/` as Staged, so dbimp and usql can still use
them. Each is in `docs/BACKLOG.md` with what ends the wait.
