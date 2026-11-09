# D278. The views that a view reads are a kind of their own

Status: Decided.

A dump writes a view after the views that it reads (D274), and no reader said what
a view reads. The maintainers decided on 2026-11-08 that dbmeta adds the readers that the dump
needs.

## Decision

`dbmeta.ViewDependencies` yields one `ViewDependency` for each relation that a view
reads: the view, the relation, and whether the relation is a table, a view or a
materialized view. It is a kind of its own and not a field of `View`, because a
view reads many relations, and a child of an object is flat rows (D47).

PostgreSQL answers it from the rewrite rule of the view and `pg_depend`, with the
dependency type n. The view depends on its own rule and its own columns, and the
query leaves those out. The cost is one join of the rules with the dependencies
and is the same for a catalog of any size, because it reads the rules of views
only.

No other model answers it yet. SQL Server has `sys.sql_expression_dependencies`
and Oracle has `all_dependencies`, which are named from memory and are for the
step that makes them targets. A product that does not answer gets its views in
order of name, and the snapshot says so with `view_dependencies` set to false.
The first view of such a script carries a warning that the order is not checked.

## Consequence

`dump.Read` puts the views that a view reads in `View.DependsOn`, as
`schema.name`, and `Script` writes the views in that order, each after the views
it reads.
