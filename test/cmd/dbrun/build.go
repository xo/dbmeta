package main

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Two images are built here rather than pulled, and the reason differs.
//
// Cassandra's published image refuses three things the queries read: a user
// defined function, a materialized view and a role. The Containerfile beside
// this turns them on. See D62.
//
// Oracle publishes no free 19c image at all. It publishes Dockerfiles and an
// installer archive that a person downloads once under the developer licence,
// and the image is built from those.
//
// Both were shell scripts. The orchestration is Go now, because it is the
// part with the digest check and the error handling in it. What stays shell
// is Oracle's own build script, which is theirs and which we call.

// buildSpec says how to make one image.
type buildSpec struct {
	// Ref is the image this produces, which is what container names.
	Ref string
	// Build makes it.
	Build func(context.Context, runner, target) error
}

// buildFor returns how to build this target's image, and whether it needs
// building at all. Most targets pull a published image and need none.
func buildFor(t target) (buildSpec, bool) {
	switch {
	case t.Product == "cassandra":
		return buildSpec{
			Ref:   "localhost/dbmeta/cassandra:" + t.Release,
			Build: buildCassandra,
		}, true
	case t.Product == "oracle" && t.Release == "19.3.0":
		return buildSpec{
			Ref:   "localhost/oracle/database:19.3.0-ee",
			Build: buildOracle19c,
		}, true
	}
	return buildSpec{}, false
}

// ensureImage builds the image when it is missing, and says nothing when it
// is not needed.
func ensureImage(ctx context.Context, r runner, t target) error {
	spec, needed := buildFor(t)
	if !needed || r.imageExists(ctx, spec.Ref) {
		return nil
	}
	fmt.Printf("  building %s\n", spec.Ref)
	return spec.Build(ctx, r, t)
}

// cmdBuild builds an image whether or not it is already there, which is what
// somebody wants after changing a Containerfile.
func cmdBuild(ctx context.Context, picked []target) error {
	r, err := newRunner()
	if err != nil {
		return err
	}
	var built int
	for _, t := range picked {
		spec, needed := buildFor(t)
		if !needed {
			continue
		}
		fmt.Printf("=== %s ===\n", spec.Ref)
		if err := spec.Build(ctx, r, t); err != nil {
			return err
		}
		built++
	}
	if built == 0 {
		return errors.New("none of those has an image to build." +
			" Only Cassandra and Oracle 19c do")
	}
	return nil
}

// cassandraContainerfile is built into dbrun, so that the command works from
// any directory and a person who has the binary has the build.
//
//go:embed image/cassandra.Containerfile
var cassandraContainerfile []byte

// buildCassandra turns on what the published image refuses.
//
// The Containerfile edits cassandra.yaml and then checks the result, because
// the key names changed between releases and a sed that matches nothing
// changes nothing and says so to nobody.
//
// It copies nothing in, so the build context is an empty directory and the
// Containerfile written into it is the whole input.
func buildCassandra(ctx context.Context, r runner, t target) error {
	spec, _ := buildFor(t)
	dir, err := os.MkdirTemp("", "dbmeta-cassandra-")
	if err != nil {
		return fmt.Errorf("making a build directory: %w", err)
	}
	defer os.RemoveAll(dir)
	file := filepath.Join(dir, "Containerfile")
	if err := os.WriteFile(file, cassandraContainerfile, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", file, err)
	}
	return r.build(ctx, spec.Ref, file, dir,
		map[string]string{"RELEASE": t.Release})
}

// oracle19c is what the build needs from the person running it.
const (
	oracle19cArchive = "LINUX.X64_193000_db_home.zip"
	// The digest Oracle publishes for that archive. It is three gigabytes that
	// the build then runs as root inside an image, so it is worth knowing it
	// is the file Oracle shipped and that it arrived whole. That matters more
	// now that the archive can come from a mirror than it did when it only
	// ever arrived by hand.
	oracle19cSHA256 = "ba8329c757133da313ed3b6d7f86c5ac42cd9970a28bf2e6233f3235233aa8d8"
	// Where the archive comes from when it is not already on the machine.
	//
	// Not Oracle: their download needs an account, an accepted licence and a
	// browser session, so it cannot be fetched by a command. This is a copy of
	// the same file, and the digest above is what says so. Oracle's licence
	// still governs what the file is used for, and it is the caller's to
	// accept, exactly as before.
	oracle19cURL  = "https://archive.org/download/linux.-x-64-193000-db-home/" + oracle19cArchive
	oracle19cRepo = "https://github.com/oracle/docker-images.git"
)

