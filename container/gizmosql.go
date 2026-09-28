package container

import (
	"fmt"
	"net/url"
)

// The GizmoSQL releases dbrun starts.
//
// GizmoSQL is a server for Arrow Flight SQL, which usql reaches with the
// scheme flightsql and the driver in github.com/apache/arrow-go. It holds a
// DuckDB database. dbmeta has no Flight SQL model. The entry is the server
// for that driver, because Flight SQL is a protocol and GizmoSQL is the server
// of it that is maintained: voltrondata/flight-sql stopped in 2024, and
// voltrondata/sqlflite after it. See D118.
//
// # The range
//
// docker.io/gizmodata/gizmosql builds each release tag once, so the rule in
// D112 applies: the newest release of each of the last two lines. Checked on
// 2026-09-28, that is v1.39.0, of 2026-09-14, and v1.38.5, of 2026-09-11. It
// releases several times a month, so the ceiling moves often. The tag that
// ends in -slim does not turn TLS on, and the other tags make a certificate
// of their own. The core is under the Apache 2.0 licence, and the enterprise
// features need a licence key that nothing here needs.
//
// # One user
//
// The core has one user, admin with [Password], and no other principal. The
// database file is /tmp/dbmeta.duckdb, so its catalog is named dbmeta.

// gizmosql is the GizmoSQL image.
var gizmosql = product{
	name:      "gizmosql",
	image:     "docker.io/gizmodata/gizmosql",
	tagPrefix: "v",
	tagSuffix: "-slim",
	port:      31337,
	env: map[string]string{
		"GIZMOSQL_USERNAME": "admin",
		"GIZMOSQL_PASSWORD": Password,
		"DATABASE_FILENAME": "/tmp/dbmeta.duckdb",
		"PRINT_QUERIES":     "0",
	},
	ready: []string{"sh", "-c", "GIZMOSQL_PASSWORD='" + Password +
		"' gizmosql_client --host 127.0.0.1 --port 31337 --username admin --command 'SELECT 1'"},
	dsn: func(port int) string {
		return fmt.Sprintf("flightsql://admin:%s@127.0.0.1:%d", url.QueryEscape(Password), port)
	},
}

// GizmoSQL is every GizmoSQL release dbrun starts.
//
// Staged, because dbmeta has no model that reads it, so CI runs none of
// them. See D119.
var GizmoSQL = list{}.add(gizmosql, Staged, "1.38.5", "1.39.0")
