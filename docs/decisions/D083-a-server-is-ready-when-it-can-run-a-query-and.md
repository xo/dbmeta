# D83. A server is ready when it can run a query, and keeps being able to

Status: Decided.

The readiness check for Presto and Trino creates a schema and drops it. It
asked for a constant, and that was not the same thing.

`presto-cli --execute "SELECT 1"` succeeded and the fixture's first statement
then failed:

	NO_NODES_AVAILABLE: No nodes available to run query

It took three attempts, and the first two are kept here because each one was
a reasonable inference that a real server refuted.

## The first fix was wrong, and it is worth saying why

`EXPLAIN (TYPE DISTRIBUTED)` looked conclusive, on Presto 0.299 and Trino 483
alike:

| query | plan |
| --- | --- |
| `SELECT 1` | one SINGLE fragment |
| `SELECT * FROM (VALUES 1) t(x)` | one SINGLE fragment |
| `SELECT count(*) FROM system.runtime.nodes` | a SINGLE and a SOURCE fragment |

A SINGLE fragment is evaluated by the coordinator alone and a SOURCE fragment
has to be scheduled on a node, so reading `system.runtime.nodes` ought to have
waited for one. It shipped, and Presto failed the same way on the next run:
every test, from the first, inside three seconds of the server being called
up.

The plan was true and the inference from it was not. The coordinator serves
its own node list before the scheduler will place connector work on it, so a
query that must be scheduled is still not a query that proves the thing the
fixture needs. Reasoning about an engine's internals from what it says about
a plan is how this went wrong twice in one day.

## The second fix was also not enough

The check then created a schema and dropped it, which is the operation that
was failing. It passed, and Presto failed again, 1.1 seconds later:

	19:02:12.3   up, after the check created and dropped a schema
	19:02:13.4   CREATE TABLE memory.dbmeta_fixture.author
	             NO_NODES_AVAILABLE: No nodes available to run query

Doing the work is necessary and it is not sufficient, because the answer
expires. The server's own log says why, on startup:

	internal-communication.node-discovery-polling-interval-millis   5000

The coordinator refreshes which nodes it will schedule on by polling
discovery every five seconds. Between two refreshes the set is a snapshot, so
a server that runs a statement now can refuse the next one. This is not a
startup race at all. It is a set that lapses, and no single check of any kind
can see it.

## What it is now

Two things, and both are needed.

The check is the work:
`CREATE SCHEMA IF NOT EXISTS memory.dbmeta_ready; DROP SCHEMA IF EXISTS memory.dbmeta_ready`,
in one call, for both products. Both
statements run in one invocation and the client exits non-zero when either
fails, which is what `dbrun` reads. Nothing is left behind: `dbrun` stops
polling on a zero exit, and a poll that leaked the schema is not a poll that
returned zero. Measured on 0.299: the schema list before and after is
`default` and `information_schema` both times, and a statement against a
catalog that does not exist exits 1.

The check has to keep passing. [container.Server.Settle] is how long, and
`waitReady` restarts the clock on any failure. Presto and Trino set twelve
seconds, which is two discovery refreshes and a margin. Every other product
leaves it zero and returns on the first pass, as before.

Trino takes both on the same evidence rather than on a failure of its own. It
has not been unlucky yet and it is the same server at this level.

`TestSettleRestartsAfterAFailure` is the one worth keeping. It drives the
real `waitReady` with a command that passes, fails once and passes again, and
fails if the settle is served from the first pass rather than the second.

## The rule

A readiness check has to be the work, and it has to keep being true.
Anything cheaper is a guess about the server's startup order, and a single
pass is a guess that the answer does not expire. This pair cost three
attempts to learn both halves.

## D82 is what exposed it

The lapse was always there and the compile was hiding it. Every job spent
ninety seconds building the tests between `dbrun` declaring the server ready
and the first statement running, which was ample for the node set to be
refreshed into a usable state and stay there. D82 removed that and the gap
closed to nothing.

This is the second time that ninety seconds was load bearing. Anything else in
`container/` whose check is cheaper than the work that follows it is now
unprotected in the same way, and `Settle` is what to reach for when one of
them starts failing.

This is worth stating plainly, because the obvious reading is that D82 broke
Presto. It did not. It removed an accidental delay that a readiness check was
quietly depending on, and a readiness check that needs a ninety second pause
after it is not one. The same reasoning applies to anything else in
`container/` whose check is cheaper than the work that follows it.

## What was rejected

`/v1/info` reports `"starting":false` and is what Presto's own deployments
probe. It was not used, because it is another thing that correlates with
being ready rather than the thing itself, and the point of this decision is
that the correlation is what failed twice.

## How it was verified

By removing the containers and running `dbrun test presto-0.299`,
`dbrun test trino-483` and `dbrun test trino-476` from cold, all of which
pass, and a cold `dbrun start presto-0.299` taking 20 seconds against 8
before, which is the settle doing its work.

That is not proof and it was not proof the last two times either. This
machine runs Presto's readiness check 34 times in a row without a single
failure, so the lapse does not happen here and both earlier attempts also
passed locally. What carries the weight is the server's own configuration:
the set is refreshed every five seconds, so a check that holds for twelve
has seen two refreshes.

The settle logic is tested rather than trusted. Four tests drive the real
`waitReady` through a shell command, and the one that matters watches a
check that passes, fails once and passes again.
