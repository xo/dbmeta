package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"sort"
	"strings"
	"time"
)

// runner is the container command, podman unless DBMETA_RUNNER says otherwise.
type runner struct {
	name string
}

// newRunner finds the container command.
//
// DBMETA_RUNNER names one outright. Otherwise podman is preferred and docker
// is used when podman is absent, so this works on a machine with either and
// nobody has to say which.
func newRunner() (runner, error) {
	if name := os.Getenv("DBMETA_RUNNER"); name != "" {
		if _, err := exec.LookPath(name); err != nil {
			return runner{}, fmt.Errorf("DBMETA_RUNNER is %s and it is not on the path", name)
		}
		return runner{name: name}, nil
	}
	for _, name := range []string{"podman", "docker"} {
		if _, err := exec.LookPath(name); err == nil {
			return runner{name: name}, nil
		}
	}
	return runner{}, errors.New("neither podman nor docker is on the path." +
		" Set DBMETA_RUNNER to the one you have")
}

// podman reports whether the runner is podman, for the few places the two
// commands differ.
func (r runner) podman() bool { return strings.Contains(r.name, "podman") }

// imageExists reports whether the runner already has an image.
//
// podman has `image exists`, which answers by exit code. docker does not, and
// `image inspect` is the way to ask it the same question.
func (r runner) imageExists(ctx context.Context, ref string) bool {
	if r.podman() {
		return r.quiet(ctx, "image", "exists", ref)
	}
	return r.quiet(ctx, "image", "inspect", ref)
}

// build builds an image from a Containerfile. Both runners take the same
// arguments for this.
func (r runner) build(ctx context.Context, ref, file, dir string, args map[string]string) error {
	argv := []string{"build", "--tag", ref, "--file", file}
	for _, k := range sortedKeys(args) {
		argv = append(argv, "--build-arg", k+"="+args[k])
	}
	argv = append(argv, dir)
	cmd := exec.CommandContext(ctx, r.name, argv...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("building %s: %w", ref, err)
	}
	return nil
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// output runs the runner and returns what it said, stdout and stderr together.
func (r runner) output(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, r.name, args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// quiet runs the runner and reports only whether it worked.
func (r runner) quiet(ctx context.Context, args ...string) bool {
	_, err := r.output(ctx, args...)
	return err == nil
}

// state is what the runner says about a container, which is "running",
// "created", "exited" or empty when there is no such container.
func (r runner) state(ctx context.Context, name string) string {
	out, err := r.output(ctx, "inspect", "--format", "{{.State.Status}}", name)
	if err != nil {
		return ""
	}
	return out
}

func (r runner) running(ctx context.Context, name string) bool {
	return r.state(ctx, name) == "running"
}

func (r runner) exists(ctx context.Context, name string) bool {
	return r.state(ctx, name) != ""
}

// hostPorts reads what an existing container actually publishes, as a map of
// container port to host port. The second result is false when there is no
// such container or the runner will not say.
//
// Both runners report the same shape: {"8080/tcp":[{"HostIp":"...",
// "HostPort":"55038"}]}.
func (r runner) hostPorts(ctx context.Context, name string) (map[string]string, bool) {
	out, err := r.output(ctx, "inspect", "--format", "{{json .NetworkSettings.Ports}}", name)
	if err != nil {
		return nil, false
	}
	var raw map[string][]struct {
		HostPort string `json:"HostPort"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, false
	}
	ports := make(map[string]string, len(raw))
	for port, binds := range raw {
		if len(binds) != 0 {
			ports[strings.TrimSuffix(port, "/tcp")] = binds[0].HostPort
		}
	}
	return ports, true
}

// create starts a container that does not exist yet.
//
// It says what the runner said rather than swallowing it. A name held in
// container storage by a killed run reports a clash here and nothing else,
// and hiding that behind "could not start" cost an afternoon once, so the one
// command that clears it is printed with the error.
func (r runner) create(ctx context.Context, t target) error {
	out, err := r.output(ctx, t.Run...)
	if err == nil {
		return nil
	}
	last := out
	if _, after, found := strings.Cut(out, "\n"); found {
		// The runner writes a paragraph and the last line is the error.
		lines := strings.Split(after, "\n")
		last = lines[len(lines)-1]
	}
	if strings.Contains(out, "already in use") && r.podman() {
		// podman can leave a name held in container storage where rm --force
		// does not reach it. docker has no such state and no such command.
		return fmt.Errorf("%s\n  clear it with: %s rm --storage %s", last, r.name, t.Name)
	}
	return fmt.Errorf("%s", last)
}

// waitReady waits until the server answers, not until the port opens.
//
// Several of these images start, bootstrap a data directory and restart, so a
// connection made in between is refused and a fixed pause is not enough. A
// Windows machine is worse: it boots the whole operating system before SQL
// Server listens, which is why the wait is measured in minutes.
//
// A server with a Settle does not count as up on the first pass. The check
// has to keep passing for that long, and one failure starts it again, because
// Presto and Trino can run a statement and then refuse the next one while
// their coordinator refreshes which nodes it will schedule on. See D83.
func (r runner) waitReady(ctx context.Context, t target, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var since time.Time
	for {
		switch {
		case !r.quiet(ctx, t.Ready...):
			// Not ready, or ready and then not. Either way the clock on a
			// settle starts again from the next pass.
			since = time.Time{}
		case t.Settle <= 0:
			return nil
		case since.IsZero():
			since = time.Now()
		case time.Since(since) >= t.Settle:
			return nil
		}
		if time.Now().After(deadline) {
			if !since.IsZero() {
				return fmt.Errorf("it answered and did not keep answering for %s, in %s",
					t.Settle, timeout)
			}
			return fmt.Errorf("it never answered in %s", timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

// runningSince lists the containers this project knows about that are up,
// oldest first. The second value of each pair is when it started.
//
// It asks the runner for every running container and keeps the ones whose
// name is a target here, so a container somebody else is running on the same
// machine is never touched.
func (r runner) runningSince(ctx context.Context, known map[string]bool) []string {
	out, err := r.output(ctx, "ps", "--format", "{{.StartedAt}}\t{{.Names}}")
	if err != nil {
		return nil
	}
	type up struct {
		at   string
		name string
	}
	var ours []up
	for line := range strings.SplitSeq(out, "\n") {
		at, name, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if !ok || !known[name] {
			continue
		}
		ours = append(ours, up{at: at, name: name})
	}
	// StartedAt sorts lexically in the order it sorts chronologically for
	// both runners, because both write a fixed width timestamp.
	slices.SortFunc(ours, func(a, b up) int { return strings.Compare(a.at, b.at) })
	names := make([]string, len(ours))
	for i, u := range ours {
		names[i] = u.name
	}
	return names
}
