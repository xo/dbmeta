# D150. Oracle 11g reads two views slowly

Status: Decided.

## The decision

A backlog item asked why Oracle 11g XE reads four queries slowly with the
system objects included: tables, types, privileges and column_stats. The
plans were read on 2026-09-30, on a fresh 11g container with nothing else
running on its one CPU.

Two views are the cost, and nothing a query can do avoids them. Counting
`all_objects` takes 196 seconds and counting `all_types` takes 126.
`dba_objects` takes 0.1 seconds, and every other view the queries read takes
about a second. Each row of the two views runs a chain of privilege checks
against the fixed tables `X$KZSPR` and `X$KZSRO`, and an ordinary user waits
as long as SYSTEM. The image's dictionary statistics date from 2011 and its
fixed objects had none. Gathering both took five minutes and changed neither
time. 18c and later read both views in seconds.

So tables, which reads `all_objects`, and types, which reads `all_types`,
stay slow on 11g, and the scan test goes on reading 11g without the system
objects. `dba_objects` is not a way out, because an ordinary user cannot read
it.

## What was fixed

privileges looked up the type of each object in `all_objects`, once for each
object with a grant. From 12c `all_tab_privs` has a `TYPE` column. On 26ai it
agrees with the lookup for all but 121 of 9,965 objects. It says UNKNOWN for
a domain, a consumer group, a job class and an evaluation context, and it
names three queues and two users that `all_objects` does not. So from 18c the
statement takes `TYPE` and runs the lookup only where it says UNKNOWN, and
11g keeps the lookup. On 26ai the query takes 0.1 seconds over every schema.

column_stats joined `all_tables` for the row count of each table. Over every
schema on 11g the join took 304 seconds, where each view alone takes a
second. A lookup for each row, by owner and table name, takes 41 seconds on
11g and 0.8 on 26ai. For one schema it takes 0.3 seconds, where the join
took 0.4. Materializing `all_tables` first took 18 seconds over every schema
and 10 for one schema, so the lookup is the better of the two.
