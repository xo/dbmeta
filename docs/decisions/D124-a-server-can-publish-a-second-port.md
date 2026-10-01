# D124. A server can publish a second port

Status: Amends D68.

## The decision

A container entry can name a second port inside the container, and dbrun
publishes it as well as the first. The second host port is the first plus
1000. `container.SecondHostPort` computes it, and `Server.SecondPort` holds
the port inside the container.

D68 gave each server one host port, 55000 plus its position in
`container.All`, so that a release gets the same port however it is chosen.
The second port keeps that property, because it is fixed by the first. A URL
function is given the first port, and computes the second from it, so a
product's DSN and URL can each name a different interface.

## Why

QuestDB is the first product that needs two. dburl's `questdb` scheme, from
v0.36.0, opens pgx on the PostgreSQL interface on 8812, and usql reaches
QuestDB through it. The entry published only the HTTP interface on 9000, so
usql was not able to test its `questdb://` URL against it. dbimp plans a
QuestDB driver of its own for the HTTP interface, and the entry's check uses
it.

The first version of this decision said that dbimp's driver reads the HTTP
interface, and kept the DSN on it. That was wrong: the driver is planned and
does not exist yet, and dbimp reads the URL field and not the DSN. When
`models/questdb` arrived, the DSN moved to the PostgreSQL interface, which pgx
takes, so both the DSN and the URL are on the second host port, and the first
is HTTP.

The first host ports run from 55000 to about 55150, so the second ones, from
56000, cannot meet them. dbrun already reads every published port back when
it checks that an existing container matches what it asks for, so a container
created before a change to its ports is found the way D68 finds one.

## Rejected

A second list of host ports, assigned by position the way the first is. It
needs a second counter and a second place for a port to move when a release
is added, and gives nothing the offset does not.

Moving QuestDB to 8812 alone. dbimp's planned driver reads the HTTP
interface, and the entry's check needs it too.
