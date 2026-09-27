# D15. CI runs on GitHub Actions, on ubuntu-latest only

Status: Decided.

The other `xo` projects test on several runners because their code is platform
dependent. `dbmeta` is not. The queries are SQL and the code is pure Go, so one
runner is enough.

Keep the workflow small. Build, vet, and test. Do not copy the matrix
workflows from the other `xo` projects.

The repository already exists on GitHub. Use the GitHub MCP tools to read and
write workflows rather than guessing at the repository state.
