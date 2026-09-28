package container

import "time"

// The Milvus releases dbrun starts.
//
// dbmeta has no Milvus model. The releases are here so that dbrun can start
// a server for the tests of the Milvus driver in github.com/xo/dbimp, which
// reads the REST interface on 19530. No dialect is named yet, because dbimp
// settles the name with the driver. See D118.
//
// # The range
//
// docker.io/milvusdb/milvus builds each release tag once, so the rule in D112
// applies: the newest release of each of the last two lines. Checked on
// 2026-09-28, that is v3.0.2, of 2026-09-18, and v2.6.24, of 2026-09-16. The
// dated tags and the -gpu and -debug tags are other builds. Milvus is under
// the Apache 2.0 licence.
//
// # One container
//
// The standalone server runs with etcd inside it and its data on the local
// disk, with no MinIO, as the vendor's standalone_embed.sh runs it. The
// command writes the etcd configuration that script mounts.
//
// # The users
//
// The server makes root with [Password] on the first start. Init makes the
// database dbmeta, [MilvusUser] and a role that may read every collection in
// dbmeta, and grants the role. A call that makes a thing that is there
// answers 200 with a code that is not 0, so Init ends by asking whether the
// user holds the role.

// MilvusUser may read every collection in the database dbmeta. Its password
// is [Password].
const MilvusUser = "dbmeta_user"

// milvusServe writes the etcd configuration and starts the server.
const milvusServe = `cat > /tmp/embedEtcd.yaml <<'YAML'
listen-client-urls: http://0.0.0.0:2379
advertise-client-urls: http://0.0.0.0:2379
quota-backend-bytes: 4294967296
auto-compaction-mode: revision
auto-compaction-retention: '1000'
YAML
export ETCD_CONFIG_PATH=/tmp/embedEtcd.yaml
exec milvus run standalone`

// milvusCall posts to the REST interface as root, and prints the answer.
func milvusCall(path, body string) string {
	return `curl -sf -H 'Authorization: Bearer root:` + Password + `' -H 'Content-Type: application/json' -d '` +
		body + `' http://127.0.0.1:19530/v2/vectordb/` + path
}

// milvus is the Milvus image.
var milvus = product{
	name:      "milvus",
	image:     "docker.io/milvusdb/milvus",
	tagPrefix: "v",
	port:      19530,
	env: map[string]string{
		"ETCD_USE_EMBED":                       "true",
		"ETCD_DATA_DIR":                        "/var/lib/milvus/etcd",
		"COMMON_STORAGETYPE":                   "local",
		"DEPLOY_MODE":                          "STANDALONE",
		"COMMON_SECURITY_AUTHORIZATIONENABLED": "true",
		"COMMON_SECURITY_DEFAULTROOTPASSWORD":  Password,
	},
	runFlags: []string{"--security-opt", "seccomp=unconfined"},
	args:     []string{"bash", "-c", milvusServe},
	ready:    []string{"sh", "-c", milvusCall("databases/list", "{}") + ` | grep -q '"code":0'`},
	init: []string{"sh", "-c", `set -e
` + milvusCall("databases/create", `{"dbName":"dbmeta"}`) + `
` + milvusCall("users/create", `{"userName":"`+MilvusUser+`","password":"`+Password+`"}`) + `
` + milvusCall("roles/create", `{"roleName":"dbmeta_role"}`) + `
` + milvusCall("roles/grant_privilege_v2", `{"roleName":"dbmeta_role","privilege":"DatabaseReadOnly","dbName":"dbmeta","collectionName":"*"}`) + `
` + milvusCall("users/grant_role", `{"userName":"`+MilvusUser+`","roleName":"dbmeta_role"}`) + `
` + milvusCall("users/describe", `{"userName":"`+MilvusUser+`"}`) + ` | grep -q dbmeta_role`},
	startup: 3 * time.Minute,
	dsn:     keyHTTP("root", Password),
	users:   []Principal{{Role: User, User: MilvusUser, dsn: keyHTTP(MilvusUser, Password)}},
}

// Milvus is every Milvus release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. See D119.
var Milvus = list{}.add(milvus, Staged, "2.6.24", "3.0.2")
