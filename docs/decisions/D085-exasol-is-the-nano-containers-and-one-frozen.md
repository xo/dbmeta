# D85. Exasol is the nano containers and one frozen virtual machine

Status: Decided.

Exasol is tested two ways, because the product ships two ways.

`docker.io/exasol/nano` is an ordinary container and goes in
`container/container.go` with every other release. D84 measured it:
unprivileged, rootless podman's default network, up in about five seconds.

The Community Edition is a virtual machine appliance, and one release of it
is imported and then frozen. It is reached the way the old SQL Servers are,
which is D57 and `docs/WINDOWS.md`, so a machine stays a target like any
other and `dbrun start`, `stop`, `status`, `version`, `dsn`, `usql` and
`test` all keep working on it.

Ken decided this.

## Why both, when the containers are so much cheaper

Because one release cannot test a version gate.

The nano line is `2026.2.0` and nothing else. A product with one release is
Presto, where D73 records that the floor is also the ceiling and there is no
second release to compare against. A model written against one release can
carry no gate that anything ever exercises, and rule 3's whole apparatus is
unused.

The Community Edition is Exasol 8, build 2025.2.1, which is a release line
behind. That is the point rather than a drawback: it gives the model an old
server to answer against, so a fragment that claims a column arrived in
2026.2 has something that predates it. Without the machine, Exasol is a
single release product and its coverage says so.

## Frozen, and what that means

The Community Edition release does not move. It is imported once, recorded
with its build, and left there. Exasol shipping 2025.2.2 is not a reason to
rebuild it, and only a new decision changes that.

It is Verified under D40, the same tier as the four SQL Server machines and
for the same reasons: it needs KVM, it is about ten gigabytes, and CI cannot
run one. Nothing is claimed for it beyond what a person ran before a release.

## Where the SQL Server path does not carry over

Three differences, and the first one decides the design.

The image cannot be fetched unattended. `container/windows.go` gives each
release an `Installer` URL on `download.microsoft.com`, and `dbrun provision`
downloads it. The Community Edition is behind a signup form at
`exasol.com/free-signup-community-edition`, so there is no URL to put in a Go
file and no way for a machine to fetch it. The Exasol entry names a file and
its checksum rather than a URL, and `dbrun provision` imports the file the
person already downloaded, from the state directory, and fails with a message
naming the page when it is not there.

There is nothing to install. The Windows path boots an evaluation Windows,
runs an unattended SQL Server setup from an OEM folder, and sets a registry
key to pin the port. The Community Edition is a prebuilt appliance: the
database is already in it. So there is no OEM folder, no unattended answer
file, no installer bootstrapper and no `LicenseFlag`. That is most of
`WindowsVM` gone.

No Windows license and no rearm. D65's `slmgr /rearm` machinery exists
because a Windows evaluation edition expires after 180 days. The appliance is
Ubuntu with Exasol on it, so none of that applies and none of it carries over.

## How it runs

`dockurr/windows` is Windows only. Its sibling `qemux/qemu` boots an
arbitrary disk image in a container under KVM, is from the same family, and
was rebuilt three weeks before this was written. That keeps the shape the
SQL Server machines already have, where a virtual machine is a container
`dbrun` starts, so nothing above it has to learn a second mechanism.

An OVA is a tar of an OVF descriptor and one or more VMDK disks, which qemu
does not boot as a unit. So importing means unpacking the tar, taking the
disks, and converting each with `qemu-img convert -O qcow2`. That conversion
is the import, it happens once, and the qcow2 files live in the state
directory, which `stateDir` puts under `~/.local/share/dbmeta` and never in
the working tree.

Exasol publishes two OVAs, one tuned for VMware and one for VirtualBox. The
VirtualBox one was tried first and it worked, so the VMware one has not been
tried.

## What the first boot measured

On 2026-09-27 the VirtualBox OVA,
`Exasol_Community_Edition_v8_202521_virtualbox.ova`, ran under
`qemux/qemu` on Ken's machine and answered `usql`. It was started by hand,
with Ken's agreement, because `dbrun` has no entry for it yet. This is the
one exception to rule 15 and it ends when the entry exists.

The OVA holds two disks, not one. The first is the system disk, 100 GiB
virtual and 10 GiB used. The second is a data disk, 500 GiB virtual and 41 MiB
used. The manifest's SHA256 sums all passed. Converted, they are 21 GB and
353 MB of qcow2.

`qemux/qemu` takes them with no extra configuration if they are placed where
it looks. Its first disk is `/storage/data.qcow2` and its second is
`/storage2/data2.qcow2`, each a directory mounted as a volume. `DISK_SIZE`
and `DISK2_SIZE` are set to the virtual sizes, 100G and 500G, so that it
does not try to resize either.

The machine uses what the OVF declares: 4 CPUs, 8 GB of memory and UEFI
firmware, which is `BOOT_MODE=uefi` and is also the default. The OVF names an
NVMe controller. The default virtual SCSI disk boots it anyway, so the
controller does not have to match.

It boots straight to the appliance's desktop with no login. The appliance
reports its address as 10.0.2.15, which is VirtualBox's NAT address, while
qemu gives the guest 172.30.1.2. The desktop has an "Update IPs" button for
this. It was not needed: the database answered through the published port
without it.

The ports published on the host were 58563 for SQL, 58443 for the admin
interface, 52222 for SSH and 8110 for the viewer. They are provisional and
were chosen only because nothing used them. The entry in `container` decides
the real ones.

This connected and read the version as 2025.2.1, as `SYS`:

	usql 'exasol://sys:exasol@localhost:58563?validateservercertificate=0'

The appliance's certificate is issued for `exacluster.local` and
`*.exacluster.local`, so a connection to `localhost` fails verification.
`validateservercertificate=0` turns verification off. The alternative is
`certificatefingerprint=`, which pins the certificate, but the fingerprint
has to be read from the machine first. That choice is made with the machine
entry.

`qemux/qemu` warns that copy on write is on for both disk images, which it
recommends against on btrfs. `chattr +C` on the directories before the
conversion avoids that, and the import step must do it.

## What the Community Edition answers with

Recorded here so the fixture and the DSN are not guesswork later. From
`exasol-labs/exasol-labs-community-edition`: the database is `sys` with
password `exasol` on port 8563, the appliance's Ubuntu login is `exasol` and
`exasol`, and an admin interface is on 8443. That password is the vendor's
and is not [Password], which every other server here uses. A frozen appliance
cannot be handed a password at start time the way a container is, so this is
the one server whose credentials are not this project's to choose.

## What was checked rather than assumed

The Community Edition is not abandoned, and the plan does not depend on it
being abandoned. `exasol-labs/exasol-labs-community-edition` was last pushed
on 2026-04-07, with a commit named `2025.2.1_Update`. So it is maintained on
a slower cadence and a release line behind the nano images, which is exactly
the shape this decision wants: a current product for CI and an older one to
test a gate against.

## Nothing is built yet

There is no `models/exasol`, no dialect constant, no container entry and no
virtual machine entry. This decision records the approach and creates none of
them, for the reason D77 gave and D84 repeated: a constant with no model
claims something this project cannot do.

When it is built, `docs/DIALECT.md` is still every step in order. The two
extra steps are a `container/exasol.go` for the nano releases and a machine
entry for the frozen one, and the question of whether the machine list stays
`container/windows.go` or becomes something that holds both is answered then
rather than now.
