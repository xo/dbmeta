package container

// The Apache Jena Fuseki releases dbrun starts.
//
// dbmeta has no Fuseki model. The releases are here so that dbrun can start
// a server for the tests of the SPARQL driver in github.com/xo/dbimp, which
// sends queries to /{dataset}/query. No dialect is named yet, because dbimp
// settles the name with the driver. See D118.
//
// # The range
//
// Apache publishes no image, and each release once, as a jar on Maven
// Central. So the rule in D112 applies: the newest release of each of the
// last two lines. Checked on 2026-09-28, that is 6.2.0, of 2026-07-27, and
// 6.1.0, of 2026-05. Jena is under the Apache 2.0 licence.
//
// # The image is built here
//
// The Containerfile in test/cmd/dbrun/image puts the jar on the Java 21
// runtime, which Jena 6 needs.
//
// # The users
//
// Fuseki reads its users from a password file with one name and password on
// each line, and the configuration says which user may use which endpoint.
// The command writes both. admin may query, update and write the graphs of
// the dataset dbmeta, which TDB2 keeps on disk. [FusekiUser] may only query.
// The configuration asks for basic authentication, because the default is
// digest. Fuseki reads both files on every start, so nothing else sets up.

// FusekiUser may only query the dataset dbmeta. Its password is [Password].
const FusekiUser = "dbmeta_user"

// fusekiServe writes the password file and the configuration and starts the
// server.
var fusekiServe = `set -e
mkdir -p /var/lib/fuseki
printf 'admin: %s\n` + FusekiUser + `: %s\n' '` + Password + `' '` + Password + `' > /tmp/passwd
cat > /tmp/config.ttl <<'TTL'
PREFIX fuseki: <http://jena.apache.org/fuseki#>
PREFIX rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#>
PREFIX tdb2: <http://jena.apache.org/2016/tdb#>

[] rdf:type fuseki:Server ;
	fuseki:passwd "/tmp/passwd" ;
	fuseki:auth "basic" .

<#service> rdf:type fuseki:Service ;
	fuseki:name "dbmeta" ;
	fuseki:endpoint [ fuseki:operation fuseki:query ; fuseki:name "query" ; fuseki:allowedUsers "admin", "` + FusekiUser + `" ] ;
	fuseki:endpoint [ fuseki:operation fuseki:update ; fuseki:name "update" ; fuseki:allowedUsers "admin" ] ;
	fuseki:endpoint [ fuseki:operation fuseki:gsp-rw ; fuseki:name "data" ; fuseki:allowedUsers "admin" ] ;
	fuseki:dataset <#dataset> .

<#dataset> rdf:type tdb2:DatasetTDB2 ;
	tdb2:location "/var/lib/fuseki/dbmeta" .
TTL
exec java -Xmx1g -jar /opt/fuseki/fuseki-server.jar --conf /tmp/config.ttl --port 3030`

// fuseki is the Apache Jena Fuseki image, built here.
var fuseki = product{
	name:  "fuseki",
	image: "localhost/dbmeta/fuseki",
	port:  3030,
	args:  []string{"bash", "-c", fusekiServe},
	ready: bashRequest(3030, "GET", "/dbmeta/query?query=ASK%7B%7D", "",
		map[string]string{"Authorization": adminBasic}, 200),
	dsn:   keyHTTP("admin", Password),
	users: []Principal{{Role: User, User: FusekiUser, dsn: keyHTTP(FusekiUser, Password)}},
}

// Fuseki is every Apache Jena Fuseki release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. See D119.
var Fuseki = list{}.add(fuseki, Staged, "6.1.0", "6.2.0")
