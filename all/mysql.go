//go:build (!no_base || mysql) && !no_mysql

package all

import _ "github.com/xo/dbmeta/models/mysql"
