# D68. Every container is started by the runner and named product-release

Status: Amended by D70 and D124.

Nobody reaches for podman or docker by hand. One command starts every
container this project uses, and every container is named
`<product>-<release>`, which is what `container.Server.Name` returns:
`postgres-18`, `clickhouse-26.9`, `oracle-26ai`.

## Why it needed saying

Because the machine filled up with containers nobody could place. A session
debugging one thing left `ch268`, `pg12`, `pg96` and `chplain` behind, on ports
chosen by whoever typed the command, while the runner used its own names and
its own ports for the same releases. Two sets of the same servers, and the only
way to tell which was which was to read the image tag.

It is worse than untidy. `version` could not reach two servers that were
plainly running, because the port it computes is not the port somebody typed.
A container named for the release but started by hand is the confusing case,
not the obviously wrong one.

## What the runner had to grow to make the rule keepable

A rule that cannot be followed is a rule that gets broken, and the reason
people went around it is that it only knew how to start a server, test it and
throw it away. It now does the things a person actually wants:

| | |
| --- | --- |
| `start` | start it and leave it running, then print its DSN |
| `stop` | stop it, keeping it so `start` resumes it |
| `remove` | stop and delete it |
| `status` | what is running, with a URL for each |
| `version` | connect and print what dbmeta reads, per server |
| `dsn` | the dburl style URL, running or not |
| `usql` | connect to it with usql |
| `all` | every server, spelled the way a person says it |

`help` lists them. It runs from anywhere, which the shell version did not:
it had to find its own directory to find anything, and `./test/run.sh --help`
failed with a path error. D70 is why that is no longer possible.

## Two faults the rule exposed

The port a server gets was its index in the filtered list rather than in the
whole one, so every server started on its own got the first port. Two of them
collided, and the loser sat in Created state holding a name that
`podman rm --force` does not free. A port is now a server's place in
`container.All`, so a release always gets the same one and a URL a person
learned keeps working.

It also hid the runner's error behind "could not start", which is what made
that take an afternoon. It prints what the runner said, and for a name held in
storage it prints the one command that clears it.
