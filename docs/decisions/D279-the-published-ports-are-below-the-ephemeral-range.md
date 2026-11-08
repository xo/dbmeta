# D279. The published ports are below the ephemeral range

Status: Amends D68.

D68 published each container on 55000 plus its place in `container.All`. Two
failures on 2026-11-08 showed that this is inside the ephemeral range of Linux,
which is 32768 to 60999. The kernel gives those ports to the outgoing connections
of the host, such as the pull of an image, so a container failed to start with
`failed to bind host port 0.0.0.0:55022/tcp: address already in use` on a fresh
runner of GitHub, and `pasta failed ... Listen failed for HOST TCP port */55018`
on a developer machine. usql reported both.

## Decision

The published ports start at 20000, and the second port of a container is 1000
above its first, as D124 says. The list is under 300 long, so every port is below
32768, and the kernel never hands one out to a connection.

A caller reads a port from `dbrun dsn --json`, as `docs/DBRUN.md` says, so the
move changes no code outside `dbrun` and its tests. A script that wrote a port
down must read it again. `dbrun` rebuilds a container whose port no longer
matches, as it does when a release is added earlier in the list.

## Rejected

Binding each port before the start and choosing another when the bind fails. It
changes the URL of a server from one start to the next, and the fixed place of a
release in the list is what lets two sessions agree on a name and a port.

Reserving the range with `net.ipv4.ip_local_reserved_ports`. It works on a
runner and nowhere else, and it needs root.
