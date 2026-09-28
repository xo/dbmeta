package main

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// ownerLabel is the container label that names who started a server.
//
// Several sessions share one machine, and before this a start that made a
// fifth server stopped the oldest one of any session. It stopped a Couchbase
// server that another session was using. With an owner on every server,
// dbrun acts only on the caller's own servers unless --force says otherwise.
// See D98.
const ownerLabel = "dbmeta.owner"

// ownerNameLabel is the container label that holds the friendly name of the
// owner, such as dbimp, which a person can read where the owner is a session
// ID. It is set from DBMETA_OWNER_NAME, and it is only ever shown: the owner
// label alone decides who may act on a server. See D115.
const ownerNameLabel = "dbmeta.owner.name"

// currentOwnerName is the friendly name the caller gave itself, or empty.
func currentOwnerName() string {
	return os.Getenv("DBMETA_OWNER_NAME")
}

// currentOwner names the caller, in the form the label records.
//
// DBMETA_OWNER wins, so a person or a workflow can name itself. A coding
// agent is named by its session, which stays the same across every command
// the agent runs and differs between two agents. A person at a terminal is
// named by the login name.
func currentOwner() string {
	if s := os.Getenv("DBMETA_OWNER"); s != "" {
		return s
	}
	if s := os.Getenv("CLAUDE_CODE_SESSION_ID"); s != "" {
		return "claude-code:" + s
	}
	if s := os.Getenv("USER"); s != "" {
		return "user:" + s
	}
	return "unknown"
}

// withOwner returns the arguments of a run command with the owner label added
// after the word run, and the friendly name of the owner when there is one.
// Every run command here begins with run.
func withOwner(args []string, owner, name string) []string {
	if len(args) == 0 || args[0] != "run" {
		return args
	}
	out := make([]string, 0, len(args)+4)
	out = append(out, args[0], "--label", ownerLabel+"="+owner)
	if name != "" {
		out = append(out, "--label", ownerNameLabel+"="+name)
	}
	return append(out, args[1:]...)
}

// label reads one label of a container, and is empty when it has none or
// there is no such container.
func (r runner) label(ctx context.Context, name, label string) string {
	c, _ := r.look(ctx, name)
	return c.labels[label]
}

// who says who owns a server, for a person to read: the friendly name and
// the owner, or the owner alone.
func (r runner) who(ctx context.Context, name string) string {
	owner := r.owner(ctx, name)
	if friendly := r.label(ctx, name, ownerNameLabel); friendly != "" && owner != "" {
		return friendly + " (" + showOwner(owner) + ")"
	}
	return showOwner(owner)
}

// owner reads the owner label of a container. It is empty for a container
// created before servers had owners, and for one that does not exist.
func (r runner) owner(ctx context.Context, name string) string {
	return r.label(ctx, name, ownerLabel)
}

// startedAt reads when a container last started, as the runner writes it.
func (r runner) startedAt(ctx context.Context, name string) string {
	c, _ := r.look(ctx, name)
	return c.started
}

// mayTouch reports whether the caller may stop, remove, restart or rebuild a
// server with this owner.
//
// A server with no owner was created before servers had one. It is treated
// as it was then, so that the change breaks nothing that already runs.
// Removing and starting it again gives it an owner.
func mayTouch(owner, me string, force bool) bool {
	return force || owner == "" || owner == me
}

// claimable reports whether the caller may remove or rebuild a server with
// this owner. It is mayTouch, and also any stopped container, because a
// stopped container belongs to nobody. A machine keeps its owner when it
// stops, because it takes an hour to create again. See D108.
func claimable(t target, owner, me string, force, running bool) bool {
	return mayTouch(owner, me, force) || (t.Kind == kindContainer && !running)
}

// notYours is the error for a server that belongs to somebody else. who is
// the owner as runner.who writes it.
func notYours(who string) error {
	return fmt.Errorf("it belongs to %s. Ask them, or pass --force to act on it anyway", who)
}

// showOwner shortens an owner for a person to read. A session name is a UUID,
// and its first part is enough to tell two sessions apart.
func showOwner(owner string) string {
	if owner == "" {
		return "nobody"
	}
	if kind, id, ok := strings.Cut(owner, ":"); ok && kind == "claude-code" && len(id) > 8 {
		return kind + ":" + id[:8]
	}
	return owner
}

// holder is one running server and who started it. who is the owner as a
// person reads it.
type holder struct {
	name  string
	owner string
	who   string
}

// pickEvictee chooses which running server to stop so that one more can start.
// up is oldest first.
//
// It chooses the oldest of the caller's own servers. A server with no owner
// is not chosen, although a command that names it may stop it: every server
// that ran before owners existed is somebody's, and stopping one silently is
// the fault that owners exist to end. When there is none it refuses and names
// who holds the servers. --force chooses the oldest of any owner, which is
// what D75 did for every start.
func pickEvictee(up []holder, me string, force bool) (string, error) {
	for _, h := range up {
		if force || h.owner == me {
			return h.name, nil
		}
	}
	held := make([]string, len(up))
	for i, h := range up {
		who := h.who
		if who == "" {
			who = showOwner(h.owner)
		}
		held[i] = h.name + " (" + who + ")"
	}
	return "", fmt.Errorf("%d servers are running and none of them is yours: %s."+
		" Stop one of yours, ask the owner of one, or pass --force to stop the oldest",
		len(up), strings.Join(held, ", "))
}
