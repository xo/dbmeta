package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/xo/dbmeta/container"
	"github.com/xo/dbmeta/hosted"
)

// embeddedClients are the command line clients of the embedded databases. An
// embedded database has no server, so the client opens the file. A library
// that is not here has no such client.
var embeddedClients = map[string]string{
	"sqlite3":       "sqlite3",
	"moderncsqlite": "sqlite3",
	"duckdb":        "duckdb",
}

// serverOf finds the container entry behind a target, so that its native
// client is known. A machine has no entry of its own, so it takes the newest
// server of the same product.
func serverOf(t target) (container.Server, bool) {
	var found container.Server
	var ok bool
	for _, s := range container.All() {
		if s.Name() == t.Name {
			return s, true
		}
		if strings.ToLower(s.Product) == t.Product {
			found, ok = s, true
		}
	}
	return found, ok
}

// nativeCall builds the call of the native client of a target, as the
// principal named by role. An empty role is the administrator. It never runs
// anything and it never prints the password.
func nativeCall(t target, role string) (container.Call, error) {
	if t.Kind == kindHosted {
		return hostedCall(t, role)
	}
	if t.Kind == kindEmbedded {
		command, ok := embeddedClients[t.Name]
		if !ok {
			return container.Call{}, fmt.Errorf("%s has no native client, because it is a Go library."+
				" Use: dbrun usql %s", t.Name, t.Name)
		}
		return container.Call{Command: command, Args: []string{t.DSN}}, nil
	}
	s, ok := serverOf(t)
	if !ok {
		return container.Call{}, fmt.Errorf("%s is not in the container list", t.Name)
	}
	if s.Client.Command == "" {
		return container.Call{}, fmt.Errorf("%s has no native client, because %s. Use: dbrun usql %s",
			t.Product, s.Client.Why, t.Name)
	}
	raw := t.URL
	switch {
	case role != "" && len(t.Principals) == 0:
		return container.Call{}, fmt.Errorf("%s has one user, and --as %s names another", t.Name, role)
	case role != "":
		raw = ""
		for _, p := range t.Principals {
			if p.Role == role || p.User == role {
				raw = p.URL
				break
			}
		}
		if raw == "" {
			return container.Call{}, fmt.Errorf("%s has no principal called %q. Try: dbrun dsn --json %s",
				t.Name, role, t.Name)
		}
	case len(t.Principals) > 0:
		raw = t.Principals[0].URL
	}
	first := 0
	if p, err := strconv.Atoi(t.wantPorts()[strconv.Itoa(s.Port)]); err == nil {
		first = p
	}
	conn, err := container.ParseConnection(raw, first)
	if err != nil {
		return container.Call{}, fmt.Errorf("reading the URL of %s: %w", t.Name, err)
	}
	return s.Client.Call(conn), nil
}

// shellWord quotes an argument for printing, when it needs it.
func shellWord(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\"'$&|;<>(){}*?!\\`") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// describeCall is the call as a person reads it, with the password masked.
// It names the environment variables and never their values.
func describeCall(c container.Call) string {
	words := []string{c.Command}
	for _, a := range c.Masked() {
		words = append(words, shellWord(a))
	}
	out := strings.Join(words, " ")
	if names := c.EnvNames(); len(names) > 0 {
		out += "\n  with the environment variables " + strings.Join(names, " ")
	}
	return out
}

// findClient looks the client up on the path of the host. If the host has
// none, the error names the command and the product, and offers usql, which
// dbrun can always use.
func findClient(t target, command string) (string, error) {
	path, err := exec.LookPath(command)
	if err != nil {
		return "", fmt.Errorf("%s is not on the path, and it is the native client of %s."+
			" Use: dbrun usql %s", command, t.Product, t.Name)
	}
	return path, nil
}

// doCLI runs the native client of the server, attached to the terminal.
//
// It starts nothing, the way usql starts nothing. The server must be running.
// The password goes in the environment of the client where the client reads it
// from there, and in its arguments where it does not, and then a note says so.
// See D236.
func doCLI(ctx context.Context, r runner, t target, o options) error {
	call, err := nativeCall(t, o.as)
	if err != nil {
		return err
	}
	if o.printOnly {
		fmt.Println(describeCall(call))
		return nil
	}
	if t.Kind == kindContainer || t.Kind == kindMachine {
		if !r.running(ctx, t.Name) {
			return fmt.Errorf("it is not running. Start it with: dbrun start %s", t.Name)
		}
	}
	if t.Kind == kindEmbedded {
		if err := extractSamples(t); err != nil {
			return err
		}
	}
	path, err := findClient(t, call.Command)
	if err != nil {
		return err
	}
	if call.InArgs() {
		fmt.Fprintf(os.Stderr, "dbrun: %s takes the password only as an argument,"+
			" so a process list shows it while the client runs\n", call.Command)
	}
	args := append(append([]string(nil), call.Args...), o.extra...)
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = append(os.Environ(), call.Env...)
	if err := cmd.Run(); err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			return fmt.Errorf("%s ended with status %d", call.Command, exit.ExitCode())
		}
		return fmt.Errorf("running %s: %w", call.Command, err)
	}
	return nil
}

// hostedCall builds the call for a hosted service from its connection
// string, which is read in memory and never printed. The password goes only in
// the environment of the client. No error here quotes the connection string.
//
// dbrun resolves one credential for each service and no second one for a
// principal, so --as is not supported for a hosted service. A person who wants
// another user puts it in the connection string of the service.
func hostedCall(t target, role string) (container.Call, error) {
	if role != "" {
		return container.Call{}, fmt.Errorf("--as is not supported for the hosted service %s,"+
			" because dbrun resolves one credential for it. Put the user in that credential", t.Name)
	}
	var client container.Client
	for _, s := range hosted.All() {
		if s.Name == t.Name {
			client = s.Client
		}
	}
	if client.Command == "" {
		return container.Call{}, fmt.Errorf("%s has no native client, because %s. Use: dbrun usql %s",
			t.Name, client.Why, t.Name)
	}
	conn, err := container.ParseConnection(t.secret, 0)
	if err != nil {
		return container.Call{}, fmt.Errorf("reading the credential of %s: %w", t.Name, err)
	}
	return client.Call(conn), nil
}
