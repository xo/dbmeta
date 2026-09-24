package dbmeta

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"testing"
)

func init() {
	Columns.Register(testDialect, &Binding[Column]{
		Stmt:   Always(`SELECT c.relname AS "table", a.attname AS "name" FROM pg_attribute a`),
		Fields: []Field{{Name: "table"}, {Name: "name"}},
		// the kind of function a generator writes. D30 keeps reflection out
		// by emitting this alongside the query.
		Scan: func(rows *sql.Rows) (Column, error) {
			var c Column
			return c, rows.Scan(&c.Table, &c.Name)
		},
	})
}

func TestAllStreamsRows(t *testing.T) {
	// not parallel: these tests share the replay map, keyed by statement text
	m := meta(t, "18")
	sqlstr, _, err := Columns.SQL(m, nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	record(sqlstr, []string{"table", "name"}, [][]driver.Value{
		{"t", "id"},
		{"t", "name"},
	}, nil)
	db, err := openFake()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer db.Close()

	var got []string
	for col, err := range Columns.All(context.Background(), m, db, nil) {
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		got = append(got, col.Table+"."+col.Name)
	}
	if len(got) != 2 || got[0] != "t.id" || got[1] != "t.name" {
		t.Errorf("expected two columns, got %v", got)
	}
}

// TestAllStoppingEarlyReleases checks the hazard D33 names. An iterator holds a
// connection, so breaking out of the loop must give it back. With one
// connection in the pool, a second query after an early break would block
// forever if it did not.
func TestAllStoppingEarlyReleases(t *testing.T) {
	// not parallel: these tests share the replay map, keyed by statement text
	m := meta(t, "18")
	sqlstr, _, _ := Columns.SQL(m, nil)
	record(sqlstr, []string{"table", "name"}, [][]driver.Value{
		{"t", "a"}, {"t", "b"}, {"t", "c"},
	}, nil)
	db, err := openFake()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for range Columns.All(context.Background(), m, db, nil) {
		break
	}
	// the connection must be back, so a second pass can run
	n := 0
	for range Columns.All(context.Background(), m, db, nil) {
		n++
	}
	if n != 3 {
		t.Errorf("expected the connection to be released, read %d rows on the second pass", n)
	}
}

func TestAllReportsQueryFailure(t *testing.T) {
	// not parallel: these tests share the replay map, keyed by statement text
	m := meta(t, "18")
	sqlstr, _, _ := Columns.SQL(m, nil)
	record(sqlstr, nil, nil, io.ErrClosedPipe)
	db, err := openFake()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer db.Close()
	var seen error
	for _, err := range Columns.All(context.Background(), m, db, nil) {
		seen = err
	}
	if seen == nil {
		t.Error("expected the iterator to report the failure")
	}
}

// TestAllReportsRenderFailure checks that a failure before any row reaches the
// caller through the same channel, so there is one error path and not two.
func TestAllReportsRenderFailure(t *testing.T) {
	t.Parallel()
	db, err := openFake()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer db.Close()
	_, ok, err := First(Tables.All(context.Background(), meta(t, "18"), db, map[string]any{"bad": 1}))
	if ok {
		t.Error("expected no value")
	}
	if !errors.Is(err, ErrUnknownParam) {
		t.Errorf("expected ErrUnknownParam, got: %v", err)
	}
}
