package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/xo/dbmeta"
)

// moviesFiles holds the Cypher of the movies fixture (D259).
//
//go:embed movies/neo4j.cypher
var moviesFiles embed.FS

// doMovies replaces what the database of a Neo4j server holds with a small movie
// graph. The script begins by deleting every node, so the fixture owns the whole
// database. usql applies it.
func doMovies(ctx context.Context, r runner, t target) error {
	if t.Dialect != dbmeta.Neo4j || t.Kind != kindContainer {
		return fmt.Errorf("the movies fixture is for Neo4j servers, and %s is not one", t.Name)
	}
	if !r.running(ctx, t.Name) {
		return fmt.Errorf("it is not running. Start it with: dbrun start %s", t.Name)
	}
	b, err := moviesFiles.ReadFile("movies/neo4j.cypher")
	if err != nil {
		return fmt.Errorf("reading the script: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if err := usqlFile(ctx, t.URL, string(b)); err != nil {
		return fmt.Errorf("applying movies/neo4j.cypher: %w", err)
	}
	out, err := usqlOutput(ctx, t.URL,
		"MATCH (m:Movie) WITH count(m) AS movies MATCH (p:Person) RETURN movies, count(p) AS people")
	if err != nil {
		return fmt.Errorf("counting the nodes: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	var movies, people int
	if _, err := fmt.Sscanf(lines[len(lines)-1], "%d|%d", &movies, &people); err != nil {
		return errors.Join(fmt.Errorf("reading the count %q", out), err)
	}
	fmt.Printf("  %-*s movies: %d movies and %d people\n", nameWidth(), t.Name, movies, people)
	return nil
}
