# D98. A server has an owner, and dbrun acts only on the caller's own

Status: Amends D75, amended by D102, D108 and D115.

Ken decided on 2026-09-27, answering the question D97 raised. Several coding
agents use this machine at once, and D75 let one agent's fifth start stop a
Couchbase server that another agent was using.

## What dbrun does now

Every container and machine that `dbrun` creates carries the label
`dbmeta.owner`. The owner is `DBMETA_OWNER` when it is set, then the session
of a coding agent from `CLAUDE_CODE_SESSION_ID`, and then the login name. A
session's ID stays the same across every command it runs and differs between
two sessions, so it tells two agents apart with no setup.

- A start that brings the count to five stops the caller's own oldest server.
  When none of the four is the caller's, it refuses and names each server and
  its owner. This amends D75, which stopped the oldest server of any session.
- `stop`, `remove`, a start of a stopped server, and the rebuild of a server
  whose port moved all refuse a server of another owner.
- A start of a running server shares it, whoever owns it, and says whose it
  is. Two sessions that test one release use one server. `test` never removes
  a server that it shared.
- `--force` overrides every refusal, and on a fifth start it stops the oldest
  server of any owner, which is what D75 did.
- `status` shows each owner, and `status --json` prints the owner, whether it
  is the caller, and when the server started. `version --json` prints JSON.
  The help said both did before either one did.

`TestPickEvicteeStopsOnlyYourOwn` holds the rule for room, and it was run on
the machine with three owners named by `DBMETA_OWNER`. The fifth start of one
owner stopped that owner's oldest server, a start by an owner of none was
refused, and a stop of another owner's server was refused.

## A server from before owners

A container that existed before this change has no label, and a label cannot
be added to a container that exists. Rebuilding it loses what is in it.
So it keeps no owner until it is removed and started again. A command that
names it acts on it as before, and a start that needs room never stops it,
because every such server is somebody's and stopping one silently is the
fault that owners end. A start of such a stopped container resumes it without
an owner, which the measurement showed with `postgres-15`.

## What was not chosen

The `usql` and `dbimp` sessions asked for more, and Ken chose these for now:

- No pin. A server is stopped for room only by its owner, which covers the
  long measurement and the design session that a pin was for.
- A failure still exits 1, a plain `dsn` still prints two columns, and `dsn`
  still prints only the administrator, in one scheme.

`DBRUN.md` holds the rules and says what is not done.
