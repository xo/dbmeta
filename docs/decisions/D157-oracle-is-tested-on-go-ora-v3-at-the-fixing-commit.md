# D157. Oracle is tested on go-ora v3 at the commit that fixes it

Status: Amends D59, D136 and D154.

## The decision

D59 tested Oracle on `go-ora/v2` until `go-ora/v3` tagged the fix for its
panic on 11g and 18c. D59 also considered a pin of v3 at the commit that
fixes it, and did not take it, because a tagged release needs no explanation
later. On 2026-10-01 the fix was still untagged. The newest tag was v3.0.1,
of 2026-07-24, and the fix, commit 360b4b7 of 2026-09-14, was still the head
of master. Ken decided to test on v3 at that commit and to drop v2.

The test module now pins `github.com/sijms/go-ora/v3` at
v3.0.2-0.20260914154503-360b4b7ac9e9 and does not require v2. That is the
package dburl names for the oracle scheme, so Oracle is no longer an
exception to D154.

`test/oraclev3` is removed. It existed because v2 and v3 register the same
driver name and cannot share a binary (D136). Every Oracle test now runs on
v3, and the scan test already runs every query through its typed Scan, with
and without the system objects. Its two tests that read one table by parent
moved into `test/oracle_test.go`.

## What was measured

On 2026-10-01, each release ran the whole test module on v3 at that commit,
through `dbrun test`:

| Release | Tier | Result |
| --- | --- | --- |
| 11g | Nightly | passed, in 81 seconds |
| 18c | Verified | passed, in 29 seconds |
| 19c | Verified | not measured |
| 21c | Tested | passed, in 35 seconds |
| 23ai | Nightly | passed, in 46 seconds |
| 26ai | Tested | passed, in 38 seconds |

On 26ai the Oracle tests also ran by name. `TestOracleColumnsOfOneTable` and
`TestOracleChildrenOfOneTable`, which moved from `test/oraclev3`, passed with
the rest.

19c did not finish its first start. Its database creation stayed at 36% for 20
minutes, at the 4 GB memory limit, while the load on the machine was 30. D59
measured 19c on this same commit, so the driver is not the cause. The backlog
holds the measurement that settles the memory.

A second run of 19c found a fault in dbrun. A container that was running and
not yet ready counted as up, so the tests ran before the database was open and
failed with ORA-12514. dbrun now waits for the ready check on a running
container too.

## When this ends

When v3 tags a release that holds the fix, move the pin to that tag. The
test module is never part of a consumer's build, so the pseudo-version does
not reach anybody else.
