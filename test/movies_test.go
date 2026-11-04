package test

import (
	"bytes"
	"encoding/json"
	"os"
	osexec "os/exec"
	"reflect"
	"strings"
	"testing"
)

// moviesScript is the Cypher that dbrun fixture movies applies (D260).
const moviesScript = "cmd/dbrun/movies/neo4j.cypher"

// usqlJSON runs one query with usql, which prints the rows as JSON, and returns
// the rows. The output of usql is what the tests read, and not the output of a
// driver.
func usqlJSON(t *testing.T, url, query string) ([]map[string]any, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := osexec.CommandContext(t.Context(), "usql", "-q", "-J", url, "-c", query)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, &usqlError{err: err, stderr: stderr.String()}
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	var rows []map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &rows); err != nil {
		t.Fatalf("reading the rows of %q: %v\n%s", query, err, stdout.String())
	}
	return rows, nil
}

// usqlError is a failure of usql with what it wrote to the error output.
type usqlError struct {
	err    error
	stderr string
}

func (e *usqlError) Error() string { return e.err.Error() + "\n" + e.stderr }

func (e *usqlError) Unwrap() error { return e.err }

// TestMoviesFixtureAnswers loads the movies fixture with usql and holds ten
// questions about the graph to their answers. The questions are the ones that
// a person asks of a movie graph: a lookup, an aggregate, a path, a
// recommendation, a count and a write. Every list has a full order, because the
// server does not order ties.
func TestMoviesFixtureAnswers(t *testing.T) {
	url := os.Getenv("DBMETA_NEO4J")
	if url == "" {
		t.Skip("set DBMETA_NEO4J to run against a real server")
	}
	if _, err := osexec.LookPath("usql"); err != nil {
		t.Skip("usql is not on the path")
	}
	script, err := os.ReadFile(moviesScript)
	if err != nil {
		t.Fatalf("reading the script: %v", err)
	}
	if out, err := osexec.CommandContext(t.Context(), "usql", "-q", url, "-f", moviesScript).CombinedOutput(); err != nil {
		t.Fatalf("applying the script of %d bytes: %v\n%s", len(script), err, out)
	}
	t.Cleanup(func() {
		// The write of the last question leaves a user behind.
		osexec.Command("usql", "-q", url, "-c", "MATCH (u:User {name:'bob'}) DETACH DELETE u").Run() //nolint:errcheck,noctx // best effort
	})
	cases := []struct {
		name  string
		query string
		want  string
		// known is the text of an error that a fault of usql causes. The case
		// is skipped while usql says it, so that the fault shows in the log and
		// the case runs again when usql is fixed.
		known string
	}{
		{"actors of The Matrix",
			"MATCH (p:Person)-[r:ACTED_IN]->(m:Movie {title:'The Matrix'}) RETURN p.name AS actor, r.roles AS roles ORDER BY actor",
			`[{"actor":"Carrie-Anne Moss","roles":["Trinity"]},{"actor":"Keanu Reeves","roles":["Neo"]},{"actor":"Laurence Fishburne","roles":["Morpheus"]}]`, ""},
		{"average rating by genre",
			"MATCH (m:Movie)-[:IN_GENRE]->(g:Genre) RETURN g.name AS genre, count(m) AS movies, round(avg(m.rating),2) AS avg_rating ORDER BY avg_rating DESC, genre",
			`[{"genre":"Comedy","movies":1,"avg_rating":8.8},{"genre":"Drama","movies":1,"avg_rating":8.8},{"genre":"Thriller","movies":1,"avg_rating":8.8},{"genre":"Sci-Fi","movies":3,"avg_rating":8.23},{"genre":"Action","movies":2,"avg_rating":7.95}]`, ""},
		{"co-stars",
			"MATCH (a:Person)-[:ACTED_IN]->(m:Movie)<-[:ACTED_IN]-(b:Person) WHERE a.name < b.name RETURN a.name AS p1, b.name AS p2, m.title AS together_in ORDER BY together_in, p1, p2",
			`[{"p1":"Robin Wright","p2":"Tom Hanks","together_in":"Forrest Gump"},{"p1":"Elliot Page","p2":"Joseph Gordon-Levitt","together_in":"Inception"},{"p1":"Elliot Page","p2":"Leonardo DiCaprio","together_in":"Inception"},{"p1":"Joseph Gordon-Levitt","p2":"Leonardo DiCaprio","together_in":"Inception"},{"p1":"Carrie-Anne Moss","p2":"Keanu Reeves","together_in":"The Matrix"},{"p1":"Carrie-Anne Moss","p2":"Laurence Fishburne","together_in":"The Matrix"},{"p1":"Keanu Reeves","p2":"Laurence Fishburne","together_in":"The Matrix"},{"p1":"Carrie-Anne Moss","p2":"Keanu Reeves","together_in":"The Matrix Reloaded"}]`, ""},
		{"friends of friends of Keanu",
			"MATCH (k:Person {name:'Keanu Reeves'})-[:FRIEND_OF*2]->(fof:Person) RETURN DISTINCT fof.name AS friend_of_friend ORDER BY friend_of_friend",
			`[{"friend_of_friend":"Joseph Gordon-Levitt"}]`, ""},
		{"shortest path from Moss to Hanks",
			"MATCH p = shortestPath((a:Person {name:'Carrie-Anne Moss'})-[*..6]-(b:Person {name:'Tom Hanks'})) RETURN length(p) AS hops",
			`[{"hops":4}]`, ""},
		{"recommendation by genre",
			"MATCH (u:User {name:'alice'})-[:RATED]->(seen:Movie)-[:IN_GENRE]->(g:Genre) MATCH (rec:Movie)-[:IN_GENRE]->(g) " +
				"WHERE NOT (u)-[:RATED]->(rec) AND rec <> seen WITH rec, g ORDER BY g.name " +
				"RETURN rec.title AS recommendation, collect(DISTINCT g.name) AS shared_genres, rec.rating AS rating ORDER BY rating DESC, recommendation",
			`[{"recommendation":"The Matrix Reloaded","shared_genres":["Action","Sci-Fi"],"rating":7.2}]`, ""},
		{"recommendation through friends",
			"MATCH (u:User {name:'alice'})-[:RATED]->(m:Movie)<-[:ACTED_IN]-(actor:Person) " +
				"MATCH (actor)-[:FRIEND_OF*1..2]-(other:Person)-[:ACTED_IN]->(rec:Movie) WHERE NOT (u)-[:RATED]->(rec) " +
				"RETURN rec.title AS movie, count(DISTINCT other) AS signal ORDER BY signal DESC, movie",
			`[{"movie":"The Matrix Reloaded","signal":2}]`, ""},
		{"most connected people",
			"MATCH (p:Person) RETURN p.name AS person, COUNT { (p)--() } AS degree, COUNT { (p)-[:ACTED_IN]->() } AS acted_in, " +
				"COUNT { (p)-[:FRIEND_OF]-() } AS friends ORDER BY degree DESC, person LIMIT 5",
			`[{"person":"Keanu Reeves","degree":4,"acted_in":2,"friends":2},{"person":"Carrie-Anne Moss","degree":3,"acted_in":2,"friends":1},{"person":"Leonardo DiCaprio","degree":3,"acted_in":1,"friends":2},{"person":"Joseph Gordon-Levitt","degree":2,"acted_in":1,"friends":1},{"person":"Lana Wachowski","degree":2,"acted_in":0,"friends":0}]`,
			"// dbimp:"},
		{"genre pairs",
			"MATCH (g1:Genre)<-[:IN_GENRE]-(m:Movie)-[:IN_GENRE]->(g2:Genre) WHERE g1.name < g2.name " +
				"RETURN g1.name AS genre_a, g2.name AS genre_b, count(m) AS co_occurrences ORDER BY co_occurrences DESC, genre_a, genre_b",
			`[{"genre_a":"Action","genre_b":"Sci-Fi","co_occurrences":2},{"genre_a":"Comedy","genre_b":"Drama","co_occurrences":1},{"genre_a":"Sci-Fi","genre_b":"Thriller","co_occurrences":1}]`, ""},
		// The last two write. They go last, and the cleanup removes bob.
		{"a new rating",
			"MERGE (u:User {name:'bob'}) WITH u MATCH (m:Movie {title:'Inception'}) MERGE (u)-[r:RATED]->(m) " +
				"ON CREATE SET r.score = 5, r.at = datetime() ON MATCH SET r.score = coalesce(r.score, 5) " +
				"RETURN u.name AS user, m.title AS movie, r.score AS score",
			`[{"user":"bob","movie":"Inception","score":5}]`, ""},
		{"ratings of Inception after it",
			"MATCH (m:Movie {title:'Inception'})<-[:RATED]-(u:User) RETURN m.title AS movie, count(u) AS ratings, avg(m.rating) AS critic_avg",
			`[{"movie":"Inception","ratings":2,"critic_avg":8.8}]`, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := usqlJSON(t, url, c.query)
			if err != nil {
				if c.known != "" && strings.Contains(err.Error(), c.known) {
					t.Skipf("a fault of usql, and the server accepts the query: %v", err)
				}
				t.Fatalf("running the query: %v", err)
			}
			var want []map[string]any
			if err := json.Unmarshal([]byte(c.want), &want); err != nil {
				t.Fatalf("reading the expected rows: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				gb, _ := json.Marshal(got)
				t.Errorf("got  %s\nwant %s", gb, c.want)
			}
		})
	}
}
