package main

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xo/dbmeta/container"
)

// An appliance's import is checked here against a small archive built in the
// test, because the real one is ten gigabytes behind a signup form. What
// matters is the part that is silently wrong when it is wrong: the digest
// covers the whole file, a disk is found by its exact name, and each disk
// lands where qemux/qemu looks for it.

// ova writes a tar holding these members, in this order, and returns its
// path and its digest.
func ova(t *testing.T, members ...string) (string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "x.ova")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	tw := tar.NewWriter(f)
	for _, name := range members {
		body := []byte("contents of " + name)
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	return path, hex.EncodeToString(sum[:])
}

func spec(sum string, members ...string) container.ApplianceSpec {
	a := container.ApplianceSpec{SHA256: sum}
	for _, m := range members {
		a.Disks = append(a.Disks, container.Disk{Member: m, Size: "1G"})
	}
	return a
}

// TestUnpackWritesTheNamedDisks checks that the disks come out and nothing
// else does, with the manifest after them as it is in a real OVA.
func TestUnpackWritesTheNamedDisks(t *testing.T) {
	t.Parallel()
	src, sum := ova(t, "a.ovf", "a-disk001.vmdk", "a-disk002.vmdk", "a.mf")
	work := t.TempDir()
	if err := unpack(src, spec(sum, "a-disk001.vmdk", "a-disk002.vmdk"), work); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(work)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		got = append(got, e.Name())
	}
	if want := []string{"a-disk001.vmdk", "a-disk002.vmdk"}; !slices.Equal(got, want) {
		t.Errorf("unpacked %v, want %v", got, want)
	}
	body, err := os.ReadFile(filepath.Join(work, "a-disk002.vmdk"))
	if err != nil || string(body) != "contents of a-disk002.vmdk" {
		t.Errorf("a-disk002.vmdk holds %q, %v", body, err)
	}
}

// TestUnpackRefusesAnotherFile checks the digest, which is the only check
// there is on a file that cannot be fetched again.
func TestUnpackRefusesAnotherFile(t *testing.T) {
	t.Parallel()
	src, sum := ova(t, "a-disk001.vmdk", "a.mf")
	wrong := strings.Repeat("0", len(sum))
	err := unpack(src, spec(wrong, "a-disk001.vmdk"), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), sum) {
		t.Errorf("a file with another digest was accepted, or the error does not say what it has: %v", err)
	}
}

// TestUnpackRefusesAMissingDisk checks that a disk the list names and the
// file lacks is an error rather than a machine with a disk it creates empty.
func TestUnpackRefusesAMissingDisk(t *testing.T) {
	t.Parallel()
	src, sum := ova(t, "a-disk001.vmdk")
	err := unpack(src, spec(sum, "a-disk001.vmdk", "a-disk002.vmdk"), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "a-disk002.vmdk") {
		t.Errorf("a missing disk was not reported: %v", err)
	}
}

// TestDisksLandWhereQemuLooks pins the names qemux/qemu reads, and the run
// arguments that mount them.
func TestDisksLandWhereQemuLooks(t *testing.T) {
	t.Parallel()
	a := container.ApplianceSpec{
		Memory: "8G", CPUs: 4, Port: 8563,
		Disks: []container.Disk{{Member: "s.vmdk", Size: "100G"}, {Member: "d.vmdk", Size: "500G"}},
	}
	disks := placeDisks("/state", a)
	want := []placedDisk{
		{Disk: a.Disks[0], Dir: "/state/storage", File: "data.qcow2", Mount: "/storage", SizeEnv: "DISK_SIZE"},
		{Disk: a.Disks[1], Dir: "/state/storage2", File: "data2.qcow2", Mount: "/storage2", SizeEnv: "DISK2_SIZE"},
	}
	if !slices.Equal(disks, want) {
		t.Fatalf("placed %+v, want %+v", disks, want)
	}
	args := strings.Join(applianceRunArgs("x-1", container.Machine{
		Port: 58563, Viewer: 8110, Appliance: &a,
	}, disks), " ")
	for _, part := range []string{
		"--name x-1",
		"RAM_SIZE=8G", "CPU_CORES=4", "DISK_SIZE=100G", "DISK2_SIZE=500G",
		"127.0.0.1:58563:8563", "127.0.0.1:8110:8006",
		"/state/storage:/storage", "/state/storage2:/storage2",
	} {
		if !strings.Contains(args, part) {
			t.Errorf("the run arguments lack %q: %s", part, args)
		}
	}
	if !strings.HasSuffix(args, qemuImage) {
		t.Errorf("the image is not last: %s", args)
	}
}
