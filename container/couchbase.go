package container

import (
	"fmt"
	"net/url"

	"github.com/xo/dbmeta"
)

// The Couchbase Server releases dbmeta is tested against.
//
// dbmeta has no Couchbase model yet. The releases are here so that dbrun can
// start a server for the n1ql driver's own tests, which is the one thing a
// consumer of this package asked for first. A model follows the steps in
// docs/DIALECT.md, and it reads this list when it does.
//
// # The floor, by the docs/EVALUATION.md procedure
//
// Step 2 decides it: the oldest release whose image is still rebuilt. On
// docker.io/library/couchbase, checked on 2026-09-27, 7.2.9 was rebuilt on
// 2026-08-18, and 7.6.12 and 8.0.3 on 2026-09-16. 7.0 and 7.1 were last
// rebuilt in November 2024, and 6.6 in December 2023. Every one has a
// linux/amd64 build.
//
// There is no tag for a release line such as 7.6, only one per point release,
// so the release here is the newest point release of each line.
//
// A bare tag is the Enterprise edition. It is free for development and
// testing, and D90 says that qualifies. The Community edition lags: its newest
// 7.6 tag is 7.6.2.
//
// # The cluster is made after the server starts
//
// A new container runs a node that belongs to no cluster and serves no query.
// Init makes the cluster with the data, index and query services, and a bucket
// named dbmeta, because SQL++ can create a scope, a collection and an index
// and cannot create a bucket. Init checks for each first, so it is safe on
// every start, and it waits until the query service answers.
//
// Ready and Init both run inside the container against 8091, the cluster
// manager, which is never published. Only 8093, the query service, is
// published, and the DSN names it. go_n1ql, the driver usql uses, tries a
// DSN as a cluster address first and then as a query address, and a cluster
// address would hand back the container's own address for the query service.
//
// The administrator is Administrator with [Password].

// couchbaseCLI runs couchbase-cli against the node's own cluster manager.
const couchbaseCLI = `/opt/couchbase/bin/couchbase-cli`

// couchbase is the Couchbase Server image.
var couchbase = product{
	dialect: dbmeta.N1QL,
	name:    "couchbase",
	image:   "docker.io/library/couchbase",
	port:    8093,
	// The cluster manager answers before any cluster exists, and Init needs
	// nothing more than that.
	ready: []string{"/opt/couchbase/bin/curl", "-sf", "-o", "/dev/null", "http://127.0.0.1:8091/pools"},
	init: []string{"bash", "-c", `set -e
pw='` + Password + `'
cli=` + couchbaseCLI + `
if ! $cli server-list -c 127.0.0.1 -u Administrator -p "$pw" >/dev/null 2>&1; then
  $cli cluster-init -c 127.0.0.1 --cluster-username Administrator \
    --cluster-password "$pw" --services data,index,query \
    --cluster-ramsize 512 --cluster-index-ramsize 256 \
    --index-storage-setting default
fi
if ! $cli bucket-list -c 127.0.0.1 -u Administrator -p "$pw" | grep -qx dbmeta; then
  $cli bucket-create -c 127.0.0.1 -u Administrator -p "$pw" --bucket dbmeta \
    --bucket-type couchbase --bucket-ramsize 128 --wait
fi
i=0
until /opt/couchbase/bin/curl -sf -o /dev/null -u "Administrator:$pw" \
    -d 'statement=SELECT 1' http://127.0.0.1:8093/query/service; do
  i=$((i + 1))
  [ "$i" -gt 120 ] && { echo "the query service never answered"; exit 1; }
  sleep 1
done`},
	dsn: func(port int) string {
		u := url.URL{
			Scheme: "http",
			User:   url.UserPassword("Administrator", Password),
			Host:   fmt.Sprintf("127.0.0.1:%d", port),
		}
		return u.String()
	},
	url: func(port int) string {
		return fmt.Sprintf("couchbase://Administrator:%s@127.0.0.1:%d/",
			url.QueryEscape(Password), port)
	},
}

// Couchbase is every Couchbase Server release dbmeta is tested against.
//
// 7.2.9 and 8.0.3 on every push, because they are the two ends. 7.6.12 runs
// nightly.
var Couchbase = list{}.add(couchbase, Tested, "7.2.9", "8.0.3").
	add(couchbase, Nightly, "7.6.12")
