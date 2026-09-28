package container

// The Typesense releases dbrun starts.
//
// dbmeta has no Typesense model. The releases are here so that dbrun can
// start a server for the tests of the Typesense driver in github.com/xo/dbimp,
// which reads the HTTP interface. No dialect is named yet, because dbimp
// settles the name with the driver. See D118.
//
// # The range
//
// docker.io/typesense/typesense builds each release tag once, so the rule in
// D112 applies: the newest release of each of the last two lines. Checked on
// 2026-09-28, that is 30.2 and 29.1, both of 2026-04-16. 31.0 is a release
// candidate. Typesense is under the GPL 3.0 licence.
//
// # Keys, not users
//
// Typesense has no users. It checks a bootstrap key, which is [Password], and
// keys made with it. Init makes [TypesenseReadKey], which may only search the
// collection dbmeta. A request sends a key in the X-TYPESENSE-API-KEY header,
// and the DSN carries each key as its password.
//
// # The setup
//
// The server exits when its data directory is missing, so the command makes
// one. The image has bash and no curl, so every request goes through bash's
// /dev/tcp. Init makes the collection dbmeta and the key, and each answers 409
// when it is there.

// TypesenseUser is the name the read only key goes by. Typesense itself has
// no user names.
const TypesenseUser = "dbmeta_user"

// TypesenseReadKey is the key that may only search the collection dbmeta. It
// differs from [Password], because Typesense refuses a key it already has.
const TypesenseReadKey = Password + "-read"

// typesenseKey is the header that carries the administrator's key.
var typesenseKey = map[string]string{"X-TYPESENSE-API-KEY": Password}

// typesense is the Typesense image.
var typesense = product{
	name:     "typesense",
	image:    "docker.io/typesense/typesense",
	port:     8108,
	env:      map[string]string{"TYPESENSE_API_KEY": Password},
	runFlags: []string{"--entrypoint", "/bin/bash"},
	args:     []string{"-c", "mkdir -p /data && exec /opt/typesense-server --data-dir /data --enable-cors=false"},
	ready:    bashRequest(8108, "GET", "/collections", "", typesenseKey, 200),
	init: bashAll(
		bashRequest(8108, "POST", "/collections",
			`{"name":"dbmeta","fields":[{"name":".*","type":"auto"}]}`, typesenseKey, 201, 409),
		bashRequest(8108, "POST", "/keys",
			`{"value":"`+TypesenseReadKey+`","description":"`+TypesenseUser+`","actions":["documents:search"],"collections":["dbmeta"]}`,
			typesenseKey, 201, 409)),
	dsn:   keyHTTP("admin", Password),
	users: []Principal{{Role: User, User: TypesenseUser, dsn: keyHTTP(TypesenseUser, TypesenseReadKey)}},
}

// Typesense is every Typesense release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. Each keeps the cadence it would have if a model read it, which is
// what dbimp runs on each push and at night. See D119 and D120.
var Typesense = list{}.staged(typesense, Tested, "29.1", "30.2")
