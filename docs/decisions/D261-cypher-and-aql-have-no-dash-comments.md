# D261. Cypher and AQL have no comment that starts with two dashes

Status: Amends D143.

usql read two dashes as the start of a comment in every dialect, so it cut
`MATCH (p:Person) RETURN count{(p)--()}` after `count{(p)` and sent half a
statement. Neo4j reads `--` as part of a relationship pattern, and `5--3` is 8.
usql asked for a flag that says which dialect reads two dashes as a comment. Ken decided on 2026-11-04 to record it here.

## Decision

`Syntax.DashComments` says that two dashes start a comment to the end of the
line. It is true for every model but Neo4j and ArangoDB. The flag is positive, like
`SlashComments` and `HashComments`, and the zero value means no dash comments.
An earlier version of this decision had the negative flag `NoDashComments`, and
Ken chose the positive sense on 2026-11-04 (transit D139).

Every SQL product reads two dashes as a comment, so a model that forgets the
flag makes a client stop reading them as one.
`TestEveryModelHasDashCommentsButGraphLanguages` fails for a dialect with a model
that does not say it, and for Neo4j and ArangoDB if they do.

## What was measured

Neo4j 2026.10 answered `RETURN 5--3` with 8, and ran `COUNT { (p)--() }` as a
count of the relationships of a node. The comment of Cypher is two slashes,
which `SlashComments` already records. `TestNeo4jDashesAreNotAComment` holds the
model to a server.

ArangoDB 3.12 answered `RETURN 5--3` with 8 and `RETURN 1 -- 7` followed by
`+ 2` with 10, so two dashes are not a comment in AQL either, and `//` is.
`TestArangoDBDashesAreNotAComment` holds the model to a server.

The other models are set to true without a run. A client has read two dashes as a
comment for every dialect until now, and the flag keeps that. The Cosmos DB model
says in its comment that the SQL of Cosmos DB has comments that start with two
hyphens.
