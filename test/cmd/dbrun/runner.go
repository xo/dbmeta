package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
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
func (r runner) waitReady(ctx context.Context, t target, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if r.quiet(ctx, t.Ready...) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("it never answered in %s", timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}
