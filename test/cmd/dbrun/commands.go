package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"
)

// defaultTimeout is how long a container gets to answer. A machine carries
// its own, in container.Machine.Startup, because Windows boots for minutes
// before SQL Server listens and an appliance does not.
const (
	defaultTimeout = 90 * time.Second
	// How long one readiness check waits for a machine to answer, while
	// start waits for it.
	machineProbe = 20 * time.Second
	// How long status waits for a machine to answer. It is short because
	// status is a question about what is there, not a wait for it to arrive.
	statusProbe = 5 * time.Second
)

// maxRunning is how many of these servers can be up at once.
//
// Each one is bounded to container.MemoryLimit, so the ceiling is that times
// this, and the rest of the machine is left alone. Without a cap a session
// that starts a server per question ends with a dozen up, all idle, and the
// host in swap.
//
// Starting one when this many are already up stops the one that is running
// longest. That is the right one to lose: the server in use is the
// one most recently started, and starting is a minute for a container.
//
// It was 4 until Ken raised it to 8 (D108). Eight at 4 GB is 32 GB, and the
// machine has 60.
const maxRunning = 8

// makeRoom stops the longest running servers of the caller until starting
// one more keeps the count at or below maxRunning.
//
// It only ever stops a container this project knows about, and never the one
// being started. A machine is left alone: stopping Windows mid install is
// how an hour is lost, and D57 keeps a machine for that reason.
//
// It stops only the caller's own servers. When none of the running servers
// is the caller's it refuses rather than stop another session's server, and
// --force stops the oldest anyway. See D98.
func makeRoom(ctx context.Context, r runner, keep string, o options) error {
	known := map[string]bool{}
	for _, t := range targets() {
		if t.Kind == kindContainer && t.Name != keep {
			known[t.Name] = true
		}
	}
	names := r.runningSince(ctx, known)
	// One inspect reads the owner of every running server, rather than
	// three calls for each.
	r.load(ctx, names)
	up := make([]holder, len(names))
	for i, name := range names {
		up[i] = holder{name: name, owner: r.owner(ctx, name), who: r.who(ctx, name)}
	}
	me := currentOwner()
	for len(up) >= maxRunning {
		name, err := pickEvictee(up, me, o.force)
		if err != nil {
			return err
		}
		up = slices.DeleteFunc(up, func(h holder) bool { return h.name == name })
		if !r.quiet(ctx, "stop", name) {
			continue
		}
		fmt.Printf("  %-*s stopped to stay within %d running\n", nameWidth(), name, maxRunning)
	}
	return nil
}

func (t target) timeout(o options) time.Duration {
	if o.timeout > 0 {
		return o.timeout
	}
	if t.Startup > 0 {
		return t.Startup
	}
	return defaultTimeout
}

// cmdList says what a selector expands to and touches nothing.
func cmdList(picked []target, o options) error {
	if o.asJSON {
		return printJSON(picked, o)
	}
	for _, t := range picked {
		// A server with no dialect yet ends the row at its tier, with no
		// blanks after it.
		fmt.Println(strings.TrimRight(fmt.Sprintf("%-*s %-10s %-9s %s", nameWidth(), t.Name, t.Kind, t.Tier, t.Dialect), " "))
	}
	return nil
}

