# D267. usql drops GizmoSQL, so dbmeta drops its model

Status: Supersedes D187.

usql dropped its h2 and gizmosql drivers, and dburl v0.56.0 removes the two
schemes and the aliases gz and gizmo. The maintainers decided on 2026-11-05 that the
gizmosql model goes with them. Two things led to it. The gizmosql driver cannot
keep its session, which is why D187 had the tests open one themselves. The
module of the h2 driver was last released in 2020.

## Decision

The `gizmosql` model, its fixture, its container entry and its tests are
removed. So are the `GizmoSQL` dialect, the version key of the same name in the
duckdb model, and the one fragment that gated on it. `internalOf` is one
statement again. The Flight SQL helper that opened a session in the test module
is removed.

`models/duckdb` is unchanged for DuckDB. The cast of the `@types` parameter in
the table query stays, because DuckDB cannot infer the type of a bare parameter
in a `||` chain.

dbmeta has no h2 model. The H2 entries in `container/` are not part of this
decision.
