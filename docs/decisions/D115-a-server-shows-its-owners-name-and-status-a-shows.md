# D115. A server shows its owner's name, and status -a shows the stopped ones

Status: Amends D98.

Ken asked on 2026-09-28 for two changes to `dbrun status`, so that he can see
which session owns which server.

## The owner's name

The owner of a server is the session of a coding agent, such as
`claude-code:b3cc5e94`, which tells two sessions apart and which no person can
read. No variable in a session's environment holds its friendly name, such as
`dbimp`, and `dbrun` cannot tell it from its working directory, because every
session runs `dbrun` from the dbmeta checkout. So the session gives it.

`DBMETA_OWNER_NAME` is written to a second label, `dbmeta.owner.name`, when a
server is created. `status`, every refusal and the list of who holds the
servers when there is no room print it with the owner, as
`dbmeta (claude-code:b3cc5e94)`. `status --json` prints it as `ownerName`. It
is only shown. The owner label alone decides who may act on a server, as D98
decided, so two sessions that give the same name are still two owners.

`DBRUN.md` asks every coding agent to set it to the name of its session on
every command. Setting `DBMETA_OWNER` to a readable name instead, as dbimp's
sessions did with `dbimp-w8`, loses the session, so that is not the way.

## status -a

`status` prints the running servers. `-a` or `--all` also prints each stopped
server that the runner still holds, marked `stopped`, with who made it, the
way `podman ps -a` does. `status --json -a` prints them with the state
`stopped`. A stopped container belongs to nobody (D108), and the label still
says who made it.

## What was measured

On 2026-09-28, `rqlite-10.3.6` was started with `DBMETA_OWNER_NAME=dbmeta`.
`status` showed it as the caller's, and a stop by another owner was refused
with "it belongs to dbmeta (claude-code:b3cc5e94)". After a stop, `status`
showed nothing, and `status -a` showed it and the 30 other stopped containers
on the machine, 20 of them with no owner.
