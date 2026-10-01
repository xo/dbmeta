package main

import (
	"slices"
	"strings"
	"testing"
)

// TestCurrentOwnerPrefersWhatIsNamed checks the order the owner is read in:
// DBMETA_OWNER, then an agent's session, then the login name.
func TestCurrentOwnerPrefersWhatIsNamed(t *testing.T) {
	t.Setenv("DBMETA_OWNER", "")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	t.Setenv("USER", "ken")
	if got := currentOwner(); got != "user:ken" {
		t.Errorf("a person: got %q", got)
	}
	t.Setenv("CLAUDE_CODE_SESSION_ID", "b3cc5e94-7287-4d6b-92d7-1890ecc80e38")
	if got := currentOwner(); got != "claude-code:b3cc5e94-7287-4d6b-92d7-1890ecc80e38" {
		t.Errorf("an agent: got %q", got)
	}
	t.Setenv("DBMETA_OWNER", "ci")
	if got := currentOwner(); got != "ci" {
		t.Errorf("named: got %q", got)
	}
}

// TestWithOwnerLabelsARun checks that the label goes after the word run and
// that anything else is left alone.
func TestWithOwnerLabelsARun(t *testing.T) {
	t.Parallel()
	got := withOwner([]string{"run", "--detach", "--name", "postgres-18", "image"}, "user:ken", "")
	want := []string{"run", "--label", "dbmeta.owner=user:ken", "--detach", "--name", "postgres-18", "image"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	start := []string{"start", "postgres-18"}
	if got := withOwner(start, "user:ken", "dbimp"); !slices.Equal(got, start) {
		t.Errorf("a command that is not run changed: %v", got)
	}
}

// TestPickEvicteeStopsOnlyYourOwn is the rule D98 adds to D75. A start that
// needs room stops the caller's oldest server, and refuses when every running
// server is somebody else's or has no owner.
func TestPickEvicteeStopsOnlyYourOwn(t *testing.T) {
	t.Parallel()
	const me = "claude-code:aaaa"
	up := []holder{
		{name: "postgres-18", owner: "claude-code:bbbb"},
		{name: "mysql-8.4", owner: me},
		{name: "oracle-26ai", owner: "claude-code:bbbb"},
		{name: "mariadb-13.0", owner: me},
	}
	if got, err := pickEvictee(up, me, false); err != nil || got != "mysql-8.4" {
		t.Errorf("expected your oldest, mysql-8.4, got %q and %v", got, err)
	}
	legacy := []holder{{name: "postgres-18"}, {name: "mysql-8.4", owner: me}}
	if got, err := pickEvictee(legacy, me, false); err != nil || got != "mysql-8.4" {
		t.Errorf("expected your own server and not the one with no owner, got %q and %v", got, err)
	}
	if _, err := pickEvictee([]holder{{name: "postgres-18"}}, me, false); err == nil {
		t.Error("expected a refusal rather than stopping a server with no owner")
	}
	theirs := []holder{{name: "postgres-18", owner: "claude-code:bbbb"}, {name: "oracle-26ai", owner: "user:ken"}}
	_, err := pickEvictee(theirs, me, false)
	if err == nil {
		t.Fatal("expected a refusal when none of the servers is yours")
	}
	for _, want := range []string{"postgres-18 (claude-code:bbbb)", "oracle-26ai (user:ken)", "--force"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not say %q: %v", want, err)
		}
	}
	if got, err := pickEvictee(theirs, me, true); err != nil || got != "postgres-18" {
		t.Errorf("--force: expected the oldest, postgres-18, got %q and %v", got, err)
	}
}

// TestMayTouch checks who can stop, remove, restart or rebuild a server.
func TestMayTouch(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		owner, me string
		force     bool
		want      bool
	}{
		{"user:ken", "user:ken", false, true},
		{"claude-code:bbbb", "user:ken", false, false},
		{"claude-code:bbbb", "user:ken", true, true},
		{"", "user:ken", false, true},
	} {
		if got := mayTouch(test.owner, test.me, test.force); got != test.want {
			t.Errorf("owner %q, me %q, force %v: got %v", test.owner, test.me, test.force, got)
		}
	}
	if got := showOwner("claude-code:b3cc5e94-7287-4d6b-92d7-1890ecc80e38"); got != "claude-code:b3cc5e94" {
		t.Errorf("a session is shortened to its first part: got %q", got)
	}
}

// TestClaimable checks that a stopped container belongs to nobody, and that a
// running container and a stopped machine keep their owner. See D108.
func TestClaimable(t *testing.T) {
	t.Parallel()
	const me, them = "user:ken", "claude-code:bbbb"
	container, machine := target{Kind: kindContainer}, target{Kind: kindMachine}
	for _, test := range []struct {
		name    string
		t       target
		owner   string
		running bool
		want    bool
	}{
		{"your running container", container, me, true, true},
		{"their running container", container, them, true, false},
		{"their stopped container", container, them, false, true},
		{"their running machine", machine, them, true, false},
		{"their stopped machine", machine, them, false, false},
	} {
		if got := claimable(test.t, test.owner, me, false, test.running); got != test.want {
			t.Errorf("%s: got %v, want %v", test.name, got, test.want)
		}
	}
}

// TestWithOwnerNamesTheOwner checks that a friendly name goes in a label of
// its own beside the owner, and only when it is given. See D115.
func TestWithOwnerNamesTheOwner(t *testing.T) {
	t.Parallel()
	got := withOwner([]string{"run", "image"}, "claude-code:aaaa", "dbimp")
	want := []string{"run", "--label", ownerLabel + "=claude-code:aaaa",
		"--label", ownerNameLabel + "=dbimp", "image"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
