package main

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/container"
	"github.com/xo/dbmeta/hosted"
)

// kind is what sort of thing a target is, which decides how it starts and
// whether it is kept.
type kind int

const (
	// kindContainer is a database in a container, which is created fresh and
	// removed after a test run because rebuilding it costs a minute.
	kindContainer kind = iota
	// kindMachine is a database on a virtual machine, which is provisioned
	// once and kept. A Windows machine takes an hour to build and an
	// appliance takes minutes to import, and both are kept for that.
	kindMachine
	// kindEmbedded is a library with no server at all. There is nothing to
	// start and the tests run against whatever the driver links.
	kindEmbedded
	// kindHosted is a service that runs somewhere else, reached with a
	// connection string that a person provisions. There is nothing to start,
	// and it exists only while its connection string resolves. See D117.
	kindHosted
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
	case kindHosted:
		return "hosted"
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
	// Cadence is how often a Staged target will be tested if a model reads
	// it, and is empty for any other. dbimp runs the tested ones on each
	// push and the nightly ones at night. See D120.
	Cadence container.Tier `json:"cadence,omitempty"`
	Dialect dbmeta.Dialect `json:"dialect"`

	// Directory says an embedded database is a directory rather than a
	// file, as chai and csvq are.
	Directory bool `json:"directory,omitempty"`

	// Env is the variable the integration tests read a connection string
	// from, such as DBMETA_POSTGRES.
	Env string `json:"env,omitempty"`
	// Also is every other dialect the server answers, and AlsoEnv is the
	// variable of each. dbrun sets each to the same connection string. See
	// D114.
	Also    []dbmeta.Dialect `json:"also,omitempty"`
	AlsoEnv []string         `json:"alsoEnv,omitempty"`
	// DSN is what the driver takes and URL is what a person types. They
	// differ where a driver takes another form, such as MySQL's, which is
	// not a URL.
	DSN string `json:"dsn,omitempty"`
	URL string `json:"url,omitempty"`
	// API is the address of the server's HTTP API, which dbimp's tools read,
	// and is empty for a server that has none. See container.Server.API and
	// D167.
	API string `json:"api,omitempty"`
	// SecondAddress is the host and port that a container's second port is
	// published on, such as the controller of Pinot, and is empty when the
	// container has no second port. See D124.
	SecondAddress string `json:"secondAddress,omitempty"`
	// Credential says where a hosted service's connection string came from,
	// such as env DBMETA_SNOWFLAKE_DSN, and never holds the secret. DSN and
	// URL hold the connection string with its secret masked, and secret
	// holds it whole, as the URL a person pastes. driverDSN holds what
	// sql.Open takes, from the same URL, for the commands that connect.
	// See D117 and D167.
	Credential string `json:"credential,omitempty"`
	// License is the license file on the host that dbrun mounts, for a
	// product that does not start without one. See D118.
	License   string `json:"license,omitempty"`
	secret    string
	driverDSN string

	// Principals is every user a test reaches the server as, the
	// administrator first, with the connection string of each. It is empty
	// for a machine and an embedded database. See D102.
	Principals []principal `json:"principals,omitempty"`

	// Run, Ready and Remove are the runner's arguments, for a container.
	Run    []string `json:"-"`
	Ready  []string `json:"-"`
	Remove []string `json:"-"`
	// Init installs the catalog once the server answers, for a product
	// whose catalog is not there until it is. Empty for every product but
	// Apache Hive.
	Init []string `json:"-"`
	// InitInput is sent to Init on its standard input. See
	// container.Server.InitInput.
	InitInput string `json:"-"`

	// Viewer is the port a machine's screen is on, so that somebody can
	// watch an install that is not finishing.
	Viewer int `json:"viewer,omitempty"`
	// Rebuild says what building a machine again costs, for the messages
	// that refuse to do it without asking. Empty for anything else.
	Rebuild string `json:"-"`

	// Startup is how long this server needs before it answers, where the
	// default is not enough. It comes from the container list rather than
	// from a flag, because how long a product takes to start is a property
	// of the product and not of the run.
	Startup time.Duration `json:"-"`

	// Settle is how long Ready has to keep passing before the server counts
	// as up. Zero means the first pass is enough. See D83.
	Settle time.Duration `json:"-"`
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
	for _, e := range unmodeled {
		path := filepath.Join(stateDir("DBMETA_EMBEDDED_STATE", "embedded"), e.name+e.ext)
		// A library that a model reads, such as moderncsqlite, which the
		// sqlite3 model reads, is tested in CI. The rest are Staged, as a
		// server is that no model reads. See D119.
		tier, cadence := container.Staged, container.Tested
		if _, read := e.dialect.Info(); read {
			tier, cadence = container.Tested, ""
		}
		out = append(out, target{
			Name: e.name, Product: e.name, Kind: kindEmbedded,
			Tier: tier, Cadence: cadence, Dialect: e.dialect, Directory: e.ext == "",
			Env: "DBMETA_" + strings.ToUpper(e.name),
			DSN: path, URL: e.name + ":" + path,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// unmodeled are the embedded databases that have no dbmeta model yet, so no
// model declares them. Ken asked on 2026-09-28 for dbrun to know them before a
// dialect is written for them, and the list goes when a model does. The
// dialect is the one dburl names. moderncsqlite is SQLite without cgo, which
// dburl gives the dialect sqlite3, and it has a file of its own beside the
// sqlite3 entry. A database with no extension is a directory. See D116.
var unmodeled = []struct {
	name    string
	dialect dbmeta.Dialect
	ext     string
}{
	{name: "chai", dialect: dbmeta.Chai},
	{name: "csvq", dialect: dbmeta.CSVQ},
	{name: "moderncsqlite", dialect: dbmeta.SQLite3, ext: ".db"},
}

// embeddedExt is the file extension for a library's database.
//
// It decides which product a file: URL resolves to when the file does not
// exist yet: dburl matches the header first and falls back to the extension,
// and .db is sqlite3 there. Anything that follows that URL opens a DuckDB
// database called .db as SQLite.
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
// port, read back out of the arguments that create it.
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

// targets returns every target dbrun knows, in the order sortTargets gives.
func targets() []target {
	servers := container.All()
	embedded := embeddedTargets()
	out := make([]target, 0, len(servers)+len(container.Machines())+len(embedded))
	for i, s := range servers {
		port := basePort + i
		var flags []string
		license := ""
		if s.License != "" {
			p, ok, err := resolveLicense(s.Product)
			if err != nil {
				fmt.Fprintln(os.Stderr, "dbrun:", s.Name(), "is not available:", err)
			}
			if !ok {
				continue
			}
			license = p
			flags = licenseMount(p, s.License)
		}
		out = append(out, target{
			Name:          s.Name(),
			Product:       s.Product,
			Release:       s.Release,
			Kind:          kindContainer,
			Tier:          s.Tier,
			Cadence:       s.Cadence,
			Dialect:       s.Dialect,
			Env:           envFor(s.Dialect, s.Product),
			Also:          s.Also,
			AlsoEnv:       alsoEnv(s.Also),
			DSN:           s.DSN(port),
			URL:           s.URL(port),
			API:           s.API(port),
			SecondAddress: secondAddress(s, port),
			Principals:    principalsOf(s, port),
			Run:           s.RunArgs(s.Name(), port, flags...),
			License:       license,
			Ready:         s.ReadyArgs(s.Name()),
			Init:          s.InitArgs(s.Name()),
			InitInput:     s.InitInput,
			Startup:       s.Startup,
			Settle:        s.Settle,
			Remove:        s.RemoveArgs(s.Name()),
		})
	}
	for _, m := range container.Machines() {
		out = append(out, target{
			Name:    m.Name(),
			Product: strings.ToLower(m.Product),
			Release: m.Release,
			Kind:    kindMachine,
			Tier:    m.Tier,
			Dialect: m.Dialect,
			Env:     envFor(m.Dialect, m.Product),
			DSN:     m.DSN(),
			URL:     m.URL(),
			Viewer:  m.Viewer,
			Rebuild: rebuildCost(m),
			Startup: m.Startup,
		})
	}
	out = append(out, embedded...)
	out = append(out, hostedTargets()...)
	sortTargets(out)
	return out
}

// connectDSN is the connection string a command connects with. It is what
// sql.Open takes with the secret in it for a hosted service, and the DSN for
// everything else.
func (t target) connectDSN() string {
	if t.Kind == kindHosted {
		return t.driverDSN
	}
	return t.DSN
}

// principal is one user of a server, as list and dsn print it.
type principal struct {
	Role string `json:"role"`
	User string `json:"user,omitempty"`
	DSN  string `json:"dsn"`
	URL  string `json:"url"`
	API  string `json:"api,omitempty"`
}

// principalsOf lists every principal of a container server on its host port.
func principalsOf(s container.Server, port int) []principal {
	var out []principal
	for _, p := range s.Principals() {
		out = append(out, principal{Role: p.Role, User: p.User, DSN: p.DSN(port), URL: p.URL(port), API: p.API(port)})
	}
	return out
}

// secondAddress is the address a server's second port is published on, and
// empty when it has none.
func secondAddress(s container.Server, port int) string {
	if s.SecondPort == 0 {
		return ""
	}
	return fmt.Sprintf("127.0.0.1:%d", container.SecondHostPort(port))
}

// rebuildCost says what building a machine again costs.
func rebuildCost(m container.Machine) string {
	if m.Appliance != nil {
		return "importing it again takes minutes and needs " + m.Appliance.File
	}
	return "provisioning it again takes about an hour"
}

// envFor names the variable the integration tests read a connection string
// from. One per dialect, because one test run reaches one server per dialect.
//
// A server whose dialect is not settled yet is named for its product. Several
// servers are listed for dbimp's drivers before dburl has a scheme for them,
// as Neo4j was before D109. See D112.
func envFor(d dbmeta.Dialect, product string) string {
	if d == "" {
		return "DBMETA_" + strings.ToUpper(product)
	}
	return "DBMETA_" + strings.ToUpper(string(d))
}

// alsoEnv names the variable of each other dialect a server answers.
func alsoEnv(ds []dbmeta.Dialect) []string {
	var out []string
	for _, d := range ds {
		out = append(out, envFor(d, ""))
	}
	return out
}

// env is every variable the tests read for this target, each set to its DSN.
func (t target) env() []string {
	out := []string{t.Env + "=" + t.connectDSN()}
	for _, e := range t.AlsoEnv {
		out = append(out, e+"="+t.connectDSN())
	}
	return out
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
// A bare product resolving to the newest is the one that can surprise
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
			arg == string(container.Verified), arg == string(container.Staged):
			for _, t := range all {
				if string(t.Tier) == arg {
					add(t)
				}
			}
			continue
		}
		// An exact name wins over a product, so that a release called the
		// same as a product is still reachable.
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
// handle the machines, which are a separate list.
func newestOf(product []target) target {
	sorted := append([]target(nil), product...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return dbmeta.ParseVersion(sorted[i].Release).
			Compare(dbmeta.ParseVersion(sorted[j].Release)) < 0
	})
	return sorted[len(sorted)-1]
}

// kindRank orders the kinds in a listing: the libraries first, then the
// servers, whether a container or a machine runs one, and the hosted services
// last.
func kindRank(k kind) int {
	switch k {
	case kindEmbedded:
		return 0
	case kindHosted:
		return 2
	}
	return 1
}

// sortTargets orders targets by kind, as kindRank says, then by product in
// alphabetical order, then by release, oldest first. A release is compared by
// its numbers, so that postgres-9.6 comes before postgres-10. The host port of
// a server does not depend on this order, because it is fixed by the server's
// place in container.All.
func sortTargets(ts []target) {
	slices.SortStableFunc(ts, func(a, b target) int {
		return cmp.Or(
			cmp.Compare(kindRank(a.Kind), kindRank(b.Kind)),
			cmp.Compare(a.Product, b.Product),
			dbmeta.ParseVersion(a.Release).Compare(dbmeta.ParseVersion(b.Release)),
			cmp.Compare(a.Name, b.Name),
		)
	})
}

// nameWidth is the width of the name column in what dbrun prints: the
// longest name among the servers, the machines, the libraries and the hosted
// services, and at least 20. Every command pads to it, so that the columns
// line up whichever targets it prints.
var nameWidth = sync.OnceValue(func() int {
	w := 20
	for _, s := range container.All() {
		w = max(w, len(s.Name()))
	}
	for _, m := range container.Machines() {
		w = max(w, len(m.Name()))
	}
	for _, t := range embeddedTargets() {
		w = max(w, len(t.Name))
	}
	for _, s := range hosted.All() {
		w = max(w, len(s.Name))
	}
	return w
})
