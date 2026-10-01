package container

import (
	"fmt"
	"net/url"

	"github.com/xo/dbmeta"
)

// The Neo4j releases dbrun starts.
//
// models/neo4j reads them, and dbrun also starts them for the tests of the
// Neo4j driver in github.com/xo/dbimp, which is its third driver. Ken agreed
// to the product and to its license on 2026-09-27. See D106 and D162.
//
// # The range, by the docs/EVALUATION.md procedure
//
// Step 2 decides it. On docker.io/library/neo4j, checked on 2026-09-27, 4.4.48,
// 5.26.31 and 2026.09.0 were rebuilt on 2026-09-26. A monthly release stops
// being rebuilt when the next one arrives: 2026.08 was last rebuilt on
// 2026-09-19. Each tag has a linux/amd64 build.
//
// 4.4 is not in the range. Its Enterprise image starts only with the
// commercial license, and a person cannot run that without paying. So the
// floor is 5.26, the LTS line, and the ceiling is the newest monthly release.
// The ceiling moves each month.
//
// # The edition and the license
//
// The image is the Enterprise Edition, because the Community Edition has one
// database and no roles. It starts only when NEO4J_ACCEPT_LICENSE_AGREEMENT is
// set, and eval accepts the Neo4j Software Evaluation Agreement: 30 days, for
// internal development only. The agreement collects usage data unless a
// setting turns it off, and the setting is off here.
//
// # The setup
//
// Init runs cypher-shell against the system database as the administrator.
// It makes the database dbmeta, and makes [Neo4jUser] with the role publisher.
// That role reads and writes, and creates new labels, property keys and
// relationship types. It cannot create an index or a constraint.
//
// The administrator is neo4j with [Password], which NEO4J_AUTH sets on the
// first start.

// Neo4jUser is the ordinary user that Init makes on every Neo4j release. Its
// password is [Password].
const Neo4jUser = "dbmeta_user"

// neo4jDatabase is the database that Init makes.
const neo4jDatabase = "dbmeta"

// neo4jShell is cypher-shell as one user, on one database.
func neo4jShell(user, database string) string {
	return fmt.Sprintf("cypher-shell -a bolt://127.0.0.1:7687 -u %s -p '%s' -d %s",
		user, Password, database)
}

// neo4j is the Neo4j Enterprise image.
var neo4j = product{
	dialect:   dbmeta.Neo4j,
	name:      "neo4j",
	image:     "docker.io/library/neo4j",
	tagSuffix: "-enterprise",
	port:      7474,
	env: map[string]string{
		"NEO4J_AUTH":                     "neo4j/" + Password,
		"NEO4J_ACCEPT_LICENSE_AGREEMENT": "eval",
		// The image turns each variable of this form into a setting, and a
		// double underscore into one. This is dbms.usage_report.enabled,
		// which stops the usage report the evaluation agreement describes.
		// With it on, the log says "Anonymous Usage Data is being sent to
		// Neo4j", and with it off that line is gone.
		"NEO4J_dbms_usage__report_enabled": "false",
		// Neo4j sizes its heap and its page cache from the memory it has.
		// The JVM took a quarter of the 4 GB limit for the heap, and the
		// image set the page cache to 512 MiB, measured on 5.26.31. They are
		// fixed at those values, so that a larger limit elsewhere changes
		// nothing. The server used 1.7 GB after the setup, with the whole heap
		// taken at the start.
		"NEO4J_server_memory_heap_initial__size": "1g",
		"NEO4J_server_memory_heap_max__size":     "1g",
		"NEO4J_server_memory_pagecache_size":     "512m",
	},
	// The system database refuses RETURN 1, because it runs only system
	// commands, so the check asks it for the databases.
	ready: []string{"sh", "-c", neo4jShell("neo4j", "system") + " 'SHOW DATABASES YIELD name'"},
	init: []string{"sh", "-c", "set -e\n" +
		neo4jShell("neo4j", "system") + " 'CREATE DATABASE " + neo4jDatabase + " IF NOT EXISTS WAIT'\n" +
		neo4jShell("neo4j", "system") + " \"CREATE OR REPLACE USER " + Neo4jUser +
		" SET PLAINTEXT PASSWORD '" + Password + "' CHANGE NOT REQUIRED\"\n" +
		neo4jShell("neo4j", "system") + " 'GRANT ROLE publisher TO " + Neo4jUser + "'\n" +
		neo4jShell(Neo4jUser, neo4jDatabase) + " 'RETURN 1'\n",
	},
	// dbimp's driver takes only the neo4j:// URL, so the DSN is that URL,
	// and the api is the http:// address that dbimp's tools read (D167).
	dsn: neo4jURL("neo4j"),
	api: neo4jHTTP("neo4j"),
	users: []Principal{{
		Role: User, User: Neo4jUser,
		dsn: neo4jURL(Neo4jUser), api: neo4jHTTP(Neo4jUser),
	}},
}

// neo4jHTTP is the address of the HTTP API, with one user's credentials.
func neo4jHTTP(user string) func(port int) string {
	return func(port int) string {
		u := url.URL{
			Scheme: "http",
			User:   url.UserPassword(user, Password),
			Host:   fmt.Sprintf("127.0.0.1:%d", port),
		}
		return u.String()
	}
}

// neo4jURL is the URL that the dbimp Neo4j driver takes and dburl parses. The
// path names the database, and the port is the HTTP port. dbimp settled the
// form in its D60 and D61. See D109.
func neo4jURL(user string) func(port int) string {
	return func(port int) string {
		u := url.URL{
			Scheme: "neo4j",
			User:   url.UserPassword(user, Password),
			Host:   fmt.Sprintf("127.0.0.1:%d", port),
			Path:   "/" + neo4jDatabase,
		}
		return u.String()
	}
}

// Neo4j is every Neo4j release dbmeta is tested against.
//
// models/neo4j reads both, so each takes the cadence it recorded while it was
// Staged, and both run on every push (D120, D162). 5.26.31 is the floor of the
// model and 2026.09.0 the newest monthly release.
var Neo4j = list{}.add(neo4j, Tested, "5.26.31", "2026.09.0")
