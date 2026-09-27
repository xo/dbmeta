# D10. Take the initial design from dbtpl and its models directory

Status: Decided.

Do not design the model shape from nothing. Read `dbtpl/models` first, as a
worked example of the shape: the struct per object, the query function per
lookup, and the shared package file. It is another project's code and it is
read for its shape rather than run.

D71 records that nothing here is produced by a tool. The shape above is worth
reading. The way that project builds its own files is not, because this one
does not build files.

Skim, then adapt. `dbtpl/models` is one flat package with the driver in the
function name, as in `PostgresTables` and `MysqlTables`. D2 sets a package per
driver instead, so the names lose the prefix and become `postgres.Tables`.
