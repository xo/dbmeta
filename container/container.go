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

	"github.com/xo/dbmeta"
)

const (
	// Password is what every server here is started with. These containers
	// hold fixture data and live for the length of a test run.
	Password = "P4ssw0rd"
	// Registry is where the images come from. podman needs a fully qualified
	// name and docker does not, so [Server.Qualified] spells it out and
	// [Server.Ref] does not.
	Registry = "docker.io/library"
)

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
	// test/run.sh, and CI does not.
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
	// Tier is how often this release is tested.
	Tier Tier

	// Image is the container image, without the tag.
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

	// dsn builds a connection string for a port on the host.
	dsn func(port int) string
}

// Ref returns the image and tag, such as "postgres:18". A CI workflow that
// names an image wants this form.
func (s Server) Ref() string { return s.Image + ":" + s.Tag }

// Qualified returns the image with its registry, such as
// "docker.io/library/postgres:18". podman refuses an unqualified name unless
// the machine is configured to guess, so a command line uses this.
func (s Server) Qualified() string { return Registry + "/" + s.Ref() }

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
	return append(args, s.Qualified())
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

// RemoveArgs returns the arguments that stop and remove the named container.
func (s Server) RemoveArgs(name string) []string {
	return []string{"rm", "--force", name}
}

// HealthCmd returns the readiness command as one string, which is the form a
// GitHub Actions service container wants for its --health-cmd option.
func (s Server) HealthCmd() string { return strings.Join(s.Ready, " ") }

// Name returns a short name for this server, such as "mariadb-11.8". It is
// safe to use as a container name and as a CI job name.
func (s Server) Name() string {
	return strings.ToLower(s.Product) + "-" + s.Release
}

// DSN returns a connection string for this server, reached on port at
// 127.0.0.1. The caller passes the port it mapped, which is not the port
// inside the container when more than one server runs at a time.
func (s Server) DSN(port int) string { return s.dsn(port) }

// Environ returns the environment as NAME=value, sorted, for a command line.
func (s Server) Environ() []string {
	out := make([]string, 0, len(s.Env))
	for _, k := range slices.Sorted(maps.Keys(s.Env)) {
		out = append(out, k+"="+s.Env[k])
	}
	return out
}

// PostgreSQL is every PostgreSQL release dbmeta supports.
//
// All ten majors from 9.6, which is further back than psql itself goes: psql
// dropped 9.6 in release 20, so the queries for it are translated from an
// older checkout. See D20 and hard rule 5.
var PostgreSQL = releases(postgres, Tested, "9.6", "12", "15", "18").
	add(postgres, Nightly, "10", "11", "13", "14", "16", "17")

// MariaDB is the MariaDB releases dbmeta is tested against.
//
// The floor is 10.6, the oldest long term release still maintained. The
// ceiling is 13.0, the current stable. 11.8 is the long term release most
// installations run, and it sits above the 11.5 gate where the view that
// lists sequences arrived.
var MariaDB = releases(mariadb, Tested, "10.6", "13.0").
	add(mariadb, Nightly, "10.11", "11.4", "11.8", "12.3")

// MySQL is the MySQL releases dbmeta is tested against.
//
// 8.4 is the long term release. 26.7 is the current innovation release, which
// is where MySQL's new year based numbering starts: it follows 9.7, and it is
// a larger number than any MariaDB release will reach for years. That is the
// reason a fragment gates on the product key and never on the number. See D44.
//
// 8.4 and 26.7 sit on either side of the only gate this model has for MySQL,
// which is where a system variable moved in release 9.
var MySQL = releases(mysql, Tested, "8.4", "26.7").
	add(mysql, Nightly, "9.7")

// All returns every server, PostgreSQL first.
func All() []Server {
	return slices.Concat(PostgreSQL, MariaDB, MySQL)
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
	port    int
	env     map[string]string
	ready   []string
	dsn     func(port int) string
}

var (
	postgres = product{
		dialect: dbmeta.PostgreSQL,
		name:    "postgres",
		image:   "postgres",
		port:    5432,
		env:     map[string]string{"POSTGRES_PASSWORD": Password},
		// -h forces TCP. On the local socket pg_isready reports ready during
		// the bootstrap phase, before the server restarts to accept network
		// connections, and a test that connects then is refused.
		ready: []string{"pg_isready", "-U", "postgres", "-h", "127.0.0.1"},
		dsn: func(port int) string {
			return fmt.Sprintf("postgres://postgres:%s@127.0.0.1:%d/postgres?sslmode=disable", Password, port)
		},
	}
	mariadb = product{
		dialect: dbmeta.MySQL,
		name:    "mariadb",
		image:   "mariadb",
		port:    3306,
		env:     map[string]string{"MARIADB_ROOT_PASSWORD": Password},
		ready:   []string{"healthcheck.sh", "--connect", "--innodb_initialized"},
		dsn:     mysqlDSN,
	}
	mysql = product{
		dialect: dbmeta.MySQL,
		name:    "mysql",
		image:   "mysql",
		port:    3306,
		env:     map[string]string{"MYSQL_ROOT_PASSWORD": Password},
		ready:   []string{"mysqladmin", "ping", "-h", "127.0.0.1", "-uroot", "-p" + Password},
		dsn:     mysqlDSN,
	}
)

func mysqlDSN(port int) string {
	return fmt.Sprintf("root:%s@tcp(127.0.0.1:%d)/?parseTime=true", Password, port)
}

// releases builds the list for one product.
func releases(p product, tier Tier, versions ...string) list {
	return list(nil).add(p, tier, versions...)
}

// list is a slice of servers under construction, so that the declarations
// above read as one product with its releases grouped by tier.
type list []Server

func (l list) add(p product, tier Tier, versions ...string) list {
	for _, v := range versions {
		l = append(l, Server{
			Dialect: p.dialect,
			Product: p.name,
			Release: v,
			Tier:    tier,
			Image:   p.image,
			Tag:     v,
			Port:    p.port,
			Env:     p.env,
			Ready:   p.ready,
			dsn:     p.dsn,
		})
	}
	slices.SortStableFunc(l, func(a, b Server) int { return compareRelease(a.Release, b.Release) })
	return l
}

// compareRelease orders two release strings by their numbers, so that 9.6
// sorts below 10 and 26.7 above 9.7.
func compareRelease(a, b string) int {
	return dbmeta.ParseVersion(a).Compare(dbmeta.ParseVersion(b))
}
