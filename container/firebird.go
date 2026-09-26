package container

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/xo/dbmeta"
)

// The Firebird releases dbmeta is tested against.
//
// # The floor, by the docs/EVALUATION.md procedure
//
// Step 2 decides it, and for once step 3 says the same thing, which is worth
// recording because the two usually have to be weighed against each other.
//
// Step 2 asks for the oldest release whose image is still rebuilt.
// firebirdsql/firebird carries 145 tags and rebuilt every one of them on
// 2026-08-31, back through the whole 3.0 series. So the oldest still rebuilt
// is 3.0, and nothing between there and 5.0 is dead.
//
// Step 3 asks what the vendor still supports. Firebird released 3.0.14, 4.0.7
// and 5.0.4 on the same day, 2026-04-17, which is a project patching three
// series in parallel rather than one series with a tail. The vendor's answer
// is the same three.
//
// The ceiling is 5.0. A 6 exists on the registry and only as 6-snapshot,
// which is a nightly build of an unreleased series rather than a release.
//
// Firebird numbers a series with two parts and this package names one with
// two as well, so these are firebird-3.0, firebird-4.0 and firebird-5.0 and
// never firebird-3. The release pinned in each is the newest patch, because a
// patch is what the vendor ships to a series and there is nothing to learn
// from an older one.
//
// # Why this pair runs on every push
//
// 3.0 and 5.0 are the ends. Nine years separate them, which is the widest
// span of any product here after PostgreSQL, so a query that holds on both
// holds in between.
// firebird is the official image, from the Firebird project itself.
var firebird = product{
	dialect: dbmeta.Firebird,
	name:    "firebird",
	image:   "docker.io/firebirdsql/firebird",
	major:   seriesOf,
	port:    3050,
	env: map[string]string{
		// SYSDBA is the built in administrator and the image leaves its
		// password at the Firebird default without this.
		"FIREBIRD_ROOT_PASSWORD": Password,
		// A Firebird database is one file and a connection names it, so
		// there is no server wide catalog to connect to instead. The
		// entrypoint creates this one under FIREBIRD_DATA on first start.
		"FIREBIRD_DATABASE": firebirdFile,
	},
	// isql -x extracts the schema, which needs a real connection and a real
	// database, so it fails while the entrypoint is still creating the file.
	// A plain connection test would pass too early.
	ready: []string{"isql", "-u", "SYSDBA", "-p", Password, "-x", firebirdPath},
	dsn: func(port int) string {
		return fmt.Sprintf("SYSDBA:%s@127.0.0.1:%d%s",
			url.QueryEscape(Password), port, firebirdPath)
	},
	url: func(port int) string {
		return fmt.Sprintf("firebird://SYSDBA:%s@127.0.0.1:%d%s",
			url.QueryEscape(Password), port, firebirdPath)
	},
}

// Where the entrypoint puts the database. FIREBIRD_DATABASE is resolved
// against FIREBIRD_DATA, which the image sets and this must match.
const (
	firebirdFile = "dbmeta.fdb"
	firebirdPath = "/var/lib/firebird/data/" + firebirdFile
)

// seriesOf returns the two part series a Firebird patch release belongs to,
// so that 5.0.4 is named 5.0.
func seriesOf(release string) string {
	parts := strings.SplitN(release, ".", 3)
	if len(parts) < 2 {
		return release
	}
	return parts[0] + "." + parts[1]
}

// Firebird is every Firebird release dbmeta is tested against.
var Firebird = list{}.add(firebird, Tested, "3.0.14", "5.0.4").
	add(firebird, Nightly, "4.0.7")
