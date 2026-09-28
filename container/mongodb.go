package container

import (
	"fmt"
	"net/url"
)

// The MongoDB releases dbrun starts.
//
// dbmeta has no MongoDB model. The releases are here so that dbrun can start
// a server for the tests of the MongoDB driver in github.com/xo/dbimp. MongoDB
// has no HTTP interface of its own, so the driver speaks the wire protocol on
// 27017, through go.mongodb.org/mongo-driver. No dialect is named yet, because
// dbimp settles the name with the driver. See D118.
//
// # The range
//
// docker.io/library/mongo is an official image and a point release is
// rebuilt until the next one replaces it. Checked on 2026-09-28, 8.3.11 and
// 8.0.32 were rebuilt on 2026-09-16 and 7.0.43 on 2026-09-14. 8.2.12 was last
// rebuilt on 2026-07-23, and 6.0 and 5.0 on 2026-05-15. So the floor is 7.0.43
// and the ceiling is 8.3.11, and 8.0.32, the line with long term support
// between them, is kept too. The community server is under the SSPL.
//
// # The users
//
// The image makes the administrator admin in the database admin, with
// [Password], on the first start. Init makes [MongoDBUser] in the database
// dbmeta, with the role read on it, and makes the collection dbmeta, because a
// database exists only while it holds a collection.
//
// # The check
//
// On the first start the image runs a server with no authentication on
// 127.0.0.1, makes the administrator, and starts the real server. A check
// inside the container can reach the first one. So the check also asks
// whether authorization is on, which it is only on the real server.

// MongoDBUser may only read the database dbmeta. Its password is [Password].
const MongoDBUser = "dbmeta_user"

// mongoshAdmin runs mongosh as the administrator.
const mongoshAdmin = "mongosh --quiet -u admin -p '" + Password + "' --authenticationDatabase admin --eval "

// mongodb is the MongoDB image.
var mongodb = product{
	name:  "mongodb",
	image: "docker.io/library/mongo",
	port:  27017,
	env: map[string]string{
		"MONGO_INITDB_ROOT_USERNAME": "admin",
		"MONGO_INITDB_ROOT_PASSWORD": Password,
		// The usage report that mongosh sends to the vendor.
		"DO_NOT_TRACK": "1",
	},
	// The cache takes half of the memory the server sees, and the server
	// sees the whole host.
	args: []string{"--wiredTigerCacheSizeGB", "1"},
	ready: []string{"sh", "-c", mongoshAdmin +
		`'db.adminCommand({getCmdLineOpts: 1}).parsed.security?.authorization == "enabled" || quit(1)'`},
	init: []string{"sh", "-c", mongoshAdmin + `'
const d = db.getSiblingDB("dbmeta");
const u = {pwd: "` + Password + `", roles: [{role: "read", db: "dbmeta"}]};
if (d.getUser("` + MongoDBUser + `")) { d.updateUser("` + MongoDBUser + `", u); } else { d.createUser({user: "` + MongoDBUser + `", ...u}); }
if (!d.getCollectionNames().includes("dbmeta")) { d.createCollection("dbmeta"); }'`},
	dsn:   mongoURL("admin", "admin"),
	users: []Principal{{Role: User, User: MongoDBUser, dsn: mongoURL(MongoDBUser, "dbmeta")}},
}

// mongoURL is the address of the database dbmeta as one user, who is kept in
// the database source.
func mongoURL(user, source string) func(port int) string {
	return func(port int) string {
		return fmt.Sprintf("mongodb://%s:%s@127.0.0.1:%d/dbmeta?authSource=%s",
			user, url.QueryEscape(Password), port, source)
	}
}

// MongoDB is every MongoDB release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. See D119.
var MongoDB = list{}.add(mongodb, Staged, "7.0.43", "8.0.32", "8.3.11")
