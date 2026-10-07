package container

// The DynamoDB Local releases dbrun starts.
//
// Amazon DynamoDB is a hosted service, and the package hosted names it. AWS
// publishes DynamoDB Local, which runs on one machine, and this entry runs
// it, so that a person and CI can test without an account. usql reaches it
// with github.com/btnguyen2k/godynamo, whose Endpoint option points it here.
// dbmeta has no DynamoDB model. See D117 and D118.
//
// # The range
//
// docker.io/amazon/dynamodb-local builds each release tag once, so the rule in
// D112 applies: the newest release of each of the last two lines. Checked on
// 2026-09-28, that is 3.3.1, of 2026-07-31, and 3.2.0, of 2026-01-12.
//
// # The license
//
// DynamoDB Local is under the Amazon DynamoDB Local License Agreement, which
// is proprietary and binds whoever uses it. It needs no account. Ken accepted
// it for this project on 2026-09-28.
//
// # No users
//
// DynamoDB Local checks no key, so there is no user with fewer rights. The key
// dbmeta and [Password] are sent and not checked, and a key can hold only
// letters and digits. -sharedDb gives every key and region one store, and
// -inMemory keeps the store in memory, so it is empty after a restart.

// dynamodb is the DynamoDB Local image.
var dynamodb = product{
	name:  "dynamodb",
	image: "docker.io/amazon/dynamodb-local",
	port:  8000,
	args:  []string{"-jar", "DynamoDBLocal.jar", "-inMemory", "-sharedDb", "-disableTelemetry"},
	ready: []string{"sh", "-c", `curl -sf -o /dev/null --aws-sigv4 'aws:amz:us-east-1:dynamodb' -u 'dbmeta:` + Password +
		`' -H 'Content-Type: application/x-amz-json-1.0' -H 'X-Amz-Target: DynamoDB_20120810.ListTables' -d '{}' http://127.0.0.1:8000/`},
	dsn: dynamoURL("dbmeta", Password),
	api: bareHTTP,
}

// DynamoDB is every DynamoDB Local release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. Each keeps the cadence it will have if a model reads it, which is
// what dbimp runs on each push and at night. See D119 and D120.
var DynamoDB = list{}.staged(dynamodb, Tested, "3.2.0", "3.3.1")
