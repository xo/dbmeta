# D170. InfluxDB 3 answers Triggers from the processing engine

Status: Amends D152.

## The decision

D152 left Triggers unanswered on InfluxDB 3, because the entry had no plugin
directory and the server refuses a trigger without one. Ken asked on
2026-10-07 to build it, from the item in the backlog.

A processing engine trigger runs a Python plugin when something happens: a
write to one table, a write to any table, a schedule or a request.
`system.processing_engine_triggers` lists the triggers of the database of the
connection, and the model reads it. The kinds map as follows:

- The schema is iox, because every measurement is there.
- The table is read out of the specification, which is JSON text such as
  `{"single_table_wal_write":{"table_name":"author"}}`, for a write to one
  table. It is empty for the other three kinds, which have no table.
- The state is enabled, or disabled where the trigger was made disabled.
- The definition is the specification as the server keeps it. `Trigger` has
  no field for the plugin file, its arguments or its error behavior, so they
  are not carried.

The entry now starts the server with a plugin directory, and its setup writes
a plugin that does nothing and makes one trigger, `dbmeta_noop`, on
`author`. The setup is safe to run again: the server answers 409 to a trigger
that exists, and the setup accepts that. The server accepts a trigger on a
table that does not exist yet, so the trigger can be made before the fixture
writes its first point.

The model answers 9 of the 56 kinds, and the release lists of 3.10.6,
3.11.6 and 3.12.0 were measured on 2026-10-07.
