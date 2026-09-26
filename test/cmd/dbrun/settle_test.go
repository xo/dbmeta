package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// waitReady returns on the first pass for a server with no settle, and only
// after the check has kept passing for a server with one. D83 is why, and the
// failure it is for is a server that runs a statement and then refuses the
// next one.
//
// These drive the real loop through a shell command rather than a fake, so
// that what is tested is the code dbrun runs.

// flapper returns a command that succeeds, fails on the nth call, and
// succeeds after that. The count is a file, because each call is its own
// process.
func flapper(t *testing.T, n int) []string {
	t.Helper()
	count := filepath.Join(t.TempDir(), "n")
	return []string{"sh", "-c", fmt.Sprintf(
		`n=$(cat %[1]s 2>/dev/null || echo 0); n=$((n+1)); echo $n > %[1]s; [ $n -ne %[2]d ]`,
		count, n)}
}

func TestSettleReturnsOnTheFirstPassWhenThereIsNone(t *testing.T) {
	t.Parallel()
	r := runner{name: "env"}
	start := time.Now()
	if err := r.waitReady(context.Background(), target{
		Ready: []string{"true"},
	}, 30*time.Second); err != nil {
		t.Fatalf("waiting: %v", err)
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Errorf("took %s with no settle, and the first pass should have done", took)
	}
}

func TestSettleWaitsForTheCheckToKeepPassing(t *testing.T) {
	t.Parallel()
	r := runner{name: "env"}
	start := time.Now()
	if err := r.waitReady(context.Background(), target{
		Ready:  []string{"true"},
		Settle: 3 * time.Second,
	}, 30*time.Second); err != nil {
		t.Fatalf("waiting: %v", err)
	}
	if took := time.Since(start); took < 3*time.Second {
		t.Errorf("took %s, which is less than the %s settle", took, 3*time.Second)
	}
}

// TestSettleRestartsAfterAFailure is the one that matters. A server that
// passes, lapses and passes again has to serve the whole settle from the
// second time, because the lapse is the fault being waited out.
func TestSettleRestartsAfterAFailure(t *testing.T) {
	t.Parallel()
	// Passes on call 1, fails on call 2, passes from 3. With a 3 second
	// settle and a one second poll, a loop that did not restart would return
	// at about 4 seconds and one that does returns at about 6.
	r := runner{name: "env"}
	start := time.Now()
	if err := r.waitReady(context.Background(), target{
		Ready:  flapper(t, 2),
		Settle: 3 * time.Second,
	}, 30*time.Second); err != nil {
		t.Fatalf("waiting: %v", err)
	}
	if took := time.Since(start); took < 5*time.Second {
		t.Errorf("took %s, so the failure on the second check did not restart"+
			" the settle. That lapse is the whole reason for it.", took)
	}
}

func TestSettleSaysWhichKindOfTimeoutItWas(t *testing.T) {
	t.Parallel()
	r := runner{name: "env"}
	err := r.waitReady(context.Background(), target{
		Ready:  []string{"true"},
		Settle: 30 * time.Second,
	}, 2*time.Second)
	if err == nil {
		t.Fatal("expected a timeout, because the settle is longer than the budget")
	}
	if !strings.Contains(err.Error(), "did not keep answering") {
		t.Errorf("the error is %q and does not say the server stopped answering", err)
	}

	err = r.waitReady(context.Background(), target{
		Ready:  []string{"false"},
		Settle: 30 * time.Second,
	}, 2*time.Second)
	if err == nil {
		t.Fatal("expected a timeout for a server that never answered")
	}
	if !strings.Contains(err.Error(), "never answered") {
		t.Errorf("the error is %q and does not say the server never answered", err)
	}
}
