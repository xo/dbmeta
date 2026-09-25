// Command dbrun starts the databases dbmeta is tested against.
//
// It is the only thing that starts one. D68 says so, and this exists because
// a rule nobody can follow is a rule that gets broken: the shell script it
// replaces could start a server, test it and throw it away, and nothing else,
// so anybody who wanted to keep one reached for podman and named it whatever
// they were thinking. docs/RUNNER.md is the design.
//
// It lives in the test module rather than at the repository root, because
// version opens a connection, which needs a driver, which hard rule 1 keeps
// out of the root module.
//
//	dbrun help
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		if !errors.Is(err, errSilent) {
			fmt.Fprintln(os.Stderr, "dbrun:", err)
		}
		os.Exit(1)
	}
}

// errSilent is returned by a command that has already said what went wrong,
// so that main exits without printing a second line.
var errSilent = errors.New("already reported")

type options struct {
	allReleases bool
	keep        bool
	remove      bool
	asJSON      bool
	namesOnly   bool
	yes         bool
	follow      bool
	render      bool
	watch       bool
	timeout     time.Duration
}

func run(args []string) error {
	// Bare prints the help. It used to start every release of every database
	// and run the whole suite, which is half an hour of containers and is
	// almost never what somebody meant by typing the command with nothing
	// after it.
	if len(args) == 0 {
		usage(os.Stdout)
		return nil
	}
	command, rest := args[0], args[1:]

	var o options
	fs := flag.NewFlagSet("dbrun "+command, flag.ContinueOnError)
	fs.BoolVar(&o.allReleases, "releases", false,
		"widen a product selector to every release of it")
	fs.BoolVar(&o.keep, "keep", false, "after test, leave the server running")
	fs.BoolVar(&o.remove, "remove", false, "after test, delete the server")
	fs.BoolVar(&o.asJSON, "json", false, "print machine readable output")
	fs.BoolVar(&o.namesOnly, "names", false,
		"with --json, print just the names, which is what a CI matrix takes")
	fs.BoolVar(&o.yes, "yes", false, "do not ask before a destructive command")
	fs.BoolVar(&o.follow, "f", false, "follow the log")
	fs.BoolVar(&o.render, "render", false,
		"with provision, write the OEM folder and stop, without downloading or starting anything")
	fs.BoolVar(&o.watch, "watch", false,
		"with provision, leave the machine running rather than waiting for it")
	fs.DurationVar(&o.timeout, "timeout", 0,
		"how long to wait for a server to answer, zero for the default")
	fs.Usage = func() { usage(os.Stderr) }

	switch command {
	case "help", "-h", "--help":
		usage(os.Stdout)
		return nil
	}
	// Flags are accepted anywhere, not only before the first selector. Go's
	// flag package stops at the first non-flag argument, so `list postgres
	// --releases` would read the flag as the name of a database. Splitting
	// them first is the whole fix.
	flags, selectors := splitArgs(rest)
	if err := fs.Parse(flags); err != nil {
		return errSilent
	}
	if o.keep && o.remove {
		return errors.New("--keep and --remove ask for opposite things")
	}

	// Ctrl-C has to reach the container, or a cancelled run leaves one bound
	// to a port with nothing to say why.
	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer stop()

	all := targets()
	if len(selectors) == 0 {
		// status and version answer a question about what is already there
		// and change nothing, so asking them about everything is what
		// somebody meant. Every other command acts, and has to be told what
		// to act on.
		switch command {
		case "status", "version":
			selectors = []string{"all"}
		default:
			return fmt.Errorf("%s needs a selector. Try: dbrun %s all, or dbrun list all",
				command, command)
		}
	}
	picked, notes, err := resolve(all, selectors, o.allReleases)
	if err != nil {
		return err
	}
	// A bare product resolves to the newest release, and every command says
	// which one before it acts. The answer changes the month a release ships
	// and a silent default is the part that would bite.
	for _, n := range notes {
		fmt.Println(n)
	}

	switch command {
	case "list":
		return cmdList(picked, o)
	case "dsn":
		return cmdDSN(picked, o)
	case "start", "stop", "remove", "status", "version", "usql", "test", "logs":
		return withRunner(ctx, command, picked, o)
	case "build":
		return cmdBuild(ctx, picked)
	case "provision":
		return cmdProvision(ctx, picked, o)
	}
	return fmt.Errorf("no command called %q. Try: dbrun help", command)
}

// splitArgs separates the flags from the selectors, wherever they appear.
//
// Every flag dbrun takes is a boolean except --timeout, so only that one
// consumes the word after it, and only when it is not written with an equals
// sign.
func splitArgs(args []string) ([]string, []string) {
	var flags, selectors []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			return flags, append(selectors, args[i+1:]...)
		case arg == "-timeout" || arg == "--timeout":
			flags = append(flags, arg)
			if i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
		case strings.HasPrefix(arg, "-") && arg != "-":
			flags = append(flags, arg)
		default:
			selectors = append(selectors, arg)
		}
	}
	return flags, selectors
}

// once in the list and once in an example
//
//nolint:dupword // a help text names each command twice on purpose,
func usage(w *os.File) {
	fmt.Fprint(w, `dbrun starts the databases dbmeta is tested against.

It is the only thing that starts one, which is D68. The list of releases lives
in the Go package github.com/xo/dbmeta/container, so this command and the CI
workflow cannot disagree about what was tested.

Usage: dbrun <command> [selector...] [flags]

Commands:
  start       start it, leave it running, print its URL
  stop        stop it, keeping it so start resumes it
  remove      stop and delete it
  status      what is running, with a URL for each
  version     connect and print the version dbmeta parses
  dsn         print the dburl style URL, running or not
  usql        connect to it with usql, which must be on the path
  test        run the integration tests against it
  logs        show what the server said
  list        show what a selector expands to, without touching anything
  build       build the images this repository makes, for Cassandra and Oracle 19c
  provision   build a Windows machine, which takes about an hour
  help        this

Selectors:
  postgres-18     that release
  postgres        the newest PostgreSQL, and only that one
  sqlite3         an embedded library, which has no server to start
  tested          the releases CI runs on every push
  nightly         the releases CI runs at night
  all             every release of every product

Flags:
  --releases      widen a product selector to every release of it
  --keep          after test, leave the server running
  --remove        after test, delete the server
  --timeout       how long to wait for a server to answer
  --json          machine readable output, for list, dsn, status and version
  --names         with --json, just the names, which is what a CI matrix takes
  --yes           do not ask before deleting a Windows machine
  --render        with provision, write the OEM folder and stop
  --watch         with provision, leave the machine running rather than waiting
  -f              follow the log

Environment:
  DBMETA_RUNNER         podman by default, set to docker to use that instead
  DBMETA_VM_STATE       where the Windows disks live, which are tens of
                        gigabytes each. Defaults under $XDG_DATA_HOME/dbmeta
  DBMETA_ORACLE_STATE   where the Oracle 19c checkout and archive live

Examples:
  dbrun start postgres            the newest PostgreSQL, left running
  dbrun test clickhouse-25.8      one release, tested and removed
  dbrun test sqlite3              no container, it is a library
  dbrun status                    what is up
  dbrun usql oracle               a shell on the newest Oracle
  dbrun test all                  everything, which is what a release needs
`)
}
