# D239. The MaxCompute tests retry a reset connection

Status: Decided.

## The decision

The endpoint of MaxCompute in Jakarta resets a connection often. The driver
reports "read: connection reset by peer", and it did so for many statements on
2026-10-29, both when it sends a statement and when it reads the result. The same
statement works on a later attempt.

The tests open a driver that wraps the driver of Alibaba. It runs a query or a
statement again, up to a hundred times with one second between the attempts, when the
error says that the connection was reset. It retries no other error. The wrapper
is in `test/maxcompute_driver_test.go` and it registers as `odps-retry`.

The statements that group a view, which are `Privileges` and `Functions`, run for
longer and were reset far more often than most others. Each failed all of 20 and then
all of 30 attempts in three runs of the tests.
The cause was not found. One guess is that something between the test machine and
the endpoint resets a connection that is idle for a few seconds, and the statement
polls the instance while it runs. That is why the limit is a hundred.

Each connection of the driver builds an HTTP client of its own, and that client keeps
its sockets open. A socket that sits idle can be closed by the service, and the next
statement on it is then reset. The wrapper says that a connection is never valid when
it goes back to the pool, so every statement runs on a new socket. It was not
measured how much this lowers the number of resets.

The library does not retry. A caller of `dbmeta` decides what to do with a failed
statement, and a retry inside `Query.All` hides a fault that a caller needs
to see. `dbrun version` opens the plain driver and reports the reset as it is.

A statement that the wrapper runs again is a query or a statement that can run
twice. The fixture uses `IF NOT EXISTS` and `IF EXISTS`, and its teardown ignores
an object that is gone.