// buildOracle19c builds the image from Oracle's own Dockerfiles.
//
// Oracle publishes no free 19c image, and 19c is the long term release most
// installations run. The archive is the caller's to obtain and the licence is
// the caller's to accept: nothing here downloads it.
func buildOracle19c(ctx context.Context, r runner, _ target) error {
	archive, fetched, err := oracleArchive(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("  checking %s\n", archive)
	if err := checkSHA256(archive, oracle19cSHA256); err != nil {
		if fetched {
			// It came from the mirror and it is not the file Oracle shipped,
			// so leaving it there means every later run fails the same way
			// with nothing to do about it.
			if rmErr := os.Remove(archive); rmErr != nil {
				return fmt.Errorf("%w\n  and %s could not be removed: %v."+
					" Delete it before running this again", err, archive, rmErr)
			}
			return fmt.Errorf("%w\n  the download was removed,"+
				" so running this again fetches it afresh", err)
		}
		return err
	}

	// Oracle's Dockerfiles, checked out outside the repository: they are a
	// few hundred megabytes and none of it is ours. Not in the temporary
	// directory either, because the archive is hard linked in beside them and
	// three gigabytes is not something to download again after a reboot.
	work := stateDir("DBMETA_ORACLE_STATE", "oracle")
	tree := filepath.Join(work, "docker-images")
	if _, err := os.Stat(tree); os.IsNotExist(err) {
		fmt.Printf("  cloning Oracle's Dockerfiles into %s\n", tree)
		if err := os.MkdirAll(work, 0o755); err != nil {
			return fmt.Errorf("making %s: %w", work, err)
		}
		if err := shell(ctx, work, "git", "clone", "--depth", "1",
			"--filter=blob:none", "--sparse", oracle19cRepo, tree); err != nil {
			return err
		}
		if err := shell(ctx, tree, "git", "sparse-checkout", "set",
			"OracleDatabase/SingleInstance"); err != nil {
			return err
		}
	}

	dir := filepath.Join(tree, "OracleDatabase", "SingleInstance", "dockerfiles")
	version := filepath.Join(dir, "19.3.0")
	if _, err := os.Stat(version); err != nil {
		return fmt.Errorf("%s is missing, so Oracle's layout has changed: %w", version, err)
	}
	// The archive has to sit beside the Dockerfile, which is how Oracle's
	// script expects to find it.
	if err := link(archive, filepath.Join(version, oracle19cArchive)); err != nil {
		return err
	}

	fmt.Println("  running Oracle's build, which takes about half an hour")
	// Oracle's own script, called rather than reimplemented. It is theirs, it
	// changes with their layout, and rewriting it here would mean owning a
	// build we do not control.
	if err := shell(ctx, dir, "./buildContainerImage.sh", "-v", "19.3.0", "-e"); err != nil {
		return err
	}
	if !r.imageExists(ctx, "localhost/oracle/database:19.3.0-ee") {
		// The build has reported success on a broken image before: a relink
		// failed with [FATAL] in the middle of the log and the script carried
		// on. Check the result rather than the exit code.
		return errors.New("the build finished and the image is not there." +
			" Read the output above for [FATAL]")
	}
	return nil
}

// stateDir is where something large that the harness fetched or built lives.
//
// Not in the repository: a Windows disk is tens of gigabytes and a grep or an
// editor index over the working tree should not have to walk it. The XDG data
// directory is the conventional home, and the named variable overrides it.
func stateDir(env, name string) string {
	if dir := os.Getenv(env); dir != "" {
		return dir
	}
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "dbmeta", name)
}

// oracleArchive finds the installer archive, downloading it if it is not here.
//
// It reports whether it downloaded it, because a digest that fails on a file
// this fetched is a bad download to discard, and a digest that fails on a file
// the person put there is theirs to look at.
func oracleArchive(ctx context.Context) (string, bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	downloads := filepath.Join(home, "Downloads")
	for _, dir := range []string{".", downloads, home} {
		path := filepath.Join(dir, oracle19cArchive)
		if info, err := os.Stat(path); err == nil && info.Size() > 0 {
			return path, false, nil
		}
	}
	if err := os.MkdirAll(downloads, 0o755); err != nil {
		return "", false, fmt.Errorf("making %s: %w", downloads, err)
	}
	path := filepath.Join(downloads, oracle19cArchive)
	fmt.Printf("  %s is not here, so fetching about 3GB to %s\n", oracle19cArchive, path)
	fmt.Printf("  from %s\n", oracle19cURL)
	if err := download(ctx, oracle19cURL, path); err != nil {
		return "", false, err
	}
	return path, true, nil
}

// checkSHA256 reads the file and compares its digest.
func checkSHA256(path, want string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()
	sum := sha256.New()
	if _, err := io.Copy(sum, f); err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	got := hex.EncodeToString(sum.Sum(nil))
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("%s has digest %s and Oracle publishes %s,"+
			" so it is not the file Oracle shipped or it did not arrive whole",
			path, got, want)
	}
	return nil
}

// link puts the archive where the build expects it, without copying three
// gigabytes when a hard link will do.
func link(from, to string) error {
	if _, err := os.Stat(to); err == nil {
		return nil
	}
	if err := os.Link(from, to); err == nil {
		return nil
	}
	// A hard link fails across filesystems, so fall back to copying.
	src, err := os.Open(from)
	if err != nil {
		return fmt.Errorf("opening %s: %w", from, err)
	}
	defer src.Close()
	dst, err := os.Create(to)
	if err != nil {
		return fmt.Errorf("creating %s: %w", to, err)
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copying to %s: %w", to, err)
	}
	return nil
}

// shell runs a command in a directory with its output on the terminal.
func shell(ctx context.Context, dir, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}
