// Command vms prints the Windows virtual machines dbmeta is verified against,
// for a shell script to provision.
//
// The list lives in github.com/xo/dbmeta/container and this prints it, the
// same way ./tool/servers does for the Linux containers. A shell script cannot
// import a Go package, so this is the bridge, and it keeps test/vm/provision.sh
// from carrying a second copy of the list.
//
// Usage:
//
//	go run ./tool/vms [release ...]
//
// With no argument it prints every machine. An argument selects one by its
// SQL Server release, such as 2012.
//
// It prints one machine per line, with the fields separated by a unit
// separator, U+001F: the name, the SQL Server release, the dockur VERSION, the
// host port, the viewer port, the registry key, the license flag, the
// installer URL, the installer file name and the connection string.
//
// The separator is not a tab, and that is not a style choice. One field is
// empty, because 2008 R2 takes no license flag, and bash treats a tab as IFS
// whitespace and collapses a run of them into one. Reading tab separated
// fields therefore shifted every field after the empty one by a place, and the
// installer name came back holding a connection string. A unit separator is
// not IFS whitespace, so an empty field survives.
//
// In bash:
//
//	while IFS=$'\x1f' read -r name release image ... ; do
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/microsoft/go-mssqldb"

	"github.com/xo/dbmeta/container"
)

// sep separates the fields of a line. See the package comment for why it is
// not a tab.
const sep = "\x1f"

func main() {
	wait := flag.Duration("wait", 0,
		"wait up to this long for the named machine to answer a query, then exit")
	flag.Parse()
	var err error
	if *wait > 0 {
		err = waitFor(flag.Args(), *wait)
	} else {
		err = run(flag.Args())
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "vms:", err)
		os.Exit(1)
	}
}

// waitFor blocks until the machine answers a query, or the deadline passes.
//
// It opens a connection and runs a statement rather than testing the port.
// The port is published by the container runtime and accepts a connection the
// moment the container exists, which is twenty seconds after it is created and
// about forty minutes before Windows has finished installing. A port check
// therefore reports ready immediately and always, which is worse than no check
// at all.
func waitFor(args []string, limit time.Duration) error {
	if len(args) != 1 {
		return errors.New("wait takes one release")
	}
	v, ok := container.WindowsVMByRelease(args[0])
	if !ok {
		return fmt.Errorf("nothing matches %q", args[0])
	}
	db, err := sql.Open("sqlserver", v.DSN())
	if err != nil {
		return fmt.Errorf("opening %s: %w", v.Name(), err)
	}
	defer db.Close()

	deadline := time.Now().Add(limit)
	for attempt := 1; ; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		var version string
		err = db.QueryRowContext(ctx, `SELECT @@VERSION`).Scan(&version)
		cancel()
		if err == nil {
			fmt.Println(strings.SplitN(version, "\n", 2)[0])
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s never answered in %s, last error: %w", v.Name(), limit, err)
		}
		if attempt%10 == 0 {
			fmt.Fprintf(os.Stderr, "  still waiting for %s (%s)\n", v.Name(), err)
		}
		time.Sleep(15 * time.Second)
	}
}

func run(args []string) error {
	vms := container.WindowsVMs
	if len(args) > 0 {
		var picked []container.WindowsVM
		for _, arg := range args {
			v, ok := container.WindowsVMByRelease(arg)
			if !ok {
				return fmt.Errorf("nothing matches %q", arg)
			}
			picked = append(picked, v)
		}
		vms = picked
	}
	for _, v := range vms {
		license := ""
		if v.LicenseFlag {
			license = "/IACCEPTSQLSERVERLICENSETERMS"
		}
		fields := []string{
			v.Name(),
			v.Release,
			v.Image,
			strconv.Itoa(v.Port),
			strconv.Itoa(v.Viewer),
			v.RegistryKey,
			license,
			v.Installer,
			v.InstallerFile(),
			v.DSN(),
		}
		for _, f := range fields {
			if strings.ContainsAny(f, "\n"+sep) {
				return fmt.Errorf("%s: a field contains a newline or a separator: %q",
					v.Name(), f)
			}
		}
		fmt.Println(strings.Join(fields, sep))
	}
	return nil
}
