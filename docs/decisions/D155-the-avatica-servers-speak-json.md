# D155. The Avatica servers speak JSON

Status: Amends D113.

## The decision

D113 ran the standalone Avatica server and the Phoenix Query Server with
protobuf, their default. dbimp's Avatica driver speaks JSON alone, with no
protobuf (dbimp D153), and a JSON request to a server that speaks protobuf
answers with a protobuf error. So on 2026-10-01 both entries were changed to
JSON, at dbimp's request.

- The standalone server takes `--serialization JSON`, on 1.28.0 and 1.29.0.
- The Phoenix Query Server reads `phoenix.queryserver.serialization` from
  `hbase-site.xml`. The image's start script edits that file with
  xmlstarlet, so the entry adds the property the same way, once, and then runs
  the script.
- The ready check of both posts `{"request":"openConnection"}` in JSON and
  passes only when the answer is `{"response":"openConnection"}`, and then
  closes the connection.

Each release answered openConnection and closeConnection in JSON, and
Phoenix did so again after a stop and a start.

Ken decided the URL of dbimp's Avatica driver the same day (dbimp D156):
`avatica://user:password@host:port`, with no path, the default port 8765, and
the keys tls and auth alone. The driver sends the user and the password in the
info of openConnection. So each principal's url is that form,
`avatica://SA@127.0.0.1:<port>` and `avatica://dbmeta_user:<password>@...` on
the standalone server and `avatica://phoenix@...` on Phoenix, and the dsn stays
`http://` for dbimp's recorder.

Druid is not an Avatica server here any more. Ken decided on 2026-10-01 that
Druid gets a driver of its own in dbimp, on its SQL API, with the name druid
(dbimp D154), so its entry keeps what D113 describes until that driver asks
for more.

## An ordinary user on the standalone server

D113 found that a user SA made through the standalone server was not found
by a second connection, and made none. The cause was the case of the name,
measured again on 1.29.0 in JSON. HSQLDB folds a name that is not quoted to
upper case, so `CREATE USER dbmeta_user` makes `DBMETA_USER`, and a login as
`dbmeta_user` is not found. A login as `DBMETA_USER` works. That user read a
table SA granted it, and was refused a table it was not granted and a CREATE
TABLE, with "user lacks privilege or object not found".

So the entry has an ordinary user, at dbimp's request the same day. The
database is in memory and begins empty on each start, so the setup runs on
every start, as SA through the JSON API. It makes `"dbmeta_user"` with
`container.Password`, quoted so that the name keeps its case, the schema
DBMETA and the table DBMETA.READABLE, and grants the user SELECT on that table
and nothing else. It ends by reading the table as the user. Avatica answers
an error with HTTP 500, so a statement that finds its object already there on
a rerun fails and is ignored, and the read decides.

On 1.28.0 and 1.29.0, and on 1.29.0 after a stop and a start, the user read
DBMETA.READABLE, and was refused CREATE TABLE and a table SA made and did not
grant, both with "user lacks privilege or object not found". The setup ran a
second time against a live server and succeeded.

Phoenix checks users only through Kerberos, which the image does not
configure, so its ordinary user is absent, as on InfluxDB 3 Core.
