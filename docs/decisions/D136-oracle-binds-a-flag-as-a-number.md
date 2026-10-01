# D136. Oracle binds a flag as a number, and is tested on go-ora v3 too

Status: Amends D59, amended by D157.

## What was found

usql moved its Oracle commands onto dbmeta on 2026-09-30, and every one of
them failed on its oracle scheme with "no parameter coder registered for go
type bool". usql's oracle scheme opens `go-ora/v3`. Every Oracle statement
takes `with_system`, and binds it as a Go bool, which is false when the caller
passes nothing. `go-ora/v2` binds a bool as a number, and `go-ora/v3` refuses
one. Oracle has no boolean before 23ai.

The test module reached Oracle only with v2, so nothing here found it. D59
said the statements are the same whichever major version of the driver
carries them. That is true of the statements and false of the parameters.

## The decision

`Info.BindValue` converts a parameter value before it is bound, and is nil
for every model but Oracle's. The Oracle model binds a bool as 1 or 0, and
every Oracle statement compares a flag with 1, so both drivers carry it. A
model with a Literal renders its values instead, and BindValue does not apply
to it.

A test in `models/oracle` builds every query with the flag false and true,
and fails when a value is a bool.

## Oracle is tested on go-ora v3 too

`test/oraclev3` is a package of its own, because both major versions register
the driver name `oracle` and cannot share a binary. It runs every query the
model supports through v3, with and without the system objects, reads every
row, and reads the columns of one table by `parent`. It uses the v3 commit
that D59 measured, v3.0.2-0.20260914154503-360b4b7ac9e9, because v3.0.1
panics on 11g and 18c. Without BindValue it fails on every query, and with it
every query runs. The rest of the Oracle tests keep v2, as D59 says, until v3
tags the fix.
