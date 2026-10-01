package container

import (
	"fmt"
	"net/url"
)

// The Azure Cosmos DB emulator releases dbrun starts.
//
// Azure Cosmos DB is a hosted service, and the package hosted names it.
// Microsoft publishes an emulator of it, and this entry runs the emulator, so
// that a person and CI can test without an account. usql reaches Cosmos DB
// with github.com/btnguyen2k/gocosmos. dbmeta has no Cosmos DB model. See D117
// and D118.
//
// # The range
//
// mcr.microsoft.com/cosmosdb/linux/azure-cosmos-emulator publishes the vNext
// emulator as a dated build each month, and builds each tag once, so the rule
// in D112 gives the newest. Checked on 2026-09-28, that is vnext-EN20260907.
// The older emulator, which needs a Windows host or much more memory, is left
// out.
//
// # The license
//
// The emulator is under a Microsoft license, and is free. Ken accepted it for
// this project on 2026-09-28.
//
// # What it answers
//
// Microsoft's list of features marks reading the feed of databases and the
// feed of collections as not yet done, and gocosmos needs both to list them,
// so the emulator answers less than the service does.
//
// # The key
//
// The emulator has one account with a well known key, [CosmosKey], and no
// other principal. dburl takes the key as the user name of the URL, and always
// uses https, so the emulator serves https with a certificate it makes, and
// InsecureSkipVerify trusts it. The key is percent encoded in the URL.

// CosmosKey is the key of the emulator's one account. Microsoft publishes it,
// and every copy of the emulator has it.
const CosmosKey = "C2y6yDjf5/R+ob0N8A7Cgv30VRDJIWEHLM+4QDU5DE2nQ9nDuVTqobD4b8mGGyPMbIZnqyMsEcaGQy67XIw/Jw=="

// cosmos is the Cosmos DB emulator image.
var cosmos = product{
	name:      "cosmos",
	image:     "mcr.microsoft.com/cosmosdb/linux/azure-cosmos-emulator",
	tagPrefix: "vnext-",
	port:      8081,
	env: map[string]string{
		"PROTOCOL":        "https",
		"ENABLE_EXPLORER": "false",
		// The usage report that goes to the vendor.
		"ENABLE_TELEMETRY": "false",
	},
	ready: []string{"sh", "-c", "curl -sf -o /dev/null http://127.0.0.1:8080/ready"},
	dsn: func(port int) string {
		return fmt.Sprintf("AccountEndpoint=https://127.0.0.1:%d/;AccountKey=%s;InsecureSkipVerify=true", port, CosmosKey)
	},
	url: func(port int) string {
		return fmt.Sprintf("cosmos://%s@127.0.0.1:%d/?InsecureSkipVerify=true", url.QueryEscape(CosmosKey), port)
	},
}

// Cosmos is every Cosmos DB emulator release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. Each keeps the cadence it will have if a model reads it, which is
// what dbimp runs on each push and at night. See D119 and D120.
var Cosmos = list{}.staged(cosmos, Tested, "EN20260907")
