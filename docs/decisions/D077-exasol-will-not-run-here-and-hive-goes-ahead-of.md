# D77. Exasol will not run here, and Hive goes ahead of it

Status: Amends D66, amended by D84.

Exasol is number 8 in D66's order and Hive is number 9. Hive goes first,
because Exasol does not start and four separate things had to be got past
before that became clear.

There is no `models/exasol`, no dialect constant and no container entry. A
constant with no model and a container entry that cannot start are both worse
than nothing: they claim something this project cannot do. What was learned is
here instead, so that whoever tries again starts from the fourth problem
rather than the first.

## What was got past

Each of these is a real fix and each one revealed the next. Measured on
`exasol/docker-db:2026.1.2`, rootless podman, on 2026-09-26.

The container needs a bridge network. Exasol picks its own address by looking
for the first interface whose state is UP, and rootless podman's default
networking gives an interface whose state is UNKNOWN, so initialization fails
before anything else happens:

	exadt:: searching for the first interface with state UP
	IndexError: list index out of range

The container needs to be privileged, which Ken granted on 2026-09-26. The
image's own README says so: privileged mode is required for permissions
management, UDF support and environment configuration. `--cap-add SYS_ADMIN`
alone gets past `sethostname`, which is the first thing to fail, and then
`bucketfsd` restarts forever with "Master authentication service rejected
authentication: Unauthenticated". With `--privileged` the initialization runs
to "All stages finished", which it never does otherwise.

The container needs a longer readiness budget than anything but SAP HANA. It
builds a single node cluster on first start.

## What it did not get past

The database process starts, runs for about three minutes and aborts:

	*** Exception caught in init of ObjectMgmt:
	    ObjectClient: Invalid hash value ***

Then the controller shuts down cleanly and the container stays up with nothing
listening on 8563, so the symptom a caller sees is a readiness timeout rather
than an error. That is the worst shape a failure can have and it is why this
took as long as it did.

Two hypotheses were tested and both were wrong. Memory is not it: the error is
identical at 4g and at 8g, and the container was using 357MB when it failed.
The password is not it either: `init-sc` has an `--encode-passwd` flag and its
sibling `--root-passwd` documents that a password is expected already encoded,
so passing the password as cleartext looked like exactly what "Invalid hash
value" would say. Encoding it changes nothing.

The evidence points at storage and that is where the next person should start.
Exasol's device is a 6GB file at `/exa/data/storage/dev.1` and it sits on
overlayfs. The image's README says the host must support O_DIRECT, which
overlayfs does not, and `--no-odirect` is already passed. "Invalid hash value"
reads as an object checksum failure against that device rather than anything
to do with a password.

So the next thing to try is a real volume for `/exa`, which the README
documents under managing disks and devices. That needs a volume field on
`container.Server` and a host directory for `dbrun` to create and remove,
which is machinery no other product here needs.

## Why that was not tried

Judgement rather than difficulty. Exasol had by then cost more than the whole
Firebird model did, including its queries, fixture, tests, conformance, parity
and documentation. Four gates were already behind it and the fifth needed a
new field in a shared package.

Hive needs none of it. It is Apache 2.0, multiarch, was rebuilt the day before
this was written, and asks for no licence, no privileged container and no
special networking. Taking the cheap one first is the same reasoning D66
already uses to put the products that run in a container ahead of the ones
that need an account.

Exasol is not struck the way Impala was in D67. Impala cannot be a model
because it has no queryable catalog. Exasol has `EXA_` and there is every
reason to think the queries would be good. It is blocked on starting the
server, which is a different thing and may take one volume mount to fix.

That last paragraph aged well and the rest of this decision did not. D84 has
the measurement: Exasol publishes a second image, it needs none of the four
gates above, and the blocking reason is gone.
