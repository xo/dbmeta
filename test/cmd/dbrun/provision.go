package main

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xo/dbmeta/container"
)

// oemFiles is the payload Windows setup runs at the end of an unattended
// install: a configuration file for SQL Server setup, the batch file that
// runs it, and the one that keeps the evaluation licence alive.
//
// They are embedded rather than read from disk so that dbrun is one binary
// with nothing to find. It used to locate them relative to the script, which
// is why the script had to know where it was.
//
//go:embed oem
var oemFiles embed.FS

// vmState is where the machine disks live.
func vmState() string { return stateDir("DBMETA_VM_STATE", "vm") }

// cmdProvision builds the machines the named targets are on.
//
// A Windows machine installs Windows and then SQL Server on its first run,
// which takes about an hour. An appliance is imported from the file a person
// downloaded, which takes minutes. A later run starts a machine that already
// exists, whichever kind it is.
func cmdProvision(ctx context.Context, picked []target, o options) error {
	for _, t := range picked {
		if t.Kind != kindMachine {
			return fmt.Errorf("%s is a %s and there is nothing to provision", t.Name, t.Kind)
		}
	}
	if o.from != "" && len(picked) != 1 {
		return errors.New("--from names one file, so provision one machine with it")
	}
	r, err := newRunner()
	if err != nil {
		return err
	}
	if !o.render {
		if _, err := os.Stat("/dev/kvm"); err != nil {
			return errors.New("/dev/kvm is missing. Without it QEMU emulates," +
				" and a Windows install that takes 30 minutes takes most of a day." +
				" Enable virtualization")
		}
	}
	var failed []string
	for _, t := range picked {
		m, ok := container.MachineByName(t.Name)
		if !ok {
			return fmt.Errorf("no machine is described for %s", t.Name)
		}
		switch {
		case m.Windows != nil:
			fmt.Printf("=== %s (SQL Server %s on %s) ===\n", t.Name, m.Release, m.Windows.Windows)
			err = provisionWindows(ctx, r, t, m, o)
		case m.Appliance != nil:
			fmt.Printf("=== %s (imported from %s) ===\n", t.Name, m.Appliance.File)
			err = importAppliance(ctx, r, t, m, o)
		default:
			err = errors.New("the machine says neither how to install it nor how to import it")
		}
		if err != nil {
			fmt.Printf("  %v\n", err)
			failed = append(failed, t.Name)
		}
	}
	if len(failed) != 0 {
		return fmt.Errorf("failed: %s", strings.Join(failed, " "))
	}
	return nil
}

// provisionWindows installs Windows and then SQL Server, or starts a machine
// that already has them.
func provisionWindows(ctx context.Context, r runner, t target, m container.Machine, o options) error {
	w := *m.Windows
	state := filepath.Join(vmState(), t.Name)
	oem := filepath.Join(state, "oem")
	shared := filepath.Join(state, "shared")
	for _, dir := range []string{oem, shared, filepath.Join(state, "storage")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("making %s: %w", dir, err)
		}
	}
	// install.bat looks for this marker to find the shared drive, because the
	// letter Windows gives it varies by release.
	if err := os.WriteFile(filepath.Join(shared, ".shared"), nil, 0o644); err != nil {
		return fmt.Errorf("writing the shared marker: %w", err)
	}

	if err := fetchInstaller(ctx, w, oem, o); err != nil {
		return err
	}
	if err := writeOEM(m.Release, w, oem); err != nil {
		return err
	}
	if o.render {
		fmt.Printf("  wrote %s\n", oem)
		return nil
	}
	if !r.exists(ctx, t.Name) {
		fmt.Println("  creating the machine, which installs Windows and then SQL Server")
	}
	if err := createMachine(ctx, r, t.Name, windowsRunArgs(t.Name, m, state, oem, shared)); err != nil {
		return err
	}
	return awaitMachine(ctx, t, m, o,
		"the first run installs Windows and then SQL Server, so allow an hour",
		"the install log, if it got that far: "+filepath.Join(shared, "provision-*.log"))
}

// awaitMachine waits for a machine's database to answer, after it was
// created or started.
//
// It waits on a query rather than on the port. The runtime publishes the port
// when the container is created, so a connection to it succeeds within
// seconds and keeps succeeding while Windows is still installing. The first
// version of this reported every machine ready twenty seconds in.
func awaitMachine(ctx context.Context, t target, m container.Machine, o options, expect, where string) error {
	fmt.Printf("  watch it at http://127.0.0.1:%d\n", m.Viewer)
	if o.watch {
		fmt.Println("  --watch given, leaving it running")
		return nil
	}
	timeout := m.Provision
	if o.timeout > 0 {
		timeout = o.timeout
	}
	fmt.Printf("  waiting for %s on 127.0.0.1:%d\n", m.Dialect, m.Port)
	fmt.Printf("  %s\n", expect)
	if err := waitForAnswer(ctx, t, timeout); err != nil {
		return fmt.Errorf("%w\n  %s\n  and the screen is at http://127.0.0.1:%d",
			err, where, m.Viewer)
	}
	fmt.Printf("  answering: %s\n", t.URL)
	return nil
}

// fetchInstaller downloads the SQL Server package onto the host.
//
// Not inside the machine: Windows Server 2008 R2 has no TLS 1.2 and cannot
// reach Microsoft's download servers at all.
func fetchInstaller(ctx context.Context, w container.WindowsSpec, oem string, o options) error {
	path := filepath.Join(oem, w.InstallerFile())
	if o.render {
		fmt.Printf("  --render given, not downloading %s\n", w.InstallerFile())
		return nil
	}
	if info, err := os.Stat(path); err == nil && info.Size() > 0 {
		fmt.Println("  installer already present")
		return nil
	}
	fmt.Printf("  downloading %s\n", w.InstallerFile())
	return download(ctx, w.Installer, path)
}

