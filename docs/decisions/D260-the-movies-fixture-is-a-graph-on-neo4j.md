# D260. The movies fixture is a graph on Neo4j

Status: Decided.

`dbrun fixture movies <selector>` replaces what a Neo4j database holds with a
small movie graph, and usql applies the script `movies/neo4j.cypher`. Ken gave
the graph on 2026-11-04. It is a fixture for the describe commands of usql and
for the people who write Cypher.

The script begins with `MATCH (n) DETACH DELETE n`, so the fixture owns the
database it runs in. dbrun does nothing before that. Two runs leave the same
graph.

## The graph

- `Person` has `name` and `born`. `Movie` has `title`, `year` and `rating`.
  `Genre` has `name`. `User` has `name`.
- `ACTED_IN` has `roles`, `DIRECTED` has no property, `IN_GENRE` goes from a
  movie to a genre, `FRIEND_OF` has `since`, and `RATED` has `score`.
- 12 people, 4 movies, 5 genres and one user, `alice`.

The model that came with the script says that a person rates a movie. The data
makes a `User` rate it, and the comment of the script says the user is
simulated. The data decides, and the comment at the top of the file names `User`.

## How it is tested

`TestMoviesFixtureAnswers` in the `test` module loads the script with usql and
asks ten questions about the graph: a lookup, an average by genre, co-stars,
friends of friends, a shortest path, two recommendations, a count of
connections, genre pairs and a write. Ken decided on 2026-11-04 that the tests
read the output of usql, with `-J`, and not the output of the Neo4j driver.
Every list has a full order, because the server does not order ties. The write
goes last, and the cleanup removes the user that it makes.

One case is skipped while usql has a fault. usql reads `--` in the pattern
`(p)--()` as the start of an SQL comment, so the query for the most connected
people stops before the end of its `COUNT { }`. The server accepts the query,
and its answer was read through the HTTP interface. The case runs again when
usql is fixed.
