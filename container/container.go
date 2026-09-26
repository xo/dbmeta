// Package container names the database servers dbmeta is tested against.
//
// It holds data and nothing else. It starts no container, runs no command and
// imports nothing outside the standard library. A caller reads the list and
// decides what to do with it.
//
// It exists because the same list was written out three times: in the CI
// workflow, in the script that runs the release matrix, and by hand at a
// terminal. Three copies drift, and the one that drifts quietly is the one
// that decides which releases a release was tested against. Now there is one
// copy and the others are checked against it.
//
// A downstream project reads this to run the same servers. dbtpl generates
// against a live database and needs one of each. usql tests a metacommand
// against every product. Both want the list dbmeta itself uses, rather than a
// list of their own that ages differently.
//
// # Reading it
//
//	for _, s := range container.All() {
//		fmt.Println(s.Ref(), s.Tier, s.DSN(s.Port))
//	}
//
// # Launching one
//
// [Server.RunArgs] and [Server.ReadyArgs] return the arguments a container
// command takes, as strings. They run nothing. The caller brings its own
// client, whether that is podman, docker, an exec.Command over either, or a Go
// client library, and dbmeta never depends on any of them. A consumer picks
// its own and picks its own version of it, the same way it picks its database
// driver.
//
//	s := container.MariaDB[0]
//	out, err := exec.CommandContext(ctx, "podman", s.RunArgs("mdb", 33306)...).Output()
//
// # What it does not hold
//
// No password policy and no fixed port. [Server.DSN] takes the port the caller
// mapped, because a caller running six servers at once maps six ports and only
// the caller knows which. [Password] is one value for every server here,
// because these are throwaway containers on a development machine and nothing
// in them is worth protecting.
package container

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/xo/dbmeta"
)

// Password is what every server here is started with. These containers hold
// fixture data and live for the length of a test run, so nothing in them is
// worth protecting.
//
// It is shaped to clear the strictest policy any of these products enforces,
// which is SQL Server's, so that one value works everywhere. It is ten
// characters and it carries all four classes: an upper case letter, lower
// case letters, digits and a symbol. SQL Server refuses a password that is
// too short or that draws on too few classes, and it refuses to start at all
// rather than starting without a usable sa, so getting this wrong looks like
// a broken image.
//
// Anything with a password policy tends to ask for some subset of the same
// four things, so a value that satisfies SQL Server satisfies the rest. Keep
// all four classes and the length if this ever changes.
//
// There used to be a second constant for SQL Server alone, and the cost of it
// was a reader having to know which product took which. The symbol is escaped
// where a DSN is a URL, which is why each one builds through
// [net/url.QueryEscape] rather than concatenating.
const Password = "P4ssw0rd!x"

// MemoryLimit is how much memory one of these containers is allowed.
//
// It is one value for every product, the way [Password] is, and for the same
// reason: these are throwaway servers on a development machine and none of
// them is doing real work. A database given the whole machine will take it,
// and several at once then push the host into swap, which looks like a slow
// server rather than an overcommitted one.
//
// Four gigabytes, which every product here answers within but one.
//
// SAP HANA is the exception and [Server.Memory] carries it. An exception is
// allowed only where the product was measured and the number written down
// beside it, never because a server looked slow.
const MemoryLimit = "4g"

// Tier is how thoroughly a release is tested. It is the support tier from D40,
// attached to the thing that does the testing.
type Tier string

// Tiers.
const (
	// Tested means CI runs this release on every push.
	Tested Tier = "tested"
	// Nightly means CI runs this release once a night.
	Nightly Tier = "nightly"
	// Verified means a person runs this release before a release, with
	// dbrun, and CI does not.
	Verified Tier = "verified"
)

