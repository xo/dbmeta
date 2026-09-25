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

	"github.com/xo/dbmeta"
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

// cmdProvision builds the Windows machines the named targets are on.
//
// The first run installs Windows and then SQL Server and takes about an hour.
// A later run starts a machine that already exists, which is about a minute.
func cmdProvision(ctx context.Context, picked []target, o options) error {
	for _, t := range picked {
		if t.Kind != kindMachine {
			return fmt.Errorf("%s is a %s and there is nothing to provision", t.Name, t.Kind)
		}
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
		vm, ok := container.WindowsVMByRelease(t.Release)
		if !ok {
			return fmt.Errorf("no machine is described for %s", t.Name)
		}
		fmt.Printf("=== %s (SQL Server %s on %s) ===\n", t.Name, vm.Release, vm.Windows)
		if err := provisionOne(ctx, r, t, vm, o); err != nil {
			fmt.Printf("  %v\n", err)
			failed = append(failed, t.Name)
		}
	}
	if len(failed) != 0 {
		return fmt.Errorf("failed: %s", strings.Join(failed, " "))
	}
	return nil
}

func provisionOne(ctx context.Context, r runner, t target, vm container.WindowsVM, o options) error {
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

	if err := fetchInstaller(ctx, vm, oem, o); err != nil {
		return err
	}
	if err := writeOEM(vm, oem); err != nil {
		return err
	}
	if o.render {
		fmt.Printf("  wrote %s\n", oem)
		return nil
	}
	if err := startMachine(ctx, r, t, vm, state, oem, shared); err != nil {
		return err
	}
	fmt.Printf("  watch it at http://127.0.0.1:%d\n", vm.Viewer)
	if o.watch {
		fmt.Println("  --watch given, leaving it running")
		return nil
	}

	// Waiting on a query rather than on the port. The runtime publishes the
	// port when the container is created, so a connection to it succeeds
	// within seconds and keeps succeeding while Windows is still installing.
	// The first version of this reported every machine ready twenty seconds
	// in.
	timeout := 90 * time.Minute
	if o.timeout > 0 {
		timeout = o.timeout
	}
	fmt.Printf("  waiting for SQL Server on 127.0.0.1:%d\n", vm.Port)
	fmt.Println("  the first run installs Windows and then SQL Server, so allow an hour")
	if err := waitForSQLServer(ctx, vm.DSN(), timeout); err != nil {
		return fmt.Errorf("%w\n  the install log, if it got that far: %s\n"+
			"  and the screen is at http://127.0.0.1:%d",
			err, filepath.Join(shared, "provision-*.log"), vm.Viewer)
	}
	fmt.Printf("  answering: %s\n", vm.DSN())
	return nil
}

// fetchInstaller downloads the SQL Server package onto the host.
//
// Not inside the machine: Windows Server 2008 R2 has no TLS 1.2 and cannot
// reach Microsoft's download servers at all.
func fetchInstaller(ctx context.Context, vm container.WindowsVM, oem string, o options) error {
	path := filepath.Join(oem, vm.InstallerFile())
	if o.render {
		fmt.Printf("  --render given, not downloading %s\n", vm.InstallerFile())
		return nil
	}
	if info, err := os.Stat(path); err == nil && info.Size() > 0 {
		fmt.Println("  installer already present")
		return nil
	}
	fmt.Printf("  downloading %s\n", vm.InstallerFile())
	return download(ctx, vm.Installer, path)
}

// writeOEM renders the payload for this release.
func writeOEM(vm container.WindowsVM, oem string) error {
	config, err := oemFiles.ReadFile("oem/ConfigurationFile.ini")
	if err != nil {
		return fmt.Errorf("reading the embedded configuration: %w", err)
	}
	text := string(config)
	if vm.Release == "2008R2" {
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
	if vm.LicenseFlag {
		license = "/IACCEPTSQLSERVERLICENSETERMS"
	}
	text = strings.NewReplacer(
		"@@REGISTRY_KEY@@", vm.RegistryKey,
		"@@SA_PASSWORD@@", container.SQLServerPassword,
		"@@INSTALLER_FILE@@", vm.InstallerFile(),
		"@@LICENSE_FLAG@@", license,
	).Replace(string(install))
	if strings.Contains(text, "@@") {
		// A placeholder left behind means a rename somewhere, and the machine
		// would run the literal text for the next forty minutes before
		// failing.
		return fmt.Errorf("a placeholder is unfilled in install.bat for %s", vm.Release)
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

// startMachine creates the machine, or starts one that is already there.
func startMachine(ctx context.Context, r runner, t target, vm container.WindowsVM,
	state, oem, shared string,
) error {
	if r.exists(ctx, t.Name) {
		fmt.Println("  the machine exists, starting it")
		if !r.quiet(ctx, "start", t.Name) {
			return errors.New("it would not start")
		}
		return nil
	}
	fmt.Println("  creating the machine, which installs Windows and then SQL Server")
	args := []string{
		"run", "--detach", "--name", t.Name,
		"--env", "VERSION=" + vm.Image,
		"--env", "DISK_SIZE=64G",
		"--env", "RAM_SIZE=4G",
		"--env", "CPU_CORES=4",
		"--publish", fmt.Sprintf("127.0.0.1:%d:1433", vm.Port),
		"--publish", fmt.Sprintf("127.0.0.1:%d:8006", vm.Viewer),
		"--device=/dev/kvm", "--device=/dev/net/tun", "--cap-add", "NET_ADMIN",
		"--volume", filepath.Join(state, "storage") + ":/storage",
		"--volume", oem + ":/oem",
		"--volume", shared + ":/shared",
		"--stop-timeout", "120",
		"docker.io/dockurr/windows",
	}
	if out, err := r.output(ctx, args...); err != nil {
		return fmt.Errorf("it would not start: %s", lastLine(out))
	}
	return nil
}

// waitForSQLServer opens a real connection and runs a statement.
func waitForSQLServer(ctx context.Context, dsn string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if answered(ctx, dsn, 20*time.Second) {
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

// answered reports whether SQL Server on the other end of a DSN answers a
// query within the timeout.
func answered(ctx context.Context, dsn string, timeout time.Duration) bool {
	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		return false
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	// The version query rather than SELECT 1, so that a machine which
	// answers but has not finished configuring is not called ready.
	_, err = dbmeta.SQLServer.Version(ctx, db)
	return err == nil
}

func lastLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		return s[i+1:]
	}
	return s
}
