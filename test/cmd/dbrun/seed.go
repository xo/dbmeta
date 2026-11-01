package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// seedBuildTimeout is how long the first start of a seeded server has to
// finish building its database. Oracle 19c runs DBCA then, and DBCA takes
// between a quarter of an hour and half an hour.
const seedBuildTimeout = 45 * time.Minute

// seedReady is the line the Oracle entrypoint prints when the database is
// built, its files are moved to the data directory and it is open. The
// readiness check passes earlier than that, while the files are still being
// moved, and a copy taken then holds no spfile.
const seedReady = "DATABASE IS READY TO USE!"

// seedDirs is where a seeded server keeps its database on the host: the
// golden copy, which no container ever runs on, and the copy the current
// container runs on.
type seedDirs struct {
	golden, live, done string
}

func seedPlace(name string) seedDirs {
	root := filepath.Join(stateDir("DBMETA_SEED_STATE", "seed"), name)
	return seedDirs{
		golden: filepath.Join(root, "golden"),
		live:   filepath.Join(root, "live"),
		done:   filepath.Join(root, "golden.done"),
	}
}

// seeded reports whether start keeps the database of a target and copies it.
//
// Only podman does. The copy runs in the user namespace of podman, which is
// what lets it keep the owner of every file, and docker has no such thing.
// A target that is not seeded builds its database in the container, as it
// always did.
func (r runner) seeded(t target) bool {
	return t.Kind == kindContainer && t.Seed != "" && r.podman()
}

