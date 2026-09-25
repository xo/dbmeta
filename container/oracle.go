package container

import "fmt"

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
// # Service names, which differ per release
//
// A connection names a service rather than a database. Express calls it XE,
// Free calls it FREE, and the image built from Oracle's Dockerfiles calls it
// ORCLCDB. That is why each release sets its own connection string instead of
// sharing one from the product.

// oracleService builds a connection string for a service on a port.
func oracleService(service string) func(port int) string {
	return func(port int) string {
		return fmt.Sprintf("oracle://system:%s@127.0.0.1:%d/%s", Password, port, service)
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
