# D84. Exasol runs after all, on the nano image

Status: Amends D77.

D77 said Exasol will not run here. That is no longer true, and the reason is
not that anything here got better at starting it. Exasol publishes a second
image.

`docker.io/exasol/nano` has 29 tags, amd64 and arm64, one release line so
far, `2026.2.0-nano.1` through `nano.5`. `nano.5` was pushed on 2026-09-24,
three days before this was written. That is criterion 2 of
`docs/EVALUATION.md` answered, which `exasol/docker-db` could never answer
whatever the tag list said, because the image did not initialize.

## What it does not need

Every gate D77 recorded is gone. Measured on Ken's machine, on a container
the Exasol installer started, read with `podman inspect`:

	privileged=false  network=pasta  memory=0

`pasta` is rootless podman's default networking, so the bridge network D77
had to create is not needed. The container is unprivileged, so the
`--privileged` grant D77 asked for and got is not needed. The installer
reported the database up in about five seconds against the startup budget
D77 spent, and the storage question that D77 identified and did not attempt
never arises.

The fourth gate was the one D77 said might take a single volume mount. It
takes none.

## What this decision does not do

It does not add Exasol. There is still no `models/exasol`, no dialect
constant and no container entry, and this decision creates none of them, for
the reason D77 gave: a constant with no model claims something this project
cannot do. What has changed is that the work is now possible, so Exasol goes
back in D66's order rather than sitting behind a blocked note.

It also does not adopt the running container. The installer names it
`exasol-nano`, and every container here is `<product>-<release>`, so `dbrun`
cannot see it, `dbrun version` would report nothing for a server that is
plainly running, and a second copy could end up on the machine with nothing
to tell them apart. That is D68 exactly. If Exasol is added, it is added to
`container/container.go` and started by `dbrun` like everything else.

## The installer, since it is the way in

`curl https://www.exasol.com/install/starter-kit.sh | sh` is what publishes
the image to a machine, and the script is worth reading before it runs. It
is 189 lines, refuses to run as root, wraps everything in `main` so a
truncated download cannot execute half a script, pins `curl --proto
'=https'`, and unpacks to `~/.exasol-starter-kit/kit` so every script it
hands off to can be read. `EXAKIT_DRY_RUN=1` prints the plan and installs
nothing, and `EXAKIT_PREFLIGHT=1` checks the machine.

Three things it does that are worth knowing rather than discovering:

It installs from the `main` branch by default, with no checksum on the kit
archive. `EXAKIT_REF` pins a tag. What runs today is not what runs tomorrow.

It edits the user's shell profile unless `EXAKIT_NO_PROFILE_EDIT=1` is set.

It writes a Claude Code skill into `~/.claude/skills`, and it did so on a run
where the AI client question was answered "Skip for now". A skill is
instructions that every later agent session on that machine can load, so an
installer that writes one has a channel into work that has nothing to do with
Exasol. The one it wrote reads as an ordinary onboarding guide and tells an
agent to show the SQL before running it, so this is about the mechanism
rather than about that file.

None of that blocks using the image. `podman pull docker.io/exasol/nano` and
a container entry need none of the installer, which is how this project would
reach it.
