package container

import (
	"fmt"
	"net/url"

	"github.com/xo/dbmeta"
)

// The GizmoSQL releases dbrun starts.
//
// GizmoSQL is a server for Arrow Flight SQL, which usql reaches with the
// driver in github.com/apache/arrow-go, through dburl's gizmosql scheme from
// v0.36.0. The DSN is the driver's flightsql form, and the URL is the
// gizmosql one. The dialect is gizmosql, which is dburl's. It holds a
// DuckDB database, which is the default backend. The model reads that engine
// and shares the duckdb model's statements (D187). Flight SQL is a protocol and
// GizmoSQL is the server of it that is maintained: voltrondata/flight-sql
// stopped in 2024, and voltrondata/sqlflite after it. See D118.
//
// # The range
//
// docker.io/gizmodata/gizmosql builds each release tag once, so the rule in
// D112 applies: the newest release of each of the last two lines. Checked on
// 2026-10-07, that is v1.41.0 and v1.40.0. It
// releases several times a month, so the ceiling moves often. The tag that
// ends in -slim does not turn TLS on, and the other tags make a certificate
// of their own. The core is under the Apache 2.0 license, and the enterprise
// features need a license key that nothing here needs.
//
// # The session
//
// The flightsql driver never makes the handshake that opens a session, so a
// connection with the DSN below fails on every statement. The tests make the
// handshake and put the token in the DSN. See D187.
//
// # One user
//
// The core has one user, admin with [Password], and no other principal. The
// database file is /tmp/dbmeta.duckdb, so its catalog is named dbmeta.

// gizmosql is the GizmoSQL image.
var gizmosql = product{
	dialect:   dbmeta.GizmoSQL,
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
	url: func(port int) string {
		u := url.URL{
			Scheme: "gizmosql",
			User:   url.UserPassword("admin", Password),
			Host:   fmt.Sprintf("127.0.0.1:%d", port),
		}
		return u.String()
	},
}

// GizmoSQL is every GizmoSQL release dbrun starts. They are Tested, the
// cadence they recorded while they were Staged (D120).
var GizmoSQL = list{}.add(gizmosql, Tested, "1.40.0", "1.41.0")
