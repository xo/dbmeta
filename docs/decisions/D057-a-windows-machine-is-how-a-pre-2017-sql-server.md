# D57. A Windows machine is how a pre 2017 SQL Server gets tested, and it is Verified

Status: Decided.

SQL Server on Linux begins at 2017. D54 put everything older in Archived,
which claims nothing, because no container exists and CI cannot run one. This
is how that changes: a Windows virtual machine, provisioned without a person
watching, hosting one old SQL Server.

| SQL Server | Windows host | dockur `VERSION` |
| --- | --- | --- |
| 2008 R2 SP2 Express | Windows Server 2008 R2 | `2008r2` |
| 2012 SP4 Express | Windows Server 2012 R2 | `2012r2` |
| 2014 Express | Windows Server 2012 R2 | `2012r2` |
| 2016 SP2 Express | Windows Server 2016 | `2016` |

2008 R2 is the floor, and it is a media floor rather than a judgement.
Microsoft still publishes the Express installer for 2008 R2, 2012 and 2014, and
the 2012 release page is gone while every 2012 service pack page is still
there. The plain 2008 page is gone entirely.

## The tier

Verified, never Tested. A machine needs KVM and the better part of an hour, so
CI cannot run one, and calling it Tested would put an untestable release in the
table beside a release CI runs on every push. D40 already has the right word
and this uses it. `container/windows_test.go` fails if a machine is given any
other tier.

That does add the thing D54 said was missing. Tested, Nightly and Verified all
mean "runs again", and a frozen release is one nobody ships anything for, so
one verified run stays true. That is a property of the release rather than a
fourth tier, and it did not need a new word after all.

## One machine per release

SQL Server installs side by side, so four releases could share two machines.
Both reviews said not to, for the same two reasons, and both are right.

A second release on a host has to be a named instance. A named instance takes a
dynamic port and needs the SQL Server Browser, where a default instance is
1433 and needs neither. And the releases disagree about prerequisites, because
2008 R2 and 2012 want .NET Framework 3.5 where 2016 wants 4.6, so a shared host
is a host where at least one release is installed unusually. The whole point is
to see what a normal installation of that release reports.

They run one at a time rather than together.

## Where it lives

The same shape as the Linux side, because it is the same problem. The list is
Go data in `container/windows.go` and `dbrun provision` reads it from there.
One copy, and a test that fails when the payload and the list disagree.

## Licensing, and what this deliberately does not do

Every Windows image is a Microsoft evaluation edition, fetched from Microsoft
by dockur: the Server 2016 ISO is `Windows_Server_2016_Datacenter_EVAL`, and the
2008 R2 one is `GRMSXEVAL`. An evaluation edition is free for 180 days of
testing and needs no product key and no activation, and `slmgr /rearm` extends
it, which is Microsoft's own mechanism.

So nothing here activates Windows. There is an existing script outside this
repository that does, by installing a generic volume licence key and pointing
`slmgr /skms` at a public KMS emulator. That is circumventing licensing rather
than complying with it, and it is also unnecessary, because the evaluation
editions already permit exactly this use. It was not carried over and it should
not be.

SQL Server Express is free on the same footing, and it is enough: every catalog
view these queries read exists in Express, and the fixture builds nothing that
Express cannot.

## Three things that were not obvious, and one that was wrong

The installer is downloaded on the Linux host. Windows Server 2008 R2 has no
TLS 1.2 and cannot reach Microsoft's download servers, so fetching it inside
the machine works on three releases and fails on the oldest.

The listening port has to be written to the registry after setup. Express
installs with TCP disabled on a dynamic port whatever `TCPENABLED=1` says, and
the instance key is named for the release, `MSSQL10_50` through `MSSQL13`. A
key for the wrong release puts the port where nothing reads it, setup reports
success, and the machine is unreachable with nothing in any log.
`TestTheRegistryKeyMatchesTheRelease` holds that mapping.

2008 R2 differs twice: `[SQLSERVER2008]` rather than `[OPTIONS]` as the section
header, and setup refuses `/IACCEPTSQLSERVERLICENSETERMS`, which arrived in
2012.

The one that was wrong is worth recording. Readiness was first tested by
opening the published port, and the container runtime publishes that port when
the container is created, so it answered twenty seconds in and every machine
was declared ready before Windows had begun installing. Readiness is a query
now. A check that cannot fail is worse than no check, because it is believed.

## What may be claimed after a machine runs

That the queries ran against that release, on that build, on that date. Not
that they run today, because nothing runs again. Record the build in
`docs/COVERAGE.md` beside the claim, the same way the tested releases record
theirs.
