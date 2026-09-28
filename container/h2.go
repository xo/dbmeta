package container

import (
	"fmt"
	"net/url"
)

// The H2 releases dbrun starts.
//
// usql reaches H2 with the scheme h2 and github.com/jmrobles/h2go. dbmeta has
// no H2 model. The releases are here so that dbrun can start a server for
// usql and for a model that comes later. See D118.
//
// # The range
//
// H2 publishes no image, and each release once, as a jar on Maven Central.
// So the rule in D112 applies: the newest release of each of the last two
// lines. Checked on 2026-09-28, that is 2.5.252, of 2026-09-24, and 2.4.240,
// of 2025-09-22. H2 is under the MPL 2.0 and the EPL 1.0.
//
// # The image is built here
//
// The Containerfile in test/cmd/dbrun/image puts the jar on the Java 21
// runtime.
//
// # In memory
//
// h2go opens a database in memory and not a file, so the database is
// mem:dbmeta, and it lasts while the server runs because of DB_CLOSE_DELAY.
// The first connection to a database that H2 makes is its administrator, so
// the check makes it as sa with [Password]. Init makes [H2User], who may read
// the schema PUBLIC. A restart loses the database, and the check and Init
// make it again.

// H2User may read the schema PUBLIC. Its password is [Password].
const H2User = "dbmeta_user"

// h2URL is the JDBC address of the database, which the Shell of H2 takes.
const h2URL = "jdbc:h2:tcp://127.0.0.1:9092/mem:dbmeta;DB_CLOSE_DELAY=-1"

// h2SQL runs one statement through the Shell of H2 as sa.
func h2SQL(stmt string) string {
	return `java -cp /opt/h2.jar org.h2.tools.Shell -url '` + h2URL + `' -user sa -password '` + Password + `' -sql "` + stmt + `"`
}

// h2 is the H2 image, built here.
var h2 = product{
	name:  "h2",
	image: "localhost/dbmeta/h2",
	port:  9092,
	args: []string{"java", "-Xmx512m", "-cp", "/opt/h2.jar", "org.h2.tools.Server",
		"-tcp", "-tcpAllowOthers", "-tcpPort", "9092", "-ifNotExists"},
	ready: []string{"sh", "-c", h2SQL("SELECT 1")},
	init: []string{"sh", "-c", h2SQL("CREATE USER IF NOT EXISTS "+H2User+" PASSWORD '"+Password+"'") + " && " +
		h2SQL("GRANT SELECT ON SCHEMA PUBLIC TO "+H2User)},
	dsn:   h2DSN("sa"),
	users: []Principal{{Role: User, User: H2User, dsn: h2DSN(H2User)}},
}

// h2DSN is the address of the database as one user, in the form h2go and
// dburl both take.
func h2DSN(user string) func(port int) string {
	return func(port int) string {
		return fmt.Sprintf("h2://%s:%s@127.0.0.1:%d/dbmeta?mem=true", user, url.QueryEscape(Password), port)
	}
}

// H2 is every H2 release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. Each keeps the cadence it would have if a model read it, which is
// what dbimp runs on each push and at night. See D119 and D120.
var H2 = list{}.staged(h2, Tested, "2.4.240", "2.5.252")
