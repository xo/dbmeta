package container

import "fmt"

// The Vitess releases dbrun starts.
//
// Vitess speaks the MySQL wire protocol, and usql reaches it with the scheme
// vitess. The entry names no dialect yet. models/mysql sets the version key
// of MySQL for any VERSION() that does not say MariaDB, and Vitess answers
// 8.4.6-Vitess, so the model would read it as MySQL 8.4.6. The entry takes the
// mysql dialect when the model detects Vitess and sets its own version key,
// which is D44. See D118.
//
// # The range
//
// docker.io/vitess/vttestserver builds each release tag once, so the rule in
// D112 applies: the newest release of each of the last two lines. Checked on
// 2026-09-28, that is v24.0.3 and v23.0.6, both of 2026-09-03. The tag names
// the MySQL it runs, and the entry takes -mysql84. Vitess is under the Apache
// 2.0 licence.
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

// Vitess is every Vitess release dbrun starts.
//
// Staged until models/mysql detects Vitess and gives it a version key of
// its own, so CI runs none of them. See D118 and D119.
var Vitess = list{}.add(vitess, Staged, "23.0.6", "24.0.3")