// writeOEM renders the payload for this release.
func writeOEM(release string, w container.WindowsSpec, oem string) error {
	config, err := oemFiles.ReadFile("oem/ConfigurationFile.ini")
	if err != nil {
		return fmt.Errorf("reading the embedded configuration: %w", err)
	}
	text := string(config)
	if release == "2008R2" {
		// 2008 R2 wants its own section header. It does take the license
		// flag, despite a review saying otherwise, and refuses to install
		// without it.
		//
		// A whole line, not a substring. The file explains the difference in
		// a comment above the header, so replacing the first [OPTIONS] in the
		// text rewrites the comment and leaves the header alone, which is
		// what the first version of this did.
		text = replaceLine(text, "[OPTIONS]", "[SQLSERVER2008]")
	}
	if err := writeCRLF(filepath.Join(oem, "ConfigurationFile.ini"), text); err != nil {
		return err
	}

	install, err := oemFiles.ReadFile("oem/install.bat")
	if err != nil {
		return fmt.Errorf("reading the embedded installer script: %w", err)
	}
	license := ""
	if w.LicenseFlag {
		license = "/IACCEPTSQLSERVERLICENSETERMS"
	}
	text = strings.NewReplacer(
		"@@REGISTRY_KEY@@", w.RegistryKey,
		"@@SA_PASSWORD@@", container.Password,
		"@@INSTALLER_FILE@@", w.InstallerFile(),
		"@@LICENSE_FLAG@@", license,
	).Replace(string(install))
	if strings.Contains(text, "@@") {
		// A placeholder left behind means a rename somewhere, and the machine
		// would run the literal text for the next forty minutes before
		// failing.
		return fmt.Errorf("a placeholder is unfilled in install.bat for %s", release)
	}
	if err := writeCRLF(filepath.Join(oem, "install.bat"), text); err != nil {
		return err
	}

	// rearm.bat needs no substitution: it reads the grace period rather than
	// being told anything about the release. See D65.
	rearm, err := oemFiles.ReadFile("oem/rearm.bat")
	if err != nil {
		return fmt.Errorf("reading the embedded rearm script: %w", err)
	}
	return writeCRLF(filepath.Join(oem, "rearm.bat"), string(rearm))
}

// replaceLine swaps a line that is exactly from for to, leaving a line that
// merely contains it alone.
func replaceLine(text, from, to string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.TrimRight(line, "\r") == from {
			lines[i] = to
		}
	}
	return strings.Join(lines, "\n")
}

// writeCRLF writes a file with the line endings Windows reads.
//
// A batch file with Unix endings runs, mostly, and then fails somewhere
// specific and unhelpful. Normalising first means a file that already has
// them does not end up with two.
func writeCRLF(path, text string) error {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\n", "\r\n")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// createMachine creates a machine from its run arguments, or starts one that
// is already there.
//
// A machine that exists is never recreated here, because its disk is the
// hour of work or the import that made it.
func createMachine(ctx context.Context, r runner, name string, args []string) error {
	if r.exists(ctx, name) {
		fmt.Println("  the machine exists, starting it")
		if !r.quiet(ctx, "start", name) {
			return errors.New("it would not start")
		}
		return nil
	}
	if out, err := r.output(ctx, withOwner(args, currentOwner())...); err != nil {
		return fmt.Errorf("it would not start: %s", lastLine(out))
	}
	return nil
}

// windowsRunArgs are the arguments that create a Windows machine.
func windowsRunArgs(name string, m container.Machine, state, oem, shared string) []string {
	return []string{
		"run", "--detach", "--name", name,
		"--env", "VERSION=" + m.Windows.Image,
		"--env", "DISK_SIZE=64G",
		"--env", "RAM_SIZE=4G",
		"--env", "CPU_CORES=4",
		"--publish", fmt.Sprintf("127.0.0.1:%d:%d", m.Port, m.GuestPort()),
		"--publish", fmt.Sprintf("127.0.0.1:%d:8006", m.Viewer),
		"--device=/dev/kvm", "--device=/dev/net/tun", "--cap-add", "NET_ADMIN",
		"--volume", filepath.Join(state, "storage") + ":/storage",
		"--volume", oem + ":/oem",
		"--volume", shared + ":/shared",
		"--stop-timeout", "120",
		"docker.io/dockurr/windows",
	}
}

// waitForAnswer opens a real connection and runs the version query until it
// answers.
func waitForAnswer(ctx context.Context, t target, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if answered(ctx, t, 20*time.Second) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("it never answered in %s", timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
		}
	}
}

// answered reports whether the database on a machine answers a query within
// the timeout.
//
// It connects with the driver the tests use for that dialect and runs the
// version query dbmeta runs, rather than SELECT 1, so that a machine which
// answers but has not finished configuring is not called ready. It was
// written for SQL Server alone, and the dialect is what made it general.
func answered(ctx context.Context, t target, timeout time.Duration) bool {
	driver, ok := drivers[t.Dialect]
	if !ok {
		return false
	}
	db, err := sql.Open(driver, t.DSN)
	if err != nil {
		return false
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	_, err = t.Dialect.Version(ctx, db)
	return err == nil
}

// lastLines returns the last n lines of s, each indented, for an error that
// carries what a command said.
func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return "    " + strings.Join(lines, "\n    ")
}

func lastLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		return s[i+1:]
	}
	return s
}
