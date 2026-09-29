# D137. A child kind takes parent for its owner and name for itself

Status: Decided.

## What was found

usql found on 2026-09-30 that Oracle's child kinds take the table as `name`.
Columns, indexes, index columns, constraints, constraint columns, triggers,
routine parameters and column statistics took `schema`, `name` and
`with_system`, and `name` filtered the table. Every other model took
`parent` for the table and `name` for the column, index, constraint or
trigger. So a caller that asked for the columns of one table with
`dbmeta.Args{Parent: table}` got ErrUnknownParam from Oracle alone, and had to
know which model it was talking to.

A test over every model found the same thing in four more. ClickHouse took
the table as `name` on columns, indexes, index columns and constraints.
Cassandra and ScyllaDB did on six kinds, Couchbase on indexes, index columns
and routine parameters, and Vertica took no `parent` on triggers. SAP HANA and
Firebird took `parent` on routine parameters and no `name`. Seven models
described the `parent` of a routine parameter as a table, when it is the
routine.

## The decision

Every query of a child kind takes `parent` for the object it belongs to and
`name` for the object itself: the table of a column, an index, a constraint, a
trigger or a column statistic, and the routine of a parameter.
`TestEveryChildKindTakesParent` in `all` holds every model to it. It fails
when a supported child kind takes no `parent` or no `name`, and when the
`parent` of a routine parameter is described as a table.

Where the product cannot filter, the parameter is still declared. Cassandra
narrows nothing, because CQL cannot express an optional filter (D62), and its
`parent` says so. A Vertica trigger names no table, so its `parent` compares
the empty table name, and a filter on a table matches no trigger.

The change is to the names of parameters, and it is not compatible with a
caller that passed the table as `name` to one of these models. dbmeta has no
tag yet, and usql had a fallback for it, which it can now remove.
