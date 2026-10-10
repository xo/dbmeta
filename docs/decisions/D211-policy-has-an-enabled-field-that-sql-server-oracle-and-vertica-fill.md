# D211. Policy has an Enabled field that SQL Server, Oracle and Vertica fill

Status: Amends D206.

## The decision

Ken decided on 2026-10-10 to add `Policy.Enabled`, a `sql.Null[bool]`, so that
a policy that is switched off can be told from one that is active.

D206 left a disabled policy out of `Policies` for SQL Server and Oracle,
because `Policy` had no field to say that it is off, and a row claimed that the
policy is active. Vertica listed a disabled policy as an ordinary one. Both
were wrong in a way a consumer cannot see. Now `Policies` returns every
policy, and `Enabled` says whether it restricts anything. This amends D206:
a disabled policy is a row.

`Table.RowSecurity` does not change. It still reads only an enabled policy,
because the table is protected only by a policy that is on.

## What each product fills

I measured each source on a running server. Nothing here comes from the
documentation alone.

| Product | `Enabled` | Source |
| --- | --- | --- |
| SQL Server | true or false, from 2016 | `sys.security_policies.is_enabled`. `STATE = OFF` is false |
| Oracle | true or false | `ALL_POLICIES.ENABLE`, which is YES or NO. `DBMS_RLS.ENABLE_POLICY` with `enable => FALSE` makes NO |
| Vertica | true or false, from 9.1 | `v_catalog.access_policy.is_policy_enabled`, which holds the words Enabled and Disabled |
| PostgreSQL | NULL | no source, see below |
| ClickHouse | NULL | no source, see below |

SQL Server has no state on a predicate. `sys.security_predicates` has the
columns `predicate_type`, `operation`, `predicate_definition` and the target,
and no flag. The state belongs to the policy, so every row that comes from one
policy has the same `Enabled`. A test checks that.

The column in Vertica is `is_policy_enabled`, and not `is_enabled`. It is text.
The statement maps Enabled to true and Disabled to false, and any other word to
NULL, so a new word cannot become a wrong answer.

PostgreSQL has no switch for one policy. A policy exists and is active.
Row security is switched on or off for a table with `ALTER TABLE ... ENABLE ROW
LEVEL SECURITY`, which is `Table.RowSecurity`. A true here repeats that
field and a false is wrong, so `Enabled` is NULL. Every other model that
has no `Policies` kind is unchanged.

ClickHouse has no switch either. `system.row_policies` on 25.8 and 26.9 has the
columns `name`, `short_name`, `database`, `table`, `id`, `storage`,
`select_filter`, `is_restrictive`, `apply_to_all`, `apply_to_list` and
`apply_to_except`. There is no `ALTER ROW POLICY ... DISABLE`. A policy that is
not wanted is dropped. `Enabled` is NULL, and the test says so.

## What each release answers

| Release | Answer |
| --- | --- |
| SQL Server 2017, 2019, 2022 and 2025 | the state, run and tested |
| SQL Server 2016 | the state. A Windows machine is needed, so it did not run |
| SQL Server below 2016 | no `Policies` kind, as before |
| Oracle 21c and 26ai | the state, run and tested |
| Oracle 11g, 18c and 19c | the state. The column `ENABLE` is in `ALL_POLICIES` in all of them. 11g is Nightly, and 18c and 19c are Verified. They did not run |
| Vertica 25.1 | the state, run and tested |
| Vertica 9.1 and 10.1 | the same statement. An ordinary user is refused `access_policy` there, so the state is read as an administrator. They are Nightly and did not run |
| ClickHouse 25.8 and 26.9 | NULL, run and tested |

## The fixtures and the tests

Each fixture makes one policy that is off, next to the one that is on, and the
test reads both back as typed values.

- SQL Server adds `secret_policy_off` with `STATE = OFF` on the same table as
  `secret_policy`. Only one policy can filter a table at a time, and one that
  is off does not count. The test reads `secret_policy` as a real true,
  `secret_policy_off` as a real false, and checks that every predicate of one
  policy agrees.
- Oracle adds `SECRET_POLICY_OFF` for DELETE and switches it off with
  `DBMS_RLS.ENABLE_POLICY`. The test reads the three rows of `SECRET_POLICY`
  as enabled and the one row of the other as a real false.
- Vertica adds a table `vault` with a row access policy and runs
  `ALTER ACCESS POLICY ... DISABLE` on it. The test reads `ledger` as true and
  `vault` as false.
- ClickHouse changes no fixture. The test checks that `Enabled` is NULL.

## The cost

`Enabled` is a column of the row that each statement already reads, so it adds
no join, no subquery and no second statement. SQL Server read `is_enabled` in
the WHERE clause before, and Oracle read `ENABLE` there. Both now return it
and filter on neither. I did not time a catalog with a thousand policies,
because no plan changed: the only change is that a disabled policy is a row,
and a consumer that wants only active policies filters on the field.

## Parity and conformance

The privilege parity and the conformance goldens ran for the four products on
every release above and did not change. A lesser principal reads the same rows
as before, because the sources are the same.
