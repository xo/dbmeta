// Command servers prints the database servers dbmeta is tested against, for a
// shell script to run.
//
// The list lives in github.com/xo/dbmeta/container and this prints it. Nothing
// here decides which releases are tested. A shell script cannot import a Go
// package, so this is the bridge, and it keeps run.sh from carrying a second
// copy of the list that ages on its own.
//
// Usage:
//
//	go run ./tool/servers [tier|product|release ...]
//
// With no argument it prints every server. An argument selects by tier
// (tested, nightly, verified), by product (postgres, mariadb, mysql) or by an
// exact name (mariadb-11.8).
//
// It prints one server per line, tab separated: the name, the connection
// string, the environment variable the test reads it from, the arguments that
// start it, and the arguments that ask whether it is ready. Every field after
// the first two is a command line with its arguments separated by spaces, and
// no value here contains a space.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/container"
)

// basePort is where the published ports start. Each server gets the next one,
// so that several can run at the same time.
const basePort = 55000

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "servers:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	servers := container.All()
	if len(args) > 0 {
		var picked []container.Server
		for _, arg := range args {
			found := selectServers(servers, arg)
			if len(found) == 0 {
				return fmt.Errorf("nothing matches %q", arg)
			}
			picked = append(picked, found...)
		}
		servers = picked
	}
	for i, s := range servers {
		port := basePort + i
		fields := []string{
			s.Name(),
			s.DSN(port),
			envFor(s.Dialect),
			strings.Join(s.RunArgs(s.Name(), port), " "),
			strings.Join(s.ReadyArgs(s.Name()), " "),
			strings.Join(s.RemoveArgs(s.Name()), " "),
		}
		for _, f := range fields {
			if strings.ContainsAny(f, "\t\n") {
				return fmt.Errorf("%s: a field contains a tab or a newline: %q", s.Name(), f)
			}
		}
		fmt.Println(strings.Join(fields, "\t"))
	}
	return nil
}

// selectServers returns the servers an argument names, by tier, by product or
// by exact name.
func selectServers(servers []container.Server, arg string) []container.Server {
	var out []container.Server
	for _, s := range servers {
		if string(s.Tier) == arg || s.Product == arg || s.Name() == arg {
			out = append(out, s)
		}
	}
	return out
}

// envFor names the variable the integration tests read a connection string
// from. One per dialect, because one test run reaches one server per dialect.
func envFor(d dbmeta.Dialect) string {
	if d == dbmeta.MySQL {
		return "DBMETA_MYSQL"
	}
	return "DBMETA_" + strings.ToUpper(string(d))
}
