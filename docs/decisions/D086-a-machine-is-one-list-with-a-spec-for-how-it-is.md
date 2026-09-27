# D86. A machine is one list, with a spec for how it is built

Status: Decided.

D85 left one question for when the Exasol machine was built: whether the
machine list stays `container/windows.go` or becomes something that holds
both kinds. This answers it. Ken decided it on 2026-09-27, after Gemini and
DeepSeek reviewed the design and agreed on its shape.

## One list

`container.Machine` is one database release on a virtual machine, and
`container.Machines()` returns every one. It holds what every machine has:
the dialect, product, release, tier, the host port for the database, the
viewer port, the time allowed to provision it and to start it later, and how
to build the connection string.

How the machine is built is held beside that, in one of two fields, and
exactly one is set. `Windows` is a `WindowsSpec`, which is what
`container.WindowsVM` used to hold beyond the common part: the Windows
release, the dockurr image, the installer, the registry key and the licence
flag. `Appliance` is an `ApplianceSpec`: the file the vendor publishes, its
SHA256, the page to download it from, the disks inside it, the memory, the
processors and the port inside the guest.

There is no interface. Nothing dispatches on the kind of machine. `dbrun`
reads fields, and only `provision` looks at which spec is set.

`container/machine.go` holds the type and the list. `container/windows.go`
keeps `WindowsSpec` and the four SQL Server machines. A machine's name is
still `<product>-<release>`, so `sqlserver-2012` did not change.

## What was rejected

Two lists, one per kind. Every reader would join them, and the checks that
matter most, that no name and no port is used twice, span both anyway.

Fields for the user, the password and TLS options. The connection string is
a function, the same as a `Server`'s, so an appliance whose credentials are
the vendor's writes them there. The project password stays the default
everywhere else without any field saying so.

A probe described as data, with a driver name and a query. A driver name
belongs to the `test` module and not to the root one. `dbrun` already has a
map from dialect to driver, and the dialect's own version query is the probe.

A map of environment variables for the virtual machine. Memory, processors
and disk sizes come from the vendor's descriptor and are typed fields.
`dbrun` builds the variables from them.

## One verb

`provision` builds a machine of either kind. For an appliance it takes the
file with `--from`, or finds it in the machine's state directory, and fails
with the download page and the expected SHA256 when neither has it. A second
verb would be a second place for start, status and remove to disagree.

The import reads the file once. It checks the SHA256 of the whole file and
unpacks the disks in the same pass, then converts each disk to qcow2 with the
`qemu-img` inside `qemux/qemu`, so the host needs only the container runner.
It turns copy on write off for the disk directories first, and says so when
the file system has no such attribute.

## One way to ask a machine whether it answers

A container is asked with its own readiness command, run inside it. A
machine is asked by connecting with the driver the tests use for its dialect
and running the version query. That was written for SQL Server alone and now
takes the dialect.

This also fixed a fault. `dbrun start` on a machine ran the machine's
readiness command, which is empty, so the runner ran with no arguments,
which always fails. Starting a machine that was answering waited out the
whole fifteen minutes and then said it never answered.

## What this does not do

It adds no Exasol machine. The entry needs `dbmeta.Exasol`, and D85 still
holds: there is no dialect constant until there is a model.
