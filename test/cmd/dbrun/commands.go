package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// defaultTimeout is how long a container gets to answer, and a machine gets
// far longer because it boots Windows first.
const (
	defaultTimeout = 90 * time.Second
	machineTimeout = 15 * time.Minute
	// How long status waits for a machine to answer. It is short because
	// status is a question about what is there, not a wait for it to arrive.
	statusProbe = 5 * time.Second
)

// maxRunning is how many of these servers may be up at once.
//
// Each one is bounded to container.MemoryLimit, so the ceiling is that times
// this, and the rest of the machine is left alone. Without a cap a session
// that starts a server per question ends with a dozen up, all idle, and the
// host in swap.
//
// Starting one when this many are already up stops the one that has been
// running longest. That is the right one to lose: the server in use is the
// one most recently started, and starting is a minute for a container.
const maxRunning = 4

// makeRoom stops the longest running servers until starting one more keeps
// the count at or below maxRunning.
//
// It only ever stops a container this project knows about, and never the one
// being started. A machine is left alone: stopping Windows mid install is
// how an hour is lost, and D57 keeps a machine for that reason.
func makeRoom(ctx context.Context, r runner, keep string) {
	known := map[string]bool{}
	for _, t := range targets() {
		if t.Kind == kindContainer && t.Name != keep {
			known[t.Name] = true
		}
	}
	up := r.runningSince(ctx, known)
	for len(up) >= maxRunning {
		oldest := up[0]
		up = up[1:]
		if !r.quiet(ctx, "stop", oldest) {
			continue
		}
		fmt.Printf("  %-20s stopped to stay within %d running\n", oldest, maxRunning)
	}
}

func (t target) timeout(o options) time.Duration {
	if o.timeout > 0 {
		return o.timeout
	}
	if t.Startup > 0 {
		return t.Startup
	}
	if t.Kind == kindMachine {
		return machineTimeout
	}
	return defaultTimeout
}

// cmdList says what a selector expands to and touches nothing.
func cmdList(picked []target, o options) error {
	if o.asJSON {
		return printJSON(picked, o)
	}
	for _, t := range picked {
		fmt.Printf("%-20s %-10s %-9s %s\n", t.Name, t.Kind, t.Tier, t.Dialect)
	}
	return nil
}

// cmdDSN prints the URL a person pastes, running or not.
func cmdDSN(picked []target, o options) error {
	if o.asJSON {
		return printJSON(picked, o)
	}
	for _, t := range picked {
		fmt.Printf("%-20s %s\n", t.Name, t.URL)
	}
	return nil
}

func printJSON(picked []target, o options) error {
	var (
		out []byte
		err error
	)
	if o.namesOnly {
		// One line, which is what a GitHub Actions matrix expands with
		// fromJSON. See D69.
		names := make([]string, len(picked))
		for i, t := range picked {
			names[i] = t.Name
		}
		out, err = json.Marshal(names)
	} else {
		out, err = json.MarshalIndent(picked, "", "  ")
	}
	if err != nil {
		return fmt.Errorf("encoding the targets: %w", err)
	}
	fmt.Println(string(out))
	return nil
}

// withRunner does the commands that need the container runner.
func withRunner(ctx context.Context, command string, picked []target, o options) error {
	r, err := newRunner()
	if err != nil {
		return err
	}
	var failed []string
	for _, t := range picked {
		if err := one(ctx, r, command, t, o); err != nil {
			fmt.Printf("  %s: %v\n", t.Name, err)
			failed = append(failed, t.Name)
		}
	}
	if len(failed) != 0 {
		return fmt.Errorf("failed: %s", strings.Join(failed, " "))
	}
	return nil
}

