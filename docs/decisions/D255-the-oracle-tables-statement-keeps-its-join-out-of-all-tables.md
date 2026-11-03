# D255. The Oracle tables statement keeps its join out of all_tables

Status: Amends D253.

D253 recorded that the statement that reads tables took 77 seconds on Oracle
19c with two different drivers, and that the cause was not found. The `oracle`
session found it on 2026-11-04.

## The cause

The server had fresh dictionary statistics. This happens after a machine
restarts and dbrun makes the container again from the kept copy, and the
nightly statistics task of a server that runs for a while does the same. With
those statistics the optimizer expects one row from `all_objects`, which holds
about 9,500. It then drives from `USER$` and `OBJ$` and pushes the join
predicate into `all_tables`, so it runs that view once for each object, and
each run scans all of `OBJ$`. In 40 seconds it ran the view 978 times. The hard
parse takes 0.4 seconds, and the time is in the run. A fresh 26ai container
shows the same fault once its statistics look like this.

## The decision

The tables statement of `models/oracle` starts with the hint
`/*+ NO_PUSH_PRED(t) */`. The columns and the rows are the same.

## What was measured

With the whole dictionary statistics gathered, on 19c, the statement with all
the system objects took more than 200 seconds without the hint and 5.1 seconds
with it, and returned 9,542 rows. Without the system objects it took 4.0
seconds. The `oracle` session measured the same on 26ai, 129 seconds against
1.4. On 11g the statement with the system objects takes more than 400 seconds
either way, and the test already skips that case (D150).