// cmdDSN prints the URL a person pastes, running or not. The secret of a
// hosted service is masked unless --reveal asks for it (D117).
func cmdDSN(picked []target, o options) error {
	if o.reveal {
		for i, t := range picked {
			if t.Kind == kindHosted {
				picked[i].DSN, picked[i].URL = t.secret, t.secret
			}
		}
	}
	if o.asJSON {
		return printJSON(picked, o)
	}
	for _, t := range picked {
		fmt.Printf("%-*s %s\n", nameWidth(), t.Name, t.URL)
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
	// One listing says which targets have no container, which is most of
	// them. status and version only read, so one inspect of the rest answers
	// them too. A command that acts inspects each one it reaches.
	readOnly := command == "status" || command == "version"
	if r, err = r.remember(ctx, picked, readOnly); err != nil {
		return err
	}
	if o.asJSON {
		switch command {
		case "status":
			return statusJSON(ctx, r, picked, o.all)
		case "version":
			return versionJSON(ctx, r, picked)
		}
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
		return doStop(ctx, r, t, o)
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
	if t.Kind == kindHosted {
		// A hosted service is a target only while its connection string
		// resolves, so status shows it with where the string came from. It
		// is not probed, because the drivers of the hosted services are not
		// in the test module. See D117.
		fmt.Printf("  %-*s %s  (hosted, %s)\n", nameWidth(), t.Name, t.URL, t.Credential)
		return nil
	}
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
		fmt.Printf("  %-*s %s  %s\n", nameWidth(), t.Name, t.URL, note)
		return nil
	}
	if !r.running(ctx, t.Name) {
		// A stopped server is shown only when asked for, the way podman ps
		// -a shows it, with who made it. See D115.
		if o.all && r.exists(ctx, t.Name) {
			fmt.Printf("  %-*s %s  (stopped, %s)\n", nameWidth(), t.Name, t.URL, r.who(ctx, t.Name))
		}
		return nil
	}
	if have, ok := r.hostPorts(ctx, t.Name); ok && !t.portsMatch(have) {
		fmt.Printf("  %-*s running on %v, and the list now says %v."+
			" Run: dbrun start %s\n", nameWidth(), t.Name, have, t.wantPorts(), t.Name)
		return nil
	}
	if t.Kind == kindMachine {
		probe := statusProbe
		if o.timeout > 0 {
			probe = o.timeout
		}
		if !answered(ctx, t, probe) {
			fmt.Printf("  %-*s %-*s  screen http://127.0.0.1:%d\n", nameWidth(),
				t.Name, len(t.URL), "starting, not answering yet", t.Viewer)
			return nil
		}
		fmt.Printf("  %-*s %s  screen http://127.0.0.1:%d%s\n", nameWidth(),
			t.Name, t.URL, t.Viewer, whose(ctx, r, t))
		return nil
	}
	fmt.Printf("  %-*s %s%s\n", nameWidth(), t.Name, t.URL, whose(ctx, r, t))
	return nil
}

// whose says who started a running server, for the text form of status. The
// caller's own servers say so too, so that a glance tells which ones a
// session can stop. See D98.
func whose(ctx context.Context, r runner, t target) string {
	switch owner := r.owner(ctx, t.Name); owner {
	case currentOwner():
		return "  (yours)"
	case "":
		return "  (no owner)"
	default:
		return "  (" + r.who(ctx, t.Name) + ")"
	}
}

// startLine prints what start says about one server: its state, the
// variable the tests read with its DSN, and the URL that usql takes, which is
// what a person pastes. A hosted service's DSN and URL are masked (D117).
func startLine(t target, state, dsn, suffix string) {
	line := fmt.Sprintf("  %-*s %s: %s=%s", nameWidth(), t.Name, state, t.Env, dsn)
	if t.URL != "" {
		line += "  usql: " + t.URL
	}
	fmt.Println(line + suffix)
}

// doStart brings a server up and leaves it there.
//
// A container that exists and is stopped is started rather than rebuilt, so
// that whatever was created in it survives. A machine is only ever started,
// because building one takes an hour and dbrun will not do that by accident.
func doStart(ctx context.Context, r runner, t target, o options) error {
	switch t.Kind {
	case kindHosted:
		// Nothing to start. The service runs somewhere else.
		startLine(t, "hosted", t.DSN, "  ("+t.Credential+")")
		return nil
	case kindEmbedded:
		// Nothing to start, and the file is made by whatever opens it. A
		// database that starts from sample files gets them. Print the same
		// line a server prints so a caller can use it either way.
		if err := extractSamples(t); err != nil {
			return err
		}
		startLine(t, "embedded", t.DSN, "")
		return nil
	case kindMachine:
		if !r.exists(ctx, t.Name) {
			return fmt.Errorf("not provisioned. Run: dbrun provision %s", t.Name)
		}
	case kindContainer:
	}
	// A container built before a release was added to container.All keeps the
	// port it was given then, because a port is an index in that list. It
	// stays running and answers nothing on the port everything now computes,
	// which looks like a broken server rather than a stale one.
	me := currentOwner()
	owner := r.owner(ctx, t.Name)
	if have, ok := r.hostPorts(ctx, t.Name); ok && !t.portsMatch(have) {
		if !claimable(t, owner, me, o.force, r.running(ctx, t.Name)) {
			return fmt.Errorf("it publishes %v and the list now says %v, and %w",
				have, t.wantPorts(), notYours(r.who(ctx, t.Name)))
		}
		if t.Kind == kindMachine {
			return fmt.Errorf(
				"it publishes %v and the list now says %v."+
					" Remove it and provision again, and %s",
				have, t.wantPorts(), t.Rebuild)
		}
		fmt.Printf("  %-*s published %v and the list now says %v, rebuilding\n", nameWidth(),
			t.Name, have, t.wantPorts())
		r.quiet(ctx, t.Remove...)
	}
	// A server that is already up is shared, whoever started it, so two
	// sessions that test one release use one server. Only its owner stops it.
	//
	// A running container is not always a server that answers. Oracle 19c
	// creates its database on the first start, which takes longer than the
	// default timeout. A test started again against it found the listener
	// up and the database not yet open. So the ready check runs here too,
	// and it costs one check on a server that is ready. A settle is for a
	// server that has just begun to answer, so it is not waited out again.
	if r.running(ctx, t.Name) {
		note := ""
		if owner != me {
			note = ", started by " + r.who(ctx, t.Name)
		}
		up := t
		up.Settle = 0
		if err := r.waitReady(ctx, up, t.timeout(o)); err != nil {
			return fmt.Errorf("it is running and not ready: %w", err)
		}
		startLine(t, "already up"+note, t.connectDSN(), "")
		return nil
	}
	// A stopped container belongs to nobody. Its owner can be a session that
	// ended, or one whose server stopped when the computer restarted, and a
	// refusal sent the next agent to an older release. A label cannot change
	// on a container that exists, so it is created again under the caller.
	// A machine keeps its owner, because it takes an hour to create. See D108.
	if r.exists(ctx, t.Name) && !mayTouch(owner, me, o.force) {
		if t.Kind == kindMachine {
			return fmt.Errorf("it is stopped, and %w", notYours(r.who(ctx, t.Name)))
		}
		fmt.Printf("  %-*s stopped, and created by %s. Creating it again as yours\n", nameWidth(),
			t.Name, r.who(ctx, t.Name))
		if !r.quiet(ctx, t.Remove...) {
			return errors.New("the stopped container was not removed")
		}
	}
	// An image this repository builds is made here, so that start and test
	// both get it and neither caller has to remember.
	if err := ensureImage(ctx, r, t); err != nil {
		return err
	}
	if r.exists(ctx, t.Name) {
		if t.Kind == kindContainer {
			if err := makeRoom(ctx, r, t.Name, o); err != nil {
				return err
			}
		}
		if !r.quiet(ctx, "start", t.Name) {
			if t.Kind == kindMachine {
				return errors.New("the machine did not start")
			}
			// A container that will not start is rebuilt, because it is a
			// minute. A machine is not, because it is an hour.
			r.quiet(ctx, t.Remove...)
		}
	}
	if !r.running(ctx, t.Name) && t.Kind == kindContainer {
		if err := makeRoom(ctx, r, t.Name, o); err != nil {
			return err
		}
		if err := r.create(ctx, t); err != nil {
			return err
		}
	}
	if err := r.waitReady(ctx, t, t.timeout(o)); err != nil {
		if t.Kind == kindMachine {
			return fmt.Errorf("%w. Watch it at http://127.0.0.1:%d", err, t.Viewer)
		}
		// The log says why. Oracle 21c answers in under a minute on a
		// GitHub runner, and once it did not answer in five, and the error
		// alone did not say whether the database was slow or stuck.
		if out, lerr := r.output(ctx, "logs", "--tail", "20", t.Name); lerr == nil && out != "" {
			return fmt.Errorf("%w\n  the last lines of its log:\n%s", err, lastLines(out, 20))
		}
		return err
	}
	// A server that answers is not always a server with a catalog. Hive's
	// is installed here, after it is ready and before anything reads it.
	if len(t.Init) > 0 {
		if err := r.install(ctx, t, initPause); err != nil {
			return err
		}
	}
	startLine(t, "up", t.connectDSN(), "")
	for _, e := range t.AlsoEnv {
		fmt.Printf("  %-*s also: %s=%s\n", nameWidth(), "", e, t.connectDSN())
	}
	return nil
}

// initAttempts is how many times install runs a server's Init before it
// gives up, and initPause is the wait between two attempts.
const (
	initAttempts = 3
	initPause    = 10 * time.Second
)

// install runs a server's Init, and runs it again when it fails.
//
// Every Init runs on every start, so each one is already safe to run twice.
// Hive's failed twice in CI, soon after HiveServer2 first answered: once on a
// SerDeException and once on a ParseException in a script that does not
// change. Neither failed on a development machine in three fresh starts. A
// GitHub runner is a slower machine, and a server that has just begun to
// answer can still refuse a statement there.
//
// The output of each failure is printed, because the exit status alone says
// nothing. The first Hive failure was "exit status 2", with no way to tell
// which statement beeline refused.
func (r runner) install(ctx context.Context, t target, pause time.Duration) error {
	var err error
	for attempt := 1; attempt <= initAttempts; attempt++ {
		var out string
		if out, err = r.outputIn(ctx, t.InitInput, t.Init...); err == nil {
			return nil
		}
		err = fmt.Errorf("installing the catalog: %w\n%s", err, lastLines(out, 10))
		if attempt == initAttempts {
			break
		}
		fmt.Printf("  %-*s attempt %d of %d failed, trying again in %s: %v\n", nameWidth(),
			t.Name, attempt, initAttempts, pause, err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pause):
		}
	}
	return err
}

