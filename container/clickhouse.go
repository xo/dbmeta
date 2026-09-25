package container

import (
	"fmt"

	"github.com/xo/dbmeta"
)

// The ClickHouse releases dbmeta is tested against.
//
// # The floor, by the docs/EVALUATION.md procedure
//
// Step 2 decides it: the oldest release whose image is still rebuilt. On
// clickhouse/clickhouse-server, 23.8 was last rebuilt in August 2024 and
// 24.3 and 24.8 in February 2025, so all three are dead. 25.3 was rebuilt in
// February 2026, 25.8 in August, and 26.8 and 26.9 in September. The floor is
// 25.3.
//
// ClickHouse releases monthly and marks some releases long term, so a floor
// here goes stale faster than anywhere else in this package. That is what the
// tiers are for, and D21's removal trigger applies the same way.
//
// # Why this pair runs on every push
//
// 25.8 and 26.9 are the ends, and they span the one fragment the model has.
// system.constraints does not exist on 25.3, 25.8 or 26.1, and does exist on
// 26.8 and 26.9, so a pair either side of it exercises both.
var clickhouse = product{
	dialect: dbmeta.ClickHouse,
	name:    "clickhouse",
	image:   "docker.io/clickhouse/clickhouse-server",
	// The native protocol, which is what clickhouse-go speaks. 8123 is the
	// HTTP interface and nothing here uses it.
	port: 9000,
	env: map[string]string{
		"CLICKHOUSE_PASSWORD": Password,
		// Without this the default user cannot CREATE ROLE, CREATE USER or
		// GRANT, so the fixture cannot build the objects the roles, role
		// grants and privileges queries read.
		"CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT": "1",
	},
	// The image warns about the open file limit and starts anyway, which was
	// checked rather than assumed, so there is no ulimit to set here.
	//
	// Named collections stay out of reach. They are gated by
	// access_control_improvements.named_collection_control in the server
	// configuration, which the image exposes no variable for, so the fixture
	// builds none and the foreign servers query returns no rows. Enabling it
	// would mean building an image, which is not worth it for one query that
	// is already verified to run.
	ready: []string{"clickhouse-client", "--password", Password, "-q", "SELECT 1"},
	dsn: func(port int) string {
		return fmt.Sprintf("clickhouse://default:%s@127.0.0.1:%d/default", Password, port)
	},
}

// ClickHouse is every ClickHouse release dbmeta is tested against.
var ClickHouse = list{}.add(clickhouse, Tested, "25.8", "26.9").
	add(clickhouse, Nightly, "25.3", "26.8")