func one(ctx context.Context, r runner, command string, t target, o options) error {
	switch command {
	case "status":
		return doStatus(ctx, r, t, o)
	case "start":
		return doStart(ctx, r, t, o)
	case "stop":
		return doStop(ctx, r, t)
	case "remove":
		return doRemove(ctx, r, t, o)
	case "logs":
		return doLogs(ctx, r, t, o)
	case "version":
		return doVersion(ctx, r, t)
	case "usql":
		return doUsql(ctx, r, t, o)
	case "test":
		return doTest(ctx, r, t, o)
	}
	return fmt.Errorf("no command called %q", command)
}

// doStatus says what is up, and for a machine says whether it answers.
//
// A running machine is not a machine that answers. The container is up for the
// whole hour Windows takes to install itself, and for every reboot after that,
// so printing a URL on the strength of the container prints one that refuses
// connections. A container needs no such probe, because start does not return
// until the server has answered.
func doStatus(ctx context.Context, r runner, t target, o options) error {
	if t.Kind == kindEmbedded {
		// A library has a file rather than a server, and the file is the
		// thing a person connects to, so status says where it is. The marker
		// trails the way the machine viewer does, because the name is the
		// fixed column a reader scans and everything after it is detail.
		note := "(embedded)"
		if _, err := os.Stat(t.DSN); err != nil {
			// Not an error. Nothing creates the file until something opens
			// it, so its absence is a state worth reporting rather than a
			// failure to report the state.
			note = "(embedded, no file yet)"
		}
		fmt.Printf("  %-20s %s  %s\n", t.Name, t.URL, note)
		return nil
	}
	if !r.running(ctx, t.Name) {
		return nil
	}
	if have, ok := r.hostPorts(ctx, t.Name); ok && !t.portsMatch(have) {
		fmt.Printf("  %-20s running on %v, and the list now says %v."+
			" Run: dbrun start %s\n", t.Name, have, t.wantPorts(), t.Name)
		return nil
	}
	if t.Kind == kindMachine {
		probe := statusProbe
		if o.timeout > 0 {
			probe = o.timeout
		}
		if !answered(ctx, t.DSN, probe) {
			fmt.Printf("  %-20s %-*s  screen http://127.0.0.1:%d\n",
				t.Name, len(t.URL), "starting, not answering yet", t.Viewer)
			return nil
		}
		fmt.Printf("  %-20s %s  screen http://127.0.0.1:%d\n", t.Name, t.URL, t.Viewer)
		return nil
	}
	fmt.Printf("  %-20s %s\n", t.Name, t.URL)
	return nil
}

// doStart brings a server up and leaves it there.
//
// A container that exists and is stopped is started rather than rebuilt, so
// that whatever was created in it survives. A machine is only ever started,
// because building one takes an hour and dbrun will not do that by accident.
func doStart(ctx context.Context, r runner, t target, o options) error {
	switch t.Kind {
	case kindEmbedded:
		// Nothing to start, and the file is made by whatever opens it. Print
		// the same line a server prints so a caller can use it either way.
		fmt.Printf("  %-20s embedded: %s=%s\n", t.Name, t.Env, t.DSN)
		return nil
	case kindMachine:
		if !r.exists(ctx, t.Name) {
			return fmt.Errorf("not provisioned. Run: dbrun provision %s, which takes about an hour", t.Name)
		}
	case kindContainer:
	}
	// A container built before a release was added to container.All keeps the
	// port it was given then, because a port is an index in that list. It
	// stays running and answers nothing on the port everything now computes,
	// which looks like a broken server rather than a stale one.
	if have, ok := r.hostPorts(ctx, t.Name); ok && !t.portsMatch(have) {
		if t.Kind == kindMachine {
			return fmt.Errorf(
				"it publishes %v and the list now says %v."+
					" Remove it and provision again, which takes about an hour",
				have, t.wantPorts())
		}
		fmt.Printf("  %-20s published %v and the list now says %v, rebuilding\n",
			t.Name, have, t.wantPorts())
		r.quiet(ctx, t.Remove...)
	}
	if r.running(ctx, t.Name) {
		fmt.Printf("  %-20s already up: %s=%s\n", t.Name, t.Env, t.DSN)
		return nil
	}
	// An image this repository builds is made here, so that start and test
	// both get it and neither caller has to remember.
	if err := ensureImage(ctx, r, t); err != nil {
		return err
	}
	if r.exists(ctx, t.Name) {
		if t.Kind == kindContainer {
			makeRoom(ctx, r, t.Name)
		}
		if !r.quiet(ctx, "start", t.Name) {
			if t.Kind == kindMachine {
				return errors.New("the machine would not start")
			}
			// A container that will not start is rebuilt, because it is a
			// minute. A machine is not, because it is an hour.
			r.quiet(ctx, t.Remove...)
		}
	}
	if !r.running(ctx, t.Name) && t.Kind == kindContainer {
		makeRoom(ctx, r, t.Name)
		if err := r.create(ctx, t); err != nil {
			return err
		}
	}
	if err := r.waitReady(ctx, t, t.timeout(o)); err != nil {
		if t.Kind == kindMachine {
			return fmt.Errorf("%w. Watch it at http://127.0.0.1:%d", err, t.Viewer)
		}
		return err
	}
	fmt.Printf("  %-20s up: %s=%s\n", t.Name, t.Env, t.DSN)
	return nil
}

