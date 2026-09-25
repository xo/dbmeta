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
// start it, the arguments that ask whether it is ready, and the arguments that
// remove it.
//
// The three command fields hold their arguments separated by a unit separator,
// U+001F, and not by a space. A space was used until an argument needed one:
// the SQL Server readiness command runs the query "SELECT 1", and a shell
// splitting the field on whitespace turned it into two arguments and passed a
// command that could never succeed. The failure looked like a server that
// never started.
//
// A reader splits a command field on U+001F. In bash:
//
//	IFS=$'\x1f' read -r -a cmd <<< "$readyargs"
//	"$RUNNER" "${cmd[@]}"
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/container"
)

// sep separates the arguments of a command within one field. It is a unit
// separator, U+001F, because an argument can contain a space and cannot
// sensibly contain this.
const sep = "\x1f"

// basePort is where the published ports start.
//
// A server's port is its position in container.All plus this, so a release
// always gets the same one however it was selected. Numbering the filtered
// list instead gave every single server run.sh started the first port, so two
// of them collided and the second was left in Created state holding a name
// that podman rm --force does not free. It also means a port a person learned
// once keeps working, which matters because run.sh up prints a DSN somebody
// then pastes into usql.
const basePort = 55000

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "servers:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	// --json prints just the names, as a JSON array, for a GitHub Actions
	// matrix. The workflow expands it with fromJSON and names no image, no
	// port and no release of its own, so there is nothing for it to disagree
	// with container.All about. See D69.
	asJSON := false
	if len(args) > 0 && args[0] == "--json" {
		asJSON, args = true, args[1:]
	}
	all := container.All()
	// The port every server gets, by its place in the whole list rather than
	// in whatever subset was asked for.
	ports := make(map[string]int, len(all))
	for i, s := range all {
		ports[s.Name()] = basePort + i
	}
	servers := all
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
	if asJSON {
		names := make([]string, len(servers))
		for i, s := range servers {
			names[i] = s.Name()
		}
		out, err := json.Marshal(names)
		if err != nil {
			return fmt.Errorf("encoding the names: %w", err)
		}
		fmt.Println(string(out))
		return nil
	}
	for _, s := range servers {
		port := ports[s.Name()]
		commands := [][]string{
			s.RunArgs(s.Name(), port),
			s.ReadyArgs(s.Name()),
			s.RemoveArgs(s.Name()),
		}
		fields := []string{s.Name(), s.DSN(port), envFor(s.Dialect), s.URL(port)}
		for _, cmd := range commands {
			for _, arg := range cmd {
				// A separator inside an argument would split it, which is the
				// fault this format exists to avoid. Nothing produces one, and
				// this says so rather than trusting it.
				if strings.ContainsAny(arg, "\t\n"+sep) {
					return fmt.Errorf("%s: an argument contains a tab, a newline or a separator: %q",
						s.Name(), arg)
				}
			}
			fields = append(fields, strings.Join(cmd, sep))
		}
		for _, f := range fields[:4] {
			if strings.ContainsAny(f, "\t\n"+sep) {
				return fmt.Errorf("%s: a field contains a tab, a newline or a separator: %q",
					s.Name(), f)
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
