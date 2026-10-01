package dbmeta

import (
	"context"
	"database/sql/driver"
	"errors"
	"io"
	"testing"
)

// TestEachPassesOnlyWhatTheQueryTakes checks that Each leaves out an argument
// the query does not take, where All refuses it. Columns here takes none.
func TestEachPassesOnlyWhatTheQueryTakes(t *testing.T) {
	// not parallel: these tests share the replay map, keyed by statement text
	m := meta(t, "18")
	query, _, _ := Columns.Build(m, nil)
	record(query, []string{"table", "name"}, [][]driver.Value{{"t", "id"}}, nil)
	db, err := openFake()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer db.Close()
	args := Args{Schema: "public", Name: "id", Server: "s", WithSystem: true}
	if _, _, err := First(Columns.All(context.Background(), m, db, args.Map())); !errors.Is(err, ErrUnknownParam) {
		t.Errorf("All: expected ErrUnknownParam, got: %v", err)
	}
	v, ok, err := First(Columns.Each(context.Background(), m, db, args))
	if err != nil || !ok || v.Name != "id" {
		t.Errorf("Each: got %+v, %v, %v, want the column id", v, ok, err)
	}
}

// TestEachReportsWhatIsNotAnswered checks that Each yields the same error as
// All for a kind the model does not answer.
func TestEachReportsWhatIsNotAnswered(t *testing.T) {
	t.Parallel()
	m := meta(t, "18")
	if _, _, err := First(Sequences.Each(context.Background(), m, nil, Args{})); !errors.Is(err, ErrNotSupported) {
		t.Errorf("expected ErrNotSupported, got: %v", err)
	}
}

// TestOpenReadsTheVersion checks that Open reads the version and builds the
// metadata, and that a version it cannot read is an error and not a guess.
func TestOpenReadsTheVersion(t *testing.T) {
	// not parallel: these tests share the replay map, keyed by statement text
	db, err := openFake()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer db.Close()
	record("SHOW server_version", []string{"server_version"}, [][]driver.Value{{"18.1"}}, nil)
	m, err := Open(context.Background(), testDialect, db)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if got := m.Version().Main().Raw; got != "18.1" {
		t.Errorf("expected the version 18.1, got %q", got)
	}
	record("SHOW server_version", nil, nil, io.ErrClosedPipe)
	if m, err := Open(context.Background(), testDialect, db); m != nil || !errors.Is(err, io.ErrClosedPipe) {
		t.Errorf("expected no metadata and the error, got %v and %v", m, err)
	}
}
