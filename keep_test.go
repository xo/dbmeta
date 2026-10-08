package dbmeta

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"strings"
	"testing"
)

const keepDialect Dialect = "keepdb"

// keepSeen holds the arguments the last call of Keep received.
var keepSeen map[string]any

func init() {
	RegisterDialect(keepDialect, &Info{Placeholder: func(int) string { return "?" }})
	Columns.Register(keepDialect, &Binding[Column]{
		Stmt:   Always(`SELECT t, n FROM keep WHERE @name = '' OR @name <> ''`),
		Fields: []Field{{Name: "table"}, {Name: "name"}},
		Params: []Param{{Name: "name", Default: ""}},
		Scan: func(rows *sql.Rows) (Column, error) {
			var c Column
			return c, rows.Scan(&c.Table, &c.Name)
		},
		Keep: func(c Column, args map[string]any) bool {
			keepSeen = args
			name, _ := args["name"].(string)
			return Like(name, c.Name)
		},
	})
}

// keepMeta returns the Meta of the keep dialect and records rows for its
// statement.
func keepMeta(t *testing.T, args map[string]any, rows [][]driver.Value) *Meta {
	t.Helper()
	var s VersionSet
	s.Set("", ParseVersion("1"))
	m, err := New(keepDialect, s)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	query, _, err := Columns.Build(m, args)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	record(query, []string{"table", "name"}, rows, nil)
	return m
}

func TestKeepRejectsRows(t *testing.T) {
	// not parallel: these tests share the replay map, keyed by statement text
	args := map[string]any{"name": "a%"}
	m := keepMeta(t, args, [][]driver.Value{
		{"t", "alpha"}, {"t", "beta"}, {"t", "apple"},
	})
	db, err := openFake()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer db.Close()
	var got []string
	for c, err := range Columns.All(context.Background(), m, db, args) {
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		got = append(got, c.Name)
	}
	if strings.Join(got, ",") != "alpha,apple" {
		t.Errorf("expected alpha and apple, got %v", got)
	}
}

func TestKeepReceivesTheDefaults(t *testing.T) {
	// not parallel: these tests share the replay map, keyed by statement text
	m := keepMeta(t, nil, [][]driver.Value{{"t", "alpha"}, {"t", "beta"}})
	db, err := openFake()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer db.Close()
	keepSeen = nil
	n := 0
	for _, err := range Columns.All(context.Background(), m, db, nil) {
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		n++
	}
	if n != 2 {
		t.Errorf("expected both rows with the default filter, got %d", n)
	}
	if v, ok := keepSeen["name"]; !ok || v != "" {
		t.Errorf("expected the default for name, got %v", keepSeen)
	}
}

func TestKeepRefusesAnUnknownArgument(t *testing.T) {
	// not parallel: these tests share the replay map, keyed by statement text
	m := keepMeta(t, nil, nil)
	db, err := openFake()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer db.Close()
	_, _, err = First(Columns.All(context.Background(), m, db, map[string]any{"bad": 1}))
	if err == nil {
		t.Error("expected an error for an argument the query does not take")
	}
}

// TestKeepPassesAReadError checks that a row Scan cannot read reaches the
// caller, and that the rows after it still arrive when the caller goes on.
func TestKeepPassesAReadError(t *testing.T) {
	// not parallel: these tests share the replay map, keyed by statement text
	m := keepMeta(t, nil, [][]driver.Value{
		{"t", "alpha"}, {"t", nil}, {"t", "beta"},
	})
	db, err := openFake()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer db.Close()
	var names []string
	var failed int
	for c, err := range Columns.All(context.Background(), m, db, nil) {
		if err != nil {
			failed++
			continue
		}
		names = append(names, c.Name)
	}
	if failed != 1 || strings.Join(names, ",") != "alpha,beta" {
		t.Errorf("expected one error and two rows, got %d and %v", failed, names)
	}
}

func TestKeepStoppingEarlyReleases(t *testing.T) {
	// not parallel: these tests share the replay map, keyed by statement text
	args := map[string]any{"name": "a%"}
	m := keepMeta(t, args, [][]driver.Value{
		{"t", "b"}, {"t", "a1"}, {"t", "a2"},
	})
	db, err := openFake()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for range Columns.All(context.Background(), m, db, args) {
		break
	}
	n := 0
	for range Columns.All(context.Background(), m, db, args) {
		n++
	}
	if n != 2 {
		t.Errorf("expected the connection to be released, read %d rows on the second pass", n)
	}
}
