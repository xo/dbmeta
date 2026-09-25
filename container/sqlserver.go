package container

import (
	"fmt"
	"net/url"

	"github.com/xo/dbmeta"
)

// SQL Server on Linux, which begins at 2017. Every older release needs a
// Windows machine and those are in windows.go. See D54 and D57.

var sqlserver = product{
	dialect: dbmeta.SQLServer,
	name:    "sqlserver",
	// Microsoft's own registry rather than Docker Hub, and the image is
	// the only one there is: there is no community SQL Server.
	image: "mcr.microsoft.com/mssql/server",
	// Microsoft tags every release "-latest" and publishes no bare tag.
	tagSuffix: "-latest",
	port:      1433,
	// The password has to satisfy the SQL Server policy, which wants a
	// symbol, so it is not the shared one.
	env: map[string]string{
		"ACCEPT_EULA":       "Y",
		"MSSQL_SA_PASSWORD": SQLServerPassword,
		"MSSQL_PID":         "Developer",
	},
	ready: sqlcmd("/opt/mssql-tools18/bin/sqlcmd"),
	dsn: func(port int) string {
		return fmt.Sprintf(
			"sqlserver://sa:%s@127.0.0.1:%d?database=master&encrypt=disable",
			url.QueryEscape(SQLServerPassword), port)
	},
}

// SQLServer is the Microsoft SQL Server releases dbmeta is tested against.
//
// Every major release that ships a Linux container, and all of them at the
// Tested tier. That is four jobs rather than two, and it buys the whole claim:
// dbmeta is tested on every SQL Server a person can run on Linux.
//
// 2017 is the floor and it is a hard one. Microsoft shipped SQL Server on
// Linux from 2017, so 2016 and earlier have no container and cannot be tested
// at all. D54 says what is claimed for them, which is less than support.
//
// Splitting these across tiers would have saved little. Every version gate the
// model has sits below 2017, so the releases here differ by what they added
// rather than by what they lack, and the newest is the one most likely to
// break. See D54.
var SQLServer = list{}.add(sqlserver, Tested, "2017", "2019", "2022", "2025").
	// 2017 is the one image built on Ubuntu 16.04. It ships the older sqlcmd,
	// at /opt/mssql-tools rather than /opt/mssql-tools18.
	on("2017", func(s *Server) { s.Ready = sqlcmd("/opt/mssql-tools/bin/sqlcmd") })

// sqlcmd is the readiness command for a SQL Server image, at the path that
// image installs sqlcmd to. -C trusts the server certificate, which both the
// old client and the new one accept.
func sqlcmd(path string) []string {
	return []string{
		path, "-S", "localhost",
		"-U", "sa", "-P", SQLServerPassword, "-C", "-Q", "SELECT 1",
	}
}
