// A movie graph for Neo4j, with people, movies, genres and what links them:
//
// (:Person {name, born})-[:ACTED_IN {roles}]->(:Movie {title, year, rating})
// (:Person)-[:DIRECTED]->(:Movie)
// (:Movie)-[:IN_GENRE]->(:Genre {name})
// (:Person)-[:FRIEND_OF {since}]->(:Person)
// (:User)-[:RATED {score}]->(:Movie)

// Clean slate
MATCH (n) DETACH DELETE n;

// --- Genres ---
UNWIND ['Sci-Fi','Action','Drama','Thriller','Comedy'] AS g
CREATE (:Genre {name: g});

// --- People ---
UNWIND [
  {name:'Keanu Reeves',        born:1964},
  {name:'Carrie-Anne Moss',    born:1967},
  {name:'Laurence Fishburne',  born:1961},
  {name:'Lana Wachowski',      born:1965},
  {name:'Lilly Wachowski',     born:1967},
  {name:'Leonardo DiCaprio',   born:1974},
  {name:'Joseph Gordon-Levitt',born:1981},
  {name:'Elliot Page',         born:1987},
  {name:'Christopher Nolan',   born:1970},
  {name:'Tom Hanks',           born:1956},
  {name:'Robin Wright',        born:1966},
  {name:'Robert Zemeckis',     born:1952}
] AS p
CREATE (:Person {name:p.name, born:p.born});

// --- Movies ---
UNWIND [
  {title:'The Matrix',            year:1999, rating:8.7},
  {title:'The Matrix Reloaded',   year:2003, rating:7.2},
  {title:'Inception',             year:2010, rating:8.8},
  {title:'Forrest Gump',          year:1994, rating:8.8}
] AS m
CREATE (:Movie {title:m.title, year:m.year, rating:m.rating});

// --- ACTED_IN ---
MATCH (k:Person {name:'Keanu Reeves'}),        (m:Movie {title:'The Matrix'})           CREATE (k)-[:ACTED_IN {roles:['Neo']}]->(m);
MATCH (c:Person {name:'Carrie-Anne Moss'}),    (m:Movie {title:'The Matrix'})           CREATE (c)-[:ACTED_IN {roles:['Trinity']}]->(m);
MATCH (l:Person {name:'Laurence Fishburne'}),  (m:Movie {title:'The Matrix'})           CREATE (l)-[:ACTED_IN {roles:['Morpheus']}]->(m);
MATCH (k:Person {name:'Keanu Reeves'}),        (m:Movie {title:'The Matrix Reloaded'})  CREATE (k)-[:ACTED_IN {roles:['Neo']}]->(m);
MATCH (c:Person {name:'Carrie-Anne Moss'}),    (m:Movie {title:'The Matrix Reloaded'})  CREATE (c)-[:ACTED_IN {roles:['Trinity']}]->(m);
MATCH (d:Person {name:'Leonardo DiCaprio'}),   (m:Movie {title:'Inception'})            CREATE (d)-[:ACTED_IN {roles:['Cobb']}]->(m);
MATCH (j:Person {name:'Joseph Gordon-Levitt'}),(m:Movie {title:'Inception'})            CREATE (j)-[:ACTED_IN {roles:['Arthur']}]->(m);
MATCH (e:Person {name:'Elliot Page'}),         (m:Movie {title:'Inception'})            CREATE (e)-[:ACTED_IN {roles:['Ariadne']}]->(m);
MATCH (t:Person {name:'Tom Hanks'}),           (m:Movie {title:'Forrest Gump'})         CREATE (t)-[:ACTED_IN {roles:['Forrest']}]->(m);
MATCH (r:Person {name:'Robin Wright'}),        (m:Movie {title:'Forrest Gump'})         CREATE (r)-[:ACTED_IN {roles:['Jenny']}]->(m);

// --- DIRECTED ---
MATCH (lw:Person {name:'Lana Wachowski'}),  (m:Movie {title:'The Matrix'})          CREATE (lw)-[:DIRECTED]->(m);
MATCH (lw:Person {name:'Lilly Wachowski'}), (m:Movie {title:'The Matrix'})          CREATE (lw)-[:DIRECTED]->(m);
MATCH (lw:Person {name:'Lana Wachowski'}),  (m:Movie {title:'The Matrix Reloaded'}) CREATE (lw)-[:DIRECTED]->(m);
MATCH (lw:Person {name:'Lilly Wachowski'}), (m:Movie {title:'The Matrix Reloaded'}) CREATE (lw)-[:DIRECTED]->(m);
MATCH (n:Person {name:'Christopher Nolan'}),(m:Movie {title:'Inception'})           CREATE (n)-[:DIRECTED]->(m);
MATCH (z:Person {name:'Robert Zemeckis'}),  (m:Movie {title:'Forrest Gump'})        CREATE (z)-[:DIRECTED]->(m);

// --- IN_GENRE ---
MATCH (m:Movie {title:'The Matrix'}),           (g:Genre {name:'Sci-Fi'})   CREATE (m)-[:IN_GENRE]->(g);
MATCH (m:Movie {title:'The Matrix'}),           (g:Genre {name:'Action'})   CREATE (m)-[:IN_GENRE]->(g);
MATCH (m:Movie {title:'The Matrix Reloaded'}),  (g:Genre {name:'Sci-Fi'})   CREATE (m)-[:IN_GENRE]->(g);
MATCH (m:Movie {title:'The Matrix Reloaded'}),  (g:Genre {name:'Action'})   CREATE (m)-[:IN_GENRE]->(g);
MATCH (m:Movie {title:'Inception'}),            (g:Genre {name:'Sci-Fi'})   CREATE (m)-[:IN_GENRE]->(g);
MATCH (m:Movie {title:'Inception'}),            (g:Genre {name:'Thriller'}) CREATE (m)-[:IN_GENRE]->(g);
MATCH (m:Movie {title:'Forrest Gump'}),         (g:Genre {name:'Drama'})    CREATE (m)-[:IN_GENRE]->(g);
MATCH (m:Movie {title:'Forrest Gump'}),         (g:Genre {name:'Comedy'})   CREATE (m)-[:IN_GENRE]->(g);

// --- FRIEND_OF ---
MATCH (a:Person {name:'Keanu Reeves'}),     (b:Person {name:'Carrie-Anne Moss'}) CREATE (a)-[:FRIEND_OF {since:1999}]->(b);
MATCH (a:Person {name:'Keanu Reeves'}),     (b:Person {name:'Leonardo DiCaprio'}) CREATE (a)-[:FRIEND_OF {since:2010}]->(b);
MATCH (a:Person {name:'Leonardo DiCaprio'}),(b:Person {name:'Joseph Gordon-Levitt'}) CREATE (a)-[:FRIEND_OF {since:2010}]->(b);
MATCH (a:Person {name:'Tom Hanks'}),        (b:Person {name:'Robin Wright'})     CREATE (a)-[:FRIEND_OF {since:1994}]->(b);

// --- RATED (user "alice" simulated as Person) ---
MERGE (u:User {name:'alice'})
WITH u
MATCH (m:Movie) WHERE m.title IN ['The Matrix','Inception','Forrest Gump']
MERGE (u)-[:RATED {score: CASE m.title WHEN 'The Matrix' THEN 5 WHEN 'Inception' THEN 4 ELSE 3 END}]->(m);
