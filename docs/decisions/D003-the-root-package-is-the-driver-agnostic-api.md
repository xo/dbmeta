# D3. The root package is the driver agnostic API

Status: Decided.

External projects use the root package. They do not import
`dbmeta/models/<driver>` to do ordinary work.
