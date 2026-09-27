package container

import (
	"fmt"
	"net/url"

	"github.com/xo/dbmeta"
)

// The Couchbase Server releases dbmeta is tested against.
//
// The tests reach it through github.com/xo/dbimp/couchbase, the driver that
// usql moves to, and the dialect is couchbase, which is its dburl driver name.
// See D101. A model follows the steps in docs/DIALECT.md.
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
// Init also makes a primary index on dbmeta, so that an ordinary user can
// read the bucket with no index of its own. 7.2 has no sequential scan at all,
// and 8.0 grants one only through a role that this user does not hold. It
// waits until a count over the bucket succeeds. On a start after a stop the
// query service answers before the bucket has warmed up, and 7.2 refused an
// INSERT in that window.
//
// Ready and Init both run inside the container against 8091, the cluster
// manager, which is never published. Only 8093, the query service, is
// published, and the DSN names it. The driver speaks to the query service and
// nothing else.
//
// The administrator is Administrator with [Password]. Init also makes
// [CouchbaseUser], an ordinary user with the same password, for a driver's
// tests and for parity (D61), and [Server.Principals] lists it (D102). It can run SELECT, INSERT, UPDATE and DELETE on
// the dbmeta bucket and read the system: catalog, and it cannot administer the
// cluster. user-manage --set makes the user or resets it, so it is safe on
// every start.

// CouchbaseUser is the ordinary user that Init makes on every Couchbase
// release. Its password is [Password]. A consumer reads the name from here
// rather than writing it again.
const CouchbaseUser = "dbmeta_user"

// couchbaseCLI runs couchbase-cli against the node's own cluster manager.
const couchbaseCLI = `/opt/couchbase/bin/couchbase-cli`

// couchbase is the Couchbase Server image.
var couchbase = product{
	dialect: dbmeta.Couchbase,
	name:    "couchbase",
	image:   "docker.io/library/couchbase",
	port:    8093,
	// Ready means the cluster manager answers, which is all Init needs. It
	// answers 200 on a new node and 401 once the cluster exists, because an
	// initialized cluster refuses /pools to a request with no credentials.
	// The first version asked for 200 alone, so a stopped server that was
	// started again never became ready.
	ready: []string{"bash", "-c", `code=$(/opt/couchbase/bin/curl -s -o /dev/null -w '%{http_code}' ` +
		`http://127.0.0.1:8091/pools); [ "$code" = 200 ] || [ "$code" = 401 ]`},
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
$cli user-manage -c 127.0.0.1 -u Administrator -p "$pw" --set \
  --rbac-username ` + CouchbaseUser + ` --rbac-password "$pw" --auth-domain local \
  --roles 'query_select[dbmeta],query_insert[dbmeta],query_update[dbmeta],query_delete[dbmeta],query_system_catalog' \
  >/dev/null
# q runs one statement as Administrator and succeeds only when the query
# service reports success.
q() {
  /opt/couchbase/bin/curl -s -u "Administrator:$pw" \
    --data-urlencode "statement=$1" http://127.0.0.1:8093/query/service |
    grep -q '"status": "success"'
}
# wait runs a statement until it succeeds, because the query service answers
# before the index service and the bucket are ready, most of all on a start
# after a stop.
wait() {
  i=0
  until q "$1"; do
    i=$((i + 1))
    [ "$i" -gt 120 ] && { echo "never succeeded: $1"; exit 1; }
    sleep 1
  done
}
wait 'SELECT 1'
wait 'CREATE PRIMARY INDEX IF NOT EXISTS ON dbmeta'
wait 'SELECT RAW COUNT(*) FROM dbmeta'`},
	dsn:   couchbaseDSN("Administrator"),
	users: []Principal{{Role: User, User: CouchbaseUser, dsn: couchbaseDSN(CouchbaseUser)}},
}

// couchbaseDSN builds the URL that github.com/xo/dbimp/couchbase takes, for
// one user with [Password], on the query service. It is the dburl form too,
// so a server's URL and its DSN are one string.
func couchbaseDSN(user string) func(port int) string {
	return func(port int) string {
		u := url.URL{
			Scheme: "couchbase",
			User:   url.UserPassword(user, Password),
			Host:   fmt.Sprintf("127.0.0.1:%d", port),
			Path:   "/",
		}
		return u.String()
	}
}

// Couchbase is every Couchbase Server release dbmeta is tested against.
//
// 7.2.9 and 8.0.3 on every push, because they are the two ends. 7.6.12 runs
// nightly.
var Couchbase = list{}.add(couchbase, Tested, "7.2.9", "8.0.3").
	add(couchbase, Nightly, "7.6.12")
