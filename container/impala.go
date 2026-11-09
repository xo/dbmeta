package container

import (
	"fmt"
	"net/url"
	"time"

	"github.com/xo/dbmeta"
)

// The Apache Impala releases dbmeta is tested against.
//
// Impala serves HiveServer2's protocol over HTTP on 28000, the port of the
// hs2_http_port flag, and github.com/xo/dbimp/hive reaches it that way and no
// other (D282 and D283). Ken asked for Impala on 2026-09-30, which amends D118
// (D145).
//
// # The range
//
// docker.io/apache/impala builds each release tag once, so the rule in D112
// applies: the newest release of each of the last two lines. Checked on
// 2026-10-22, that is 4.5.3, of 2026-10-06, and 4.4.1. Impala is under the
// Apache 2.0 license. The pin moved from 4.5.2 to 4.5.3 on that day under the
// same rule, and 4.5.3 was not run before CI.
//
// # The image is built here
//
// Apache publishes the metastore, the statestore, the catalog and the daemon
// as images of their own, and its quickstart runs four containers. dbrun
// starts one, so test/cmd/dbrun/image/impala.Containerfile builds the four
// into one image from Apache's own. See D145.
//
// # No users
//
// The image configures no authentication, so any client is let in. The DSN
// names impala, the user the image runs as, which the daemon takes as the
// user of the session.

// impala is the one container Impala image.
var impala = product{
	dialect: dbmeta.Impala,
	name:    "impala",
	client:  impalaClient(),
	image:   "localhost/dbmeta/impala",
	port:    28000,
	ready: []string{"bash", "-c", "exec 3<>/dev/tcp/127.0.0.1/25000 &&" +
		" printf 'GET /healthz HTTP/1.0\\r\\n\\r\\n' >&3 && grep -q OK <&3"},
	// The metastore makes its schema on the first start, and four JVMs
	// start after it.
	startup: 5 * time.Minute,
	// The driver of dbimp takes only the hive:// URL, so the DSN is that URL,
	// and the api is the http:// address that the tools of dbimp read.
	dsn: func(port int) string {
		return fmt.Sprintf("hive://impala:%s@127.0.0.1:%d?httpPath=cliservice", url.QueryEscape(Password), port)
	},
	api: func(port int) string {
		return fmt.Sprintf("http://impala:%s@127.0.0.1:%d/cliservice", url.QueryEscape(Password), port)
	},
	// The URL is what a person types, and dburl turns the impala:// scheme
	// into the DSN above, so a client such as usql opens Impala with the
	// impala dialect and not the hive one.
	url: func(port int) string {
		return fmt.Sprintf("impala://impala:%s@127.0.0.1:%d?httpPath=cliservice", url.QueryEscape(Password), port)
	},
}

// Impala is every Apache Impala release dbmeta is tested against.
var Impala = list{}.add(impala, Verified, "4.4.1", "4.5.3")
