package main

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/dbmeta/container"
)

// qemuImage boots an arbitrary disk under KVM. It is dockurr/windows's
// sibling, from the same family, so an appliance is a container dbrun starts
// the way a Windows machine is. It also carries qemu-img, which is why the
// conversion needs nothing installed on the host. See D85 and D86.
const qemuImage = "docker.io/qemux/qemu"

// importAppliance imports a vendor's machine image and boots it, or starts a
// machine that was imported before.
//
// The image cannot be fetched. It sits behind a signup form, so a person
// downloads it and names it with --from, or leaves it in the machine's state
// directory. It is checked against the digest the list pins, unpacked, and
// each disk converted to qcow2. The conversion is the import, and it happens
// once.
func importAppliance(ctx context.Context, r runner, t target, m container.Machine, o options) error {
	a := *m.Appliance
	if o.render {
		fmt.Println("  --render writes a Windows OEM folder, and an appliance has none")
		return nil
	}
	state := filepath.Join(vmState(), t.Name)
	disks := placeDisks(state, a)
	if !r.exists(ctx, t.Name) && !imported(disks) {
		src, err := findImage(state, a, o.from)
		if err != nil {
			return err
		}
		if err := importDisks(ctx, r, src, state, a, disks); err != nil {
			return err
		}
	}
	if err := createMachine(ctx, r, t.Name, applianceRunArgs(t.Name, m, disks)); err != nil {
		return err
	}
	return awaitMachine(ctx, t, m, o,
		"an appliance has nothing to install, so it answers within minutes",
		"its log: dbrun logs "+t.Name)
}

// placedDisk is one of an appliance's disks and where it lives once imported.
type placedDisk struct {
	container.Disk

	// Dir is the host directory mounted into the machine for this disk.
	Dir string
	// File is the qcow2 file's name inside Dir.
	File string
	// Mount is where qemux/qemu looks for the directory.
	Mount string
	// SizeEnv is the variable qemux/qemu reads this disk's size from.
	SizeEnv string
}

// placeDisks says where each disk goes.
//
// qemux/qemu names them for it. The first disk is data.qcow2 in /storage, and
// disk n is datan.qcow2 in /storagen, each a directory of its own. A disk put
// anywhere else is a disk it creates empty.
func placeDisks(state string, a container.ApplianceSpec) []placedDisk {
	out := make([]placedDisk, len(a.Disks))
	for i, d := range a.Disks {
		n := ""
		if i > 0 {
			n = strconv.Itoa(i + 1)
		}
		out[i] = placedDisk{
			Disk:    d,
			Dir:     filepath.Join(state, "storage"+n),
			File:    "data" + n + ".qcow2",
			Mount:   "/storage" + n,
			SizeEnv: "DISK" + n + "_SIZE",
		}
	}
	return out
}

// imported reports whether every disk is converted already.
func imported(disks []placedDisk) bool {
	for _, d := range disks {
		if _, err := os.Stat(filepath.Join(d.Dir, d.File)); err != nil {
			return false
		}
	}
	return true
}

// findImage finds the file a person downloaded.
func findImage(state string, a container.ApplianceSpec, from string) (string, error) {
	path := from
	if path == "" {
		path = filepath.Join(state, a.File)
	}
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("%s is not at %s. Download it from %s,"+
			" then run provision again with --from and the path, or put it there."+
			" Its SHA256 must be %s", a.File, path, a.Page, a.SHA256)
	}
	return path, nil
}

// importDisks checks and unpacks the image and converts each disk.
func importDisks(ctx context.Context, r runner, src, state string, a container.ApplianceSpec, disks []placedDisk) error {
	work := filepath.Join(state, "import")
	if err := os.MkdirAll(work, 0o755); err != nil {
		return fmt.Errorf("making %s: %w", work, err)
	}
	// The unpacked disks are as large as the image and are only a step on
	// the way to the qcow2 files, so they go whether this works or not.
	defer os.RemoveAll(work)
	for _, d := range disks {
		if err := os.MkdirAll(d.Dir, 0o755); err != nil {
			return fmt.Errorf("making %s: %w", d.Dir, err)
		}
		noCOW(ctx, d.Dir)
	}
	fmt.Printf("  checking and unpacking %s\n", src)
	if err := unpack(src, a, work); err != nil {
		return err
	}
	for _, d := range disks {
		fmt.Printf("  converting %s\n", d.Member)
		if err := convert(ctx, r, work, d); err != nil {
			return err
		}
	}
	return nil
}