func doStop(ctx context.Context, r runner, t target, o options) error {
	if t.Kind == kindEmbedded || t.Kind == kindHosted {
		return nil
	}
	if !r.running(ctx, t.Name) {
		return nil
	}
	if owner := r.owner(ctx, t.Name); !mayTouch(owner, currentOwner(), o.force) {
		return notYours(r.who(ctx, t.Name))
	}
	if !r.quiet(ctx, "stop", t.Name) {
		return errors.New("it did not stop")
	}
	fmt.Printf("  stopped %s\n", t.Name)
	return nil
}

// doRemove deletes a server. A machine is confirmed first, because rebuilding
// one is an hour and the command that deletes it is one letter from the one
// that stops it.
func doRemove(ctx context.Context, r runner, t target, o options) error {
	if t.Kind == kindHosted {
		// A hosted service is not dbrun's to remove.
		fmt.Printf("  %-*s hosted, there is nothing to remove\n", nameWidth(), t.Name)
		return nil
	}
	if t.Kind == kindEmbedded {
		// The file is the database, so this is what removing one means. It is
		// kept by everything else, including test, the same way a machine is.
		// chai and csvq are a directory, which goes with what it holds.
		remove := os.Remove
		if t.Directory {
			remove = os.RemoveAll
		}
		if err := remove(t.DSN); err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("removing %s: %w", t.DSN, err)
		}
		fmt.Printf("  removed %s\n", t.DSN)
		return nil
	}
	if owner := r.owner(ctx, t.Name); !claimable(t, owner, currentOwner(), o.force, r.running(ctx, t.Name)) {
		return notYours(r.who(ctx, t.Name))
	}
	if t.Kind == kindMachine && !o.yes {
		if !r.exists(ctx, t.Name) {
			return nil
		}
		ok, err := confirm(t.Name + " is a machine and " + t.Rebuild + ". Remove it?")
		if err != nil {
			return err
		}
		if !ok {
			fmt.Printf("  kept %s\n", t.Name)
			return nil
		}
	}
	// A container that is not there has nothing to remove, and saying
	// removed is false. It costs no call, because the listing already said
	// so.
	if !r.exists(ctx, t.Name) {
		return nil
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
	if t.Kind == kindHosted {
		fmt.Printf("  %-*s hosted, there is no log here\n", nameWidth(), t.Name)
		return nil
	}
	if t.Kind == kindEmbedded {
		fmt.Printf("  %-*s embedded, there is no log. The file is %s\n", nameWidth(), t.Name, t.DSN)
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
	if t.Kind == kindHosted {
		return usqlHosted(ctx, t)
	}
	if t.Kind != kindEmbedded && !r.running(ctx, t.Name) {
		return fmt.Errorf("it is not running. Start it with: dbrun start %s", t.Name)
	}
	if t.Kind == kindEmbedded {
		if err := extractSamples(t); err != nil {
			return err
		}
	}
	if _, err := exec.LookPath("usql"); err != nil {
		return errors.New("usql is not on the path")
	}
	cmd := exec.CommandContext(ctx, "usql", t.URL)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}
