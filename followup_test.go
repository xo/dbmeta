package dbmeta

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"
)

// Three dialects that need a follow-up version statement, one per answer the
// server can give it. Each has its own first statement, because the fake
// driver answers by statement text and the tests run in parallel. The follow-up
// is asked only of a server whose first answer names the flavor, which is the
// ScyllaDB case reduced to what the rule needs. See D92.
const (
	followServed    Dialect = "followserved"
	followRefused   Dialect = "followrefused"
	followReference Dialect = "followreference"
)

func init() {
	for _, d := range []Dialect{followServed, followRefused, followReference} {
		RegisterDialect(d, &Info{
			Placeholder:    func(int) string { return "?" },
			VersionQuery:   "SELECT product FROM " + string(d),
			VersionColumns: 1,
			ParseVersion: func(cols []string) (VersionSet, error) {
				var s VersionSet
				s.Set("", ParseVersion("3.0.8"))
				if cols[0] == "flavor" {
					s.Set("flavor", Version{Unknown: true})
				}
				s.Display = cols[0]
				return s, nil
			},
			FollowUpQuery: func(s VersionSet) (string, int) {
				if !s.Has("flavor") {
					return "", 0
				}
				return "SELECT release FROM " + string(d), 1
			},
			ParseFollowUp: func(s VersionSet, cols []string) (VersionSet, error) {
				s.Set("flavor", ParseVersion(cols[0]))
				return s, nil
			},
		})
	}
	record("SELECT product FROM followserved", []string{"product"},
		[][]driver.Value{{"flavor"}}, nil)
	record("SELECT release FROM followserved", []string{"release"},
		[][]driver.Value{{"2026.3.1"}}, nil)
	record("SELECT product FROM followrefused", []string{"product"},
		[][]driver.Value{{"flavor"}}, nil)
	record("SELECT release FROM followrefused", nil, nil,
		errors.New("unauthorized: no SELECT permission on system.versions"))
	record("SELECT product FROM followreference", []string{"product"},
		[][]driver.Value{{"reference"}}, nil)
}

// TestFollowUpReadsTheFlavorRelease checks that Version runs the second
// statement when the first one found the flavor, and records what it read.
func TestFollowUpReadsTheFlavorRelease(t *testing.T) {
	t.Parallel()
	db, err := openFake()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s, err := followServed.Version(context.Background(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if got := s.Get("flavor").String(); got != "2026.3.1" {
		t.Errorf("flavor release: got %s, want 2026.3.1", got)
	}
	if got := s.Main().String(); got != "3.0.8" {
		t.Errorf("main version: got %s, want 3.0.8", got)
	}
}

// TestFollowUpRefusedKeepsTheFirstAnswer checks that a server refusing the
// second statement leaves the set as the first one read it. The product is
// still named, and its release stays unknown. ScyllaDB refuses
// system.versions to a role granted nothing on it and serves system.local.
func TestFollowUpRefusedKeepsTheFirstAnswer(t *testing.T) {
	t.Parallel()
	db, err := openFake()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s, err := followRefused.Version(context.Background(), db)
	if err != nil {
		t.Fatalf("expected the first answer and no error, got: %v", err)
	}
	if !s.Has("flavor") || !s.Get("flavor").Unknown {
		t.Errorf("expected the flavor with an unknown release, got %v", s.Versions)
	}
}

// cancelAfterFirst answers the first statement and cancels the context before
// the second, which is the one moment a refusal and a cancellation look alike.
type cancelAfterFirst struct {
	db     *sql.DB
	cancel context.CancelFunc
	asked  int
}

// QueryContext satisfies Queryer.
func (q *cancelAfterFirst) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	q.asked++
	if q.asked == 2 {
		q.cancel()
	}
	return q.db.QueryContext(ctx, query, args...)
}

// TestFollowUpCancelledIsAnError checks that a context cancelled between the
// two statements is reported rather than taken for a refusal, because nothing
// after it can run.
func TestFollowUpCancelledIsAnError(t *testing.T) {
	t.Parallel()
	db, err := openFake()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q := &cancelAfterFirst{db: db, cancel: cancel}
	if _, err := followServed.Version(ctx, q); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if q.asked != 2 {
		t.Errorf("expected two statements, got %d", q.asked)
	}
}

// TestFollowUpNotAskedOfTheReference checks that the reference product runs
// one statement and nothing else.
func TestFollowUpNotAskedOfTheReference(t *testing.T) {
	t.Parallel()
	db, err := openFake()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s, err := followReference.Version(context.Background(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if s.Has("flavor") {
		t.Error("the reference product reported the flavor key")
	}
	if _, _, ok := followReference.FollowUpQuery(s); ok {
		t.Error("expected no follow-up for the reference product")
	}
}