// noCOW turns copy on write off for a directory, so that the disk images made
// in it are written in place.
//
// qemux/qemu warns when it is on, and on btrfs a disk image that is copied on
// every write fragments until it is slow. The attribute only takes effect on
// a file created after it is set, which is why this runs before the
// conversion rather than after it. A file system that has no such attribute
// refuses it, and that is not a failure, because it has no copy on write to
// turn off.
func noCOW(ctx context.Context, dir string) {
	if err := exec.CommandContext(ctx, "chattr", "+C", dir).Run(); err != nil {
		fmt.Printf("  copy on write is still on for %s, which matters only on btrfs\n", dir)
	}
}

// unpack checks the image against its digest and writes out the disks it
// holds, reading it once.
//
// The digest covers the whole file, so the part after the last disk is read
// too. The image is ten gigabytes and reading it twice, once to check and
// once to unpack, is a minute nobody needs to wait.
func unpack(src string, a container.ApplianceSpec, work string) error {
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening %s: %w", src, err)
	}
	defer f.Close()

	want := make(map[string]bool, len(a.Disks))
	for _, d := range a.Disks {
		want[d.Member] = true
	}
	h := sha256.New()
	in := io.TeeReader(f, h)
	tr := tar.NewReader(in)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("reading %s: %w", src, err)
		}
		if !want[hdr.Name] {
			continue
		}
		// The name is one the list gave, compared exactly, so it cannot
		// climb out of the directory. Base is there all the same.
		if err := writeMember(filepath.Join(work, filepath.Base(hdr.Name)), tr); err != nil {
			return err
		}
		delete(want, hdr.Name)
	}
	if _, err := io.Copy(io.Discard, in); err != nil {
		return fmt.Errorf("reading %s: %w", src, err)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != a.SHA256 {
		return fmt.Errorf("%s has SHA256 %s and the list pins %s."+
			" It is a different file, or a damaged download", src, got, a.SHA256)
	}
	if len(want) != 0 {
		return fmt.Errorf("%s has no disk called %s", src,
			strings.Join(slices.Sorted(maps.Keys(want)), " or "))
	}
	return nil
}

func writeMember(path string, r io.Reader) error {
	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
	}
	if _, err := io.Copy(out, r); err != nil {
		out.Close()
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// convert turns one unpacked disk into the qcow2 file the machine boots.
//
// It runs qemu-img from the machine's own image, so the host needs nothing
// but the container runner. The file is written under a temporary name and
// renamed when it is whole, so that a conversion that was interrupted is not
// mistaken for an import that finished.
func convert(ctx context.Context, r runner, work string, d placedDisk) error {
	part := d.File + ".part"
	out, err := r.output(ctx, "run", "--rm",
		"--volume", work+":/in:ro",
		"--volume", d.Dir+":/out",
		"--entrypoint", "qemu-img",
		qemuImage,
		"convert", "-O", "qcow2", "/in/"+filepath.Base(d.Member), "/out/"+part)
	if err != nil {
		// The partial file is left. Nothing reads that name, and the next
		// conversion writes over it.
		return fmt.Errorf("converting %s: %s", d.Member, lastLine(out))
	}
	if err := os.Rename(filepath.Join(d.Dir, part), filepath.Join(d.Dir, d.File)); err != nil {
		return fmt.Errorf("renaming the converted %s: %w", d.Member, err)
	}
	return nil
}

// applianceRunArgs are the arguments that create an appliance's machine.
//
// Firmware is left to qemux/qemu, whose default is UEFI, which is what the
// Exasol descriptor declares. The disk controller is left to it too: the
// descriptor names NVMe, and the default SCSI disk boots it all the same.
func applianceRunArgs(name string, m container.Machine, disks []placedDisk) []string {
	a := m.Appliance
	args := []string{
		"run", "--detach", "--name", name,
		"--env", "RAM_SIZE=" + a.Memory,
		"--env", "CPU_CORES=" + strconv.Itoa(a.CPUs),
	}
	for _, d := range disks {
		args = append(args, "--env", d.SizeEnv+"="+d.Size)
	}
	args = append(args,
		"--publish", fmt.Sprintf("127.0.0.1:%d:%d", m.Port, m.GuestPort()),
		"--publish", fmt.Sprintf("127.0.0.1:%d:8006", m.Viewer),
		"--device=/dev/kvm", "--device=/dev/net/tun", "--cap-add", "NET_ADMIN",
	)
	for _, d := range disks {
		args = append(args, "--volume", d.Dir+":"+d.Mount)
	}
	return append(args, "--stop-timeout", "120", qemuImage)
}
