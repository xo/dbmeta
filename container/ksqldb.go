package container

import "time"

// The ksqlDB releases dbrun starts.
//
// dbmeta has no ksqlDB model. The releases are here so that dbrun can start a
// server for the tests of the ksqlDB driver in github.com/xo/dbimp, which
// sends statements to /ksql and /query-stream. No dialect is named yet,
// because dbimp settles the name with the driver. See D118.
//
// # The range
//
// confluentinc/ksqldb-server stopped at 0.29.0 in 2023. The maintained image
// is Confluent Platform's docker.io/confluentinc/cp-ksqldb-server, which
// builds each release tag once, so the rule in D112 applies: the newest
// release of each of the last two lines. Checked on 2026-09-28, that is 8.3.2
// and 8.2.4, both of 2026-09-16. ksqlDB is under the Confluent Community
// License, which is source available and free for this use, and needs no
// key.
//
// # The image is built here
//
// The Containerfile in test/cmd/dbrun/image copies the Kafka broker in. The
// command starts one broker in KRaft mode, and then ksqlDB on it.
//
// # One user
//
// The command turns on basic authentication with a password file that holds
// admin with [Password]. Open source ksqlDB gives every user it lets in the
// same rights, and a user with fewer rights needs Confluent's commercial
// authorization, so there is no ordinary user. ksqlDB has no databases, so
// nothing is named dbmeta.

// ksqldbServe starts the broker, waits for its port, writes the password
// file, and starts ksqlDB. The broker's log goes to the container's, marked
// kafka.
var ksqldbServe = `set -e
cat > /tmp/server.properties <<'PROPS'
process.roles=broker,controller
node.id=1
controller.quorum.voters=1@127.0.0.1:9093
listeners=PLAINTEXT://127.0.0.1:9092,CONTROLLER://127.0.0.1:9093
advertised.listeners=PLAINTEXT://127.0.0.1:9092
controller.listener.names=CONTROLLER
listener.security.protocol.map=PLAINTEXT:PLAINTEXT,CONTROLLER:PLAINTEXT
log.dirs=/tmp/kafka-logs
offsets.topic.replication.factor=1
transaction.state.log.replication.factor=1
transaction.state.log.min.isr=1
group.initial.rebalance.delay.ms=0
PROPS
kafka-storage format -t MkU3OEVBNTcwNTJENDM2Qk -c /tmp/server.properties --ignore-formatted >/dev/null
LOG_DIR=/tmp/kafka-log KAFKA_HEAP_OPTS="-Xms512m -Xmx512m" kafka-server-start /tmp/server.properties 2>&1 | sed 's/^/kafka: /' &
until (exec 3<>/dev/tcp/127.0.0.1/9092) 2>/dev/null; do sleep 1; done
printf 'admin: %s,admin\n' '` + Password + `' > /tmp/passwd
cat > /tmp/jaas.conf <<'JAAS'
KsqlServer-Props {
	org.eclipse.jetty.security.jaas.spi.PropertyFileLoginModule required
	file="/tmp/passwd"
	debug="false";
};
JAAS
export KSQL_OPTS="-Djava.security.auth.login.config=/tmp/jaas.conf"
exec /etc/confluent/docker/entrypoint.sh ksqldb-server`

// ksqldb is ksqlDB with its Kafka broker, built here.
var ksqldb = product{
	name:  "ksqldb",
	image: "localhost/dbmeta/ksqldb",
	port:  8088,
	env: map[string]string{
		"KSQL_BOOTSTRAP_SERVERS":     "127.0.0.1:9092",
		"KSQL_LISTENERS":             "http://0.0.0.0:8088",
		"KSQL_AUTHENTICATION_METHOD": "BASIC",
		"KSQL_AUTHENTICATION_REALM":  "KsqlServer-Props",
		"KSQL_AUTHENTICATION_ROLES":  "admin",
		"KSQL_HEAP_OPTS":             "-Xms1g -Xmx1g",
		"KSQL_KSQL_SERVICE_ID":       "dbmeta_",
	},
	runFlags: []string{"--entrypoint", "/bin/bash"},
	args:     []string{"-c", ksqldbServe},
	// The image has bash and no curl, so the check goes through bash's
	// /dev/tcp. A statement proves that ksqlDB reaches the broker.
	ready: bashRequest(8088, "POST", "/ksql", `{"ksql":"SHOW STREAMS;"}`,
		map[string]string{"Authorization": adminBasic}, 200),
	startup: 3 * time.Minute,
	dsn:     keyHTTP("admin", Password),
}

// KsqlDB is every ksqlDB release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. See D119.
var KsqlDB = list{}.add(ksqldb, Staged, "8.2.4", "8.3.2")
