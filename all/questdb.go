//go:build (!no_base || questdb) && !no_questdb

package all

import _ "github.com/xo/dbmeta/models/questdb"
