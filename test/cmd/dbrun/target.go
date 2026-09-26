package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/container"
)

// kind is what sort of thing a target is, which decides how it starts and
// whether it is kept.
type kind int

const (
	// kindContainer is a database in a container, which is created fresh and
	// removed after a test run because rebuilding it costs a minute.
	kindContainer kind = iota
	// kindMachine is a database on a Windows virtual machine, which is
	// provisioned once over an hour and kept.
	kindMachine
	// kindEmbedded is a library with no server at all. There is nothing to
	// start and the tests run against whatever the driver links.
	kindEmbedded
)

// MarshalJSON writes the name rather than the number, because --json is for
// a person's jq as much as for a program.
//
//nolint:unparam // MarshalJSON's signature is encoding/json's, not ours
func (k kind) MarshalJSON() ([]byte, error) {
	return []byte(`"` + k.String() + `"`), nil
}

func (k kind) String() string {
	switch k {
	case kindContainer:
		return "container"
	case kindMachine:
		return "machine"
	case kindEmbedded:
		return "embedded"
	}
	return "unknown"
}

// target is one thing dbrun can act on, whichever kind it is.
//
// The three kinds share one name space and it already fits: a machine is
// sqlserver-2016 and a container is sqlserver-2017, because Microsoft's Linux
// images begin at 2017. Nothing had to be renamed to put them in one list.
type target struct {
	Name    string         `json:"name"`
	Product string         `json:"product"`
	Release string         `json:"release,omitempty"`
	Kind    kind           `json:"kind"`
	Tier    container.Tier `json:"tier"`
	Dialect dbmeta.Dialect `json:"dialect"`

	// Env is the variable the integration tests read a connection string
	// from, such as DBMETA_POSTGRES.
	Env string `json:"env,omitempty"`
	// DSN is what the driver takes and URL is what a person types. They
	// differ for MySQL and Cassandra, whose drivers take a form that is not
	// a URL.
	DSN string `json:"dsn,omitempty"`
	URL string `json:"url,omitempty"`

	// Run, Ready and Remove are the runner's arguments, for a container.
	Run    []string `json:"-"`
	Ready  []string `json:"-"`
	Remove []string `json:"-"`
	// Init installs the catalog once the server answers, for a product
	// whose catalog is not there until it is. Empty for every product but
	// Apache Hive.
	Init []string `json:"-"`

	// Viewer is the port a machine's screen is on, so that somebody can
	// watch an install that is not finishing.
	Viewer int `json:"viewer,omitempty"`

	// Startup is how long this server needs before it answers, where the
	// default is not enough. It comes from the container list rather than
	// from a flag, because how long a product takes to start is a property
	// of the product and not of the run.
	Startup time.Duration `json:"-"`
}

// basePort is where the published ports start.
//
// A server's port is its position in the whole list plus this, so a release
// always gets the same one however it was selected. Numbering a filtered list
// instead gave every server started on its own the first port, and two of
// them collided. See D68.
const basePort = 55000