func doStop(ctx context.Context, r runner, t target) error {
	if t.Kind == kindEmbedded {
		return nil
	}
	if !r.running(ctx, t.Name) {
		return nil
	}
	if !r.quiet(ctx, "stop", t.Name) {
		return errors.New("it would not stop")
	}
	fmt.Printf("  stopped %s\n", t.Name)
	return nil
}

// doRemove deletes a server. A machine is confirmed first, because rebuilding
// one is an hour and the command that deletes it is one letter from the one
// that stops it.
func doRemove(ctx context.Context, r runner, t target, o options) error {
	if t.Kind == kindEmbedded {
		// The file is the database, so this is what removing one means. It is
		// kept by everything else, including test, the same way a machine is.
		if err := os.Remove(t.DSN); err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("removing %s: %w", t.DSN, err)
		}
		fmt.Printf("  removed %s\n", t.DSN)
		return nil
	}
	if t.Kind == kindMachine && !o.yes {
		if !r.exists(ctx, t.Name) {
			return nil
		}
		ok, err := confirm(t.Name +
			" is a Windows machine and rebuilding it takes about an hour. Remove it?")
		if err != nil {
			return err
		}
		if !ok {
			fmt.Printf("  kept %s\n", t.Name)
			return nil
		}
	}
	if t.Kind == kindMachine {
		r.quiet(ctx, "rm", "--force", t.Name)
	} else {
		r.quiet(ctx, t.Remove...)
	}
	fmt.Printf("  removed %s\n", t.Name)
	return nil
}

func confirm(question string) (bool, error) {
	fmt.Printf("%s [y/N] ", question)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("reading the answer: %w", err)
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

// doLogs shows what the server said, which is the first thing somebody wants
// when one will not come up.
func doLogs(ctx context.Context, r runner, t target, o options) error {
	if t.Kind == kindEmbedded {
		fmt.Printf("  %-20s embedded, there is no log. The file is %s\n", t.Name, t.DSN)
		return nil
	}
	if !r.exists(ctx, t.Name) {
		return errors.New("there is no such container")
	}
	args := []string{"logs"}
	if o.follow {
		args = append(args, "--follow")
	}
	cmd := exec.CommandContext(ctx, r.name, append(args, t.Name)...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

// doUsql opens a shell on the server.
func doUsql(ctx context.Context, r runner, t target, _ options) error {
	if t.Kind != kindEmbedded && !r.running(ctx, t.Name) {
		return fmt.Errorf("it is not running. Start it with: dbrun start %s", t.Name)
	}
	if _, err := exec.LookPath("usql"); err != nil {
		return errors.New("usql is not on the path")
	}
	cmd := exec.CommandContext(ctx, "usql", t.URL)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}
