package container

import (
	"fmt"
	"net/url"

	"github.com/xo/dbmeta"
)

// The Oracle releases dbmeta is tested against.
//
// Oracle needs no virtual machine and no Windows, which is the opposite of SQL
// Server. Microsoft shipped SQL Server on Linux from 2017 and everything older
// needs a Windows machine under D57. Oracle has run on Linux since long before
// any release here, and the free Express images reach back to 11g Release 2
// from 2010, so every release below is an ordinary container.
//
// # Editions
//
// These are Express and Free editions, and that is enough. Both reviews said
// so for the same reason: the data dictionary is built by the same catproc.sql
// on every edition, so ALL_TABLES, ALL_TAB_COLUMNS, ALL_CONSTRAINTS,
// ALL_INDEXES, ALL_SEQUENCES, ALL_PROCEDURES, ALL_ARGUMENTS and ALL_TAB_PRIVS
// are the same views with the same columns. A feature Express does not have,
// such as partitioning, still has its view, and the view returns no rows.
//
// 19c is the exception and it is not an edition problem. It is the long term
// release most installations run and Oracle publishes no free image of it, so
// it is built locally from Oracle's own Dockerfiles and the installer archive.
// See test/oracle/README.md.
//
// # Service names, and why every one of them is a pluggable database
//
// A connection names a service rather than a database, and from 12c a service
// reaches either the container database or one pluggable database inside it.
// Which one it reaches decides what the tests test, and it is not a detail.
//
// A user created in the container root is a common user: it exists in the
// root and in every pluggable database at once, and it is the Oracle
// equivalent of a SQL Server server login. A user created in a pluggable
// database is a local user, which authenticates against that database and
// has nothing at the container level, and it is the equivalent of a SQL
// Server contained database user.
//
// Every service here names a pluggable database, so the fixture user is
// local on every release that has the concept. That is what a consumer
// connects to: an application names a service and that service is a
// pluggable database on any multitenant install.
//
// It was not always so. XE answered as CDB$ROOT and FREE likewise, so 18c,
// 21c, 23ai and 26ai built the fixture as a common user while 19c built it as
// a local one, and nobody had chosen that. The gvenzl images set
// common_user_prefix to empty, which is why creating a plainly named user in
// the root succeeded there and raised ORA-65096 on the 19c image, whose
// Dockerfile does not.
//
// 11g is the exception and needs none of this. It is older than multitenant,
// so it has no container database, no pluggable database and no COMMON
// column, and XE is the whole instance.
//
// Nothing now tests a connection to CDB$ROOT. That is a real thing a DBA does
// and it is worth a target of its own, named so that a failure reads root
// rather than a release. It is not written yet.

// oracleService builds a connection string for a service on a port.
func oracleService(service string) func(port int) string {
	return func(port int) string {
		return fmt.Sprintf("oracle://system:%s@127.0.0.1:%d/%s",
			url.QueryEscape(Password), port, service)
	}
}

// oracleReady is the readiness command.
//
// sqlplus takes no statement on the command line, so a shell pipes one in.
// What it pipes is "exit", which needs no quoting: connecting and leaving is
// the whole test, and -L makes sqlplus fail rather than prompt when the server
// is not up. Verified to exit 0 when the service answers and 1 for both a
// wrong service name and a wrong password.
//
// Keeping the command free of quotes matters beyond neatness.
// [Server.HealthCmd] renders this for a CI workflow, and an argument holding
// its own quotes comes out correct but unreadable.
func oracleReady(service string) []string {
	return []string{"bash", "-c",
		"echo exit | sqlplus -s -L system/" + Password + "@localhost/" + service}
}

var (
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
		ready:     oracleReady("XEPDB1"),
		dsn:       oracleService("XEPDB1"),
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
		ready:     oracleReady("FREEPDB1"),
		dsn:       oracleService("FREEPDB1"),
	}
	// 19c, built locally from Oracle's Dockerfiles because Oracle publishes no
	// free image of it. `dbrun build oracle-19c` makes it, and this names what
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
)

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

// Oracle is every Oracle release dbmeta is tested against.
//
// 21c and 26ai on every push, as the current pair. 26ai is the newest and 21c
// is where native JSON arrived.
//
// 11g Release 2 and 23ai run nightly. 11g is the one release before
// multitenant and it lacks the most: no identity column, no CDB_ views, no
// CON_ID, and identifiers are 30 bytes rather than 128. 23ai is there because
// 26ai is not a rename: 23.26.3 publishes 577 ALL_ views against 23.9's 565,
// adding ALL_ASSERTIONS, ALL_DATA_GRANTS, ALL_TAB_MODIFICATION_HISTORY and
// ALL_PG_COMMENTS among others, and removing none. A query written against
// 26ai can therefore read a view that 23ai does not have, and 23ai is still
// widely deployed.
//
// 18c and 19c are Verified. 18c is a quiet release between two that run. 19c
// has no free image and is built by hand, so it cannot run in CI.
//
// Every tag is pinned to a release. "23" floated: it resolved to 23.26.3 today
// and would have moved under the tests without anyone deciding, which is what
// the harness section of docs/PLAN.md warns about.
var Oracle = list{}.add(oraclexe, Tested, "21.3.0").
	add(oraclefree, Tested, "23.26.3").
	add(oraclexe, Nightly, "11.2.0.2").
	add(oraclefree, Nightly, "23.9").
	add(oraclexe, Verified, "18.4.0").
	add(oracle19, Verified, "19.3.0").
	on("11.2.0.2", func(s *Server) {
		// 11g predates multitenant. There is no pluggable database to name,
		// and XE is the instance itself.
		s.dsn = oracleService("XE")
		s.Ready = oracleReady("XE")
	}).
	on("19.3.0", func(s *Server) {
		// The pluggable database, not the container database.
		//
		// Oracle's Dockerfiles build a CDB called ORCLCDB holding a PDB
		// called ORCLPDB1. Connecting to the root refuses to create an
		// ordinary user at all: CREATE USER dbmeta_fixture there is
		// ORA-65096, "invalid common user or role name", because a user in
		// the root has to be a common user named C##something. A fixture
		// belongs in the PDB, which is where an application's schema lives.
		s.dsn = oracleService("ORCLPDB1")
		s.Ready = oracleReady("ORCLPDB1")
	})
