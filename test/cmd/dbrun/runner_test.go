package main

import "testing"

// TestParseInspectReadsBothRunners holds that one inspect of several
// containers is read the same under podman and docker. docker writes a
// name with a leading slash, and podman without.
func TestParseInspectReadsBothRunners(t *testing.T) {
	t.Parallel()
	body := []byte(`[
{"Name": "qdrant-1.19.1", "State": {"Status": "exited", "StartedAt": "2026-09-29T01:38:00+07:00"},
 "Config": {"Labels": {"dbmeta.owner": "claude-code:b3cc5e94"}},
 "NetworkSettings": {"Ports": {}}},
{"Name": "/chroma-1.5.9", "State": {"Status": "running", "StartedAt": "2026-09-29T01:38:02+07:00"},
 "Config": {"Labels": {"dbmeta.owner": "ken", "dbmeta.owner.name": "dbmeta"}},
 "NetworkSettings": {"Ports": {"8000/tcp": [{"HostIp": "", "HostPort": "55093"}]}}}
]`)
	seen, err := parseInspect(body)
	if err != nil {
		t.Fatal(err)
	}
	q, c := seen["qdrant-1.19.1"], seen["chroma-1.5.9"]
	switch {
	case len(seen) != 2:
		t.Errorf("got %d containers, want 2: %v", len(seen), seen)
	case q.status != "exited" || q.labels["dbmeta.owner"] != "claude-code:b3cc5e94" || len(q.ports) != 0:
		t.Errorf("qdrant-1.19.1: %+v", q)
	case c.status != "running" || c.ports["8000"] != "55093" || c.labels["dbmeta.owner.name"] != "dbmeta" ||
		c.started != "2026-09-29T01:38:02+07:00":
		t.Errorf("chroma-1.5.9: %+v", c)
	}
}

// TestARunnerCommandForgetsWhatItChanges holds the rule of the cache: a
// command that names a container forgets it, so that the next read asks the
// runner, and one that only reads does not. The runner here does not exist,
// so a read that asked it would find nothing.
func TestARunnerCommandForgetsWhatItChanges(t *testing.T) {
	t.Parallel()
	r := runner{name: "dbrun-no-such-runner", seen: &seen{byName: map[string]*seenContainer{
		"up":   {status: "running"},
		"gone": nil,
	}}}
	if c, ok := r.look(t.Context(), "up"); !ok || c.status != "running" {
		t.Fatalf("a remembered container read as %+v, %v", c, ok)
	}
	if _, ok := r.look(t.Context(), "gone"); ok {
		t.Fatal("a container remembered as absent was found")
	}
	r.forget([]string{"inspect", "up"})
	if !r.running(t.Context(), "up") {
		t.Error("inspect made the cache forget a container")
	}
	r.forget([]string{"stop", "up"})
	if r.running(t.Context(), "up") {
		t.Error("stop did not make the cache forget the container it stopped")
	}
	r.forget([]string{"exec", "gone", "true"})
	if _, remembered := r.seen.byName["gone"]; remembered {
		t.Error("exec did not make the cache forget the container it ran in")
	}
}
