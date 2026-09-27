# D108. Eight servers run at once, and a stopped container belongs to nobody

Status: Amends D75 and D98.

Ken asked on 2026-09-27 for two changes to `dbrun`.

## Eight servers, not four

D75 allowed four running servers. Ken raised it to eight. Each container is
limited to 4 GB, so eight use 32 GB at most, and the machine has 60 GB and 32
cores. A start that brings the count to nine stops the caller's own oldest
server, as D98 does for five today.

## A stopped container belongs to nobody

D98 made a start of a stopped server refuse when another owner created it.
A container's owner is a label, and a label cannot change on a container that
exists. So a server that stopped when the computer restarted, or when its
session ended, kept an owner that no longer existed. Every command refused it,
and an agent then tested an older release that was free. Ken said that most
testing belongs on the newest release, not on whichever one is free.

Now ownership counts only while a container runs:

- A start of a stopped container that another owner created removes it and
  creates it again, with the caller as the owner. It loses what was in it,
  and the setup and the tests build that again. A container takes about a
  minute.
- `remove` of a stopped container acts, whoever created it. So does the
  rebuild of a stopped container whose port moved.
- A running server keeps its owner. A start of it still shares it, and a
  `stop` or `remove` of it still refuses.
- A stopped machine keeps its owner, because it takes an hour to create
  again. A machine is Verified and a person starts it.

`DBRUN.md` now says to test on the newest release unless the task names
another, and never to move to an older release because the newest is taken.

`TestClaimable` holds the rule. It was measured with two owners named by
`DBMETA_OWNER` on `postgres-18`. A container that the first owner started and
stopped was created again under the second when the second started it. The
first owner was then refused a stop of the running server, and after the
second owner stopped it, the first owner removed it.
