package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeRunner writes a runner that answers ps with the names given and
// inspect with a stopped container for each name it is asked about, and
// that writes a line to a log for every call. It returns the log.
func fakeRunner(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	script := `#!/bin/sh
echo "$*" >> ` + log + `
case "$1" in
ps) printf '%s\n' ` + strings.Join(names, " ") + ` ;;
inspect)
	shift
	printf '['
	sep=
	for n in "$@"; do
		printf '%s{"Name":"%s","State":{"Status":"exited"},"Config":{"Labels":{}},"NetworkSettings":{"Ports":{}}}' "$sep" "$n"
		sep=,
	done
	printf ']\n' ;;
*) exit 1 ;;
esac
`
	runner := filepath.Join(dir, "runner")
	if err := os.WriteFile(runner, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DBMETA_RUNNER", runner)
	return log
}

// calls reads the log of a fake runner, one call on each line.
func calls(t *testing.T, log string) []string {
	t.Helper()
	b, err := os.ReadFile(log)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return strings.Fields(strings.ReplaceAll(strings.TrimSpace(string(b)), " ", "_"))
}

// TestStatusCallsTheRunnerTwice holds the count that D122 asks for. A status
// of every target lists the containers once and inspects the ones that
// exist once, whatever the number of targets. It was several calls for each
// target, and ten seconds.
func TestStatusCallsTheRunnerTwice(t *testing.T) {
	log := fakeRunner(t, "qdrant-1.19.1", "chroma-1.5.9")
	credentialHome(t)
	if err := withRunner(t.Context(), "status", targets(), options{all: true}); err != nil {
		t.Fatal(err)
	}
	if got := calls(t, log); len(got) != 2 {
		t.Errorf("status called the runner %d times, and D122 holds it to two: %v", len(got), got)
	}
}

// TestActingOnAbsentServersCallsTheRunnerOnce holds that a command which
// acts, on servers that have no container, asks the runner only for the one
// listing. Removing 92 absent servers took eight seconds before D122.
func TestActingOnAbsentServersCallsTheRunnerOnce(t *testing.T) {
	log := fakeRunner(t)
	credentialHome(t)
	var picked []target
	for _, x := range targets() {
		if x.Kind == kindContainer {
			picked = append(picked, x)
		}
	}
	for _, command := range []string{"stop", "remove"} {
		if err := os.Remove(log); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if err := withRunner(t.Context(), command, picked, options{}); err != nil {
			t.Fatal(err)
		}
		if got := calls(t, log); len(got) != 1 {
			t.Errorf("%s of %d absent servers called the runner %d times: %v",
				command, len(picked), len(got), got)
		}
	}
}
