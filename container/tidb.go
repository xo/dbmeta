package container

import (
	"fmt"
	"net/url"

	"github.com/xo/dbmeta"
)

// The TiDB releases dbrun starts.
//
// TiDB speaks the MySQL wire protocol, and usql reaches it with the scheme
// tidb, which dburl opens with the mysql driver. models/tidb reads it, and
// shares the mysql model's statements where they answer (D123, D133).
//
// # The range
//
// docker.io/pingcap/tidb rebuilds a release tag while its line is
// maintained, so the floor is the oldest line with long term support that is
// still maintained. Checked on 2026-09-28, that is v7.5.8, rebuilt on
// 2026-09-17 and maintained to 2026-12-01, and the ceiling is v8.5.8, of
// 2026-08-27. v8.1.2, of 2026-07-24, is kept between them. 9.0 is a beta, and the
// latest tag is from 2024 and is never used. TiDB is under the Apache 2.0
// licence.
//
// # The setup
//
// The image holds tidb-server and no MySQL client, and the server reads no
// variable for the password of root. So the command writes the setup to a
// file and starts the server with --initialize-sql-file, which runs it once,
// when the server makes its store. It gives root [Password], and makes the
// database dbmeta, [TiDBOwner], who may do anything in it, and [TiDBUser], who
// may only read it. The check asks the status port, because nothing in the
// image can log in.

// TiDBOwner may do anything in the database dbmeta. Its password is
// [Password].
const TiDBOwner = "dbmeta_owner"

// TiDBUser may only read the database dbmeta. Its password is [Password].
const TiDBUser = "dbmeta_user"

// tidbServe writes the setup and starts the server on it.
var tidbServe = `cat > /tmp/init.sql <<'SQL'
ALTER USER 'root'@'%' IDENTIFIED BY '` + Password + `';
CREATE DATABASE IF NOT EXISTS dbmeta;
CREATE USER IF NOT EXISTS '` + TiDBOwner + `'@'%' IDENTIFIED BY '` + Password + `';
CREATE USER IF NOT EXISTS '` + TiDBUser + `'@'%' IDENTIFIED BY '` + Password + `';
GRANT ALL PRIVILEGES ON dbmeta.* TO '` + TiDBOwner + `'@'%';
GRANT SELECT ON dbmeta.* TO '` + TiDBUser + `'@'%';
SQL
exec /tidb-server --store=unistore --path=/var/lib/tidb --initialize-sql-file=/tmp/init.sql`

// tidb is the TiDB image.
var tidb = product{
	dialect:   dbmeta.TiDB,
	name:      "tidb",
	image:     "docker.io/pingcap/tidb",
	tagPrefix: "v",
	port:      4000,
	runFlags:  []string{"--entrypoint", "/bin/sh"},
	args:      []string{"-c", tidbServe},
	ready:     []string{"sh", "-c", "curl -sf -o /dev/null http://127.0.0.1:10080/status"},
	dsn:       tidbDSN("root"),
	url:       tidbURL("root"),
	users: []Principal{
		{Role: User, User: TiDBOwner, dsn: tidbDSN(TiDBOwner), url: tidbURL(TiDBOwner)},
		{Role: User, User: TiDBUser, dsn: tidbDSN(TiDBUser), url: tidbURL(TiDBUser)},
	},
}

// tidbDSN is the go-sql-driver connection string of one user.
func tidbDSN(user string) func(port int) string {
	return func(port int) string {
		return fmt.Sprintf("%s:%s@tcp(127.0.0.1:%d)/dbmeta?parseTime=true", user, Password, port)
	}
}

// tidbURL is the dburl style URL of one user.
func tidbURL(user string) func(port int) string {
	return func(port int) string {
		return fmt.Sprintf("tidb://%s:%s@127.0.0.1:%d/dbmeta", user, url.QueryEscape(Password), port)
	}
}

// TiDB is every TiDB release dbmeta is tested against. Each takes the cadence
// it kept while it was Staged as its tier (D120).
var TiDB = list{}.add(tidb, Tested, "7.5.8", "8.5.8").
	add(tidb, Nightly, "8.1.2")
