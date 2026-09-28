package container

// The OpenLink Virtuoso releases dbrun starts.
//
// dbmeta has no Virtuoso model. The releases are here so that dbrun can start
// a server for the tests of the SPARQL driver in github.com/xo/dbimp, which
// sends queries to the SPARQL protocol on the HTTP interface. No dialect is
// named yet, because dbimp settles the name with the driver. See D118.
//
// # The range
//
// docker.io/openlink/virtuoso-opensource-7 rebuilds a release tag: 7.2.17 was
// built again on 2026-08-05, and 7.2.16 was last built on 2025-10-15. So by
// criterion 2 the one release still rebuilt is 7.2.17. The open source
// edition has one line, 7.2, and Virtuoso 8 is commercial only. It is under
// the GPL 2.0 licence.
//
// # The users
//
// The image gives the administrator dba [Password] on the first start. Init
// runs isql on the SQL port inside the container. It makes [VirtuosoUser] when
// it is missing, lets it run SPARQL queries and not updates, and makes the
// graph urn:dbmeta. /sparql answers anybody, and /sparql-auth asks for a user
// with HTTP digest authentication, which is what the driver has to speak.
//
// # Memory
//
// The buffers are set for about 2 GB, which is the vendor's figure for that
// much memory.

// VirtuosoUser may run SPARQL queries and not updates. Its password is
// [Password].
const VirtuosoUser = "dbmeta_user"

// virtuosoSQL runs SQL through isql as dba.
func virtuosoSQL(stmt string) string {
	return `isql 1111 dba '` + Password + `' exec="` + stmt + `"`
}

// virtuoso is the Virtuoso image.
var virtuoso = product{
	name:  "virtuoso",
	image: "docker.io/openlink/virtuoso-opensource-7",
	port:  8890,
	env: map[string]string{
		"DBA_PASSWORD":                         Password,
		"VIRT_Parameters_NumberOfBuffers":      "170000",
		"VIRT_Parameters_MaxDirtyBuffers":      "130000",
		"VIRT_SPARQL_DefaultGraph":             "urn:dbmeta",
		"VIRT_Parameters_ServerThreads":        "10",
		"VIRT_Parameters_MaxQueryMem":          "256M",
		"VIRT_HTTPServer_ServerThreads":        "10",
		"VIRT_HTTPServer_MaxClientConnections": "10",
	},
	ready: []string{"sh", "-c", virtuosoSQL("SELECT 1")},
	init: []string{"sh", "-c", `set -e
` + virtuosoSQL("SELECT U_NAME FROM DB.DBA.SYS_USERS WHERE U_NAME = '"+VirtuosoUser+"'") + ` | grep -q ` + VirtuosoUser + ` ||
	` + virtuosoSQL("DB.DBA.USER_CREATE('"+VirtuosoUser+"', '"+Password+"')") + `
` + virtuosoSQL("GRANT SPARQL_SELECT TO \\\""+VirtuosoUser+"\\\"") + `
` + virtuosoSQL("SPARQL CREATE SILENT GRAPH <urn:dbmeta>")},
	dsn:   keyHTTP("dba", Password),
	users: []Principal{{Role: User, User: VirtuosoUser, dsn: keyHTTP(VirtuosoUser, Password)}},
}

// Virtuoso is every Virtuoso release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. Each keeps the cadence it would have if a model read it, which is
// what dbimp runs on each push and at night. See D119 and D120.
var Virtuoso = list{}.staged(virtuoso, Tested, "7.2.17")