// host runs a command on the host, in the user namespace of podman. The files
// of a seeded database belong to a user id that only that namespace maps, so
// the same command run plainly fails to read, copy or delete them.
func (r runner) host(ctx context.Context, argv ...string) error {
	cmd := exec.CommandContext(ctx, r.name, append([]string{"unshare"}, argv...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("running %s: %w: %s", strings.Join(argv, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// withSeed returns the target with the directory mounted over the path the
// database lives in. The first two arguments are run and --detach, and a flag
// goes anywhere before the image.
func withSeed(t target, dir string, own bool) target {
	spec := dir + ":" + t.Seed
	if own {
		// The golden directory is made by this user, so the user in the
		// image cannot write it until podman gives it away.
		spec += ":U"
	}
	t.Run = slices.Insert(slices.Clone(t.Run), 2, "--volume", spec)
	return t
}

// prepareSeed makes the container for a seeded target start on a fresh copy
// of the kept database, and builds the kept database first when there is
// none. See D248.
func prepareSeed(ctx context.Context, r runner, t target, o options) (target, error) {
	d := seedPlace(t.Name)
	if _, err := os.Stat(d.done); errors.Is(err, os.ErrNotExist) {
		if err := buildSeed(ctx, r, t, o, d); err != nil {
			return t, err
		}
	}
	if err := r.host(ctx, "rm", "-rf", d.live); err != nil {
		return t, err
	}
	// --reflink=auto makes a copy that shares its data with the golden copy
	// on btrfs, which takes no time. Anywhere else it is a plain copy, which
	// is slow and still right.
	if err := r.host(ctx, "cp", "-a", "--reflink=auto", d.golden, d.live); err != nil {
		return t, fmt.Errorf("copying the kept database of %s: %w", t.Name, err)
	}
	if err := ensureRunImage(ctx, r, t); err != nil {
		return t, err
	}
	return withRunImage(withSeed(t, d.live, false)), nil
}

// withRunImage returns the target with the smaller image that a container
// runs on once the seed exists. It changes nothing when the server has none.
func withRunImage(t target) target {
	if t.SeedImage == "" {
		return t
	}
	t.Run = slices.Clone(t.Run)
	if i := slices.Index(t.Run, t.Ref); i >= 0 {
		t.Run[i] = t.SeedImage
	}
	return t
}

// ensureRunImage builds the smaller image when it is missing.
func ensureRunImage(ctx context.Context, r runner, t target) error {
	if t.SeedImage == "" || r.imageExists(ctx, t.SeedImage) {
		return nil
	}
	fmt.Printf("  building %s\n", t.SeedImage)
	return buildRunImage(ctx, r, t)
}

// buildRunImage builds the smaller image from the one that builds the seed.
//
// The removals are squashed into one layer, because a file deleted in a later
// layer still takes its place in an earlier one. The build copies nothing in,
// so the context is an empty directory.
func buildRunImage(ctx context.Context, r runner, t target) error {
	b, err := images.ReadFile("image/oracle19/slim.Containerfile")
	if err != nil {
		return fmt.Errorf("reading the Containerfile of %s: %w", t.SeedImage, err)
	}
	dir, err := os.MkdirTemp("", "dbmeta-slim-")
	if err != nil {
		return fmt.Errorf("making a build directory: %w", err)
	}
	defer os.RemoveAll(dir)
	file := filepath.Join(dir, "Containerfile")
	if err := os.WriteFile(file, b, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", file, err)
	}
	cmd := exec.CommandContext(ctx, r.name, "build", "--squash-all", "--tag", t.SeedImage, "--file", file, dir)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("building %s: %w", t.SeedImage, err)
	}
	return nil
}

// buildSeed starts the server on an empty directory, waits until the database
// is built, stops it cleanly and keeps the directory. A clean stop matters:
// a copy of a database that was killed makes every container that starts on
// it run instance recovery.
func buildSeed(ctx context.Context, r runner, t target, o options, d seedDirs) error {
	fmt.Printf("  %-*s building the kept database. This happens once, and takes up to %s\n",
		nameWidth(), t.Name, seedBuildTimeout)
	if err := r.host(ctx, "rm", "-rf", d.golden); err != nil {
		return err
	}
	if err := os.MkdirAll(d.golden, 0o755); err != nil {
		return fmt.Errorf("making %s: %w", d.golden, err)
	}
	build := withSeed(t, d.golden, true)
	failed := t.Name + "-seedfail"
	r.quiet(ctx, "rm", "--force", failed)
	built := false
	defer func() {
		if built {
			r.quiet(ctx, t.Remove...)
			return
		}
		// A build that failed costs a quarter of an hour, so the container
		// stays under another name for its logs and its alert log.
		if r.quiet(ctx, "rename", t.Name, failed) {
			fmt.Printf("  %-*s kept the failed build as %s. Read it with: %s logs %s\n",
				nameWidth(), t.Name, failed, r.name, failed)
		}
	}()
	if err := r.create(ctx, build); err != nil {
		return err
	}
	if err := r.waitLog(ctx, t.Name, seedReady, max(o.timeout, seedBuildTimeout)); err != nil {
		if out, lerr := r.output(ctx, "logs", "--tail", "20", t.Name); lerr == nil && out != "" {
			err = fmt.Errorf("%w\n  the last lines of its log:\n%s", err, lastLines(out, 20))
		}
		return fmt.Errorf("building the kept database of %s: %w", t.Name, err)
	}
	if out, err := r.output(ctx, "stop", "--time", "180", t.Name); err != nil {
		return fmt.Errorf("stopping %s to keep its database: %w: %s", t.Name, err, out)
	}
	built = true
	if err := os.WriteFile(d.done, []byte(time.Now().UTC().Format(time.RFC3339)+"\n"), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", d.done, err)
	}
	return nil
}

// waitLog waits until a container has printed a line.
func (r runner) waitLog(ctx context.Context, name, line string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if out, _ := r.output(ctx, "logs", name); strings.Contains(out, line) {
			return nil
		}
		if r.state(ctx, name) == "exited" {
			return errors.New("the container stopped before it said the database was ready")
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("it never said %q in %s", line, timeout)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("waiting for %s: %w", name, ctx.Err())
		case <-time.After(5 * time.Second):
		}
	}
}

// dropSeed deletes what a seeded target keeps. The copy a container ran on
// always goes with the container. The golden copy goes only when asked,
// because it is the quarter of an hour.
func dropSeed(ctx context.Context, r runner, t target, golden bool) error {
	if !r.seeded(t) {
		return nil
	}
	d := seedPlace(t.Name)
	paths := []string{d.live}
	if golden {
		paths = append(paths, d.golden, d.done)
	}
	return r.host(ctx, append([]string{"rm", "-rf"}, paths...)...)
}
