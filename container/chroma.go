package container

import "fmt"

// The Chroma releases dbrun starts.
//
// dbmeta has no Chroma model. The releases are here so that dbrun can start a
// server for the tests of the Chroma driver in github.com/xo/dbimp, which
// reads the HTTP interface under /api/v2. No dialect is named yet, because
// dbimp settles the name with the driver. See D118.
//
// # The range
//
// docker.io/chromadb/chroma builds each release tag once, so the rule in D112
// applies: the newest release of each of the last two lines. Checked on
// 2026-09-28, that is 1.5.9, of 2026-05-05, and 1.4.1, of 2026-01-14. The
// 1.5.10.dev tags are development builds. Chroma is under the Apache 2.0
// licence.
//
// # No users
//
// The server of Chroma 1 has no authentication at all, so it has no
// administrator and no ordinary user. The DSN names the user admin with no
// password, as the Avatica one names SA, and nothing checks the name. Chroma
// secures a server only through a proxy in front of it.
//
// # The setup
//
// The image has bash and no curl, so every request goes through bash's
// /dev/tcp. Init makes the database dbmeta in the default tenant, if it is
// missing.

// chroma is the Chroma image.
var chroma = product{
	name:  "chroma",
	image: "docker.io/chromadb/chroma",
	port:  8000,
	env: map[string]string{
		// The usage report that goes to the vendor.
		"ANONYMIZED_TELEMETRY": "False",
	},
	ready: bashRequest(8000, "GET", "/api/v2/heartbeat", "", nil, 200),
	init: bashEither(
		bashRequest(8000, "GET", "/api/v2/tenants/default_tenant/databases/dbmeta", "", nil, 200),
		bashRequest(8000, "POST", "/api/v2/tenants/default_tenant/databases", `{"name":"dbmeta"}`, nil, 200)),
	dsn: func(port int) string { return fmt.Sprintf("http://admin@127.0.0.1:%d", port) },
}

// Chroma is every Chroma release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. See D119.
var Chroma = list{}.add(chroma, Staged, "1.4.1", "1.5.9")
