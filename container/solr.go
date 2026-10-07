package container

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"

	"github.com/xo/dbmeta"
)

// The Apache Solr releases dbrun starts.
//
// models/solr reads them, through the Solr driver in github.com/xo/dbimp, which
// sends SQL to /solr/{collection}/sql. The dialect is solr. See D118 and D179.
//
// # The range
//
// docker.io/library/solr is an official image and is rebuilt, so the floor is
// the oldest release still rebuilt. Checked on 2026-09-28, 10.0.0, 9.10.1 and
// 9.9.0 were rebuilt on 2026-09-26, and 9.10.0 and 8.11 stopped. So the floor
// is 9.9.0, the ceiling is 10.0.0, and 9.10.1 is kept between them. Solr is under
// the Apache 2.0 license.
//
// # One container
//
// SQL needs SolrCloud, and SolrCloud runs here with the ZooKeeper that Solr
// embeds, on port 9983. The sql module holds the SQL handler.
//
// # The users
//
// Solr keeps its users in security.json in ZooKeeper. The command writes that
// file with admin and [SolrUser], each with [Password], and uploads it once
// ZooKeeper answers, on every start, so it is the same after a restart. admin
// can do anything. [SolrUser] has the role search, which can read a
// collection and run SQL on it. Solr stores a password as the SHA-256 of the
// SHA-256 of a salt and the password, which [solrHash] computes. Init makes
// the collection dbmeta.
//
// # The DSN
//
// The driver sends each statement to the SQL handler of the collection that
// the path of the DSN names, and it refuses a statement when the path is empty.
// The handler reads whichever collection the FROM names, so any collection that
// the user can read serves. The DSN of each principal ends with /dbmeta, which
// Init makes, so that the DSN is what sql.Open takes (D167, D179).

// SolrUser can read a collection and run SQL on it. Its password is
// [Password].
const SolrUser = "dbmeta_user"

// solrSalt is the salt of every password in security.json. It is fixed, so
// that the file is the same on every start.
const solrSalt = "ZGJtZXRhZGJtZXRhZGJtZXRh"

// solrHash is a password as Solr's basic authentication stores it: the hash
// and the salt, each in base64, with a space between.
func solrHash(password string) string {
	salt, err := base64.StdEncoding.DecodeString(solrSalt)
	if err != nil {
		panic(err)
	}
	first := sha256.Sum256(append(salt, password...))
	second := sha256.Sum256(first[:])
	return base64.StdEncoding.EncodeToString(second[:]) + " " + solrSalt
}

// solrSecurity is the security.json the command uploads.
var solrSecurity = `{
  "authentication": {
    "class": "solr.BasicAuthPlugin",
    "blockUnknown": true,
    "forwardCredentials": false,
    "credentials": {"admin": "` + solrHash(Password) + `", "` + SolrUser + `": "` + solrHash(Password) + `"}
  },
  "authorization": {
    "class": "solr.RuleBasedAuthorizationPlugin",
    "user-role": {"admin": ["admin"], "` + SolrUser + `": ["search"]},
    "permissions": [
      {"name": "read", "role": ["search", "admin"]},
      {"name": "sql", "collection": "*", "path": "/sql", "role": ["search", "admin"]},
      {"name": "all", "role": "admin"}
    ]
  }
}`

// solrServe starts Solr, and uploads security.json once ZooKeeper answers.
var solrServe = `cat > /tmp/security.json <<'JSON'
` + solrSecurity + `
JSON
docker-entrypoint.sh solr-foreground &
p=$!
until solr zk ls / -z localhost:9983 >/dev/null 2>&1; do
	kill -0 $p || exit 1
	sleep 2
done
solr zk cp file:/tmp/security.json zk:/security.json -z localhost:9983
wait $p`

// solrCurl is a curl of Solr as admin.
const solrCurl = "curl -sf -u 'admin:" + Password + "' "

// solr is the Solr image.
var solr = product{
	name:    "solr",
	dialect: dbmeta.Solr,
	image:   "docker.io/library/solr",
	port:    8983,
	env: map[string]string{
		"SOLR_MODE":    "solrcloud",
		"SOLR_MODULES": "sql",
		"SOLR_HEAP":    "1g",
	},
	runFlags: []string{"--entrypoint", "/bin/bash"},
	args:     []string{"-c", solrServe},
	// A request with no user is refused once security.json is in place, so
	// the check passes only after the upload.
	ready: []string{"sh", "-c", solrCurl + "-o /dev/null 'http://127.0.0.1:8983/solr/admin/collections?action=LIST' &&" +
		" [ \"$(curl -s -o /dev/null -w '%{http_code}' 'http://127.0.0.1:8983/solr/admin/collections?action=LIST')\" = 401 ]"},
	init: []string{"sh", "-c", solrCurl + "'http://127.0.0.1:8983/solr/admin/collections?action=LIST' | grep -q '\"dbmeta\"' || " +
		solrCurl + "-o /dev/null 'http://127.0.0.1:8983/solr/admin/collections?action=CREATE&name=dbmeta&numShards=1&collection.configName=_default'"},
	dsn:   solrURL("admin"),
	api:   keyHTTP("admin", Password),
	users: []Principal{{Role: User, User: SolrUser, dsn: solrURL(SolrUser), api: keyHTTP(SolrUser, Password)}},
}

// solrURL is the DSN of the driver for one user, which names the collection
// dbmeta in its path.
func solrURL(user string) func(port int) string {
	return func(port int) string {
		u := url.URL{
			Scheme: "solr",
			User:   url.UserPassword(user, Password),
			Host:   fmt.Sprintf("127.0.0.1:%d", port),
			Path:   "/dbmeta",
		}
		return u.String()
	}
}

// Solr is every Apache Solr release dbrun starts.
//
// models/solr reads them, so each keeps the cadence it recorded while it was
// Staged (D120, D179).
var Solr = list{}.add(solr, Tested, "9.9.0", "10.0.0").
	add(solr, Nightly, "9.10.1")
