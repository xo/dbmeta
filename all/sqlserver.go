//go:build (!no_base || sqlserver) && !no_sqlserver

package all

import _ "github.com/xo/dbmeta/models/sqlserver"
