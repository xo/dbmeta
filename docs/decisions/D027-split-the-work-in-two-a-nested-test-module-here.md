# D27. Split the work in two: a nested test module here, a shared harness in dbtest

Status: Decided.

Both reviews were asked and both gave the same answer, which was neither of the
two options as posed. Split by what is reusable.

A nested module in this repository holds what belongs to `dbmeta` alone:
the integration tests and the database drivers they need. It once also held the
`tool` directive that D11 moved out of the root, and D71 removed the last of
that.

The sibling repository `xo/dbtest` holds what three projects share: the
podman container harness. `usql` backlog item 6 already asks for it, and
`usql` and `dbtpl` will use it too.

Neither review liked the alternatives. Putting `dbmeta`'s own integration tests
in a separate repository means a change to a query and the test that covers it
land in two repositories, and CI must check out both and rewrite a module path
to test an unreleased `dbmeta`. Putting the shared harness in a nested module
here means `usql` has to depend on a module inside `dbmeta` to start a
container.

## What actually reaches a consumer

The two reviews disagreed here and the accurate answer is between them.

A `tool` directive adds a `require` line to the module that declares it. That
much is certain, and it is why D26 moves the pin out of the root.

What a consumer of `dbmeta` gets is less than it looks. Module graph pruning
drops a dependency that provides no imported package, so the drivers reach
neither the consumer's build list nor its `go.sum`. DeepSeek said nothing
reaches a consumer at all. Gemini added the qualification that matters: a
scanner that reads `go.mod` as text rather than resolving the build list will
report those drivers, and a consumer then sees advisories for code it never
compiles.

So the honest statement is that the root module having no driver is about the
dependency surface people read and scan, not about what the compiler links.
That is still worth having, and with D27 it costs nothing.

## Verified: a nested module needs no underscore

Gemini suggested naming the directory `_test`, because the Go tool ignores a
directory whose name begins with an underscore. DeepSeek said an underscore is
unwise. DeepSeek is right, and this was checked rather than argued.

A throwaway module was built with a nested `test/go.mod` and a separate
`_hidden/` directory. Running `go list ./...` in the parent listed neither. The
nested module is already excluded from the parent's `./...` because it has its
own `go.mod`. The underscore adds nothing and it hides the directory from
Dependabot, Renovate and editors as well.

Use a plain name. `test` is the obvious one.

Do not put it under `internal/`. It does not need to be, because a nested
module is already a boundary, and `internal/` blocks reuse if any part of
it is later shared.

## What this costs, and what to do about each

A nested module is not free. Four things need handling and none is hard.

1. `go test ./...` from the root does not descend into it. CI runs it a second
   time from inside the directory. Verified above.
2. Editors and a local `go build` across both modules want a `go.work` file.
   Do not commit it.
3. Dependabot needs a second entry pointing at the nested directory, or it will
   never see those drivers.
4. Tagging a nested module requires a path prefixed tag, such as
   `test/v0.1.0`. This only matters if something outside ever imports it, and
   nothing is meant to.

## Precedent

Both reviews named `golang.org/x/tools/gopls`, which is a nested module
specifically so that the heavy dependencies of `gopls` stay out of the
lightweight `x/tools` library. Gemini also named the OpenTelemetry Go
repository, which uses nested modules per instrumentation to keep third party
drivers out of the core SDK. Other examples they gave were not verified.
