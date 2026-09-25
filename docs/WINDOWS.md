# Windows machines for the old SQL Servers

SQL Server on Linux begins at 2017. Everything older has no container, cannot
run in CI, and is Archived under D54, which means nothing is claimed for it.
`dbrun provision` is how those releases get tested at all. D57 says what may
be claimed once a machine has run.

| SQL Server | Windows host | dockur `VERSION` | host port |
| --- | --- | --- | --- |
| 2008 R2 SP2 Express | Windows Server 2008 R2 | `2008r2` | 51433 |
| 2012 SP4 Express | Windows Server 2012 R2 | `2012r2` | 51434 |
| 2014 Express | Windows Server 2012 R2 | `2012r2` | 51435 |
| 2016 SP2 Express | Windows Server 2016 | `2016` | 51436 |

The list itself is Go data in `container/windows.go`, and `dbrun` reads it
from there. Add a release there and not here.

## Running one

```bash
cd test && go run ./cmd/dbrun provision sqlserver-2012
```

The first run installs Windows and then SQL Server and takes 30 to 60 minutes.
Later runs start the existing machine in about a minute. `--watch` prints the
web console and leaves the machine running without waiting. `--render` writes
the OEM folder and stops, which is how the templating is checked without
waiting an hour to find a typo.

A machine is a target like any other, so `dbrun start`, `stop`, `status`,
`version`, `dsn`, `usql` and `test` all take one. Only the first build is a
separate command, because an hour is not something to start by accident.

Watch an install at `http://127.0.0.1:8106` and up, one port per machine.

Then run the tests against it:

```bash
cd test && go run ./cmd/dbrun test sqlserver-2012 --keep
```

## Licensing

Every Windows image here is a Microsoft **evaluation** edition, fetched from
Microsoft by dockur. Evaluation editions are free for 180 days of testing and
need no product key and no activation, which is why nothing here activates
Windows. When the 180 days runs out, `slmgr /rearm` extends it, and that is
Microsoft's own mechanism rather than a way around one.

The rearm is automatic. `cmd/dbrun/oem/rearm.bat` runs at every startup as a
scheduled task, reads the grace period, and spends a rearm only when fewer than ten days
are left. It does not rearm on every boot, because the count is finite, three
on most of these editions, and a machine that is started often would spend the
whole budget in a week. A rearm applies at the next start, and the script does
not restart the machine, because `dbrun` starts one and waits for SQL Server,
and a reboot underneath that looks exactly like a failed boot.

When the rearms are spent, `C:\OEM\rearm.log` says so. At that point the
machine is rebuilt, which takes about an hour, or the release drops to
Archived under D40 and nothing is claimed for it. See D65.

SQL Server Express is likewise free, and is enough for this: every catalog view
the queries read is present in Express.

## How it works

`dockurr/windows` is QEMU with KVM inside a container. It downloads the
evaluation ISO, writes an unattended answer file, copies the directory mounted
at `/oem` to `C:\OEM`, and runs `C:\OEM\install.bat` at the end of setup as
SYSTEM. That hook is the whole mechanism.

`dbrun provision` writes that directory per release: the SQL Server installer,
a `ConfigurationFile.ini`, and `install.bat` with four values filled in. Then
it starts the machine and waits. The payload is built into the binary with
`//go:embed`, so the command works from any directory and has nothing to find.
The sources are in `test/cmd/dbrun/oem/`.

## Four things that are not obvious

**The installer is downloaded on the Linux host, not in the machine.** Windows
Server 2008 R2 has no TLS 1.2 and cannot reach Microsoft's download servers at
all, so fetching it inside the machine fails on the oldest release and works on
the rest, which is the worst way for something to fail.

**The listening port has to be set in the registry afterwards.** Express
installs with TCP disabled and a dynamic port whatever `TCPENABLED=1` says in
the configuration file. `install.bat` writes `TcpPort` and clears
`TcpDynamicPorts` under the instance key, and the instance key is named for the
release: `MSSQL10_50` for 2008 R2 through `MSSQL13` for 2016. A key for the
wrong release writes the port where nothing reads it, setup reports success,
and the machine is unreachable with no error anywhere.

**Readiness is a query, not a port.** The container runtime publishes the port
when the container is created, so a connection to it succeeds seconds later and
keeps succeeding for the forty minutes Windows takes to install. The first
version of this script tested the port and declared every machine ready almost
immediately. `dbrun` opens a connection and runs the version query, so a
machine that answers but has not finished configuring is not called ready.

**2008 R2 wants its own section header.** Its configuration file wants
`[SQLSERVER2008]` rather than `[OPTIONS]`. The file carries a comment saying
so, and the comment names the header, so a replacement that is not anchored to
a whole line rewrites the comment and leaves the header alone. That is what the
first Go version did.

It takes `/IACCEPTSQLSERVERLICENSETERMS` like every other release here, which
is the opposite of what was first recorded. `TestEveryReleaseTakesTheLicenseFlag`
pins it.

## If it goes wrong

Look at the web console first, then at `C:\OEM\provision.log` inside the
machine, which `install.bat` writes as it goes. It leaves `C:\OEM\ready.txt` on
success and `C:\OEM\failed.txt` on failure, so the two are easy to tell apart.

`/dev/kvm` is required. Without it QEMU emulates and a 30 minute install takes
most of a day.

dockur warns that BTRFS on `/storage` can upset Windows Setup. Set
`DBMETA_VM_STATE` to a directory on another filesystem if an install fails
oddly early.

## Where the disks live

Not in this repository. A Windows disk is tens of gigabytes, and a working
tree that holds one is a working tree where every grep and every editor index
walks it. They go under the XDG data directory instead:

```
${XDG_DATA_HOME:-$HOME/.local/share}/dbmeta/vm/<machine>/
    storage/   the Windows disk
    oem/       the payload copied to C:\OEM
    shared/    what the install writes back, including provision.log
```

`DBMETA_VM_STATE` moves that elsewhere, which is also how to put a machine on
another filesystem when BTRFS upsets Windows Setup.
