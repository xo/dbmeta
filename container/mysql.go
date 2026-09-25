package container

import (
	"fmt"
	"net/url"

	"github.com/xo/dbmeta"
)

// MySQL is the flavor of the mysql dialect. MariaDB is the reference
// product and it is in mariadb.go. See D44.

// mysql is the official image. MySQL is the flavor, and it exists here to
// catch a query that reads a MariaDB table MySQL dropped.
var mysql = product{
	dialect: dbmeta.MySQL,
	name:    "mysql",
	image:   "docker.io/library/mysql",
	port:    3306,
	env:     map[string]string{"MYSQL_ROOT_PASSWORD": Password},
	ready:   []string{"mysqladmin", "ping", "-h", "127.0.0.1", "-uroot", "-p" + Password},
	dsn:     mysqlDSN,
	url:     mysqlURL,
}

// MySQL is the MySQL releases dbmeta is tested against.
//
// 8.4 is the long term release. 26.7 is the current innovation release, which
// is where MySQL's new year based numbering starts: it follows 9.7, and it is
// a larger number than any MariaDB release will reach for years. That is the
// reason a fragment gates on the product key and never on the number. See D44.
//
// 8.4 and 26.7 sit on either side of the only gate this model has for MySQL,
// which is where a system variable moved in release 9.
var MySQL = list{}.add(mysql, Tested, "8.4", "26.7").
	add(mysql, Nightly, "9.7")

// mysqlURL is the dburl style URL for the same server. The go-sql-driver DSN
// mysqlDSN returns is not a URL, so a person cannot paste it into usql.
func mysqlURL(port int) string {
	return fmt.Sprintf("mysql://root:%s@127.0.0.1:%d/", url.QueryEscape(Password), port)
}

func mysqlDSN(port int) string {
	return fmt.Sprintf("root:%s@tcp(127.0.0.1:%d)/?parseTime=true", Password, port)
}
