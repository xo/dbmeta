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
	"net/url"
	"slices"
	"strings"

	"github.com/xo/dbmeta"
)

const (
	// Password is what every server here is started with. These containers
	// hold fixture data and live for the length of a test run.
	Password = "P4ssw0rd"
	// SQLServerPassword is what a SQL Server container is started with. It is
	// not [Password], because SQL Server refuses a password without a symbol.
	SQLServerPassword = "P4ssw0rd!x"
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

	// dsn builds a connection string for a port on the host.
	dsn func(port int) string
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
	return append(args, s.Ref())
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
var PostgreSQL = list{}.add(postgres, Tested, "9.6", "12", "15", "18").
	add(postgres, Nightly, "10", "11", "13", "14", "16", "17")

// MariaDB is the MariaDB releases dbmeta is tested against.
//
// The floor is 10.6, the oldest long term release still maintained. The
// ceiling is 13.0, the current stable. 11.8 is the long term release most
// installations run, and it sits above the 11.5 gate where the view that
// lists sequences arrived.
var MariaDB = list{}.add(mariadb, Tested, "10.6", "13.0").
	add(mariadb, Nightly, "10.11", "11.4", "11.8", "12.3")

// SQLServer is the Microsoft SQL Server releases dbmeta is tested against.
//
// Every major release that ships a Linux container, and all of them at the
// Tested tier. That is four jobs rather than two, and it buys the whole claim:
// dbmeta is tested on every SQL Server a person can run on Linux.
//
// 2017 is the floor and it is a hard one. Microsoft shipped SQL Server on
// Linux from 2017, so 2016 and earlier have no container and cannot be tested
// at all. D54 says what is claimed for them, which is less than support.
//
// Splitting these across tiers would have saved little. Every version gate the
// model has sits below 2017, so the releases here differ by what they added
// rather than by what they lack, and the newest is the one most likely to
// break. See D54.
var SQLServer = list{}.add(sqlserver, Tested, "2017", "2019", "2022", "2025").
	// 2017 is the one image built on Ubuntu 16.04. It ships the older sqlcmd,
	// at /opt/mssql-tools rather than /opt/mssql-tools18.
	on("2017", func(s *Server) { s.Ready = sqlcmd("/opt/mssql-tools/bin/sqlcmd") })

// MySQL is the MySQL releases dbmeta is tested against.
//
// 8.4 is the long term release. 26.7 is the current innovation release, which
// is where MySQL's new year based numbering starts: it follows 9.7, and it is
// a larger number than any MariaDB release will reach for years. That is the
// reason a fragment gates on the product key and never on the number. See D44.
//
// 8.4 and 26.7 sit on either side of the only gate this model has for MySQL,
// which is where a system variable moved in release 9.
var MySQL = list{}.add(mysql, Tested, "8.4", "26.7").
	add(mysql, Nightly, "9.7")

// All returns every server, PostgreSQL first.
func All() []Server {
	return slices.Concat(PostgreSQL, MariaDB, MySQL, SQLServer, Oracle)
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
	dsn       func(port int) string
}

var (
	postgres = product{
		dialect: dbmeta.PostgreSQL,
		name:    "postgres",
		image:   "docker.io/library/postgres",
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
		image:   "docker.io/library/mariadb",
		port:    3306,
		env:     map[string]string{"MARIADB_ROOT_PASSWORD": Password},
		ready:   []string{"healthcheck.sh", "--connect", "--innodb_initialized"},
		dsn:     mysqlDSN,
	}
	mysql = product{
		dialect: dbmeta.MySQL,
		name:    "mysql",
		image:   "docker.io/library/mysql",
		port:    3306,
		env:     map[string]string{"MYSQL_ROOT_PASSWORD": Password},
		ready:   []string{"mysqladmin", "ping", "-h", "127.0.0.1", "-uroot", "-p" + Password},
		dsn:     mysqlDSN,
	}
	// The Express images, which carry 11g through 21c.
	oraclexe = product{
		dialect: dbmeta.Oracle,
		name:    "oracle",
		image:   "docker.io/gvenzl/oracle-xe",
		major:   oracleMajor,
		// slim leaves out the sample schemas, which nothing here reads and
		// which cost a gigabyte and a minute of startup.
		tagSuffix: "-slim",
		port:      1521,
		env:       map[string]string{"ORACLE_PASSWORD": Password},
		ready:     oracleReady("XE"),
		dsn:       oracleService("XE"),
	}
	// 23ai, which Oracle calls Free rather than Express.
	oraclefree = product{
		dialect:   dbmeta.Oracle,
		name:      "oracle",
		image:     "docker.io/gvenzl/oracle-free",
		major:     oracleMajor,
		tagSuffix: "-slim",
		port:      1521,
		env:       map[string]string{"ORACLE_PASSWORD": Password},
		ready:     oracleReady("FREE"),
		dsn:       oracleService("FREE"),
	}
	// 19c, built locally from Oracle's Dockerfiles because Oracle publishes no
	// free image of it. test/oracle/build-19c.sh makes it, and this names what
	// that script produces.
	oracle19 = product{
		dialect: dbmeta.Oracle,
		name:    "oracle",
		image:   "localhost/oracle/database",
		major:   oracleMajor,
		// buildContainerImage.sh tags an enterprise build this way.
		tagSuffix: "-ee",
		port:      1521,
		env:       map[string]string{"ORACLE_PWD": Password},
		ready:     oracleReady("ORCLCDB"),
		dsn:       oracleService("ORCLCDB"),
	}
	sqlserver = product{
		dialect: dbmeta.SQLServer,
		name:    "sqlserver",
		// Microsoft's own registry rather than Docker Hub, and the image is
		// the only one there is: there is no community SQL Server.
		image: "mcr.microsoft.com/mssql/server",
		// Microsoft tags every release "-latest" and publishes no bare tag.
		tagSuffix: "-latest",
		port:      1433,
		// The password has to satisfy the SQL Server policy, which wants a
		// symbol, so it is not the shared one.
		env: map[string]string{
			"ACCEPT_EULA":       "Y",
			"MSSQL_SA_PASSWORD": SQLServerPassword,
			"MSSQL_PID":         "Developer",
		},
		ready: sqlcmd("/opt/mssql-tools18/bin/sqlcmd"),
		dsn: func(port int) string {
			return fmt.Sprintf(
				"sqlserver://sa:%s@127.0.0.1:%d?database=master&encrypt=disable",
				url.QueryEscape(SQLServerPassword), port)
		},
	}
)

// sqlcmd is the readiness command for a SQL Server image, at the path that
// image installs sqlcmd to. -C trusts the server certificate, which both the
// old client and the new one accept.
func sqlcmd(path string) []string {
	return []string{
		path, "-S", "localhost",
		"-U", "sa", "-P", SQLServerPassword, "-C", "-Q", "SELECT 1",
	}
}

// oracleNames maps a release to the name Oracle sells it under.
//
// It is a table rather than a rule, because there is no rule. The suffix moved
// from g to c to ai, and 26ai is version 23.26 while 23ai is version 23.9, so
// two products share the major 23 and the number does not say which is which.
// Anything derived from the digits gets that pair wrong.
//
// A release with no entry keeps its full version, which makes the name carry a
// patch level and fails TestNamesCarryOnlyTheMajor. That is deliberate: adding
// a release without saying what Oracle calls it should not pass quietly.
var oracleNames = map[string]string{
	"11.2.0.2": "11g",
	"18.4.0":   "18c",
	"19.3.0":   "19c",
	"21.3.0":   "21c",
	"23.9":     "23ai",
	"23.26.3":  "26ai",
}

func oracleMajor(release string) string {
	if name, ok := oracleNames[release]; ok {
		return name
	}
	return release
}

func mysqlDSN(port int) string {
	return fmt.Sprintf("root:%s@tcp(127.0.0.1:%d)/?parseTime=true", Password, port)
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
			Dialect: p.dialect,
			Product: p.name,
			Release: v,
			Major:   major,
			Tier:    tier,
			Image:   p.image,
			Tag:     v + p.tagSuffix,
			Port:    p.port,
			Env:     p.env,
			Ready:   p.ready,
			dsn:     p.dsn,
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
