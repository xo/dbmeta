# D102. dbrun prints every principal of a server

Status: Amends D98.

Ken decided on 2026-09-27 that `dsn --json` and `list --json` print every
principal of a server, which D98 had left out. dbimp tests Couchbase as its
ordinary user too, and it built that user's connection string by hand.

Each server now has `Principals`, in the `container` package: the
administrator first, whose name is read from the server's URL, and then each
ordinary user that the server's setup creates, declared beside the setup that
makes it. Each carries its role, its name and its connection string. `dbrun`
prints them as the `principals` field. Couchbase is the one server with an
ordinary user today, `dbmeta_user`, and the next one that makes such a user
declares it the same way.

A user that a parity test makes and drops is not a principal of the server.
It exists only during that test.

`TestEveryServerNamesItsPrincipals` checks that every server names its
administrator and that each ordinary user's connection string holds its own
name.

## test keeps a server it found running

Measuring this change found a gap in D98. `dbrun test` removed
`couchbase-8.0.3` when the test ended. The server had no owner, because it
predated owners, and D98 let a command act on such a server as before. But
the test had not started it, and it was up before the test began, so removing
it took away a server that was in use. It was started again at once. `test`
now keeps a server that was running when it began, unless the server is the
caller's own. A run as another owner against that server passed and kept it.
