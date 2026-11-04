package test

import (
	"slices"
	"testing"

	"github.com/xo/dbmeta"
)

// TestNeo4jDashesAreNotAComment sends a statement with two dashes that are not
// a comment to a Neo4j server and holds the model to the answer. In Cypher
// `5--3` is 8, and `(a)--(b)` is a relationship. See D261.
func TestNeo4jDashesAreNotAComment(t *testing.T) {
	syntax, ok := dbmeta.Neo4j.Syntax()
	if !ok {
		t.Fatal("neo4j has no model")
	}
	if syntax.DashComments {
		t.Error("the model says that two dashes start a comment")
	}
	db := openNeo4j(t)
	var got int64
	if err := db.QueryRowContext(t.Context(), "RETURN 5--3 AS a").Scan(&got); err != nil {
		t.Fatalf("running the statement: %v", err)
	}
	if got != 8 {
		t.Errorf("5--3 is %d, and the expected value is 8, which says that the two dashes are not a comment", got)
	}
}

// TestArangoDBDashesAreNotAComment is the same measurement for AQL. In AQL
// `5--3` is 8, and the comment is two slashes. See D261.
func TestArangoDBDashesAreNotAComment(t *testing.T) {
	syntax, ok := dbmeta.ArangoDB.Syntax()
	if !ok {
		t.Fatal("arangodb has no model")
	}
	if syntax.DashComments {
		t.Error("the model says that two dashes start a comment")
	}
	db := openArangoDB(t)
	var got int64
	if err := db.QueryRowContext(t.Context(), "RETURN 5--3").Scan(&got); err != nil {
		t.Fatalf("running the statement: %v", err)
	}
	if got != 8 {
		t.Errorf("5--3 is %d, and the expected value is 8, which says that the two dashes are not a comment", got)
	}
}

// noDashComments are the dialects that do not read two dashes as a comment. Every
// other dialect with a model does, because every SQL product does.
var noDashComments = []dbmeta.Dialect{dbmeta.Neo4j, dbmeta.ArangoDB}

// TestEveryModelHasDashCommentsButGraphLanguages holds that a model that forgets
// the flag is found, because a client then stops reading two dashes as a comment
// for that product. Cypher and AQL are the dialects that do not read them as
// one. See D261.
func TestEveryModelHasDashCommentsButGraphLanguages(t *testing.T) {
	t.Parallel()
	for _, d := range dbmeta.Dialects() {
		syntax, ok := d.Syntax()
		if !ok {
			continue
		}
		if want := !slices.Contains(noDashComments, d); syntax.DashComments != want {
			t.Errorf("%s: DashComments is %t, and the expected value is %t", d, syntax.DashComments, want)
		}
	}
}
