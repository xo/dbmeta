# D11. dbtpl is pinned as a tool, in the generation module

Status: Amended by D26, superseded by D71.

Nothing in this decision is in force. No `go.mod` here has a `tool` directive,
no generator is run, and D71 records why. It is kept because the reasoning
about where a build dependency belongs is still the reasoning that keeps
drivers out of the root module, which is D26.

`dbtpl` is pinned with the `tool` directive. Go 1.24 added the directive and
these modules target Go 1.27.1, so it is available.

The pin does not go in the root `go.mod`. D26 keeps database drivers out of the
root module, and `dbtpl` depends on four of them, so the pin lives in the
separate generation and testing module.

Pinning fixes the generator version. Two agents on two machines then generate
the same Go from the same SQL. Run the generator through `go tool dbtpl`, not
through whatever `dbtpl` sits on the path.

This pin is a build dependency, not a runtime dependency. It does not weaken
D7. The generated code and the root package still import the standard library
only.