// Server is one database release, and the container image that holds it.
type Server struct {
	// Dialect selects the model that reads this server.
	Dialect dbmeta.Dialect
	// Product is the name the server reports for itself, such as MariaDB.
	// Two products share a dialect where they are wire compatible.
	Product string
	// Release is the version to run, written the way the image tags it.
	Release string
	// Major is the release as a person says it, and is what names a container
	// and a CI job. It is the Release for every product but Oracle, whose
	// images are tagged with a four part version: 11.2.0.2 is 11, 19.3.0 is
	// 19. PostgreSQL is why this is not simply the part before the first dot,
	// because 9.6 is a major and 9 is not a release at all.
	Major string
	// Tier is how often this release is tested.
	Tier Tier

	// Image is the container image, fully qualified and without the tag, as
	// in "docker.io/library/postgres". The registry is always written out:
	// podman refuses an unqualified name unless the machine is configured to
	// guess, and a name that means one thing to docker and another to podman
	// is not metadata.
	Image string
	// Tag is the tag to pull. It is usually Release and is not always,
	// because an image can tag a release under a name of its own.
	Tag string
	// Port is the port the server listens on inside the container.
	Port int
	// Env is what the image needs to start with a known password.
	Env map[string]string
	// Ready is a command that succeeds once the server accepts a connection.
	// Run it inside the container. A health check that tests the local socket
	// reports ready too early on several of these images, so each one
	// connects over TCP.
	Ready []string
	// RunFlags are extra flags for the run command, before the image name.
	// Empty for every product but SAP HANA, which will not start under the
	// default open file limit.
	RunFlags []string
	// Init is a command run inside the container once it is ready, for a
	// server whose catalog is not there until it is installed.
	//
	// Apache Hive is the only one. Its metastore is relational and it is
	// not readable through SQL until the sys database is created over it,
	// and the script that does that ships in the image and needs a
	// running HiveServer2 to run against. That makes it a step after the
	// server answers rather than a layer in an image, which is where the
	// two built images here put their setup.
	//
	// It must be safe to run twice, because start runs it every time.
	Init []string
	// Memory is what this server is allowed, where MemoryLimit is not
	// enough. Empty means MemoryLimit.
	Memory string
	// Startup is how long this server needs before it answers, where the
	// usual budget is not enough. Zero means the caller's default.
	//
	// SAP HANA is the only one that sets it. It took 108 seconds on a warm
	// image on a development machine, which is past the 90 seconds every
	// other product here answers within, and a cold or loaded host is
	// slower still.
	Startup time.Duration
	// Args are arguments for the image's own entrypoint, after the image
	// name. Empty for every product but SAP HANA, whose entrypoint takes the
	// initial password on the command line and reads no environment variable
	// for it.
	//
	// This is the container's argument list rather than a shell command. A
	// value containing a space is one argument here and stays one, because
	// nothing joins them.
	Args []string

	// dsn builds a connection string for a port on the host.
	dsn func(port int) string
	// url is the dburl style URL a person types, where that differs from the
	// DSN the driver takes. Nil means they are the same string.
	//
	// This is not the scheme list hard rule 1 forbids. That rule is about
	// repeating dburl's taxonomy of schemes and aliases, which decides what a
	// URL somebody typed means. This is one connection string per server this
	// package already starts, in the second form a person needs, beside the
	// one the driver needs.
	url func(port int) string
}

// MemoryOrDefault is what this server is allowed, which is [MemoryLimit]
// unless the product was measured to need more.
func (s Server) MemoryOrDefault() string {
	if s.Memory != "" {
		return s.Memory
	}
	return MemoryLimit
}

// Ref returns the image and tag, fully qualified, such as
// "docker.io/library/postgres:18". It is what a command line and a CI workflow
// both want.
func (s Server) Ref() string { return s.Image + ":" + s.Tag }

// RunArgs returns the arguments that start this server detached, under the
// container name given, with its port published on hostPort. They go after
// the name of the command, so a caller runs podman or docker with them and
// dbmeta depends on neither.
//
// The container is named rather than left anonymous, because [Server.ReadyArgs]
// and the command that removes it both need the name.
func (s Server) RunArgs(name string, hostPort int) []string {
	args := []string{"run", "--detach", "--name", name}
	for _, e := range s.Environ() {
		args = append(args, "--env", e)
	}
	args = append(args, "--publish", fmt.Sprintf("%d:%d", hostPort, s.Port))
	args = append(args, "--memory", s.MemoryOrDefault())
	args = append(args, s.RunFlags...)
	args = append(args, s.Ref())
	return append(args, s.Args...)
}

// ReadyArgs returns the arguments that ask the named container whether it
// accepts connections yet. The command exits zero once it does, so a caller
// runs it in a loop.
//
// Waiting is not optional and a fixed pause does not do it. Several of these
// images start, bootstrap a data directory, and restart, and a connection made
// between the two is refused.
func (s Server) ReadyArgs(name string) []string {
	return append([]string{"exec", name}, s.Ready...)
}

// InitArgs returns the arguments that install the catalog in the named
// container, or nil when the product needs nothing. Run it after
// [Server.ReadyArgs] succeeds.
func (s Server) InitArgs(name string) []string {
	if len(s.Init) == 0 {
		return nil
	}
	return append([]string{"exec", name}, s.Init...)
}

// RemoveArgs returns the arguments that stop and remove the named container.
func (s Server) RemoveArgs(name string) []string {
	return []string{"rm", "--force", name}
}

// HealthCmd returns the readiness command as one string, which is the form a
// GitHub Actions service container wants for its --health-cmd option.
//
// An argument containing a space is quoted, because the receiving end is a
// shell. Joining on a space without quoting turned the SQL Server query
// "SELECT 1" into two arguments and produced a command that always failed.
func (s Server) HealthCmd() string {
	out := make([]string, 0, len(s.Ready))
	for _, arg := range s.Ready {
		if strings.ContainsAny(arg, " \t\"'") {
			out = append(out, "'"+strings.ReplaceAll(arg, "'", `'\''`)+"'")
			continue
		}
		out = append(out, arg)
	}
	return strings.Join(out, " ")
}

