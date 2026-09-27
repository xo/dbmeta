# D78. Hive reads sys, and is a model

Status: Decided.

`models/hive` answers 16 of the 55 against Apache Hive 4.2.1.
`docs/COVERAGE.md` holds the measurements.

An earlier version of this decision said Hive could not be a model. That was
right about the evidence at the time and wrong about the conclusion, and
both halves of what changed it came from outside this project.

## Hive is not Impala, which is what D66 left open

D66 put Hive last with the note that it is the shape Impala already teaches,
and D67 struck Impala because it answers only through `SHOW` and `DESCRIBE`,
which are statements rather than relations.

Hive passes that test. Hive 3.0 added a `sys` database that exposes the
metastore as 57 external tables over the JDBC storage handler, and they
answer ordinary SQL.

`sys` is not there when a server starts. The script that creates it ships in
the image and needs a running HiveServer2, because the tables are external
tables pointed at the metastore. That is neither an image layer nor a
fixture, so `container.Server.Init` was added: a command run once the server
answers and before anything reads it. It has to be safe to run twice,
because `dbrun start` runs it every time, and Hive's script is, because
every statement in it is CREATE IF NOT EXISTS or CREATE OR REPLACE.

## The protocol cannot bind, and the driver had to change

`TExecuteStatementReq`, the Thrift request that carries a statement to
HiveServer2, has five fields and none of them is parameters:

	SessionHandle  Statement  ConfOverlay  RunAsync  QueryTimeout

So HiveServer2 cannot bind server side at all, and Hive's own JDBC
`PreparedStatement` substitutes on the client. No Go driver can offer real
binding. Saying it that way matters, because "the driver is broken" invites
somebody to go looking for a better driver and this does not. The `usql`
session found this by reading the protocol after reading the driver, which
is the check that pays: read what the driver is a client of.

`sqlflow.org/gohive`, which `usql` shipped, had two defects on top of that
and either one alone rules it out.

It accepted bind parameters and discarded them. `args` is a parameter of its
`execute` and appears nowhere in the body, and `NumInput` panics with "not
implemented", so a statement reached Hive with its question marks still in
it and came back as a Thrift frame size error rather than a refusal.

Worse, it could not represent NULL. Measured:

	SELECT CAST(NULL AS string)   valid=true  ""
	SELECT ''                     valid=true  ""
	SELECT CAST(NULL AS bigint)   valid=true  0

A NULL and an empty string were the same value, and a NULL and a zero were
the same value. Every nullable field in a model built on it would have been
a lie, silently, and nothing about the model would have looked wrong.
`docs/NULLS.md` is the shortest document here and the one that cost the most
to learn, and that driver breaks all of it.

`usql` is replacing it with `github.com/beltran/gohive/v2`, which Ken
confirmed. v2 refuses parameters with a message instead of mangling them,
and it tells NULL from empty. `TestHiveTellsNullFromEmpty` asserts the
second, because a driver change that regressed it would leave no other
trace.

One trap the `usql` session recorded and this keeps: `beltran/gohive` v1 has
no `database/sql` driver at all, only a Connect and Cursor client, and it
was already in the module graph as an indirect dependency. It looks like a
candidate and is not. The driver arrived in v2.

Two things about v2 are worth knowing before anybody debugs it. It panics
rather than returning an error when the auth mode is missing or unknown, so
`auth=NONE` is not optional. And it requires the DSN to keep its `hive://`
scheme.

## What dburl had to change, and what it found

`dburl` generated the Hive DSN with `GenFromURL("truncate://localhost:10000/")`,
which strips the scheme and emits `localhost:10000/default`, and
`beltran/gohive/v2` rejects anything that does not begin with `hive://`. The
requirement was recorded here first, while the `dburl` session was not
running, and that session has since shipped it as its D16: the scheme is kept,
the `hive2` alias normalizes to `hive`, and `auth=NONE` is defaulted.

The `auth` default is not tidiness and it came out of a form table run
against the server here. The driver panics rather than returning an error
when `auth` is missing:

	hive://hive:pw@host:10000/default                 panic: Unrecognized auth
	hive://hive:pw@host:10000/default?auth=NONE       connects
	hive://hive:pw@host:10000/default?auth=CUSTOM     connects

`dburl` v0.28.0 emitted exactly the first shape, so every caller was one
`Open` from a crashed process. That is worth keeping here because it is the
clearest case yet for the rule the `usql` session wrote into its driver gate:
the check that pays is not reading the driver, it is reading what the driver
is a client of, and then asking what it does with nothing rather than with
something wrong.

`transport` behaves differently from `auth` and the difference matters.
Measured on 4.2.1:

	no transport option                connects
	transport=binary                   connects
	transport=http                     fails cleanly, the server is binary
	transport=nonsense                 panic: Unrecognized transport mode

So an absent `transport` is safe where an absent `auth` is not, and only a
wrong value panics. Nothing needs to default it.

The `dburl` session then found the mechanism, which turns the distinction
from a judgement into something checkable. `ParseDSN` fills in
`TransportMode: "binary"` and `Service: "hive"` when it builds its struct
and does not fill in `Auth`, so `Auth` reaches the connect path as the empty
string that panics while the other two arrive with working values. The form
table and that literal say the same thing from opposite ends.

There is a second trap one layer in, which that session hit while writing
the test for its own fix. `?auth=` and a bare `?auth` both parse to an empty
string, so a mechanism that overrides per key regardless of value hands back
exactly the value that panics, and the spelling most likely to be typed by
somebody trying to clear the option is the one that breaks. Checking what a
driver does with nothing is not enough on its own: whatever supplies the
default has to be able to tell nothing from empty.

Two facts the `dburl` session measured from the source, recorded so that
nobody re-measures them: the driver supplies port 10000 itself when the DSN
omits it, and TLS is selected by the `sslcert` and `sslkey` options together
rather than by a scheme suffix, so there is no `s` alias.

`container/hive.go` writes its own DSN and was never blocked on any of this.
Hard rule 1 keeps dbmeta out of dburl's taxonomy, so none of it is work for
this project.
