package container

import (
	"github.com/xo/dbmeta"
)

// MariaDB is the reference product of the mysql dialect. MySQL is the
// flavor and it is in mysql.go. A fragment gates on the product key and
// never on the number, which is D44.

// mariadb is the official image. MariaDB is the reference product of this
// dialect, so it is the one the queries are written against.
var mariadb = product{
	dialect: dbmeta.MySQL,
	name:    "mariadb",
	image:   "docker.io/library/mariadb",
	port:    3306,
	env:     map[string]string{"MARIADB_ROOT_PASSWORD": Password},
	ready:   []string{"healthcheck.sh", "--connect", "--innodb_initialized"},
	dsn:     mysqlDSN,
	url:     mysqlURL,
}

// MariaDB is the MariaDB releases dbmeta is tested against.
//
// The floor is 10.6, the oldest long term release still maintained. The
// ceiling is 13.0, the current stable. 11.8 is the long term release most
// installations run, and it sits above the 11.5 gate where the view that
// lists sequences arrived.
var MariaDB = list{}.add(mariadb, Tested, "10.6", "13.0").
	add(mariadb, Nightly, "10.11", "11.4", "11.8", "12.3")
