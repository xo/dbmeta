package test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/xo/dbmeta"
	osfixture "github.com/xo/dbmeta/models/opensearch/fixture"
)

// TestOpenSearchManyIndices checks the cost of the walk of Columns, which runs
// one DESCRIBE for each index (D47, D181). It makes 100 indices of 5 fields,
// reads their columns, and stops a walk on its first row. The walk holds one
// connection, so a pool of one is enough.
func TestOpenSearchManyIndices(t *testing.T) {
	db := openOpenSearch(t)
	m := setupOpenSearch(t, db)
	ctx := t.Context()
	dsn := os.Getenv("DBMETA_OPENSEARCH")
	const indices, fields = 100, 5
	var props []string
	for i := range fields {
		props = append(props, fmt.Sprintf(`"f%d":{"type":"keyword"}`, i))
	}
	mapping := `{"mappings":{"properties":{` + strings.Join(props, ",") + `}}}`
	var up, down []osfixture.Step
	for i := range indices {
		name := fmt.Sprintf("dbmeta_many_%03d", i)
		up = append(up, osfixture.Step{Name: name, Request: osfixture.Request{Method: http.MethodPut, Path: "/" + name, Body: mapping}})
		down = append(down, osfixture.Step{Name: name, Request: osfixture.Request{Method: http.MethodDelete, Path: "/" + name}})
	}
	if err := osRun(ctx, dsn, down, true); err != nil {
		t.Fatalf("removing earlier indices: %v", err)
	}
	if err := osRun(ctx, dsn, up, false); err != nil {
		t.Fatalf("making the indices: %v", err)
	}
	t.Cleanup(func() {
		//nolint:errcheck // a teardown is best effort
		osRun(context.WithoutCancel(ctx), dsn, down, true)
	})
	db.SetMaxOpenConns(1)

	start := time.Now()
	tables := map[string]int{}
	for v, err := range dbmeta.Columns.All(ctx, m, db, dbmeta.Args{Parent: "dbmeta_many_%"}.Map()) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		tables[v.Table]++
	}
	t.Logf("%d indices read in %s", len(tables), time.Since(start))
	if len(tables) != indices {
		t.Errorf("expected the columns of %d indices, got %d", indices, len(tables))
	}
	for name, n := range tables {
		if n != fields {
			t.Errorf("%s: expected %d columns, got %d", name, fields, n)
		}
	}
	var first int
	for _, err := range dbmeta.Columns.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		first++
		break
	}
	if first != 1 {
		t.Errorf("expected one column before the break, got %d", first)
	}
	// The connection is free again, so the next read does not wait for it.
	if n := countRows(t, dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Name: "dbmeta_many_%"}.Map())); n != indices {
		t.Errorf("expected %d tables, got %d", indices, n)
	}
}
