package container

import (
	"fmt"
	"net/url"
	"time"

	"github.com/xo/dbmeta"
)

// The Apache Impala releases dbmeta is tested against.
//
// Impala serves HiveServer2's protocol on 21050, and usql reaches it with
// github.com/sclgo/impala-go, which dburl's impala scheme names. Ken asked
// for Impala on 2026-09-30, which amends D118 (D145).
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
	port:    21050,
	ready: []string{"bash", "-c", "exec 3<>/dev/tcp/127.0.0.1/25000 &&" +
		" printf 'GET /healthz HTTP/1.0\\r\\n\\r\\n' >&3 && grep -q OK <&3"},
	// The metastore makes its schema on the first start, and four JVMs
	// start after it.
	startup: 5 * time.Minute,
	// A DDL statement on a cold catalog took longer than impala-go's
	// default socket timeout, measured on 4.5.2, so the DSN sets five
	// minutes. The URL is what a person types, and leaves it.
	dsn: func(port int) string {
		return fmt.Sprintf("impala://impala@127.0.0.1:%d?socket-timeout=5m", port)
	},
	url: func(port int) string {
		return fmt.Sprintf("impala://impala@127.0.0.1:%d", port)
	},
}

// Impala is every Apache Impala release dbmeta is tested against. Both are
// Tested, which is the cadence they kept while they were Staged (D120).
var Impala = list{}.add(impala, Verified, "4.4.1", "4.5.3")

// impalaHTTP is the Impala image with HiveServer2 reached over HTTP, on the
// port of the hs2_http_port flag, which defaults to 28000. dbimp's driver
// reaches Impala that way and no other (dbimp D232, and D282 here). It is the same
// image and the same daemon as impala, and a second port is published.
//
// The image configures no authentication, so the daemon accepts any user name
// and any password, and the PLAIN exchange of the HTTP mode is not checked. A
// user cannot be made and no Init is needed.
var impalaHTTP = product{
	dialect: dbmeta.Impala,
	name:    "impalahttp",
	client:  impalaClient(),
	image:   "localhost/dbmeta/impala",
	port:    28000,
	ready: []string{"bash", "-c", "exec 3<>/dev/tcp/127.0.0.1/25000 &&" +
		" printf 'GET /healthz HTTP/1.0\\r\\n\\r\\n' >&3 && grep -q OK <&3"},
	startup: 5 * time.Minute,
	dsn: func(port int) string {
		return fmt.Sprintf("http://impala:%s@127.0.0.1:%d/cliservice", url.QueryEscape(Password), port)
	},
	url: func(port int) string {
		return fmt.Sprintf("impala://impala:%s@127.0.0.1:%d?auth=PLAIN&transport=http&httpPath=cliservice",
			url.QueryEscape(Password), port)
	},
}

// ImpalaHTTP is the Impala releases that dbimp reaches over HTTP. They are
// Verified, so CI never starts them.
var ImpalaHTTP = list{}.add(impalaHTTP, Verified, "4.4.1", "4.5.3")
