# D54. SQL Server covers every release that ships a Linux container

Status: Amended by D63.

Four releases, 2017, 2019, 2022 and 2025, and every one of them at the Tested
tier. CI runs all four on every push. 2016 and older are Archived.

## The floor is a container fact, not a query fact

Microsoft shipped SQL Server on Linux from 2017. `mcr.microsoft.com/mssql/server`
carries 284 tags and not one of them names 2016, 2014 or 2012. So the floor is
not a judgment about which releases deserve support. It is the oldest release
anybody can run in CI, and there is nothing below it to argue about.

Two facts about the images. Microsoft publishes no bare release tag, so the tag
is `2017-latest` and never `2017`, which is why `product` carries a
`tagSuffix`. The 2017 image is built on an older base and installs sqlcmd at
`/opt/mssql-tools` where the other three use `/opt/mssql-tools18`, which is why
`container.SQLServer` overrides the readiness command for that one release.

## Why all four are Tested rather than two Tested and two Nightly

Gemini proposed 2019 and 2022 on every push with 2017 and 2025 nightly, on
installed base. That reasoning fits a product with ten releases. This one has
four.

Every version gate the model has sits below 2017, so these four releases differ
by what they added and not by what they lack. There is no old branch for a
nightly job to protect. Four service containers cost four parallel jobs, and
the claim they buy is the whole one: dbmeta is tested on every SQL Server that
runs on Linux.

## The gates below the floor, which is the part that needed deciding

The model carries two gates and both sit below 2017. `sys.sequences` and
`sys.dm_db_stats_properties` arrived in 2012, and `sys.external_tables`,
`sys.tables.is_external` and `sys.tables.temporal_type` arrived in 2016. CI
reaches the new branch of each and can never reach the old one.

The two reviews split on this, and the split is the useful part.

Gemini said keep them. The `sys` views are additive and backward compatible, so
a gate written from Microsoft's documentation will not surprise anybody, and
2008 R2 through 2016 go in an Archived tier with wording that says CI never
touched them.

DeepSeek said delete them and raise the floor, because the old branch of a gate
no test reaches is dead code. If they are kept, it said, fake the version in a
test and say plainly that the old path is not integration tested.

DeepSeek's objection is the right one and its remedy is the one this project
already has. A statement resolves against a version set, and a version set is a
value, so resolution below the floor is testable with no server at all. That is
`models/sqlserver/version_test.go`, and it holds three things: each gated query
refuses with `ErrVersionTooOld` below the release that added its catalog view
and reads that view at or above it, the one gate that pads rather than refuses
never names `temporal_type` or `is_external` on a release that has not got
them, and the set of statements that build on 2008 R2 is the set that builds on
2025 less exactly the three a gate names.

So the gates stay, and they are no longer a claim nobody checks.

## What can honestly be said about 2014 and 2016

This much: the statement resolves, it names only catalog views that release
documents, and a reviewer read it. Not that it ran, because it cannot.

Write it that way. Do not write supported, do not write compatible, and do not
put an old release in a table beside one that CI runs without saying which is
which. D40 gives three tiers and none of them fits a release with no container,
so such a release is Archived and Archived means nothing is claimed.

## What this does not decide

Whether `Query.Support` must answer no for a server too old to build the
statement. Today it answers yes and `Build` then returns `ErrVersionTooOld`, which
`TestWrongProductIsNotSupported` fixes deliberately: Support answers a question
about the product, and the release is the error's business. Writing the test
above raised the question of whether a caller is well served by that, because a
caller that trusts Support walks into a query it cannot build. It is left as it
is, and it is for Ken.

D63 answers it: Support gained a fourth value and now says so itself.

## Oracle, recorded and not decided

The same question is coming for Oracle and the container facts are these.
`gvenzl/oracle-xe` has 18.4 and 21.3, `gvenzl/oracle-free` has 23, and there is
nothing for 11g or 12c. So Oracle gets the same hard floor for the same reason.

Both reviews agree that Express Edition answers the core catalog and that it is
not a stand-in for Enterprise Edition everywhere, and they name the same gaps.
`DBA_HIST_*` needs the Diagnostics Pack and is absent. The partitioning views,
`ALL_PART_TABLES` and `ALL_TAB_PARTITIONS` among them, exist and stay empty
because XE cannot partition. `ALL_POLICIES`, the Database Vault and Label
Security views, and the encryption columns are absent or empty. `ALL_TABLES`
has in-memory columns that report nothing.

Two things matter more than the feature list. `ALL_*` shows the caller only what
the caller can see, which is the `information_schema` problem this project
already knows, and `DBA_*` needs `SELECT_CATALOG_ROLE` that an ordinary user
does not have. So the Oracle model must choose between the two deliberately and
the test user must be a named one with fixed grants. See the requirements on the
container harness above.
