# D58. One .gitignore, in the repository root

Status: Decided.

There is one `.gitignore` and it is the one in the root. Do not add a second in
a subdirectory, however local the thing being ignored feels.

## Why

The question "what does this repository ignore" has to have one answer in one
place. A reader who has to find every `.gitignore` before answering it will
miss one, and the one they miss is the one that matters.

That is not hypothetical here. Two were written for the test harness.
`test/vm/.gitignore` was committed and `test/oracle/.gitignore` was not, so the
Oracle build directory was unprotected, and 2.9 GB of Oracle's checkout stayed
out of a commit only because that work happened to be uncommitted when the gap
was noticed. Nobody had done anything wrong. The arrangement had no
place where the omission was visible.

A single file also makes the rule auditable by the tool. `git check-ignore -v`
names the file and the line, and when there is one file that output is an
answer rather than a starting point.

## What this does not mean

It does not mean the repository ignores much. It ignores `state/`, which is
where the test harness puts virtual machine disks and fetched checkouts
if somebody points `DBMETA_VM_STATE` or `DBMETA_ORACLE_STATE` back into the
working tree. Those live under `$XDG_DATA_HOME/dbmeta` instead, because a
Windows disk is tens of gigabytes and a working tree holding one is a tree
where every grep and every editor index walks it.

So the entry is insurance rather than routine. That is the point: an ignore
rule earns its place by covering the case nobody meant to create.

## For an agent working here

Ignore a new artifact by adding a line to the root file, with a comment saying
what produces it. Do not create a `.gitignore` next to it. If a pattern needs
to be scoped to one directory, write the path in the root file rather than
moving the rule closer to the thing.
