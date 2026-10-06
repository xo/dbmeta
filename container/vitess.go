package container

import (
	"fmt"

	"github.com/xo/dbmeta"
)

// The Vitess releases dbmeta is tested against.
//
// Vitess speaks the MySQL wire protocol, and usql reaches it with the scheme
// vitess. The dialect is vitess, which dburl gives it from v0.36.0 (dburl
// D37). Vitess answers VERSION() with 8.4.6-Vitess, and models/mysql reads
// that as MySQL, so Vitess has a model of its own, models/vitess, that
// shares the mysql model's statements where they answer (D123, D125, D135).
//
// # The range
//
// docker.io/vitess/vttestserver builds each release tag once, so the rule in
// D112 applies: the newest release of each of the last two lines. Checked on
// 2026-10-07, that is v24.0.4 and v23.0.7. The tag names
// the MySQL it runs, and the entry takes -mysql84. Vitess is under the Apache
// 2.0 license.
//
// # No users
//
// vttestserver runs vtcombo and mysqld in one container, and starts vtcombo
// with no authentication, so any user and any password is accepted. It has no
// setting that changes that. A user that has fewer rights needs vtcombo's
// static authentication and table rules, which vttestserver does not run. The
// keyspace dbmeta is the database, and KEYSPACES makes it.
//
// # The port
//
// vttestserver serves MySQL on PORT plus 3, so PORT is 33574 and the port is
// 33577. It keeps no data across a restart, and makes the keyspace again.

// vitess is the vttestserver image.
var vitess = product{
	dialect:   dbmeta.Vitess,
	name:      "vitess",
	image:     "docker.io/vitess/vttestserver",
	tagPrefix: "v",
	tagSuffix: "-mysql84",
	port:      33577,
	env: map[string]string{
		"PORT":              "33574",
		"KEYSPACES":         "dbmeta",
		"NUM_SHARDS":        "1",
		"MYSQL_BIND_HOST":   "0.0.0.0",
		"VTCOMBO_BIND_HOST": "0.0.0.0",
	},
	ready: []string{"sh", "-c", "mysql -h127.0.0.1 -P33577 -uroot dbmeta -e 'SELECT 1'"},
	dsn: func(port int) string {
		return fmt.Sprintf("root@tcp(127.0.0.1:%d)/dbmeta?parseTime=true", port)
	},
	url: func(port int) string {
		return fmt.Sprintf("vitess://root@127.0.0.1:%d/dbmeta", port)
	},
}

// Vitess is every Vitess release dbmeta is tested against. Both are Tested,
// which is the cadence they kept while they were Staged (D120).
var Vitess = list{}.add(vitess, Tested, "23.0.7", "24.0.4")
