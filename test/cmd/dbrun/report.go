package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

// statusEntry is one running server, as status --json prints it. It carries
// every field list --json prints, and what only a running server has.
type statusEntry struct {
	target

	// State is "running" for a container, "answering" or "starting" for a
	// machine, "moved" for a container whose port no longer matches the
	// list, "stopped" for one that is stopped, which only --all prints, and
	// "embedded" for a library.
	State string `json:"state"`
	// Owner is who started the server, empty for one from before servers
	// had owners. Mine says whether that is the caller. See D98. OwnerName
	// is the friendly name the owner gave itself, when it gave one (D115).
	Owner     string `json:"owner"`
	OwnerName string `json:"ownerName,omitempty"`
	Mine      bool   `json:"mine"`
	// Started is when the server last started, in RFC 3339, as inspect
	// writes it in JSON under both runners.
	Started string `json:"started,omitempty"`
}

// statusJSON prints the running servers among picked, and every embedded
// database, as one JSON list. A server that is not running is left out, the
// way the text form leaves it out, unless all asks for it.
func statusJSON(ctx context.Context, r runner, picked []target, all bool) error {
	me := currentOwner()
	out := []statusEntry{}
	for _, t := range picked {
		if t.Kind == kindEmbedded {
			out = append(out, statusEntry{target: t, State: "embedded", Mine: true})
			continue
		}
		if t.Kind == kindHosted {
			out = append(out, statusEntry{target: t, State: "hosted", Mine: true})
			continue
		}
		state := "running"
		if !r.running(ctx, t.Name) {
			if !all || !r.exists(ctx, t.Name) {
				continue
			}
			state = "stopped"
		}
		e := statusEntry{
			target:    t,
			State:     state,
			Owner:     r.owner(ctx, t.Name),
			OwnerName: r.label(ctx, t.Name, ownerNameLabel),
			Started:   r.startedAt(ctx, t.Name),
		}
		e.Mine = e.Owner == me
		if state == "stopped" {
			out = append(out, e)
			continue
		}
		if have, ok := r.hostPorts(ctx, t.Name); ok && !t.portsMatch(have) {
			e.State = "moved"
		} else if t.Kind == kindMachine {
			e.State = "starting"
			if answered(ctx, t, statusProbe) {
				e.State = "answering"
			}
		}
		out = append(out, e)
	}
	return writeJSON(out)
}

// versionEntry is one server's version, as version --json prints it.
type versionEntry struct {
	Name string `json:"name"`
	// Display is the line a person reads, the one the text form prints.
	Display string `json:"display,omitempty"`
	// Versions holds each version the server reports, by key. The empty key
	// is the main one.
	Versions map[string]string `json:"versions,omitempty"`
	// Error says why the version could not be read.
	Error string `json:"error,omitempty"`
}

// versionJSON prints the version of each running server among picked. A
// server whose version cannot be read is printed with the error, and the
// command then exits 1, as the text form does.
func versionJSON(ctx context.Context, r runner, picked []target) error {
	out := []versionEntry{}
	var failed []string
	for _, t := range picked {
		if t.Kind == kindEmbedded || (t.Kind != kindHosted && !r.running(ctx, t.Name)) {
			continue
		}
		e := versionEntry{Name: t.Name}
		versions, err := readVersion(ctx, t)
		if err != nil {
			e.Error = err.Error()
			failed = append(failed, t.Name)
		} else {
			e.Display = versions.String()
			e.Versions = map[string]string{}
			for _, key := range versions.Keys() {
				e.Versions[key] = versions.Get(key).String()
			}
		}
		out = append(out, e)
	}
	if err := writeJSON(out); err != nil {
		return err
	}
	if len(failed) != 0 {
		return errSilent
	}
	return nil
}

func writeJSON(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding the output: %w", err)
	}
	_, err = fmt.Fprintln(os.Stdout, string(b))
	return err
}