// Name returns a short name for this server, such as "mariadb-11.8" or
// "oracle-19". It is safe as a container name and as a CI job name.
//
// It uses the major rather than the full release, so that every name in a
// podman listing reads the same way. Oracle is the reason: its images are
// tagged 11.2.0.2 and 19.3.0, and a listing holding oracle-11.2.0.2 beside
// oracle-23 tells a reader nothing the shorter name does not.
func (s Server) Name() string {
	return strings.ToLower(s.Product) + "-" + s.Major
}

// DSN returns a connection string for this server, reached on port at
// 127.0.0.1. The caller passes the port it mapped, which is not the port
// inside the container when more than one server runs at a time.
func (s Server) DSN(port int) string { return s.dsn(port) }

// URL returns the dburl style URL for the server on this port, which is what
// usql takes. For most products it is the DSN, because the driver takes a URL
// too. MySQL and Cassandra are the exceptions: their drivers take a form that
// is not a URL at all.
func (s Server) URL(port int) string {
	if s.url != nil {
		return s.url(port)
	}
	return s.dsn(port)
}

// Environ returns the environment as NAME=value, sorted, for a command line.
func (s Server) Environ() []string {
	out := make([]string, 0, len(s.Env))
	for _, k := range slices.Sorted(maps.Keys(s.Env)) {
		out = append(out, k+"="+s.Env[k])
	}
	return out
}

// All returns every server, PostgreSQL first.
func All() []Server {
	return slices.Concat(PostgreSQL, MariaDB, MySQL, SQLServer, Oracle, Cassandra, ClickHouse, Trino, Presto, Firebird, HANA, Hive)
}

// AtTier returns the servers tested at t.
func AtTier(t Tier) []Server {
	var out []Server
	for _, s := range All() {
		if s.Tier == t {
			out = append(out, s)
		}
	}
	return out
}

// ForDialect returns the servers one model reads. Two products share a
// dialect, so this returns both of them.
func ForDialect(d dbmeta.Dialect) []Server {
	var out []Server
	for _, s := range All() {
		if s.Dialect == d {
			out = append(out, s)
		}
	}
	return out
}

// Releases returns the releases of one product, oldest first.
func Releases(servers []Server) []string {
	out := make([]string, len(servers))
	for i, s := range servers {
		out[i] = s.Release
	}
	return out
}

// The three products, as the parts that do not change per release.
type product struct {
	dialect dbmeta.Dialect
	name    string
	image   string
	// major turns a release into the name a person uses for it. Nil means
	// they are the same, which is true of every product but Oracle.
	major func(release string) string
	// tagSuffix is appended to the release to make the tag, and is usually
	// empty. Microsoft publishes no bare release tag for SQL Server: the tag
	// is 2017-latest and there is no 2017.
	tagSuffix string
	port      int
	env       map[string]string
	ready     []string
	init      []string
	runFlags  []string
	args      []string
	memory    string
	startup   time.Duration
	dsn       func(port int) string
	// url is the dburl style URL, where it differs from the DSN. See
	// [Server.URL].
	url func(port int) string
}

// list is a slice of servers under construction, so that the declarations
// above read as one product with its releases grouped by tier. Every group
// names its tier, including the first, because which release sits in which
// tier is the one thing this package exists to state.
type list []Server

func (l list) add(p product, tier Tier, versions ...string) list {
	for _, v := range versions {
		major := v
		if p.major != nil {
			major = p.major(v)
		}
		l = append(l, Server{
			Dialect:  p.dialect,
			Product:  p.name,
			Release:  v,
			Major:    major,
			Tier:     tier,
			Image:    p.image,
			Tag:      v + p.tagSuffix,
			Port:     p.port,
			Env:      p.env,
			Ready:    p.ready,
			Init:     p.init,
			RunFlags: p.runFlags,
			Args:     p.args,
			Memory:   p.memory,
			Startup:  p.startup,
			dsn:      p.dsn,
			url:      p.url,
		})
	}
	slices.SortStableFunc(l, func(a, b Server) int { return compareRelease(a.Release, b.Release) })
	return l
}

// on returns l with fn applied to the one release named. It exists because a
// product is usually uniform and is not always: the SQL Server 2017 image is
// built on an older base and ships sqlcmd at another path.
func (l list) on(release string, fn func(*Server)) list {
	for i := range l {
		if l[i].Release == release {
			fn(&l[i])
		}
	}
	return l
}

// compareRelease orders two release strings by their numbers, so that 9.6
// sorts below 10 and 26.7 above 9.7.
func compareRelease(a, b string) int {
	return dbmeta.ParseVersion(a).Compare(dbmeta.ParseVersion(b))
}
