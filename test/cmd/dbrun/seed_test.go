package main

import (
	"strings"
	"testing"
)

// TestASeededTargetMountsItsDirectory holds that the mount goes before the
// image and that only the golden directory is handed to the user in the image.
func TestASeededTargetMountsItsDirectory(t *testing.T) {
	t.Parallel()
	x := target{Seed: "/opt/oracle/oradata", Run: []string{"run", "--detach", "--name", "oracle-19c", "image"}}
	live := withSeed(x, "/state/live", false)
	if got, want := strings.Join(live.Run, " "), "run --detach --volume /state/live:/opt/oracle/oradata --name oracle-19c image"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	golden := withSeed(x, "/state/golden", true)
	if !strings.Contains(strings.Join(golden.Run, " "), "/state/golden:/opt/oracle/oradata:U") {
		t.Errorf("the golden mount does not hand the directory over: %v", golden.Run)
	}
	if len(x.Run) != 5 {
		t.Errorf("withSeed changed the arguments of the target it was given: %v", x.Run)
	}
}

// TestOnlyPodmanKeepsASeed holds that docker builds the database in the
// container, because its copy cannot keep the owner of a file.
func TestOnlyPodmanKeepsASeed(t *testing.T) {
	t.Parallel()
	x := target{Kind: kindContainer, Seed: "/opt/oracle/oradata"}
	if !(runner{name: "podman"}).seeded(x) || (runner{name: "docker"}).seeded(x) {
		t.Error("podman must keep a seed and docker must not")
	}
	if (runner{name: "podman"}).seeded(target{Kind: kindContainer}) {
		t.Error("a target with no seed was seeded")
	}
}

// TestOracle19KeepsItsDatabase holds that the one slow release is the one
// that has a seed, and that nothing else does.
func TestOracle19KeepsItsDatabase(t *testing.T) {
	t.Parallel()
	for _, x := range targets() {
		if want := x.Name == "oracle-19c"; (x.Seed != "") != want {
			t.Errorf("%s has the seed %q", x.Name, x.Seed)
		}
	}
}

// TestAContainerRunsOnTheSmallImage holds that the swap replaces the image and
// leaves the target it was given alone.
func TestAContainerRunsOnTheSmallImage(t *testing.T) {
	t.Parallel()
	x := target{Ref: "big:1", SeedImage: "small:1", Run: []string{"run", "--detach", "big:1"}}
	if got := withRunImage(x).Run[2]; got != "small:1" {
		t.Errorf("the image is %q, and the expected value is small:1", got)
	}
	if x.Run[2] != "big:1" {
		t.Error("withRunImage changed the target it was given")
	}
	y := target{Ref: "big:1", Run: []string{"run", "--detach", "big:1"}}
	if got := withRunImage(y).Run[2]; got != "big:1" {
		t.Errorf("a server with no small image runs on %q", got)
	}
}
