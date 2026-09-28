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
	"sync"
	"time"
)

// runner is the container command, podman unless DBMETA_RUNNER says otherwise.
type runner struct {
	name string

	// seen is what the runner said about each container, so that a command
	// asks once rather than once for each fact. It is nil where nothing
	// filled it, and every read then asks the runner. See [runner.remember].
	seen *seen
}

// seen holds what inspect said about each container, keyed by name. A name
// that maps to nil is one the runner does not have.
type seen struct {
	mu     sync.Mutex
	byName map[string]*seenContainer
}

// seenContainer is what inspect says about one container.
type seenContainer struct {
	status  string
	started string
	labels  map[string]string
	ports   map[string]string
}

// remember fills what the runner knows about the targets picked, in at most
// two calls, where every fact used to be a call of its own. A status of every
// target was some 160 targets and several calls each, and it took ten
// seconds. ps lists the containers that exist, and a target it does not list
// is remembered as absent, which is the answer for most targets. With all,
// one inspect of the ones that exist reads the rest, which is what status
// and version want. Without it, each one that exists is inspected once, when
// a command first asks about it.
//
// A container a command acts on is forgotten when it acts, so that the next
// read asks the runner again. See [runner.forget].
func (r runner) remember(ctx context.Context, picked []target, all bool) (runner, error) {
	r.seen = &seen{byName: map[string]*seenContainer{}}
	out, err := r.output(ctx, "ps", "--all", "--format", "{{.Names}}")
	if err != nil {
		return r, fmt.Errorf("listing the containers: %w: %s", err, out)
	}
	exist := map[string]bool{}
	for name := range strings.FieldsSeq(out) {
		exist[name] = true
	}
	var present []string
	for _, t := range picked {
		switch {
		case t.Kind != kindContainer && t.Kind != kindMachine:
		case exist[t.Name]:
			present = append(present, t.Name)
		default:
			r.seen.byName[t.Name] = nil
		}
	}
	if all {
		r.load(ctx, present)
	}
	return r, nil
}

// load inspects several containers in one call and remembers each. A name
// the runner no longer has, because it was removed after the listing, makes
// the call fail, and then nothing is remembered and each is asked for alone.
func (r runner) load(ctx context.Context, names []string) {
	if r.seen == nil || len(names) == 0 {
		return
	}
	cmd := exec.CommandContext(ctx, r.name, append([]string{"inspect"}, names...)...)
	body, err := cmd.Output()
	if err != nil {
		return
	}
	got, err := parseInspect(body)
	if err != nil {
		return
	}
	r.seen.mu.Lock()
	defer r.seen.mu.Unlock()
	for name, c := range got {
		r.seen.byName[name] = &c
	}
}

// look is what the runner says about one container, and false when it has
// no such container. It asks the runner only for a container it has not
// remembered, and remembers the answer.
func (r runner) look(ctx context.Context, name string) (seenContainer, bool) {
	if r.seen != nil {
		r.seen.mu.Lock()
		c, known := r.seen.byName[name]
		r.seen.mu.Unlock()
		if known {
			if c == nil {
				return seenContainer{}, false
			}
			return *c, true
		}
	}
	body, err := exec.CommandContext(ctx, r.name, "inspect", name).Output()
	var got map[string]seenContainer
	if err == nil {
		got, err = parseInspect(body)
	}
	c, ok := got[name]
	if err != nil || !ok {
		// Absent, or the runner would not say. Neither is remembered,
		// because a failure is not proof that the container is gone.
		return seenContainer{}, false
	}
	if r.seen != nil {
		r.seen.mu.Lock()
		r.seen.byName[name] = &c
		r.seen.mu.Unlock()
	}
	return c, true
}

// forget drops what is remembered about every container a runner command
// names, before the command runs. start, stop, rm, run and exec change what
// is true of the container or can see it change, so the next read asks the
// runner. inspect and ps only read, and change nothing.
func (r runner) forget(args []string) {
	if r.seen == nil || len(args) == 0 {
		return
	}
	switch args[0] {
	case "inspect", "ps", "image", "build":
		return
	}
	r.seen.mu.Lock()
	defer r.seen.mu.Unlock()
	for _, a := range args {
		delete(r.seen.byName, a)
	}
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
	r.forget(args)
	cmd := exec.CommandContext(ctx, r.name, args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// outputIn runs the runner with input on its standard input, and returns what
// it said. An empty input sends nothing, the same as output.
func (r runner) outputIn(ctx context.Context, input string, args ...string) (string, error) {
	r.forget(args)
	cmd := exec.CommandContext(ctx, r.name, args...)
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// quiet runs the runner and reports only whether it worked.
func (r runner) quiet(ctx context.Context, args ...string) bool {
	_, err := r.output(ctx, args...)
	return err == nil
}

// parseInspect reads what inspect writes for several containers, keyed by
// name. podman and docker write the same JSON, except that docker writes the
// name with a leading slash.
func parseInspect(body []byte) (map[string]seenContainer, error) {
	var all []struct {
		Name  string `json:"Name"`
		State struct {
			Status    string `json:"Status"`
			StartedAt string `json:"StartedAt"`
		} `json:"State"`
		Config struct {
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
		NetworkSettings struct {
			Ports map[string][]struct {
				HostPort string `json:"HostPort"`
			} `json:"Ports"`
		} `json:"NetworkSettings"`
	}
	if err := json.Unmarshal(body, &all); err != nil {
		return nil, fmt.Errorf("reading what inspect said: %w", err)
	}
	seen := make(map[string]seenContainer, len(all))
	for _, c := range all {
		ports := make(map[string]string, len(c.NetworkSettings.Ports))
		for port, binds := range c.NetworkSettings.Ports {
			if len(binds) != 0 {
				ports[strings.TrimSuffix(port, "/tcp")] = binds[0].HostPort
			}
		}
		seen[strings.TrimPrefix(c.Name, "/")] = seenContainer{
			status:  c.State.Status,
			started: c.State.StartedAt,
			labels:  c.Config.Labels,
			ports:   ports,
		}
	}
	return seen, nil
}

// state is what the runner says about a container, which is "running",
// "created", "exited" or empty when there is no such container.
func (r runner) state(ctx context.Context, name string) string {
	c, _ := r.look(ctx, name)
	return c.status
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
	c, ok := r.look(ctx, name)
	return c.ports, ok
}

// create starts a container that does not exist yet.
//
// It says what the runner said rather than swallowing it. A name held in
// container storage by a killed run reports a clash here and nothing else,
// and hiding that behind "could not start" cost an afternoon once, so the one
// command that clears it is printed with the error.
func (r runner) create(ctx context.Context, t target) error {
	out, err := r.output(ctx, withOwner(t.Run, currentOwner(), currentOwnerName())...)
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
		case !r.ready(ctx, t):
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

// ready asks once whether a server answers.
//
// A container is asked with its own readiness command, run inside it. A
// machine has none, because nothing can be run inside Windows or an
// appliance from here, and neither does a container whose image has no
// client, which Exasol's does not. Those are asked the way status asks a
// machine: by connecting and running the version query. Running a machine's empty command ran the
// runner with no arguments, which always fails, so starting a machine that
// was up waited out the whole timeout and then said it never answered.
func (r runner) ready(ctx context.Context, t target) bool {
	if t.Kind == kindMachine || len(t.Ready) == 0 {
		return answered(ctx, t, machineProbe)
	}
	return r.quiet(ctx, t.Ready...)
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