// embeddedTargets are the databases that are a library rather than a server.
//
// Which dialects those are is not decided here. Every model declares it with
// [dbmeta.Info.Embedded] and this reads it, so a model added later appears
// without anybody remembering to edit a second list.
//
// They are not in container, because D42 says an embedded database has no
// container and no release to pin and must not be in that list. They are
// here so that `dbrun test sqlite3` works and so that status can say what
// they are rather than leaving somebody wondering why the name is missing.
func embeddedTargets() []target {
	var out []target
	for _, d := range dbmeta.Dialects() {
		if !d.Embedded() {
			continue
		}
		file := filepath.Join(stateDir("DBMETA_EMBEDDED_STATE", "embedded"),
			string(d)+embeddedExt(d))
		out = append(out, target{
			Name: string(d), Product: string(d), Kind: kindEmbedded,
			Tier: container.Tested, Dialect: d,
			Env: "DBMETA_" + strings.ToUpper(string(d)),
			// The driver DSN is the path, because that is what sql.Open takes
			// for both of these. The URL is the opaque form dburl parses, and
			// it is what a person pastes into usql.
			//
			// The scheme is named rather than left as file:. Both products
			// answer to file: and dburl decides which by peeking at the
			// header, or, when the file does not exist yet, by the extension.
			// Naming the scheme says which one in either state.
			DSN: file,
			URL: string(d) + ":" + file,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// embeddedExt is the file extension for a library's database.
//
// It decides which product a file: URL resolves to when the file does not
// exist yet: dburl matches the header first and falls back to the extension,
// and .db is sqlite3 there. A DuckDB database called .db would be opened as
// SQLite by anything following that URL.
//
// A dialect with no entry gets its own name as the extension, which is
// unambiguous because it matches nothing dburl registers.
func embeddedExt(d dbmeta.Dialect) string {
	switch d {
	case dbmeta.SQLite3:
		return ".db"
	case dbmeta.DuckDB:
		return ".duckdb"
	}
	return "." + string(d)
}

// wantPorts is the mapping this target asks for, as container port to host
// port, read back out of the arguments that would create it.
func (t target) wantPorts() map[string]string {
	ports := make(map[string]string, 2)
	for i, arg := range t.Run {
		if arg != "--publish" || i+1 >= len(t.Run) {
			continue
		}
		// host:container, or ip:host:container.
		parts := strings.Split(t.Run[i+1], ":")
		if n := len(parts); n >= 2 {
			ports[parts[n-1]] = parts[n-2]
		}
	}
	return ports
}

// portsMatch reports whether an existing container publishes what this target
// asks for.
//
// A host port is a server's index in container.All, so adding a release
// shifts every release after it. A container created before that keeps the
// old mapping, stays running, and answers nothing on the port everything now
// computes. See D68.
func (t target) portsMatch(have map[string]string) bool {
	want := t.wantPorts()
	if len(want) == 0 {
		return true
	}
	for container, host := range want {
		if have[container] != host {
			return false
		}
	}
	return true
}

// targets returns every target dbrun knows, in a stable order.
func targets() []target {
	servers := container.All()
	embedded := embeddedTargets()
	out := make([]target, 0, len(servers)+len(container.WindowsVMs)+len(embedded))
	for i, s := range servers {
		port := basePort + i
		out = append(out, target{
			Name:    s.Name(),
			Product: s.Product,
			Release: s.Release,
			Kind:    kindContainer,
			Tier:    s.Tier,
			Dialect: s.Dialect,
			Env:     envFor(s.Dialect),
			DSN:     s.DSN(port),
			URL:     s.URL(port),
			Run:     s.RunArgs(s.Name(), port),
			Ready:   s.ReadyArgs(s.Name()),
			Init:    s.InitArgs(s.Name()),
			Startup: s.Startup,
			Remove:  s.RemoveArgs(s.Name()),
		})
	}
	for _, v := range container.WindowsVMs {
		out = append(out, target{
			Name:    v.Name(),
			Product: "sqlserver",
			Release: v.Release,
			Kind:    kindMachine,
			Tier:    v.Tier,
			Dialect: dbmeta.SQLServer,
			Env:     envFor(dbmeta.SQLServer),
			DSN:     v.DSN(),
			URL:     v.DSN(),
			Viewer:  v.Viewer,
		})
	}
	out = append(out, embedded...)
	return out
}

// envFor names the variable the integration tests read a connection string
// from. One per dialect, because one test run reaches one server per dialect.
func envFor(d dbmeta.Dialect) string {
	return "DBMETA_" + strings.ToUpper(string(d))
}

// resolve turns the selectors a caller typed into the targets they name.
//
// A selector is one of four things and there is no flag that changes what any
// of them means. D68's predecessor overloaded --all, which meant one thing
// after a product and another alone, and a flag whose meaning depends on the
// words around it is a flag people get wrong.
//
//	postgres-18   that release
//	postgres      the newest PostgreSQL, and only that
//	tested        a tier
//	all           every release of every product
//
// A bare product resolving to the newest is the one that could surprise
// somebody, because the answer changes the month a release ships. Every
// command that resolves one says so before it acts, which is the difference
// between a default and a trap.
func resolve(all []target, args []string, allReleases bool) ([]target, []string, error) {
	if len(args) == 0 {
		return nil, nil, errors.New("no selector given")
	}
	var (
		out   []target
		notes []string
		seen  = map[string]bool{}
	)
	add := func(t target) {
		if !seen[t.Name] {
			seen[t.Name] = true
			out = append(out, t)
		}
	}
	for _, arg := range args {
		switch {
		case arg == "all", arg == "--all":
			for _, t := range all {
				add(t)
			}
			continue
		case arg == string(container.Tested), arg == string(container.Nightly),
			arg == string(container.Verified):
			for _, t := range all {
				if string(t.Tier) == arg {
					add(t)
				}
			}
			continue
		}
		// An exact name wins over a product, so that a release called the
		// same as a product would still be reachable.
		var exact []target
		for _, t := range all {
			if t.Name == arg {
				exact = append(exact, t)
			}
		}
		if len(exact) > 0 {
			for _, t := range exact {
				add(t)
			}
			continue
		}
		var product []target
		for _, t := range all {
			if t.Product == arg {
				product = append(product, t)
			}
		}
		if len(product) == 0 {
			return nil, nil, fmt.Errorf("nothing is called %q. Try: dbrun list all", arg)
		}
		if allReleases || len(product) == 1 {
			for _, t := range product {
				add(t)
			}
			continue
		}
		newest := newestOf(product)
		notes = append(notes, fmt.Sprintf("%s -> %s", arg, newest.Name))
		add(newest)
	}
	return out, notes, nil
}

// newestOf returns the newest release of one product.
//
// container.All is already sorted by release within a product, so the last
// one is the newest. Sorting again here rather than relying on that keeps
// this correct if the order in that package ever changes, and it has to
// handle the Windows machines, which are a separate list.
func newestOf(product []target) target {
	sorted := append([]target(nil), product...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return dbmeta.ParseVersion(sorted[i].Release).
			Compare(dbmeta.ParseVersion(sorted[j].Release)) < 0
	})
	return sorted[len(sorted)-1]
}
